#!/bin/bash

# 确保脚本在错误时退出
set -e

# 创建必要的目录结构
echo "创建项目目录结构..."
mkdir -p data
mkdir -p web/static/css
mkdir -p web/static/js
mkdir -p web/templates

# 检查 Go 是否安装
if ! command -v go &> /dev/null; then
    echo "错误: 未找到 Go。请安装 Go 1.24 或更高版本。"
    exit 1
fi

# 安装依赖
echo "安装依赖..."
go mod tidy

# 创建配置文件（如果不存在）
if [ ! -f config/config.yaml ]; then
    echo "创建默认配置文件..."
    cp config/config.yaml.example config/config.yaml 2>/dev/null || :
    echo "请在config/config.yaml中配置应用设置。"
fi

# 编译应用
echo "编译应用..."
go build -o shorturl

echo "设置完成！可以通过以下命令启动应用:"
echo "  ./shorturl"
echo "或者:"
echo "  go run main.go"
echo ""
echo "默认地址: http://localhost:8080"
