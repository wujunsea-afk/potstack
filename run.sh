#!/bin/bash
# 测试启动脚本

set -e

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== PotStack 测试启动 ===${NC}"

# 创建临时数据目录
export POTSTACK_REPO_ROOT="./data"
export POTSTACK_HTTP_PORT="61080"
export POTSTACK_TOKEN="test-token-123"

echo -e "${YELLOW}配置:${NC}"
echo "  POTSTACK_REPO_ROOT: $POTSTACK_REPO_ROOT"
echo "  POTSTACK_HTTP_PORT: $POTSTACK_HTTP_PORT"
echo "  POTSTACK_TOKEN: $POTSTACK_TOKEN"
echo ""

# 清理旧数据（可选）
# if [ -d "$POTSTACK_REPO_ROOT" ]; then
#     echo -e "${YELLOW}清理旧测试数据...${NC}"
#     rm -rf "$POTSTACK_REPO_ROOT"
# fi

# 编译
echo -e "${YELLOW}编译中...${NC}"
go build -o potstack-linux .

echo -e "${GREEN}启动 PotStack...${NC}"
echo ""
echo -e "${YELLOW}按 Ctrl+C 停止服务${NC}"
echo ""

# 启动服务
./potstack-linux