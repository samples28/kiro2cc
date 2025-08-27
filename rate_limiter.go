package main

import (
	"fmt"
	"sync"
	"time"
)

// RateLimiter 智能速率限制器
type RateLimiter struct {
	mu              sync.RWMutex
	buckets         map[string]*TokenBucket
	globalBucket    *TokenBucket
	adaptiveMode    bool
	maxRequestsPerSec int
	burstSize       int
}

// TokenBucket 令牌桶
type TokenBucket struct {
	capacity     int
	tokens       int
	refillRate   int           // 每秒补充的令牌数
	lastRefill   time.Time
	requestCount int64
	lastRequest  time.Time
}

var rateLimiter = &RateLimiter{
	buckets:           make(map[string]*TokenBucket),
	globalBucket:      NewTokenBucket(100, 50), // 全局限制：每秒50个请求，突发100个
	adaptiveMode:      true,
	maxRequestsPerSec: 50,
	burstSize:         100,
}

// NewTokenBucket 创建新的令牌桶
func NewTokenBucket(capacity, refillRate int) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// AllowRequest 检查是否允许请求
func (rl *RateLimiter) AllowRequest(clientID string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// 检查全局限制
	if !rl.globalBucket.consume() {
		waitTime := rl.globalBucket.timeToRefill()
		return false, waitTime
	}

	// 检查客户端特定限制
	bucket, exists := rl.buckets[clientID]
	if !exists {
		// 为新客户端创建令牌桶
		bucket = NewTokenBucket(20, 10) // 每个客户端：每秒10个请求，突发20个
		rl.buckets[clientID] = bucket
	}

	if !bucket.consume() {
		waitTime := bucket.timeToRefill()
		return false, waitTime
	}

	// 记录请求
	bucket.requestCount++
	bucket.lastRequest = time.Now()

	// 自适应调整
	if rl.adaptiveMode {
		rl.adaptRateLimit(clientID, bucket)
	}

	return true, 0
}

// consume 消费一个令牌
func (tb *TokenBucket) consume() bool {
	tb.refill()
	
	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	return false
}

// refill 补充令牌
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)
	
	if elapsed > time.Second {
		tokensToAdd := int(elapsed.Seconds()) * tb.refillRate
		tb.tokens = min(tb.capacity, tb.tokens+tokensToAdd)
		tb.lastRefill = now
	}
}

// timeToRefill 计算下次可用时间
func (tb *TokenBucket) timeToRefill() time.Duration {
	if tb.refillRate == 0 {
		return time.Hour // 如果没有补充速率，返回很长时间
	}
	
	tokensNeeded := 1
	secondsToWait := float64(tokensNeeded) / float64(tb.refillRate)
	return time.Duration(secondsToWait * float64(time.Second))
}

// adaptRateLimit 自适应调整速率限制
func (rl *RateLimiter) adaptRateLimit(clientID string, bucket *TokenBucket) {
	now := time.Now()
	
	// 如果客户端在过去1分钟内没有请求，增加其限制
	if now.Sub(bucket.lastRequest) > time.Minute {
		if bucket.refillRate < 20 {
			bucket.refillRate++
			bucket.capacity = min(50, bucket.capacity+2)
		}
	}
	
	// 如果客户端请求频率很高，但表现良好，可以适当增加限制
	if bucket.requestCount > 100 {
		avgRequestsPerMinute := float64(bucket.requestCount) / time.Since(bucket.lastRefill).Minutes()
		if avgRequestsPerMinute > float64(bucket.refillRate*60) && bucket.refillRate < 30 {
			bucket.refillRate += 2
			bucket.capacity += 5
		}
	}
}

// GetStats 获取速率限制统计
func (rl *RateLimiter) GetStats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	clientStats := make(map[string]interface{})
	totalClients := len(rl.buckets)
	totalRequests := int64(0)

	for clientID, bucket := range rl.buckets {
		totalRequests += bucket.requestCount
		clientStats[clientID] = map[string]interface{}{
			"request_count": bucket.requestCount,
			"current_tokens": bucket.tokens,
			"refill_rate": bucket.refillRate,
			"capacity": bucket.capacity,
			"last_request": bucket.lastRequest.Unix(),
		}
	}

	return map[string]interface{}{
		"total_clients": totalClients,
		"total_requests": totalRequests,
		"global_bucket": map[string]interface{}{
			"current_tokens": rl.globalBucket.tokens,
			"capacity": rl.globalBucket.capacity,
			"refill_rate": rl.globalBucket.refillRate,
		},
		"adaptive_mode": rl.adaptiveMode,
		"client_stats": clientStats,
	}
}

// SetGlobalLimit 设置全局限制
func (rl *RateLimiter) SetGlobalLimit(requestsPerSec, burstSize int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.maxRequestsPerSec = requestsPerSec
	rl.burstSize = burstSize
	rl.globalBucket = NewTokenBucket(burstSize, requestsPerSec)
}

// SetClientLimit 设置特定客户端限制
func (rl *RateLimiter) SetClientLimit(clientID string, requestsPerSec, burstSize int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.buckets[clientID] = NewTokenBucket(burstSize, requestsPerSec)
}

// EnableAdaptiveMode 启用自适应模式
func (rl *RateLimiter) EnableAdaptiveMode(enable bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.adaptiveMode = enable
}

// CleanupInactiveClients 清理不活跃的客户端
func (rl *RateLimiter) CleanupInactiveClients() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for clientID, bucket := range rl.buckets {
		// 清理超过1小时没有请求的客户端
		if now.Sub(bucket.lastRequest) > time.Hour {
			delete(rl.buckets, clientID)
		}
	}
}

// GetClientInfo 获取客户端信息
func (rl *RateLimiter) GetClientInfo(clientID string) map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	bucket, exists := rl.buckets[clientID]
	if !exists {
		return map[string]interface{}{
			"exists": false,
		}
	}

	return map[string]interface{}{
		"exists": true,
		"request_count": bucket.requestCount,
		"current_tokens": bucket.tokens,
		"refill_rate": bucket.refillRate,
		"capacity": bucket.capacity,
		"last_request": bucket.lastRequest.Unix(),
		"time_since_last_request": time.Since(bucket.lastRequest).Seconds(),
	}
}

// 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RateLimitMiddleware 速率限制中间件
func RateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 从请求中提取客户端ID（可以是IP地址、API密钥等）
		clientID := r.RemoteAddr
		if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
			clientID = apiKey
		}

		allowed, waitTime := rateLimiter.AllowRequest(clientID)
		if !allowed {
			w.Header().Set("Retry-After", fmt.Sprintf("%.0f", waitTime.Seconds()))
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rateLimiter.maxRequestsPerSec))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(waitTime).Unix()))
			
			http.Error(w, fmt.Sprintf("Rate limit exceeded. Try again in %.1f seconds", waitTime.Seconds()), http.StatusTooManyRequests)
			return
		}

		// 添加速率限制头部信息
		if bucket, exists := rateLimiter.buckets[clientID]; exists {
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", bucket.refillRate))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", bucket.tokens))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Second).Unix()))
		}

		next(w, r)
	}
}
