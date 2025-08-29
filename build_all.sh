#!/bin/bash

# 设置Go交叉编译环境
export CGO_ENABLED=0

# 定义目标平台
PLATFORMS=("darwin/amd64" "darwin/arm64" "linux/amd64" "linux/arm64")

# 创建用于存放编译结果的目录
mkdir -p builds

# 循环编译
for platform in "${PLATFORMS[@]}"
do
    # 分割平台字符串为操作系统和架构
    GOOS=${platform%/*}
    GOARCH=${platform#*/}
    
    # 定义输出文件名
    OUTPUT_NAME="builds/kiro2cc-${GOOS}-${GOARCH}"
    
    # 编译应用
    echo "正在为 ${GOOS}/${GOARCH} 平台编译..."
    env GOOS=$GOOS GOARCH=$GOARCH go build -o $OUTPUT_NAME cmd/kiro2cc/main.go
    
    # 检查编译是否成功
    if [ $? -ne 0 ]; then
        echo "为 ${GOOS}/${GOARCH} 平台编译时发生错误"
        exit 1
    fi
done

echo "所有平台的二进制文件已成功生成。"
echo "文件列表:"
ls -l builds/
