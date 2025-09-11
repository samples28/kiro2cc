package parser

import (
	"testing"
)

func TestParseCodeWhispererEvents(t *testing.T) {
	// 创建一个简单的测试数据
	testData := []byte{}

	events := ParseEvents(testData)

	// 验证空数据返回空事件列表
	if len(events) != 0 {
		t.Errorf("Expected 0 events for empty data, got %d", len(events))
	}

	t.Log("Parser test passed - empty data handled correctly")
}
