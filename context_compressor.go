package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ContextCompressor 上下文压缩器
type ContextCompressor struct {
	mu                sync.RWMutex
	compressionCache  map[string]*CompressedContext
	summaryCache      map[string]string
	maxContextLength  int
	compressionRatio  float64
}

// CompressedContext 压缩的上下文
type CompressedContext struct {
	OriginalMessages []Message `json:"original_messages"`
	CompressedMessages []Message `json:"compressed_messages"`
	Summary          string    `json:"summary"`
	CompressionRatio float64   `json:"compression_ratio"`
	CreatedAt        time.Time `json:"created_at"`
	UsageCount       int64     `json:"usage_count"`
}

// MessageImportance 消息重要性评分
type MessageImportance struct {
	Index      int     `json:"index"`
	Score      float64 `json:"score"`
	Reason     string  `json:"reason"`
	IsSystem   bool    `json:"is_system"`
	IsRecent   bool    `json:"is_recent"`
	HasKeywords bool   `json:"has_keywords"`
}

var contextCompressor = &ContextCompressor{
	compressionCache: make(map[string]*CompressedContext),
	summaryCache:     make(map[string]string),
	maxContextLength: 4000,  // 最大上下文长度
	compressionRatio: 0.6,   // 目标压缩比例
}

// CompressRequest 压缩请求上下文
func (cc *ContextCompressor) CompressRequest(req AnthropicRequest) AnthropicRequest {
	if len(req.Messages) <= 2 {
		return req // 消息太少，不需要压缩
	}

	totalLength := cc.calculateTotalLength(req.Messages)
	if totalLength <= cc.maxContextLength {
		return req // 长度在限制内，不需要压缩
	}

	// 生成缓存键
	cacheKey := cc.generateCompressionKey(req.Messages)
	
	cc.mu.RLock()
	if compressed, exists := cc.compressionCache[cacheKey]; exists {
		cc.mu.RUnlock()
		compressed.UsageCount++
		
		// 创建新请求
		newReq := req
		newReq.Messages = compressed.CompressedMessages
		return newReq
	}
	cc.mu.RUnlock()

	// 执行压缩
	compressedMessages := cc.performCompression(req.Messages)
	
	// 缓存结果
	cc.mu.Lock()
	cc.compressionCache[cacheKey] = &CompressedContext{
		OriginalMessages:   req.Messages,
		CompressedMessages: compressedMessages,
		CompressionRatio:   float64(cc.calculateTotalLength(compressedMessages)) / float64(totalLength),
		CreatedAt:          time.Now(),
		UsageCount:         1,
	}
	cc.mu.Unlock()

	// 创建新请求
	newReq := req
	newReq.Messages = compressedMessages
	return newReq
}

// performCompression 执行压缩
func (cc *ContextCompressor) performCompression(messages []Message) []Message {
	if len(messages) <= 2 {
		return messages
	}

	// 计算消息重要性
	importance := cc.calculateMessageImportance(messages)
	
	// 按重要性排序
	sort.Slice(importance, func(i, j int) bool {
		return importance[i].Score > importance[j].Score
	})

	// 选择最重要的消息
	targetLength := int(float64(cc.calculateTotalLength(messages)) * cc.compressionRatio)
	selectedMessages := cc.selectImportantMessages(messages, importance, targetLength)

	// 确保保留系统消息和最后几条消息
	finalMessages := cc.ensureEssentialMessages(messages, selectedMessages)

	return finalMessages
}

// calculateMessageImportance 计算消息重要性
func (cc *ContextCompressor) calculateMessageImportance(messages []Message) []MessageImportance {
	importance := make([]MessageImportance, len(messages))
	
	for i, msg := range messages {
		score := 0.0
		reasons := []string{}

		// 系统消息最重要
		if msg.Role == "system" {
			score += 10.0
			reasons = append(reasons, "system_message")
		}

		// 最近的消息更重要
		recentThreshold := len(messages) - 3
		if i >= recentThreshold {
			score += 5.0 * float64(i-recentThreshold+1)
			reasons = append(reasons, "recent_message")
		}

		// 包含关键词的消息更重要
		content := getMessageContent(msg.Content)
		if cc.hasImportantKeywords(content) {
			score += 3.0
			reasons = append(reasons, "has_keywords")
		}

		// 长消息可能更重要
		if len(content) > 200 {
			score += 2.0
			reasons = append(reasons, "long_message")
		}

		// 问题消息更重要
		if strings.Contains(content, "?") || strings.Contains(content, "？") {
			score += 1.5
			reasons = append(reasons, "question")
		}

		// 用户消息比助手消息稍微重要
		if msg.Role == "user" {
			score += 1.0
			reasons = append(reasons, "user_message")
		}

		importance[i] = MessageImportance{
			Index:       i,
			Score:       score,
			Reason:      strings.Join(reasons, ","),
			IsSystem:    msg.Role == "system",
			IsRecent:    i >= recentThreshold,
			HasKeywords: cc.hasImportantKeywords(content),
		}
	}

	return importance
}

// hasImportantKeywords 检查是否包含重要关键词
func (cc *ContextCompressor) hasImportantKeywords(content string) bool {
	keywords := []string{
		"error", "错误", "问题", "help", "帮助",
		"how", "what", "why", "when", "where",
		"如何", "什么", "为什么", "怎么", "哪里",
		"code", "代码", "function", "函数",
		"bug", "fix", "修复", "解决",
	}

	contentLower := strings.ToLower(content)
	for _, keyword := range keywords {
		if strings.Contains(contentLower, keyword) {
			return true
		}
	}
	return false
}

// selectImportantMessages 选择重要消息
func (cc *ContextCompressor) selectImportantMessages(messages []Message, importance []MessageImportance, targetLength int) []int {
	selected := make(map[int]bool)
	currentLength := 0

	// 按重要性选择消息
	for _, imp := range importance {
		msgLength := len(getMessageContent(messages[imp.Index].Content))
		if currentLength+msgLength <= targetLength {
			selected[imp.Index] = true
			currentLength += msgLength
		}
	}

	// 转换为索引列表
	var selectedIndices []int
	for i := range selected {
		selectedIndices = append(selectedIndices, i)
	}
	sort.Ints(selectedIndices)

	return selectedIndices
}

// ensureEssentialMessages 确保保留必要消息
func (cc *ContextCompressor) ensureEssentialMessages(messages []Message, selectedIndices []int) []Message {
	selected := make(map[int]bool)
	for _, idx := range selectedIndices {
		selected[idx] = true
	}

	// 确保保留系统消息
	for i, msg := range messages {
		if msg.Role == "system" {
			selected[i] = true
		}
	}

	// 确保保留最后2条消息
	if len(messages) >= 2 {
		selected[len(messages)-1] = true
		selected[len(messages)-2] = true
	}

	// 构建最终消息列表
	var finalMessages []Message
	var lastIncluded = -1

	for i, msg := range messages {
		if selected[i] {
			// 如果跳过了消息，添加摘要
			if i > lastIncluded+1 {
				summary := cc.createSummary(messages[lastIncluded+1 : i])
				if summary != "" {
					finalMessages = append(finalMessages, Message{
						Role:    "system",
						Content: fmt.Sprintf("[摘要: %s]", summary),
					})
				}
			}
			finalMessages = append(finalMessages, msg)
			lastIncluded = i
		}
	}

	return finalMessages
}

// createSummary 创建消息摘要
func (cc *ContextCompressor) createSummary(messages []Message) string {
	if len(messages) == 0 {
		return ""
	}

	// 生成摘要键
	summaryKey := cc.generateSummaryKey(messages)
	
	cc.mu.RLock()
	if summary, exists := cc.summaryCache[summaryKey]; exists {
		cc.mu.RUnlock()
		return summary
	}
	cc.mu.RUnlock()

	// 创建简单摘要
	summary := cc.generateSimpleSummary(messages)
	
	// 缓存摘要
	cc.mu.Lock()
	cc.summaryCache[summaryKey] = summary
	cc.mu.Unlock()

	return summary
}

// generateSimpleSummary 生成简单摘要
func (cc *ContextCompressor) generateSimpleSummary(messages []Message) string {
	if len(messages) == 0 {
		return ""
	}

	topics := make(map[string]int)
	totalLength := 0

	for _, msg := range messages {
		content := getMessageContent(msg.Content)
		totalLength += len(content)
		
		// 提取关键词
		words := strings.Fields(strings.ToLower(content))
		for _, word := range words {
			if len(word) > 3 && cc.isImportantWord(word) {
				topics[word]++
			}
		}
	}

	// 找出最频繁的主题
	var topTopics []string
	for topic, count := range topics {
		if count >= 2 {
			topTopics = append(topTopics, topic)
		}
	}

	if len(topTopics) == 0 {
		return fmt.Sprintf("跳过了%d条消息", len(messages))
	}

	sort.Strings(topTopics)
	if len(topTopics) > 3 {
		topTopics = topTopics[:3]
	}

	return fmt.Sprintf("跳过了%d条关于%s的消息", len(messages), strings.Join(topTopics, "、"))
}

// isImportantWord 判断是否为重要词汇
func (cc *ContextCompressor) isImportantWord(word string) bool {
	// 过滤常见停用词
	stopWords := map[string]bool{
		"the": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true,
		"是": true, "的": true, "了": true, "在": true,
		"有": true, "我": true, "你": true, "他": true,
	}

	return !stopWords[word]
}

// calculateTotalLength 计算总长度
func (cc *ContextCompressor) calculateTotalLength(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += len(getMessageContent(msg.Content))
	}
	return total
}

// generateCompressionKey 生成压缩缓存键
func (cc *ContextCompressor) generateCompressionKey(messages []Message) string {
	data, _ := json.Marshal(messages)
	return fmt.Sprintf("compress_%x", md5.Sum(data))
}

// generateSummaryKey 生成摘要缓存键
func (cc *ContextCompressor) generateSummaryKey(messages []Message) string {
	data, _ := json.Marshal(messages)
	return fmt.Sprintf("summary_%x", md5.Sum(data))
}

// GetStats 获取压缩统计
func (cc *ContextCompressor) GetStats() map[string]interface{} {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	totalCompressions := int64(0)
	totalCompressionRatio := 0.0

	for _, compressed := range cc.compressionCache {
		totalCompressions += compressed.UsageCount
		totalCompressionRatio += compressed.CompressionRatio
	}

	avgCompressionRatio := 0.0
	if len(cc.compressionCache) > 0 {
		avgCompressionRatio = totalCompressionRatio / float64(len(cc.compressionCache))
	}

	return map[string]interface{}{
		"compression_cache_size":   len(cc.compressionCache),
		"summary_cache_size":       len(cc.summaryCache),
		"total_compressions":       totalCompressions,
		"avg_compression_ratio":    avgCompressionRatio,
		"max_context_length":       cc.maxContextLength,
		"target_compression_ratio": cc.compressionRatio,
	}
}

// CleanupCache 清理过期缓存
func (cc *ContextCompressor) CleanupCache() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// 清理1小时前的压缩缓存
	for key, compressed := range cc.compressionCache {
		if time.Since(compressed.CreatedAt) > time.Hour {
			delete(cc.compressionCache, key)
		}
	}

	// 限制摘要缓存大小
	if len(cc.summaryCache) > 1000 {
		// 简单清理：删除一半
		count := 0
		for key := range cc.summaryCache {
			delete(cc.summaryCache, key)
			count++
			if count >= len(cc.summaryCache)/2 {
				break
			}
		}
	}
}
