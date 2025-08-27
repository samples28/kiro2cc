package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

// CacheEntry 缓存条目
type CacheEntry struct {
	Response  interface{}
	CreatedAt time.Time
	AccessCount int64
}

// ResponseCache 响应缓存
type ResponseCache struct {
	mu       sync.RWMutex
	cache    map[string]*CacheEntry
	maxSize  int
	ttl      time.Duration
	cleanupTimer *time.Timer
}

var responseCache = &ResponseCache{
	cache:   make(map[string]*CacheEntry),
	maxSize: 1000,                // 最大缓存条目数
	ttl:     10 * time.Minute,    // 缓存TTL
}

// init 初始化缓存清理定时器
func init() {
	// 每5分钟清理一次过期缓存
	responseCache.cleanupTimer = time.NewTimer(5 * time.Minute)
	go responseCache.cleanupLoop()
}

// Get 从缓存获取响应
func (rc *ResponseCache) Get(req AnthropicRequest) (interface{}, bool) {
	// 流式请求不缓存
	if req.Stream {
		return nil, false
	}

	key := rc.generateCacheKey(req)
	if key == "" {
		return nil, false
	}

	rc.mu.RLock()
	defer rc.mu.RUnlock()

	entry, exists := rc.cache[key]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if time.Since(entry.CreatedAt) > rc.ttl {
		// 异步删除过期条目
		go rc.deleteExpired(key)
		return nil, false
	}

	// 更新访问计数
	entry.AccessCount++
	return entry.Response, true
}

// Set 设置缓存
func (rc *ResponseCache) Set(req AnthropicRequest, response interface{}) {
	// 流式请求不缓存
	if req.Stream {
		return
	}

	key := rc.generateCacheKey(req)
	if key == "" {
		return
	}

	rc.mu.Lock()
	defer rc.mu.Unlock()

	// 检查缓存大小，如果超过限制则清理最少使用的条目
	if len(rc.cache) >= rc.maxSize {
		rc.evictLRU()
	}

	rc.cache[key] = &CacheEntry{
		Response:    response,
		CreatedAt:   time.Now(),
		AccessCount: 1,
	}
}

// generateCacheKey 生成缓存键
func (rc *ResponseCache) generateCacheKey(req AnthropicRequest) string {
	// 只缓存非流式的简单文本请求
	if req.Stream || len(req.Messages) == 0 {
		return ""
	}

	// 创建缓存键的结构
	cacheKey := struct {
		Model     string    `json:"model"`
		Messages  []Message `json:"messages"`
		MaxTokens int       `json:"max_tokens,omitempty"`
	}{
		Model:     req.Model,
		Messages:  req.Messages,
		MaxTokens: req.MaxTokens,
	}

	data, err := json.Marshal(cacheKey)
	if err != nil {
		return ""
	}

	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// evictLRU 清理最少使用的缓存条目
func (rc *ResponseCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time
	var minAccessCount int64 = -1

	// 找到最少使用且最旧的条目
	for key, entry := range rc.cache {
		if minAccessCount == -1 || entry.AccessCount < minAccessCount || 
		   (entry.AccessCount == minAccessCount && entry.CreatedAt.Before(oldestTime)) {
			oldestKey = key
			oldestTime = entry.CreatedAt
			minAccessCount = entry.AccessCount
		}
	}

	if oldestKey != "" {
		delete(rc.cache, oldestKey)
	}
}

// deleteExpired 删除过期条目
func (rc *ResponseCache) deleteExpired(key string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	delete(rc.cache, key)
}

// cleanupLoop 清理循环
func (rc *ResponseCache) cleanupLoop() {
	for {
		select {
		case <-rc.cleanupTimer.C:
			rc.cleanup()
			rc.cleanupTimer.Reset(5 * time.Minute)
		}
	}
}

// cleanup 清理过期缓存
func (rc *ResponseCache) cleanup() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	now := time.Now()
	for key, entry := range rc.cache {
		if now.Sub(entry.CreatedAt) > rc.ttl {
			delete(rc.cache, key)
		}
	}
}

// GetStats 获取缓存统计信息
func (rc *ResponseCache) GetStats() map[string]interface{} {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return map[string]interface{}{
		"cache_size": len(rc.cache),
		"max_size":   rc.maxSize,
		"ttl_minutes": rc.ttl.Minutes(),
	}
}
