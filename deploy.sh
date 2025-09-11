#!/bin/bash

# Kiro2cc 部署脚本
set -e

echo "🚀 开始部署 Kiro2cc..."

# 检查Go环境
if ! command -v go &> /dev/null; then
    echo "❌ Go 未安装，请先安装 Go 1.21+"
    exit 1
fi

# 检查Go版本
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
REQUIRED_VERSION="1.21"

if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
    echo "❌ Go 版本过低，需要 $REQUIRED_VERSION+，当前版本: $GO_VERSION"
    exit 1
fi

echo "✅ Go 版本检查通过: $GO_VERSION"

# 清理旧的构建文件
echo "🧹 清理旧的构建文件..."
rm -f kiro2cc kiro2cc.exe

# 下载依赖
echo "📦 下载依赖..."
go mod tidy

# 运行测试
echo "🧪 运行测试..."
go test ./... -v

# 运行基准测试
echo "⚡ 运行性能测试..."
go test -bench=. -benchmem

# 构建应用
echo "🔨 构建应用..."
CGO_ENABLED=0 go build -ldflags="-w -s" -o kiro2cc .

# 检查构建结果
if [ ! -f "kiro2cc" ]; then
    echo "❌ 构建失败"
    exit 1
fi

echo "✅ 构建成功"

# 创建配置文件示例
echo "📝 创建配置文件示例..."
cat > kiro2cc-config.example.json << EOF
{
  "http_client": {
    "max_idle_conns": 100,
    "max_idle_conns_per_host": 10,
    "idle_conn_timeout": "90s",
    "request_timeout": "30s",
    "streaming_timeout": "300s"
  },
  "cache": {
    "max_size": 1000,
    "ttl": "10m",
    "cleanup_interval": "5m"
  },
  "batch": {
    "size": 5,
    "timeout": "100ms",
    "enabled": true
  },
  "token": {
    "cache_timeout": "5m",
    "refresh_threshold": "10m"
  }
}
EOF

# 创建systemd服务文件
echo "🔧 创建 systemd 服务文件..."
cat > kiro2cc.service << EOF
[Unit]
Description=Kiro2cc Anthropic API Proxy
After=network.target

[Service]
Type=simple
User=kiro2cc
Group=kiro2cc
WorkingDirectory=/opt/kiro2cc
ExecStart=/opt/kiro2cc/kiro2cc server 8080
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# 环境变量
Environment=GOMAXPROCS=4

# 资源限制
LimitNOFILE=65536
LimitNPROC=32768

[Install]
WantedBy=multi-user.target
EOF

# 创建安装脚本
echo "📋 创建安装脚本..."
cat > install.sh << 'EOF'
#!/bin/bash
set -e

echo "🔧 安装 Kiro2cc..."

# 创建用户和组
if ! id "kiro2cc" &>/dev/null; then
    sudo useradd -r -s /bin/false kiro2cc
    echo "✅ 创建用户 kiro2cc"
fi

# 创建目录
sudo mkdir -p /opt/kiro2cc
sudo mkdir -p /var/log/kiro2cc

# 复制文件
sudo cp kiro2cc /opt/kiro2cc/
sudo cp kiro2cc-config.example.json /opt/kiro2cc/
sudo cp kiro2cc.service /etc/systemd/system/

# 设置权限
sudo chown -R kiro2cc:kiro2cc /opt/kiro2cc
sudo chown -R kiro2cc:kiro2cc /var/log/kiro2cc
sudo chmod +x /opt/kiro2cc/kiro2cc

# 重新加载 systemd
sudo systemctl daemon-reload

echo "✅ 安装完成"
echo ""
echo "使用方法:"
echo "  sudo systemctl start kiro2cc    # 启动服务"
echo "  sudo systemctl enable kiro2cc   # 开机自启"
echo "  sudo systemctl status kiro2cc   # 查看状态"
echo "  sudo journalctl -u kiro2cc -f   # 查看日志"
EOF

chmod +x install.sh

# 创建监控脚本
echo "📊 创建监控脚本..."
cat > monitor.sh << 'EOF'
#!/bin/bash

# Kiro2cc 监控脚本
ENDPOINT="http://localhost:8080"

echo "🔍 Kiro2cc 服务监控"
echo "===================="

# 检查服务状态
echo "📡 检查服务状态..."
if curl -s "$ENDPOINT/health" > /dev/null; then
    echo "✅ 服务正常运行"
else
    echo "❌ 服务不可用"
    exit 1
fi

# 获取统计信息
echo ""
echo "📊 性能统计:"
curl -s "$ENDPOINT/stats" | jq '
{
  "总请求数": .metrics.total_requests,
  "缓存命中率": (.metrics.cache_hit_rate | tostring + "%"),
  "平均响应时间": (.metrics.avg_response_time_ms | tostring + "ms"),
  "错误率": (.metrics.error_rate | tostring + "%"),
  "缓存大小": .basic_cache.cache_size
}'

# 获取详细统计
echo ""
echo "🎯 优化效果:"
curl -s "$ENDPOINT/stats/detailed" | jq '.optimization_summary'

echo ""
echo "💡 优化建议:"
curl -s "$ENDPOINT/recommendations" | jq '.recommendations[]'
EOF

chmod +x monitor.sh

echo ""
echo "🎉 部署完成！"
echo ""
echo "文件列表:"
echo "  kiro2cc                    - 主程序"
echo "  kiro2cc-config.example.json - 配置文件示例"
echo "  kiro2cc.service            - systemd 服务文件"
echo "  install.sh                 - 安装脚本"
echo "  monitor.sh                 - 监控脚本"
echo ""
echo "下一步:"
echo "  1. 运行 ./install.sh 安装到系统"
echo "  2. 配置 Kiro token"
echo "  3. 启动服务: sudo systemctl start kiro2cc"
echo "  4. 使用 ./monitor.sh 监控服务状态"
