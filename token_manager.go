package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// TokenManager 管理token的缓存和自动刷新
type TokenManager struct {
	mu           sync.RWMutex
	cachedToken  *TokenData
	lastUpdate   time.Time
	refreshTimer *time.Timer
}

var tokenManager = &TokenManager{}

// GetToken 获取token，优先从缓存获取
func (tm *TokenManager) GetToken() (*TokenData, error) {
	tm.mu.RLock()
	
	// 检查缓存是否有效（5分钟内的token认为有效）
	if tm.cachedToken != nil && time.Since(tm.lastUpdate) < 5*time.Minute {
		defer tm.mu.RUnlock()
		return tm.cachedToken, nil
	}
	tm.mu.RUnlock()

	// 需要重新加载token
	return tm.loadAndCacheToken()
}

// loadAndCacheToken 从文件加载token并缓存
func (tm *TokenManager) loadAndCacheToken() (*TokenData, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 双重检查，防止并发加载
	if tm.cachedToken != nil && time.Since(tm.lastUpdate) < 5*time.Minute {
		return tm.cachedToken, nil
	}

	tokenPath := getTokenFilePath()
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("读取token文件失败: %v", err)
	}

	var token TokenData
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("解析token文件失败: %v", err)
	}

	// 检查token是否即将过期（提前10分钟刷新）
	if token.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, token.ExpiresAt)
		if err == nil && time.Until(expiresAt) < 10*time.Minute {
			// 异步刷新token
			go tm.refreshTokenAsync()
		}
	}

	tm.cachedToken = &token
	tm.lastUpdate = time.Now()
	
	return &token, nil
}

// refreshTokenAsync 异步刷新token
func (tm *TokenManager) refreshTokenAsync() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.cachedToken == nil {
		return
	}

	// 执行token刷新逻辑
	newToken, err := tm.performTokenRefresh(tm.cachedToken.RefreshToken)
	if err != nil {
		fmt.Printf("异步刷新token失败: %v\n", err)
		return
	}

	// 更新缓存
	tm.cachedToken = newToken
	tm.lastUpdate = time.Now()
	
	fmt.Println("Token已异步刷新")
}

// performTokenRefresh 执行实际的token刷新
func (tm *TokenManager) performTokenRefresh(refreshToken string) (*TokenData, error) {
	// 这里复用原有的refreshToken逻辑，但返回TokenData而不是直接写文件
	refreshReq := RefreshRequest{
		RefreshToken: refreshToken,
	}

	reqBody, err := json.Marshal(refreshReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	resp, err := http.Post(
		"https://prod.us-east-1.auth.desktop.kiro.dev/refreshToken",
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("刷新token请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("刷新token失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	var refreshResp RefreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&refreshResp); err != nil {
		return nil, fmt.Errorf("解析刷新响应失败: %v", err)
	}

	// 保存到文件
	newToken := TokenData(refreshResp)
	tokenPath := getTokenFilePath()
	newData, err := json.MarshalIndent(newToken, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化新token失败: %v", err)
	}

	if err := os.WriteFile(tokenPath, newData, 0600); err != nil {
		return nil, fmt.Errorf("写入token文件失败: %v", err)
	}

	return &newToken, nil
}

// InvalidateToken 使缓存的token失效
func (tm *TokenManager) InvalidateToken() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.cachedToken = nil
}
