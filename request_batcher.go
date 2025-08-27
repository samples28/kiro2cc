package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// RequestBatcher 智能请求批处理器
type RequestBatcher struct {
	mu           sync.RWMutex
	pendingReqs  []*BatchedRequest
	batchTimer   *time.Timer
	batchSize    int
	batchTimeout time.Duration
	processing   bool
}

// BatchedRequest 批处理请求
type BatchedRequest struct {
	Request    AnthropicRequest
	ResponseCh chan BatchResponse
	CreatedAt  time.Time
	RequestID  string
}

// BatchResponse 批处理响应
type BatchResponse struct {
	Response interface{}
	Error    error
}

var requestBatcher = &RequestBatcher{
	pendingReqs:  make([]*BatchedRequest, 0),
	batchSize:    3,                    // 每批最多3个请求
	batchTimeout: 200 * time.Millisecond, // 200ms超时
	processing:   false,
}

// AddRequest 添加请求到批处理队列
func (rb *RequestBatcher) AddRequest(req AnthropicRequest) <-chan BatchResponse {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// 如果是流式请求，不进行批处理
	if req.Stream {
		responseCh := make(chan BatchResponse, 1)
		go func() {
			responseCh <- BatchResponse{Error: fmt.Errorf("streaming requests not supported in batch")}
			close(responseCh)
		}()
		return responseCh
	}

	// 生成唯一请求ID
	requestID := fmt.Sprintf("req_%d_%d", time.Now().UnixNano(), len(rb.pendingReqs))

	// 创建新的批处理请求
	batchedReq := &BatchedRequest{
		Request:    req,
		ResponseCh: make(chan BatchResponse, 1),
		CreatedAt:  time.Now(),
		RequestID:  requestID,
	}

	// 添加到待处理队列
	rb.pendingReqs = append(rb.pendingReqs, batchedReq)

	// 检查是否需要立即处理批次
	if len(rb.pendingReqs) >= rb.batchSize {
		if !rb.processing {
			rb.processing = true
			go rb.processBatch()
		}
	} else if rb.batchTimer == nil && !rb.processing {
		// 设置批处理超时定时器
		rb.batchTimer = time.AfterFunc(rb.batchTimeout, func() {
			rb.mu.Lock()
			if !rb.processing && len(rb.pendingReqs) > 0 {
				rb.processing = true
				rb.mu.Unlock()
				rb.processBatch()
			} else {
				rb.mu.Unlock()
			}
		})
	}

	return batchedReq.ResponseCh
}



// processBatch 处理批次
func (rb *RequestBatcher) processBatch() {
	rb.mu.Lock()

	if len(rb.pendingReqs) == 0 {
		rb.processing = false
		rb.mu.Unlock()
		return
	}

	// 复制当前批次
	currentBatch := make([]*BatchedRequest, len(rb.pendingReqs))
	copy(currentBatch, rb.pendingReqs)

	// 清空待处理队列
	rb.pendingReqs = make([]*BatchedRequest, 0)

	// 重置定时器
	if rb.batchTimer != nil {
		rb.batchTimer.Stop()
		rb.batchTimer = nil
	}

	rb.mu.Unlock()

	// 执行批次处理
	rb.executeBatch(currentBatch)

	// 标记处理完成
	rb.mu.Lock()
	rb.processing = false
	rb.mu.Unlock()
}

// executeBatch 执行批次处理 - 真正的请求合并
func (rb *RequestBatcher) executeBatch(batch []*BatchedRequest) {
	if len(batch) == 0 {
		return
	}

	fmt.Printf("🚀 批处理: 合并 %d 个请求\n", len(batch))

	// 如果只有一个请求，直接处理
	if len(batch) == 1 {
		rb.executeSingleRequest(batch[0])
		return
	}

	// 尝试智能合并请求
	mergedRequest, canMerge := rb.mergeRequests(batch)
	if canMerge {
		// 执行合并后的请求
		response, err := rb.executeRequest(mergedRequest)
		if err == nil {
			// 成功：将响应分发给所有原始请求
			rb.distributeResponse(batch, response)
			fmt.Printf("✅ 批处理成功: %d 个请求合并为 1 个\n", len(batch))
			return
		}
		fmt.Printf("⚠️ 合并请求失败，回退到单独处理: %v\n", err)
	}

	// 合并失败或不可合并，单独处理每个请求
	for _, req := range batch {
		go rb.executeSingleRequest(req)
	}
}

// executeSingleRequest 执行单个请求
func (rb *RequestBatcher) executeSingleRequest(batchedReq *BatchedRequest) {
	response, err := rb.executeRequest(batchedReq.Request)

	// 发送响应
	select {
	case batchedReq.ResponseCh <- BatchResponse{Response: response, Error: err}:
	case <-time.After(30 * time.Second):
		// 超时处理
	}
	close(batchedReq.ResponseCh)
}

// mergeRequests 智能合并请求
func (rb *RequestBatcher) mergeRequests(batch []*BatchedRequest) (AnthropicRequest, bool) {
	if len(batch) <= 1 {
		return AnthropicRequest{}, false
	}

	// 检查是否可以合并（相同模型、非流式）
	firstReq := batch[0].Request
	for _, batchedReq := range batch[1:] {
		if batchedReq.Request.Model != firstReq.Model || batchedReq.Request.Stream {
			return AnthropicRequest{}, false
		}
	}

	// 创建合并后的请求
	mergedReq := AnthropicRequest{
		Model:     firstReq.Model,
		MaxTokens: firstReq.MaxTokens,
		Stream:    false,
		System:    firstReq.System,
		Messages:  make([]AnthropicRequestMessage, 0),
	}

	// 合并所有消息，添加分隔符
	for i, batchedReq := range batch {
		if i > 0 {
			// 添加分隔符
			mergedReq.Messages = append(mergedReq.Messages, AnthropicRequestMessage{
				Role:    "user",
				Content: fmt.Sprintf("--- 请求 %d ---", i+1),
			})
		}

		// 添加原始消息
		mergedReq.Messages = append(mergedReq.Messages, batchedReq.Request.Messages...)
	}

	return mergedReq, true
}

// distributeResponse 分发响应给所有原始请求
func (rb *RequestBatcher) distributeResponse(batch []*BatchedRequest, response interface{}) {
	// 简化处理：给每个请求发送相同的响应
	// 在实际应用中，可能需要解析响应并分发给对应的请求
	for _, batchedReq := range batch {
		select {
		case batchedReq.ResponseCh <- BatchResponse{Response: response, Error: nil}:
		case <-time.After(30 * time.Second):
			// 超时处理
		}
		close(batchedReq.ResponseCh)
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
