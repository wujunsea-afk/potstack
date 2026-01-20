#!/bin/bash
set -e

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# 配置
GITHUB_REPO_URL="https://github.com/wujunsea-afk/potstack.git" 
REMOTE_NAME="github"

echo -e "${GREEN}=== PotStack GitHub 发布助手 ===${NC}"
echo "此脚本将帮助您把代码推送到 GitHub 并触发 Release 编译。"
echo ""

# 1. 检查 GitHub Remote
EXISTING_URL=$(git remote get-url $REMOTE_NAME 2>/dev/null || true)

if [ -z "$EXISTING_URL" ]; then
    echo "添加远程仓库 '$REMOTE_NAME' -> $GITHUB_REPO_URL"
    git remote add $REMOTE_NAME "$GITHUB_REPO_URL"
else
    if [ "$EXISTING_URL" != "$GITHUB_REPO_URL" ]; then
        echo "更新远程仓库 '$REMOTE_NAME' -> $GITHUB_REPO_URL"
        git remote set-url $REMOTE_NAME "$GITHUB_REPO_URL"
    fi
fi
echo ""

# 2. 选择操作模式
echo -e "${YELLOW}请选择操作:${NC}"
echo "1) 仅同步代码 (不触发构建)"
echo "2) 发布新版本 (打标签并触发 Release 构建)"
read -p "请输入 [1/2]: " MODE

if [ "$MODE" != "1" ] && [ "$MODE" != "2" ]; then
    echo "无效选择，退出。"
    exit 1
fi
echo ""

# 3. 确定版本号
if [ "$MODE" == "2" ]; then
    DEFAULT_VERSION="v1.0.0"
    echo -e "${YELLOW}请输入要发布的版本号 [默认: $DEFAULT_VERSION]:${NC}"
    read -r VERSION
    VERSION=${VERSION:-$DEFAULT_VERSION}
    if [[ ! "$VERSION" =~ ^v ]]; then VERSION="v$VERSION"; fi
    echo -e "准备发布版本: ${GREEN}$VERSION${NC}"
fi

# 4. 确认执行
echo -e "${YELLOW}即将执行:${NC}"
if [ "$MODE" == "1" ]; then
    echo "1. 提交本地所有修改"
    echo "2. 推送代码到 GitHub ($GITHUB_REPO_URL)"
else
    echo "1. 提交本地所有修改"
    echo "2. 推送代码到 GitHub ($GITHUB_REPO_URL)"
    echo "3. 打标签 $VERSION 并推送 (触发 GitHub Actions)"
fi
echo ""

read -p "是否继续? (y/N) " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "操作取消。"
    exit 0
fi

# 5. 执行操作
echo ""
echo "[1/3] 提交本地修改..."
COMMIT_MSG="Update code"
[ "$MODE" == "2" ] && COMMIT_MSG="Release $VERSION: Sync to GitHub"

if [[ -n $(git status -s) ]]; then
    git add .
    git commit -m "$COMMIT_MSG" || echo "无变更需要提交"
else
    echo "工作区很干净，无需提交。"
fi

echo "[2/3] 推送代码到 GitHub..."
CURRENT_BRANCH=$(git symbolic-ref --short HEAD)
git push $REMOTE_NAME $CURRENT_BRANCH

if [ "$MODE" == "2" ]; then
    echo "[3/3] 打标签并推送..."
    if git rev-parse "$VERSION" >/dev/null 2>&1; then
        echo -e "${YELLOW}标签 $VERSION 已存在，正在更新...${NC}"
        git tag -d "$VERSION"
        git push $REMOTE_NAME --delete "$VERSION" || true
    fi

    git tag "$VERSION"
    git push $REMOTE_NAME "$VERSION"
    
    echo ""
    echo -e "${GREEN}✅ 发布指令已发送！${NC}"
    echo "请访问 GitHub 查看构建进度: ${GITHUB_REPO_URL%.git}/actions"
    echo "请确保已在 Settings -> Secrets 中配置 POTSTACK_RELEASE_KEY"
else
    echo ""
    echo -e "${GREEN}✅ 代码已同步到 GitHub！${NC}"
fi
