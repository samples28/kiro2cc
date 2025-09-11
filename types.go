package main

import "time"

// Message 统一的消息类型定义
type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// RequestPatternAnalytics 请求模式分析
type RequestPatternAnalytics struct {
	Pattern     string        `json:"pattern"`
	Frequency   int64         `json:"frequency"`
	AvgSize     int           `json:"avg_size"`
	AvgDuration time.Duration `json:"avg_duration"`
	Trend       string        `json:"trend"`
	LastSeen    time.Time     `json:"last_seen"`
}

// RequestPatternCache 请求模式缓存
type RequestPatternCache struct {
	BaseRequest   AnthropicRequest   `json:"base_request"`
	Variations    []AnthropicRequest `json:"variations"`
	Frequency     int64              `json:"frequency"`
	LastSeen      time.Time          `json:"last_seen"`
	NextPredicted time.Time          `json:"next_predicted"`
	SuccessRate   float64            `json:"success_rate"`
}

// 通用工具函数
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

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
