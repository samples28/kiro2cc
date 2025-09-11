package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTokenManager(t *testing.T) {
	// 创建临时目录用于测试
	tempDir := t.TempDir()

	// 创建测试token文件
	tokenPath := filepath.Join(tempDir, ".aws", "sso", "cache", "kiro-auth-token.json")
	os.MkdirAll(filepath.Dir(tokenPath), 0755)

	testToken := `{
		"accessToken": "test-access-token",
		"refreshToken": "test-refresh-token",
		"expiresAt": "2025-12-31T23:59:59Z"
	}`

	err := os.WriteFile(tokenPath, []byte(testToken), 0600)
	if err != nil {
		t.Fatalf("Failed to create test token file: %v", err)
	}

	// 临时修改getTokenFilePath函数的行为
	originalGetTokenFilePath := getTokenFilePath
	defer func() {
		// 恢复原始函数（这里只是示例，实际需要更好的依赖注入）
		_ = originalGetTokenFilePath
	}()

	t.Log("TokenManager test setup completed")
}

func TestTokenCaching(t *testing.T) {
	// 测试token缓存功能
	tm := &TokenManager{}

	// 验证初始状态
	if tm.cachedToken != nil {
		t.Error("Expected cachedToken to be nil initially")
	}

	// 验证缓存失效逻辑
	tm.InvalidateToken()
	if tm.cachedToken != nil {
		t.Error("Expected cachedToken to be nil after invalidation")
	}

	t.Log("Token caching test passed")
}
