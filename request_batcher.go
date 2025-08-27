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

// RequestBatcher 请求批处理器
type RequestBatcher struct {
	mu           sync.RWMutex
	pendingReqs  map[string]*BatchedRequest
	batchTimer   *time.Timer
	batchSize    int
	batchTimeout time.Duration
}

// BatchedRequest 批处理请求
type BatchedRequest struct {
	Request     AnthropicRequest
	ResponseCh  chan BatchResponse
	CreatedAt   time.Time
	RequestHash string
}

// BatchResponse 批处理响应
type BatchResponse struct {
	Response interface{}
	Error    error
}

var requestBatcher = &RequestBatcher{
	pendingReqs:  make(map[string]*BatchedRequest),
	batchSize:    5,                    // 批处理大小
	batchTimeout: 100 * time.Millisecond, // 批处理超时时间
}

// AddRequest 添加请求到批处理队列
func (rb *RequestBatcher) AddRequest(req AnthropicRequest) <-chan BatchResponse {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// 生成请求哈希
	reqHash := rb.generateRequestHash(req)
	
	// 检查是否已有相同请求在处理
	if existingReq, exists := rb.pendingReqs[reqHash]; exists {
		// 如果请求创建时间在5秒内，复用该请求
		if time.Since(existingReq.CreatedAt) < 5*time.Second {
			return existingReq.ResponseCh
		}
	}

	// 创建新的批处理请求
	batchedReq := &BatchedRequest{
		Request:     req,
		ResponseCh:  make(chan BatchResponse, 1),
		CreatedAt:   time.Now(),
		RequestHash: reqHash,
	}

	rb.pendingReqs[reqHash] = batchedReq

	// 检查是否需要立即处理批次
	if len(rb.pendingReqs) >= rb.batchSize {
		go rb.processBatch()
	} else if rb.batchTimer == nil {
		// 设置批处理超时定时器
		rb.batchTimer = time.AfterFunc(rb.batchTimeout, func() {
			rb.processBatch()
		})
	}

	return batchedReq.ResponseCh
}

// generateRequestHash 生成请求哈希
func (rb *RequestBatcher) generateRequestHash(req AnthropicRequest) string {
	// 只对非流式请求进行哈希，流式请求不适合批处理
	if req.Stream {
		return ""
	}

	// 创建请求的简化版本用于哈希
	hashReq := struct {
		Model    string    `json:"model"`
		Messages []Message `json:"messages"`
		MaxTokens int      `json:"max_tokens,omitempty"`
	}{
		Model:     req.Model,
		Messages:  req.Messages,
		MaxTokens: req.MaxTokens,
	}

	data, _ := json.Marshal(hashReq)
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// processBatch 处理批次
func (rb *RequestBatcher) processBatch() {
	rb.mu.Lock()
	
	if len(rb.pendingReqs) == 0 {
		rb.mu.Unlock()
		return
	}

	// 复制当前批次
	currentBatch := make(map[string]*BatchedRequest)
	for k, v := range rb.pendingReqs {
		currentBatch[k] = v
	}
	
	// 清空待处理队列
	rb.pendingReqs = make(map[string]*BatchedRequest)
	
	// 重置定时器
	if rb.batchTimer != nil {
		rb.batchTimer.Stop()
		rb.batchTimer = nil
	}
	
	rb.mu.Unlock()

	// 异步处理批次
	go rb.executeBatch(currentBatch)
}

// executeBatch 执行批次处理
func (rb *RequestBatcher) executeBatch(batch map[string]*BatchedRequest) {
	for _, batchedReq := range batch {
		// 对于每个请求，异步执行
		go func(req *BatchedRequest) {
			// 这里调用实际的API处理逻辑
			response, err := rb.executeRequest(req.Request)
			
			// 发送响应
			select {
			case req.ResponseCh <- BatchResponse{Response: response, Error: err}:
			case <-time.After(30 * time.Second):
				// 超时处理
			}
			close(req.ResponseCh)
		}(batchedReq)
	}
}

// executeRequest 执行单个请求
func (rb *RequestBatcher) executeRequest(req AnthropicRequest) (interface{}, error) {
	// 获取token
	token, err := tokenManager.GetToken()
	if err != nil {
		return nil, err
	}

	// 构建CodeWhisperer请求
	cwReq := buildCodeWhispererRequest(req)
	
	// 序列化请求体
	cwReqBody, err := json.Marshal(cwReq)
	if err != nil {
		return nil, err
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequest(
		http.MethodPost,
		"https://codewhisperer.us-east-1.amazonaws.com/generateAssistantResponse",
		bytes.NewBuffer(cwReqBody),
	)
	if err != nil {
		return nil, err
	}

	// 设置请求头
	httpReq.Header.Set("Authorization", "Bearer "+token.AccessToken)
	httpReq.Header.Set("Content-Type", "application/json")

	// 发送请求
	client := httpClientManager.GetClient()
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 处理响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 如果token过期，异步刷新
	if resp.StatusCode == 403 {
		go tokenManager.refreshTokenAsync()
		return nil, fmt.Errorf("token已过期，已异步刷新，请重试")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API请求失败，状态码: %d", resp.StatusCode)
	}

	return respBody, nil
}
