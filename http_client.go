package main

import (
	"net/http"
	"time"
)

// HTTPClientManager 管理HTTP客户端连接池
type HTTPClientManager struct {
	client *http.Client
}

var httpClientManager = &HTTPClientManager{}

// init 初始化HTTP客户端
func init() {
	// 创建优化的HTTP客户端
	transport := &http.Transport{
		MaxIdleConns:        100,              // 最大空闲连接数
		MaxIdleConnsPerHost: 10,               // 每个host的最大空闲连接数
		IdleConnTimeout:     90 * time.Second, // 空闲连接超时时间
		DisableCompression:  false,            // 启用压缩
		ForceAttemptHTTP2:   true,             // 强制尝试HTTP/2
	}

	httpClientManager.client = &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second, // 请求超时时间
	}
}

// GetClient 获取HTTP客户端
func (hcm *HTTPClientManager) GetClient() *http.Client {
	return hcm.client
}

// GetStreamingClient 获取用于流式请求的HTTP客户端
func (hcm *HTTPClientManager) GetStreamingClient() *http.Client {
	// 流式请求需要更长的超时时间
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   300 * time.Second, // 流式请求5分钟超时
	}
}
