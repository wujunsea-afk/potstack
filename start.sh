#!/bin/bash
# PotStack Linux 启动脚本

# 1. 基础配置
export POTSTACK_DATA_DIR="./data"
export POTSTACK_HTTP_PORT="61080"

# 2. 认证令牌 (建议修改)
export POTSTACK_TOKEN="changeme"

# 3. 启动应用
echo "Starting PotStack..."
echo "Dir: $POTSTACK_DATA_DIR"
echo "Port: $POTSTACK_HTTP_PORT"

chmod +x ./potstack
./potstack
