#!/bin/bash

# AWS API 请求优化测试脚本

echo "=== AWS API 请求优化测试 ==="
echo

# 检查服务器是否运行
SERVER_URL="http://localhost:8080"
if ! curl -s "$SERVER_URL/health" > /dev/null; then
    echo "错误: 服务器未运行，请先启动服务器"
    echo "运行: ./kiro2cc server 8080"
    exit 1
fi

echo "✓ 服务器运行正常"

# 测试基础功能
echo
echo "=== 测试基础功能 ==="

# 健康检查
echo -n "健康检查: "
if curl -s "$SERVER_URL/health" | grep -q "OK"; then
    echo "✓ 通过"
else
    echo "✗ 失败"
fi

# 配置信息
echo -n "配置信息: "
if curl -s "$SERVER_URL/config" | jq . > /dev/null 2>&1; then
    echo "✓ 可访问"
else
    echo "✗ 失败"
fi

# 基础统计
echo -n "基础统计: "
if curl -s "$SERVER_URL/stats" | jq . > /dev/null 2>&1; then
    echo "✓ 可访问"
else
    echo "✗ 失败"
fi

# 详细统计
echo -n "详细统计: "
if curl -s "$SERVER_URL/stats/detailed" | jq . > /dev/null 2>&1; then
    echo "✓ 可访问"
else
    echo "✗ 失败"
fi

# 测试API请求优化
echo
echo "=== 测试API请求优化 ==="

# 准备测试请求
TEST_REQUEST='{
  "model": "claude-sonnet-4-20250514",
  "messages": [
    {
      "role": "user",
      "content": "Hello, this is a test message for optimization testing."
    }
  ],
  "max_tokens": 100
}'

# 发送第一个请求（应该是MISS）
echo "发送第一个测试请求..."
RESPONSE1=$(curl -s -X POST "$SERVER_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -d "$TEST_REQUEST" \
  -w "HTTPSTATUS:%{http_code};HEADERS:%{header_json}")

HTTP_CODE1=$(echo "$RESPONSE1" | grep -o "HTTPSTATUS:[0-9]*" | cut -d: -f2)
HEADERS1=$(echo "$RESPONSE1" | grep -o "HEADERS:{.*}" | cut -d: -f2-)

echo "第一个请求状态码: $HTTP_CODE1"
if echo "$HEADERS1" | jq -r '.["x-cache"]' 2>/dev/null | grep -q "MISS"; then
    echo "✓ 第一个请求正确标记为MISS"
else
    echo "? 第一个请求缓存状态未知"
fi

# 等待一秒
sleep 1

# 发送相同请求（应该是HIT）
echo "发送相同的测试请求..."
RESPONSE2=$(curl -s -X POST "$SERVER_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -d "$TEST_REQUEST" \
  -w "HTTPSTATUS:%{http_code};HEADERS:%{header_json}")

HTTP_CODE2=$(echo "$RESPONSE2" | grep -o "HTTPSTATUS:[0-9]*" | cut -d: -f2)
HEADERS2=$(echo "$RESPONSE2" | grep -o "HEADERS:{.*}" | cut -d: -f2-)

echo "第二个请求状态码: $HTTP_CODE2"
if echo "$HEADERS2" | jq -r '.["x-cache"]' 2>/dev/null | grep -q "HIT"; then
    echo "✓ 第二个请求正确从缓存返回"
else
    echo "? 第二个请求缓存状态: $(echo "$HEADERS2" | jq -r '.["x-cache"]' 2>/dev/null || echo "未知")"
fi

# 测试上下文压缩
echo
echo "=== 测试上下文压缩 ==="

LONG_CONTEXT_REQUEST='{
  "model": "claude-sonnet-4-20250514",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "This is a very long conversation that should trigger context compression. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat."
    },
    {
      "role": "assistant",
      "content": "I understand you are testing context compression with a long message."
    },
    {
      "role": "user",
      "content": "Yes, and here is another long message to make the context even longer. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum."
    },
    {
      "role": "assistant",
      "content": "I see you are continuing to test the context compression functionality."
    },
    {
      "role": "user",
      "content": "Final test message to trigger compression."
    }
  ],
  "max_tokens": 50
}'

echo "发送长上下文请求..."
LONG_RESPONSE=$(curl -s -X POST "$SERVER_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -d "$LONG_CONTEXT_REQUEST" \
  -w "HTTPSTATUS:%{http_code}")

LONG_HTTP_CODE=$(echo "$LONG_RESPONSE" | grep -o "HTTPSTATUS:[0-9]*" | cut -d: -f2)
echo "长上下文请求状态码: $LONG_HTTP_CODE"

# 测试相似请求合并
echo
echo "=== 测试相似请求合并 ==="

SIMILAR_REQUEST1='{
  "model": "claude-sonnet-4-20250514",
  "messages": [
    {
      "role": "user",
      "content": "What is the weather like today?"
    }
  ],
  "max_tokens": 50
}'

SIMILAR_REQUEST2='{
  "model": "claude-sonnet-4-20250514",
  "messages": [
    {
      "role": "user",
      "content": "What is the weather like today in the city?"
    }
  ],
  "max_tokens": 50
}'

echo "发送相似请求1..."
curl -s -X POST "$SERVER_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -d "$SIMILAR_REQUEST1" > /dev/null &

echo "发送相似请求2..."
curl -s -X POST "$SERVER_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -d "$SIMILAR_REQUEST2" > /dev/null &

wait

echo "✓ 相似请求测试完成"

# 获取最终统计信息
echo
echo "=== 最终统计信息 ==="

echo "基础统计:"
curl -s "$SERVER_URL/stats" | jq '{
  total_requests: .metrics.total_requests,
  cache_hit_rate: .metrics.cache_hit_rate,
  avg_response_time: .metrics.avg_response_time_ms
}' 2>/dev/null || echo "无法获取基础统计"

echo
echo "优化效果:"
curl -s "$SERVER_URL/stats/detailed" | jq '.optimization_summary' 2>/dev/null || echo "无法获取优化统计"

echo
echo "缓存层统计:"
curl -s "$SERVER_URL/stats/detailed" | jq '.cache_layers' 2>/dev/null || echo "无法获取缓存统计"

echo
echo "=== 高级功能测试 ==="

# 测试高级分析
echo -n "高级分析: "
if curl -s "$SERVER_URL/analytics" | jq . > /dev/null 2>&1; then
    echo "✓ 可访问"
else
    echo "✗ 失败"
fi

# 测试优化建议
echo -n "优化建议: "
if curl -s "$SERVER_URL/recommendations" | jq . > /dev/null 2>&1; then
    echo "✓ 可访问"
else
    echo "✗ 失败"
fi

# 测试速率限制统计
echo -n "速率限制统计: "
if curl -s "$SERVER_URL/rate-limit/stats" | jq . > /dev/null 2>&1; then
    echo "✓ 可访问"
else
    echo "✗ 失败"
fi

# 测试熔断器状态
echo -n "熔断器状态: "
if curl -s "$SERVER_URL/circuit-breaker/status" | jq . > /dev/null 2>&1; then
    echo "✓ 可访问"
else
    echo "✗ 失败"
fi

echo
echo "高级分析报告:"
curl -s "$SERVER_URL/analytics" | jq '{
  active_users: .active_users,
  total_users: .total_users,
  uptime_hours: .uptime_hours,
  cost_analysis: .cost_analysis
}' 2>/dev/null || echo "无法获取分析报告"

echo
echo "优化建议:"
curl -s "$SERVER_URL/recommendations" | jq '.recommendations[]' 2>/dev/null || echo "无法获取建议"

# 清理测试
echo
echo "=== 清理测试 ==="
echo "执行缓存清理..."
CLEANUP_RESPONSE=$(curl -s -X POST "$SERVER_URL/optimize/cleanup")
if echo "$CLEANUP_RESPONSE" | jq -r '.status' 2>/dev/null | grep -q "cleanup completed"; then
    echo "✓ 缓存清理成功"
else
    echo "? 缓存清理状态未知"
fi

echo
echo "=== 测试完成 ==="
echo "请查看上述结果以验证优化功能是否正常工作"
echo
echo "持续监控命令:"
echo "  watch -n 5 'curl -s http://localhost:8080/stats | jq .metrics'"
echo "  curl -s http://localhost:8080/stats/detailed | jq .optimization_summary"
