#!/bin/bash

# 测试请求合并功能
SERVER_URL="http://localhost:8080"

echo "🚀 测试请求合并功能"
echo "===================="

# 检查服务器是否运行
if ! curl -s "$SERVER_URL/health" > /dev/null 2>&1; then
    echo "❌ 服务器未运行，请先启动服务器: ./kiro2cc server 8080"
    exit 1
fi

echo "✅ 服务器运行正常"
echo

# 准备测试请求
create_test_request() {
    local message="$1"
    cat << EOF
{
    "model": "claude-3-sonnet-20240229",
    "max_tokens": 100,
    "messages": [
        {
            "role": "user",
            "content": "$message"
        }
    ]
}
EOF
}

# 测试1: 快速发送多个请求以触发批处理
echo "📊 测试1: 快速发送3个请求以触发批处理"
echo "============================================"

# 创建临时文件存储响应
TEMP_DIR=$(mktemp -d)
echo "临时目录: $TEMP_DIR"

# 同时发送3个请求
echo "发送请求1: 什么是人工智能?"
create_test_request "什么是人工智能?" > "$TEMP_DIR/req1.json"

echo "发送请求2: 解释机器学习"
create_test_request "解释机器学习" > "$TEMP_DIR/req2.json"

echo "发送请求3: 什么是深度学习?"
create_test_request "什么是深度学习?" > "$TEMP_DIR/req3.json"

# 快速并发发送请求
echo
echo "🚀 并发发送3个请求..."
start_time=$(date +%s.%N)

curl -s -X POST "$SERVER_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d @"$TEMP_DIR/req1.json" > "$TEMP_DIR/resp1.json" &
PID1=$!

curl -s -X POST "$SERVER_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d @"$TEMP_DIR/req2.json" > "$TEMP_DIR/resp2.json" &
PID2=$!

curl -s -X POST "$SERVER_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d @"$TEMP_DIR/req3.json" > "$TEMP_DIR/resp3.json" &
PID3=$!

# 等待所有请求完成
wait $PID1 $PID2 $PID3

end_time=$(date +%s.%N)
duration=$(echo "$end_time - $start_time" | bc -l)

echo "⏱️  总耗时: ${duration}秒"
echo

# 检查响应
echo "📋 检查响应:"
for i in 1 2 3; do
    if [ -f "$TEMP_DIR/resp$i.json" ]; then
        echo "响应$i: $(head -c 100 "$TEMP_DIR/resp$i.json")..."
        
        # 检查是否有批处理标记
        if grep -q "BATCH-HIT" "$TEMP_DIR/resp$i.json" 2>/dev/null; then
            echo "  ✅ 检测到批处理标记"
        else
            echo "  ℹ️  未检测到批处理标记"
        fi
    else
        echo "响应$i: 文件不存在"
    fi
done

echo
echo "📊 获取统计信息:"
curl -s "$SERVER_URL/stats" | jq '{
    total_requests: .metrics.total_requests,
    cache_hit_rate: .metrics.cache_hit_rate,
    avg_response_time: .metrics.avg_response_time_ms
}' 2>/dev/null || echo "无法获取统计信息"

echo
echo "🔍 获取详细分析:"
curl -s "$SERVER_URL/analytics" | jq '{
    active_users: .active_users,
    total_users: .total_users,
    cost_analysis: .cost_analysis
}' 2>/dev/null || echo "无法获取分析信息"

# 测试2: 测试不同模型的请求（应该不会合并）
echo
echo "📊 测试2: 发送不同模型的请求（不应合并）"
echo "=========================================="

create_different_model_request() {
    local message="$1"
    local model="$2"
    cat << EOF
{
    "model": "$model",
    "max_tokens": 50,
    "messages": [
        {
            "role": "user",
            "content": "$message"
        }
    ]
}
EOF
}

echo "发送不同模型的请求..."
create_different_model_request "Hello" "claude-3-sonnet-20240229" > "$TEMP_DIR/diff1.json"
create_different_model_request "Hi" "claude-3-haiku-20240307" > "$TEMP_DIR/diff2.json"

start_time=$(date +%s.%N)

curl -s -X POST "$SERVER_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d @"$TEMP_DIR/diff1.json" > "$TEMP_DIR/diff_resp1.json" &

curl -s -X POST "$SERVER_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d @"$TEMP_DIR/diff2.json" > "$TEMP_DIR/diff_resp2.json" &

wait

end_time=$(date +%s.%N)
duration=$(echo "$end_time - $start_time" | bc -l)

echo "⏱️  不同模型请求耗时: ${duration}秒"
echo "ℹ️  不同模型的请求应该单独处理，不会合并"

# 清理临时文件
rm -rf "$TEMP_DIR"

echo
echo "🎯 测试完成！"
echo "============"
echo "请查看服务器日志以确认批处理行为:"
echo "- 应该看到 '🚀 批处理: 合并 X 个请求' 的日志"
echo "- 应该看到 '✅ 批处理成功: X 个请求合并为 1 个' 的日志"
echo
echo "💡 提示:"
echo "- 批处理会在200ms内收集最多3个相同模型的请求"
echo "- 流式请求不会被批处理"
echo "- 不同模型的请求不会被合并"
