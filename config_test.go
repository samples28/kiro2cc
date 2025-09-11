package main

import (
	"testing"
	"time"
)

func TestConfigDefaults(t *testing.T) {
	cfg := GetConfig()

	// 验证HTTP客户端默认配置
	if cfg.HTTPClient.MaxIdleConns != 100 {
		t.Errorf("Expected MaxIdleConns to be 100, got %d", cfg.HTTPClient.MaxIdleConns)
	}

	if cfg.HTTPClient.MaxIdleConnsPerHost != 10 {
		t.Errorf("Expected MaxIdleConnsPerHost to be 10, got %d", cfg.HTTPClient.MaxIdleConnsPerHost)
	}

	if cfg.HTTPClient.IdleConnTimeout != 90*time.Second {
		t.Errorf("Expected IdleConnTimeout to be 90s, got %v", cfg.HTTPClient.IdleConnTimeout)
	}

	// 验证缓存默认配置
	if cfg.Cache.MaxSize != 1000 {
		t.Errorf("Expected Cache.MaxSize to be 1000, got %d", cfg.Cache.MaxSize)
	}

	if cfg.Cache.TTL != 10*time.Minute {
		t.Errorf("Expected Cache.TTL to be 10m, got %v", cfg.Cache.TTL)
	}

	// 验证批处理默认配置
	if cfg.Batch.Size != 5 {
		t.Errorf("Expected Batch.Size to be 5, got %d", cfg.Batch.Size)
	}

	if cfg.Batch.Timeout != 100*time.Millisecond {
		t.Errorf("Expected Batch.Timeout to be 100ms, got %v", cfg.Batch.Timeout)
	}

	if !cfg.Batch.Enabled {
		t.Error("Expected Batch.Enabled to be true")
	}

	t.Log("Config defaults test passed")
}

func TestConfigValidation(t *testing.T) {
	cfg := GetConfig()

	// 验证API配置
	if cfg.API.CodeWhispererURL == "" {
		t.Error("CodeWhispererURL should not be empty")
	}

	if cfg.API.KiroAuthURL == "" {
		t.Error("KiroAuthURL should not be empty")
	}

	if cfg.API.ProfileArn == "" {
		t.Error("ProfileArn should not be empty")
	}

	t.Log("Config validation test passed")
}
