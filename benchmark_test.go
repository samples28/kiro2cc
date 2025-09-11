package main

import (
	"testing"
	"time"
)

func BenchmarkResponseCache(b *testing.B) {
	cache := &ResponseCache{
		cache:   make(map[string]*CacheEntry),
		maxSize: 1000,
		ttl:     10 * time.Minute,
	}

	req := AnthropicRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []AnthropicRequestMessage{
			{Role: "user", Content: "Hello, world!"},
		},
		MaxTokens: 1000,
	}

	response := map[string]interface{}{
		"content": "Hello! How can I help you today?",
		"role":    "assistant",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 测试缓存设置性能
		cache.Set(req, response)

		// 测试缓存获取性能
		_, found := cache.Get(req)
		if !found {
			b.Error("Expected to find cached response")
		}
	}
}

func BenchmarkTokenManager(b *testing.B) {
	tm := &TokenManager{}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 测试token缓存失效性能
		tm.InvalidateToken()
	}
}

func BenchmarkMetrics(b *testing.B) {
	m := &Metrics{
		lastResetTime: time.Now(),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 测试指标记录性能
		m.RecordRequest(100*time.Millisecond, false, false)
	}
}

func BenchmarkContextCompressor(b *testing.B) {
	cc := &ContextCompressor{
		compressionCache: make(map[string]*CompressedContext),
		summaryCache:     make(map[string]string),
		maxContextLength: 4000,
		compressionRatio: 0.6,
	}

	req := AnthropicRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []AnthropicRequestMessage{
			{Role: "user", Content: "This is a test message for compression benchmarking."},
			{Role: "assistant", Content: "I understand. This is a response for testing."},
		},
		MaxTokens: 1000,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 测试上下文压缩性能
		_ = cc.CompressRequest(req)
	}
}
