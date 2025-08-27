package main

import (
	"encoding/json"
	"os"
	"time"
)

// Config 应用配置
type Config struct {
	// HTTP客户端配置
	HTTPClient struct {
		MaxIdleConns        int           `json:"max_idle_conns"`
		MaxIdleConnsPerHost int           `json:"max_idle_conns_per_host"`
		IdleConnTimeout     time.Duration `json:"idle_conn_timeout"`
		RequestTimeout      time.Duration `json:"request_timeout"`
		StreamingTimeout    time.Duration `json:"streaming_timeout"`
	} `json:"http_client"`

	// 缓存配置
	Cache struct {
		MaxSize    int           `json:"max_size"`
		TTL        time.Duration `json:"ttl"`
		CleanupInterval time.Duration `json:"cleanup_interval"`
	} `json:"cache"`

	// 批处理配置
	Batch struct {
		Size    int           `json:"size"`
		Timeout time.Duration `json:"timeout"`
		Enabled bool          `json:"enabled"`
	} `json:"batch"`

	// Token管理配置
	Token struct {
		CacheTimeout    time.Duration `json:"cache_timeout"`
		RefreshThreshold time.Duration `json:"refresh_threshold"`
	} `json:"token"`

	// API配置
	API struct {
		CodeWhispererURL string `json:"codewhisperer_url"`
		KiroAuthURL      string `json:"kiro_auth_url"`
		ProfileArn       string `json:"profile_arn"`
	} `json:"api"`
}

var config = &Config{}

// init 初始化默认配置
func init() {
	// 设置默认值
	config.HTTPClient.MaxIdleConns = 100
	config.HTTPClient.MaxIdleConnsPerHost = 10
	config.HTTPClient.IdleConnTimeout = 90 * time.Second
	config.HTTPClient.RequestTimeout = 30 * time.Second
	config.HTTPClient.StreamingTimeout = 300 * time.Second

	config.Cache.MaxSize = 1000
	config.Cache.TTL = 10 * time.Minute
	config.Cache.CleanupInterval = 5 * time.Minute

	config.Batch.Size = 5
	config.Batch.Timeout = 100 * time.Millisecond
	config.Batch.Enabled = true

	config.Token.CacheTimeout = 5 * time.Minute
	config.Token.RefreshThreshold = 10 * time.Minute

	config.API.CodeWhispererURL = "https://codewhisperer.us-east-1.amazonaws.com/generateAssistantResponse"
	config.API.KiroAuthURL = "https://prod.us-east-1.auth.desktop.kiro.dev/refreshToken"
	config.API.ProfileArn = "arn:aws:codewhisperer:us-east-1:699475941385:profile/EHGA3GRVQMUK"

	// 尝试从配置文件加载
	loadConfigFromFile()
}

// loadConfigFromFile 从文件加载配置
func loadConfigFromFile() {
	configPath := "kiro2cc-config.json"
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, config); err != nil {
			// 配置文件格式错误，使用默认配置
			return
		}
	}
}

// SaveConfig 保存配置到文件
func SaveConfig() error {
	configPath := "kiro2cc-config.json"
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// GetConfig 获取配置
func GetConfig() *Config {
	return config
}
