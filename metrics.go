package main

import (
	"sync"
	"time"
)

// Metrics 性能指标收集器
type Metrics struct {
	mu                    sync.RWMutex
	totalRequests         int64
	cachedRequests        int64
	batchedRequests       int64
	tokenRefreshCount     int64
	avgResponseTime       time.Duration
	totalResponseTime     time.Duration
	requestCount          int64
	errorCount            int64
	lastResetTime         time.Time
}

var metrics = &Metrics{
	lastResetTime: time.Now(),
}

// RecordRequest 记录请求
func (m *Metrics) RecordRequest(responseTime time.Duration, cached bool, batched bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalRequests++
	m.requestCount++
	m.totalResponseTime += responseTime
	m.avgResponseTime = m.totalResponseTime / time.Duration(m.requestCount)

	if cached {
		m.cachedRequests++
	}
	if batched {
		m.batchedRequests++
	}
}

// RecordError 记录错误
func (m *Metrics) RecordError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorCount++
}

// RecordTokenRefresh 记录token刷新
func (m *Metrics) RecordTokenRefresh() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokenRefreshCount++
}

// GetStats 获取统计信息
func (m *Metrics) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uptime := time.Since(m.lastResetTime)
	requestsPerSecond := float64(m.totalRequests) / uptime.Seconds()
	cacheHitRate := float64(m.cachedRequests) / float64(m.totalRequests) * 100
	batchRate := float64(m.batchedRequests) / float64(m.totalRequests) * 100
	errorRate := float64(m.errorCount) / float64(m.totalRequests) * 100

	return map[string]interface{}{
		"total_requests":       m.totalRequests,
		"cached_requests":      m.cachedRequests,
		"batched_requests":     m.batchedRequests,
		"token_refresh_count":  m.tokenRefreshCount,
		"error_count":          m.errorCount,
		"avg_response_time_ms": m.avgResponseTime.Milliseconds(),
		"requests_per_second":  requestsPerSecond,
		"cache_hit_rate":       cacheHitRate,
		"batch_rate":           batchRate,
		"error_rate":           errorRate,
		"uptime_seconds":       uptime.Seconds(),
	}
}

// Reset 重置统计信息
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalRequests = 0
	m.cachedRequests = 0
	m.batchedRequests = 0
	m.tokenRefreshCount = 0
	m.avgResponseTime = 0
	m.totalResponseTime = 0
	m.requestCount = 0
	m.errorCount = 0
	m.lastResetTime = time.Now()
}
