package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)



// PredictiveCache 预测性缓存
type PredictiveCache struct {
	mu              sync.RWMutex
	cache           map[string]*PredictiveCacheEntry
	patterns        map[string]*RequestPattern
	prefetchQueue   chan PrefetchRequest
	maxPrefetch     int
	similarityThreshold float64
}

// PredictiveCacheEntry 预测缓存条目
type PredictiveCacheEntry struct {
	Response    interface{}
	CreatedAt   time.Time
	AccessCount int64
	LastAccess  time.Time
	Confidence  float64 // 预测置信度
	IsPrefetch  bool    // 是否为预取数据
}

// RequestPattern 请求模式
type RequestPattern struct {
	BaseRequest     AnthropicRequest
	Variations      []AnthropicRequest
	Frequency       int64
	LastSeen        time.Time
	NextPredicted   time.Time
	SuccessRate     float64
}

// PrefetchRequest 预取请求
type PrefetchRequest struct {
	Request    AnthropicRequest
	Confidence float64
	Priority   int
}

var predictiveCache = &PredictiveCache{
	cache:               make(map[string]*PredictiveCacheEntry),
	patterns:            make(map[string]*RequestPattern),
	prefetchQueue:       make(chan PrefetchRequest, 100),
	maxPrefetch:         10,
	similarityThreshold: 0.8,
}

// init 启动预取工作器
func init() {
	go predictiveCache.prefetchWorker()
	go predictiveCache.patternAnalyzer()
}

// Get 获取缓存，支持模糊匹配
func (pc *PredictiveCache) Get(req AnthropicRequest) (interface{}, bool, float64) {
	key := pc.generateKey(req)
	
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	// 精确匹配
	if entry, exists := pc.cache[key]; exists && !pc.isExpired(entry) {
		entry.AccessCount++
		entry.LastAccess = time.Now()
		return entry.Response, true, 1.0
	}

	// 模糊匹配 - 寻找相似请求
	bestMatch, similarity := pc.findSimilarRequest(req)
	if bestMatch != nil && similarity >= pc.similarityThreshold {
		bestMatch.AccessCount++
		bestMatch.LastAccess = time.Now()
		return bestMatch.Response, true, similarity
	}

	return nil, false, 0.0
}

// Set 设置缓存并学习模式
func (pc *PredictiveCache) Set(req AnthropicRequest, response interface{}) {
	key := pc.generateKey(req)
	
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.cache[key] = &PredictiveCacheEntry{
		Response:    response,
		CreatedAt:   time.Now(),
		AccessCount: 1,
		LastAccess:  time.Now(),
		Confidence:  1.0,
		IsPrefetch:  false,
	}

	// 学习请求模式
	pc.learnPattern(req)
	
	// 触发预测
	go pc.predictNextRequests(req)
}

// findSimilarRequest 寻找相似请求
func (pc *PredictiveCache) findSimilarRequest(req AnthropicRequest) (*PredictiveCacheEntry, float64) {
	var bestEntry *PredictiveCacheEntry
	var bestSimilarity float64

	for cachedKey, entry := range pc.cache {
		if pc.isExpired(entry) {
			continue
		}

		// 解析缓存的请求
		cachedReq := pc.parseKeyToRequest(cachedKey)
		if cachedReq == nil {
			continue
		}

		similarity := pc.calculateSimilarity(req, *cachedReq)
		if similarity > bestSimilarity && similarity >= pc.similarityThreshold {
			bestSimilarity = similarity
			bestEntry = entry
		}
	}

	return bestEntry, bestSimilarity
}

// calculateSimilarity 计算请求相似度
func (pc *PredictiveCache) calculateSimilarity(req1, req2 AnthropicRequest) float64 {
	score := 0.0
	factors := 0.0

	// 模型匹配 (权重: 0.3)
	if req1.Model == req2.Model {
		score += 0.3
	}
	factors += 0.3

	// 消息数量匹配 (权重: 0.2)
	if len(req1.Messages) == len(req2.Messages) {
		score += 0.2
	}
	factors += 0.2

	// 内容相似度 (权重: 0.5)
	contentSimilarity := pc.calculateContentSimilarity(req1.Messages, req2.Messages)
	score += contentSimilarity * 0.5
	factors += 0.5

	return score / factors
}

// calculateContentSimilarity 计算内容相似度
func (pc *PredictiveCache) calculateContentSimilarity(msgs1, msgs2 []Message) float64 {
	if len(msgs1) == 0 && len(msgs2) == 0 {
		return 1.0
	}
	if len(msgs1) == 0 || len(msgs2) == 0 {
		return 0.0
	}

	// 简化的文本相似度计算
	text1 := pc.extractTextFromMessages(msgs1)
	text2 := pc.extractTextFromMessages(msgs2)

	return pc.calculateTextSimilarity(text1, text2)
}

// extractTextFromMessages 从消息中提取文本
func (pc *PredictiveCache) extractTextFromMessages(msgs []Message) string {
	var texts []string
	for _, msg := range msgs {
		if content := getMessageContent(msg.Content); content != "" {
			texts = append(texts, strings.ToLower(content))
		}
	}
	return strings.Join(texts, " ")
}

// calculateTextSimilarity 计算文本相似度 (简化版Jaccard相似度)
func (pc *PredictiveCache) calculateTextSimilarity(text1, text2 string) float64 {
	words1 := strings.Fields(text1)
	words2 := strings.Fields(text2)

	if len(words1) == 0 && len(words2) == 0 {
		return 1.0
	}
	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	set1 := make(map[string]bool)
	set2 := make(map[string]bool)

	for _, word := range words1 {
		set1[word] = true
	}
	for _, word := range words2 {
		set2[word] = true
	}

	intersection := 0
	union := len(set1)

	for word := range set2 {
		if set1[word] {
			intersection++
		} else {
			union++
		}
	}

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// learnPattern 学习请求模式
func (pc *PredictiveCache) learnPattern(req AnthropicRequest) {
	patternKey := pc.generatePatternKey(req)
	
	if pattern, exists := pc.patterns[patternKey]; exists {
		pattern.Frequency++
		pattern.LastSeen = time.Now()
		pattern.Variations = append(pattern.Variations, req)
		
		// 限制变体数量
		if len(pattern.Variations) > 10 {
			pattern.Variations = pattern.Variations[1:]
		}
	} else {
		pc.patterns[patternKey] = &RequestPattern{
			BaseRequest:   req,
			Variations:    []AnthropicRequest{req},
			Frequency:     1,
			LastSeen:      time.Now(),
			NextPredicted: time.Now().Add(time.Hour),
			SuccessRate:   1.0,
		}
	}
}

// predictNextRequests 预测下一个请求
func (pc *PredictiveCache) predictNextRequests(currentReq AnthropicRequest) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	for _, pattern := range pc.patterns {
		if pattern.Frequency < 3 { // 至少出现3次才预测
			continue
		}

		// 基于历史模式预测可能的下一个请求
		for _, variation := range pattern.Variations {
			confidence := pc.calculatePredictionConfidence(currentReq, variation, pattern)
			if confidence > 0.6 {
				select {
				case pc.prefetchQueue <- PrefetchRequest{
					Request:    variation,
					Confidence: confidence,
					Priority:   int(pattern.Frequency),
				}:
				default:
					// 队列满了，跳过
				}
			}
		}
	}
}

// calculatePredictionConfidence 计算预测置信度
func (pc *PredictiveCache) calculatePredictionConfidence(current, predicted AnthropicRequest, pattern *RequestPattern) float64 {
	// 基于频率、时间间隔、相似度等因素计算置信度
	frequencyScore := float64(pattern.Frequency) / 100.0
	if frequencyScore > 1.0 {
		frequencyScore = 1.0
	}

	timeScore := 1.0 - (time.Since(pattern.LastSeen).Hours() / 24.0)
	if timeScore < 0 {
		timeScore = 0
	}

	similarityScore := pc.calculateSimilarity(current, predicted)

	return (frequencyScore*0.4 + timeScore*0.3 + similarityScore*0.3) * pattern.SuccessRate
}

// prefetchWorker 预取工作器
func (pc *PredictiveCache) prefetchWorker() {
	activePrefetches := 0
	
	for prefetchReq := range pc.prefetchQueue {
		if activePrefetches >= pc.maxPrefetch {
			continue
		}

		// 检查是否已经缓存
		if _, exists, _ := pc.Get(prefetchReq.Request); exists {
			continue
		}

		activePrefetches++
		go func(req PrefetchRequest) {
			defer func() { activePrefetches-- }()
			
			// 执行预取
			response, err := pc.executePrefetch(req.Request)
			if err == nil {
				pc.setPrefetchCache(req.Request, response, req.Confidence)
			}
		}(prefetchReq)
	}
}

// executePrefetch 执行预取请求
func (pc *PredictiveCache) executePrefetch(req AnthropicRequest) (interface{}, error) {
	// 获取token
	token, err := tokenManager.GetToken()
	if err != nil {
		return nil, err
	}

	// 构建请求
	cwReq := buildCodeWhispererRequest(req)
	cwReqBody, err := json.Marshal(cwReq)
	if err != nil {
		return nil, err
	}

	// 发送请求
	httpReq, err := http.NewRequest(
		http.MethodPost,
		config.API.CodeWhispererURL,
		bytes.NewBuffer(cwReqBody),
	)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+token.AccessToken)
	httpReq.Header.Set("Content-Type", "application/json")

	client := httpClientManager.GetClient()
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("预取请求失败: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return respBody, nil
}

// setPrefetchCache 设置预取缓存
func (pc *PredictiveCache) setPrefetchCache(req AnthropicRequest, response interface{}, confidence float64) {
	key := pc.generateKey(req)
	
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.cache[key] = &PredictiveCacheEntry{
		Response:    response,
		CreatedAt:   time.Now(),
		AccessCount: 0,
		LastAccess:  time.Now(),
		Confidence:  confidence,
		IsPrefetch:  true,
	}
}

// patternAnalyzer 模式分析器
func (pc *PredictiveCache) patternAnalyzer() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		pc.analyzeAndOptimizePatterns()
	}
}

// analyzeAndOptimizePatterns 分析和优化模式
func (pc *PredictiveCache) analyzeAndOptimizePatterns() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// 清理过期模式
	for key, pattern := range pc.patterns {
		if time.Since(pattern.LastSeen) > 24*time.Hour && pattern.Frequency < 5 {
			delete(pc.patterns, key)
		}
	}

	// 清理过期缓存
	for key, entry := range pc.cache {
		if pc.isExpired(entry) {
			delete(pc.cache, key)
		}
	}
}

// generateKey 生成缓存键
func (pc *PredictiveCache) generateKey(req AnthropicRequest) string {
	data, _ := json.Marshal(struct {
		Model     string    `json:"model"`
		Messages  []Message `json:"messages"`
		MaxTokens int       `json:"max_tokens,omitempty"`
	}{
		Model:     req.Model,
		Messages:  req.Messages,
		MaxTokens: req.MaxTokens,
	})
	
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// generatePatternKey 生成模式键
func (pc *PredictiveCache) generatePatternKey(req AnthropicRequest) string {
	// 基于请求的抽象特征生成模式键
	features := struct {
		Model        string `json:"model"`
		MessageCount int    `json:"message_count"`
		HasSystem    bool   `json:"has_system"`
		AvgLength    int    `json:"avg_length"`
	}{
		Model:        req.Model,
		MessageCount: len(req.Messages),
		HasSystem:    len(req.Messages) > 0 && req.Messages[0].Role == "system",
	}

	// 计算平均消息长度
	totalLength := 0
	for _, msg := range req.Messages {
		totalLength += len(getMessageContent(msg.Content))
	}
	if len(req.Messages) > 0 {
		features.AvgLength = totalLength / len(req.Messages)
	}

	data, _ := json.Marshal(features)
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:8]) // 使用较短的键
}

// parseKeyToRequest 从键解析请求 (简化实现)
func (pc *PredictiveCache) parseKeyToRequest(key string) *AnthropicRequest {
	// 这里需要实现反向解析，或者在缓存时同时存储原始请求
	// 简化实现，返回nil
	return nil
}

// isExpired 检查缓存是否过期
func (pc *PredictiveCache) isExpired(entry *PredictiveCacheEntry) bool {
	ttl := 10 * time.Minute
	if entry.IsPrefetch {
		ttl = 30 * time.Minute // 预取数据保存更久
	}
	return time.Since(entry.CreatedAt) > ttl
}

// GetStats 获取预测缓存统计
func (pc *PredictiveCache) GetStats() map[string]interface{} {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	prefetchCount := 0
	totalConfidence := 0.0
	
	for _, entry := range pc.cache {
		if entry.IsPrefetch {
			prefetchCount++
			totalConfidence += entry.Confidence
		}
	}

	avgConfidence := 0.0
	if prefetchCount > 0 {
		avgConfidence = totalConfidence / float64(prefetchCount)
	}

	return map[string]interface{}{
		"total_cache_entries":    len(pc.cache),
		"prefetch_entries":       prefetchCount,
		"learned_patterns":       len(pc.patterns),
		"avg_prefetch_confidence": avgConfidence,
		"prefetch_queue_size":    len(pc.prefetchQueue),
	}
}
