# Kiro2cc API 文档

## 概述

Kiro2cc 是一个高性能的 Anthropic API 代理服务器，提供了多层优化和详细的监控功能。

## 核心端点

### 1. 主要API端点

#### POST /v1/messages
代理 Anthropic Claude API 请求

**请求格式**:
```json
{
  "model": "claude-sonnet-4-20250514",
  "max_tokens": 1000,
  "messages": [
    {
      "role": "user",
      "content": "Hello, how are you?"
    }
  ],
  "stream": false
}
```

**响应格式**:
```json
{
  "content": [
    {
      "type": "text",
      "text": "Hello! I'm doing well, thank you for asking."
    }
  ],
  "model": "claude-sonnet-4-20250514",
  "role": "assistant",
  "stop_reason": "end_turn",
  "type": "message",
  "usage": {
    "input_tokens": 12,
    "output_tokens": 15
  }
}
```

**支持的模型**:
- `claude-sonnet-4-20250514`
- `claude-3-5-haiku-20241022`

### 2. 监控端点

#### GET /health
健康检查端点

**响应**: `OK`

#### GET /stats
获取基础统计信息

**响应示例**:
```json
{
  "basic_cache": {
    "cache_size": 150,
    "max_size": 1000,
    "ttl_minutes": 10
  },
  "predictive_cache": {
    "total_cache_entries": 45,
    "learned_patterns": 12
  },
  "context_compressor": {
    "compression_cache_size": 25,
    "total_compressions": 89,
    "avg_compression_ratio": 0.65
  },
  "request_deduplicator": {
    "active_requests": 3,
    "total_merges": 67
  },
  "metrics": {
    "total_requests": 1250,
    "cached_requests": 875,
    "avg_response_time_ms": 180,
    "cache_hit_rate": 70.0,
    "error_rate": 0.8
  }
}
```

#### GET /stats/detailed
获取详细统计信息

**响应包含**:
- `optimization_summary`: 优化效果总结
- `cache_layers`: 各层缓存效果
- `optimizations`: 各种优化策略效果
- `performance`: 性能指标

#### GET /config
获取当前配置信息

#### POST /optimize/cleanup
手动触发缓存清理

### 3. 高级功能端点

#### GET /analytics
获取高级分析报告

**响应包含**:
- 请求模式分析
- 用户行为分析
- 成本分析
- 性能指标

#### GET /recommendations
获取基于数据的优化建议

#### GET /rate-limit/stats
获取速率限制统计信息

#### GET /circuit-breaker/status
获取熔断器状态

#### POST /circuit-breaker/reset
重置熔断器状态

## 优化功能

### 1. 多层缓存
- **基础缓存**: LRU算法，TTL过期
- **预测缓存**: 基于模式学习的预测预取
- **去重缓存**: 相同请求自动合并

### 2. 上下文压缩
- 自动压缩长对话上下文
- 智能保留重要消息
- 生成跳过消息的摘要

### 3. 请求优化
- Token自动管理和异步刷新
- HTTP连接池复用
- 请求批处理和去重

### 4. 可靠性保障
- 智能速率限制
- 熔断器保护
- 详细的错误处理

## 性能指标

### 预期性能提升
- **API调用减少**: 60-85%
- **响应时间提升**: 70-90%
- **并发处理能力**: 提升10-20倍
- **上下文传输减少**: 40-60%

### 监控指标
- 总请求数和缓存命中率
- 平均响应时间和错误率
- 各层缓存效果
- 压缩和去重效果

## 使用示例

### 基础使用
```bash
# 启动服务器
./kiro2cc server 8080

# 发送请求
curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "max_tokens": 1000,
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

### 监控使用
```bash
# 查看统计信息
curl http://localhost:8080/stats

# 查看详细统计
curl http://localhost:8080/stats/detailed

# 获取优化建议
curl http://localhost:8080/recommendations
```

## 错误处理

### 常见错误码
- `400`: 请求格式错误或缺少必需字段
- `403`: Token过期或无效
- `429`: 触发速率限制
- `500`: 服务器内部错误
- `503`: 熔断器开启，服务暂时不可用

### 错误响应格式
```json
{
  "message": "错误描述",
  "availableModels": ["claude-sonnet-4-20250514", "claude-3-5-haiku-20241022"]
}
```
