#!/bin/bash

# AWS API 优化版本构建脚本

echo "=== AWS API 优化版本构建脚本 ==="
echo

# 检查Go版本
echo "检查Go版本..."
go version

echo
echo "=== 构建选项 ==="
echo "1. 完整优化版本 (推荐)"
echo "2. 基础版本 (仅main.go)"
echo "3. Windows版本"
echo "4. Linux版本"
echo "5. 清理构建文件"
echo

read -p "请选择构建选项 (1-5): " choice

case $choice in
    1)
        echo "构建完整优化版本..."
        if go build -o kiro2cc *.go; then
            echo "✓ 构建成功: kiro2cc"
            echo "包含所有优化功能:"
            echo "  - 多层缓存架构"
            echo "  - 智能预测缓存"
            echo "  - 上下文压缩"
            echo "  - 请求去重"
            echo "  - 性能监控"
        else
            echo "✗ 构建失败"
            echo "尝试修复编译问题..."
            echo "如果问题持续，请使用选项2构建基础版本"
        fi
        ;;
    2)
        echo "构建基础版本..."
        if go build -o kiro2cc_basic main.go parser/sse_parser.go; then
            echo "✓ 构建成功: kiro2cc_basic"
            echo "包含基础功能，无高级优化"
        else
            echo "✗ 构建失败"
        fi
        ;;
    3)
        echo "构建Windows版本..."
        if GOOS=windows GOARCH=amd64 go build -o kiro2cc.exe *.go; then
            echo "✓ 构建成功: kiro2cc.exe (Windows)"
        else
            echo "✗ 构建失败，尝试基础版本..."
            GOOS=windows GOARCH=amd64 go build -o kiro2cc_basic.exe main.go parser/sse_parser.go
        fi
        ;;
    4)
        echo "构建Linux版本..."
        if GOOS=linux GOARCH=amd64 go build -o kiro2cc_linux *.go; then
            echo "✓ 构建成功: kiro2cc_linux"
        else
            echo "✗ 构建失败，尝试基础版本..."
            GOOS=linux GOARCH=amd64 go build -o kiro2cc_basic_linux main.go parser/sse_parser.go
        fi
        ;;
    5)
        echo "清理构建文件..."
        rm -f kiro2cc kiro2cc.exe kiro2cc_linux kiro2cc_basic kiro2cc_basic.exe kiro2cc_basic_linux
        echo "✓ 清理完成"
        ;;
    *)
        echo "无效选项"
        exit 1
        ;;
esac

echo
echo "=== 构建完成 ==="

# 显示构建的文件
echo "构建的文件:"
ls -la kiro2cc* 2>/dev/null || echo "没有找到构建文件"

echo
echo "=== 使用说明 ==="
echo "启动服务器:"
echo "  ./kiro2cc server 8080"
echo
echo "监控优化效果:"
echo "  curl http://localhost:8080/stats"
echo "  curl http://localhost:8080/stats/detailed"
echo
echo "运行测试:"
echo "  ./test_optimizations.sh"
