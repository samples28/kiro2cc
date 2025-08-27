package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// RequestDeduplicator 请求去重器
type RequestDeduplicator struct {
	mu              sync.RWMutex
	activeRequests  map[string]*ActiveRequest
	recentRequests  map[string]*RecentRequest
	mergeableGroups map[string]*MergeableGroup
	cleanupTimer    *time.Timer
}

// ActiveRequest 活跃请求
type ActiveRequest struct {
	Request     AnthropicRequest
	ResponseCh  chan DedupeResponse
	Subscribers []chan DedupeResponse
	StartTime   time.Time
	RequestHash string
}

// RecentRequest 最近请求
type RecentRequest struct {
	Request   AnthropicRequest
	Response  interface{}
	Timestamp time.Time
	Hash      string
}

// MergeableGroup 可合并的请求组
type MergeableGroup struct {
	BaseRequest   AnthropicRequest
	Variations    []AnthropicRequest
	LastMerged    time.Time
	MergeCount    int64
	ResponseCache interface{}
}

// DedupeResponse 去重响应
type DedupeResponse struct {
	Response interface{}
	Error    error
	FromCache bool
	Merged   bool
}

var requestDeduplicator = &RequestDeduplicator{
	activeRequests:  make(map[string]*ActiveRequest),
	recentRequests:  make(map[string]*RecentRequest),
	mergeableGroups: make(map[string]*MergeableGroup),
}

// init 启动清理定时器
func init() {
	requestDeduplicator.cleanupTimer = time.NewTimer(5 * time.Minute)
	go requestDeduplicator.cleanupLoop()
}

// ProcessRequest 处理请求去重
func (rd *RequestDeduplicator) ProcessRequest(req AnthropicRequest) <-chan DedupeResponse {
	// 生成请求哈希
	reqHash := rd.generateRequestHash(req)
	
	rd.mu.Lock()
	defer rd.mu.Unlock()

	// 检查是否有相同的活跃请求
	if activeReq, exists := rd.activeRequests[reqHash]; exists {
		// 订阅现有请求
		responseCh := make(chan DedupeResponse, 1)
		activeReq.Subscribers = append(activeReq.Subscribers, responseCh)
		return responseCh
	}

	// 检查最近请求缓存
	if recentReq, exists := rd.recentRequests[reqHash]; exists {
		if time.Since(recentReq.Timestamp) < 2*time.Minute {
			responseCh := make(chan DedupeResponse, 1)
			responseCh <- DedupeResponse{
				Response:  recentReq.Response,
				Error:     nil,
				FromCache: true,
				Merged:    false,
			}
			close(responseCh)
			return responseCh
		}
	}

	// 检查是否可以合并到现有组
	if mergedResponse := rd.tryMergeRequest(req); mergedResponse != nil {
		responseCh := make(chan DedupeResponse, 1)
		responseCh <- *mergedResponse
		close(responseCh)
		return responseCh
	}

	// 创建新的活跃请求
	responseCh := make(chan DedupeResponse, 1)
	activeReq := &ActiveRequest{
		Request:     req,
		ResponseCh:  responseCh,
		Subscribers: []chan DedupeResponse{},
		StartTime:   time.Now(),
		RequestHash: reqHash,
	}

	rd.activeRequests[reqHash] = activeReq

	// 异步执行请求
	go rd.executeRequest(activeReq)

	return responseCh
}

// tryMergeRequest 尝试合并请求
func (rd *RequestDeduplicator) tryMergeRequest(req AnthropicRequest) *DedupeResponse {
	mergeKey := rd.generateMergeKey(req)
	
	if group, exists := rd.mergeableGroups[mergeKey]; exists {
		// 检查是否可以合并
		if rd.canMergeWithGroup(req, group) && time.Since(group.LastMerged) < 5*time.Minute {
			// 更新组信息
			group.Variations = append(group.Variations, req)
			group.MergeCount++
			group.LastMerged = time.Now()
			
			// 限制变体数量
			if len(group.Variations) > 10 {
				group.Variations = group.Variations[1:]
			}

			if group.ResponseCache != nil {
				return &DedupeResponse{
					Response:  group.ResponseCache,
					Error:     nil,
					FromCache: true,
					Merged:    true,
				}
			}
		}
	}

	return nil
}

// canMergeWithGroup 检查是否可以与组合并
func (rd *RequestDeduplicator) canMergeWithGroup(req AnthropicRequest, group *MergeableGroup) bool {
	// 模型必须相同
	if req.Model != group.BaseRequest.Model {
		return false
	}

	// 消息数量差异不能太大
	if absInt(len(req.Messages)-len(group.BaseRequest.Messages)) > 2 {
		return false
	}

	// 计算内容相似度
	similarity := rd.calculateContentSimilarity(req, group.BaseRequest)
	return similarity > 0.7
}

// calculateContentSimilarity 计算内容相似度
func (rd *RequestDeduplicator) calculateContentSimilarity(req1, req2 AnthropicRequest) float64 {
	if len(req1.Messages) == 0 && len(req2.Messages) == 0 {
		return 1.0
	}
	if len(req1.Messages) == 0 || len(req2.Messages) == 0 {
		return 0.0
	}

	// 提取最后一条用户消息进行比较
	var lastMsg1, lastMsg2 string
	for i := len(req1.Messages) - 1; i >= 0; i-- {
		if req1.Messages[i].Role == "user" {
			lastMsg1 = getMessageContent(req1.Messages[i].Content)
			break
		}
	}
	for i := len(req2.Messages) - 1; i >= 0; i-- {
		if req2.Messages[i].Role == "user" {
			lastMsg2 = getMessageContent(req2.Messages[i].Content)
			break
		}
	}

	return rd.calculateTextSimilarity(lastMsg1, lastMsg2)
}

// calculateTextSimilarity 计算文本相似度
func (rd *RequestDeduplicator) calculateTextSimilarity(text1, text2 string) float64 {
	if text1 == text2 {
		return 1.0
	}
	if text1 == "" || text2 == "" {
		return 0.0
	}

	// 简化的编辑距离相似度
	maxLen := max(len(text1), len(text2))
	if maxLen == 0 {
		return 1.0
	}

	distance := rd.levenshteinDistance(text1, text2)
	return 1.0 - float64(distance)/float64(maxLen)
}

// levenshteinDistance 计算编辑距离
func (rd *RequestDeduplicator) levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// 限制计算复杂度
	if len(s1) > 500 {
		s1 = s1[:500]
	}
	if len(s2) > 500 {
		s2 = s2[:500]
	}

	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			deletion := matrix[i-1][j] + 1
			insertion := matrix[i][j-1] + 1
			substitution := matrix[i-1][j-1] + cost
			matrix[i][j] = minInt(minInt(deletion, insertion), substitution)
		}
	}

	return matrix[len(s1)][len(s2)]
}

// executeRequest 执行请求
func (rd *RequestDeduplicator) executeRequest(activeReq *ActiveRequest) {
	// 执行实际的API请求
	response, err := rd.performAPIRequest(activeReq.Request)
	
	// 创建响应
	dedupeResp := DedupeResponse{
		Response:  response,
		Error:     err,
		FromCache: false,
		Merged:    false,
	}

	// 发送给主请求
	activeReq.ResponseCh <- dedupeResp
	close(activeReq.ResponseCh)

	// 发送给所有订阅者
	for _, subscriberCh := range activeReq.Subscribers {
		subscriberCh <- dedupeResp
		close(subscriberCh)
	}

	// 更新缓存和清理
	rd.mu.Lock()
	defer rd.mu.Unlock()

	// 添加到最近请求缓存
	if err == nil {
		rd.recentRequests[activeReq.RequestHash] = &RecentRequest{
			Request:   activeReq.Request,
			Response:  response,
			Timestamp: time.Now(),
			Hash:      activeReq.RequestHash,
		}
	}

	// 更新可合并组
	rd.updateMergeableGroup(activeReq.Request, response, err)

	// 从活跃请求中移除
	delete(rd.activeRequests, activeReq.RequestHash)
}

// performAPIRequest 执行API请求
func (rd *RequestDeduplicator) performAPIRequest(req AnthropicRequest) (interface{}, error) {
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
		return nil, fmt.Errorf("API请求失败: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return respBody, nil
}

// updateMergeableGroup 更新可合并组
func (rd *RequestDeduplicator) updateMergeableGroup(req AnthropicRequest, response interface{}, err error) {
	if err != nil {
		return // 错误响应不缓存
	}

	mergeKey := rd.generateMergeKey(req)
	
	if group, exists := rd.mergeableGroups[mergeKey]; exists {
		group.ResponseCache = response
		group.LastMerged = time.Now()
	} else {
		rd.mergeableGroups[mergeKey] = &MergeableGroup{
			BaseRequest:   req,
			Variations:    []AnthropicRequest{req},
			LastMerged:    time.Now(),
			MergeCount:    1,
			ResponseCache: response,
		}
	}
}

// generateRequestHash 生成请求哈希
func (rd *RequestDeduplicator) generateRequestHash(req AnthropicRequest) string {
	data, _ := json.Marshal(req)
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// generateMergeKey 生成合并键
func (rd *RequestDeduplicator) generateMergeKey(req AnthropicRequest) string {
	// 基于请求的抽象特征生成合并键
	key := struct {
		Model        string `json:"model"`
		MessageCount int    `json:"message_count"`
		LastUserMsg  string `json:"last_user_msg"`
	}{
		Model:        req.Model,
		MessageCount: len(req.Messages),
	}

	// 提取最后一条用户消息的前100个字符
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			content := getMessageContent(req.Messages[i].Content)
			if len(content) > 100 {
				content = content[:100]
			}
			key.LastUserMsg = content
			break
		}
	}

	data, _ := json.Marshal(key)
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:16]) // 使用较短的键
}

// cleanupLoop 清理循环
func (rd *RequestDeduplicator) cleanupLoop() {
	for {
		select {
		case <-rd.cleanupTimer.C:
			rd.cleanup()
			rd.cleanupTimer.Reset(5 * time.Minute)
		}
	}
}

// cleanup 清理过期数据
func (rd *RequestDeduplicator) cleanup() {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	now := time.Now()

	// 清理过期的最近请求
	for hash, recent := range rd.recentRequests {
		if now.Sub(recent.Timestamp) > 10*time.Minute {
			delete(rd.recentRequests, hash)
		}
	}

	// 清理过期的可合并组
	for key, group := range rd.mergeableGroups {
		if now.Sub(group.LastMerged) > 30*time.Minute {
			delete(rd.mergeableGroups, key)
		}
	}

	// 清理超时的活跃请求
	for hash, active := range rd.activeRequests {
		if now.Sub(active.StartTime) > 2*time.Minute {
			// 发送超时错误
			timeoutResp := DedupeResponse{
				Response:  nil,
				Error:     fmt.Errorf("请求超时"),
				FromCache: false,
				Merged:    false,
			}
			
			active.ResponseCh <- timeoutResp
			close(active.ResponseCh)
			
			for _, subscriberCh := range active.Subscribers {
				subscriberCh <- timeoutResp
				close(subscriberCh)
			}
			
			delete(rd.activeRequests, hash)
		}
	}
}

// GetStats 获取去重统计
func (rd *RequestDeduplicator) GetStats() map[string]interface{} {
	rd.mu.RLock()
	defer rd.mu.RUnlock()

	totalMerges := int64(0)
	for _, group := range rd.mergeableGroups {
		totalMerges += group.MergeCount
	}

	return map[string]interface{}{
		"active_requests":    len(rd.activeRequests),
		"recent_requests":    len(rd.recentRequests),
		"mergeable_groups":   len(rd.mergeableGroups),
		"total_merges":       totalMerges,
	}
}

// 辅助函数
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func absInt(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
