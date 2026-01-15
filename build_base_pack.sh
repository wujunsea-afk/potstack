#!/bin/bash
# build_base_pack.sh - 打包 PotStack 组件包
#
# 用法: ./build_base_pack.sh
# 输出: potstack-base.zip
#
# 前置条件: 需要先生成密钥对
#   ./potpacker keygen -o potstack_release

set -e

# 配置
OUTPUT_DIR="dist"
PACK_NAME="potstack-base.zip"
VERSION=$(date +%Y%m%d%H%M%S)
POTPACKER="./potpacker"
KEY_NAME="potstack_release"

echo "Building PotStack Base Pack v${VERSION}..."

# 检查 potpacker 工具
if [ ! -x "$POTPACKER" ]; then
    echo "Error: potpacker not found at $POTPACKER"
    exit 1
fi

# 检查密钥是否存在（不再自动生成）
if [ ! -f "${KEY_NAME}.key" ]; then
    echo "Error: Signing key '${KEY_NAME}.key' not found!"
    echo "Please generate it first:"
    echo "  $POTPACKER keygen -o $KEY_NAME"
    exit 1
fi

# 清理并创建目录
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# 生成批量打包配置（路径相对于 batch.yaml 所在目录）
# 1. 写入头部配置
cat > "$OUTPUT_DIR/batch.yaml" << EOF
key: "../${KEY_NAME}.key"
output_dir: "."
packages:
EOF

# 2. 自动扫描 components 目录下的所有子目录
COMPONENTS_ROOT="components"
FOUND_COUNT=0

if [ -d "$COMPONENTS_ROOT" ]; then
    for dir in "$COMPONENTS_ROOT"/*; do
        if [ -d "$dir" ]; then
            # 路径相对于 dist/batch.yaml，所以需要加 ../
            # 例如: components/potstack -> ../components/potstack
            echo "  - path: \"../$dir\"" >> "$OUTPUT_DIR/batch.yaml"
            echo "Found component: $dir"
            FOUND_COUNT=$((FOUND_COUNT + 1))
        fi
    done
else
    echo "Error: Components directory '$COMPONENTS_ROOT' not found!"
    exit 1
fi

if [ "$FOUND_COUNT" -eq 0 ]; then
    echo "Error: No components found in '$COMPONENTS_ROOT'!"
    exit 1
fi

# 执行批量打包
echo "Running potpacker batch mode..."
if ! "$POTPACKER" -c "$OUTPUT_DIR/batch.yaml"; then
    echo "Error: Batch packing failed"
    exit 1
fi

# 生成 install.yml 头部
cat > "$OUTPUT_DIR/install.yml" << EOF
# PotStack Base Pack Install Manifest
# Generated: $(date -Iseconds)
version: "$VERSION"

packages:
EOF

# 动态添加所有 .ppk 文件
for ppk in "$OUTPUT_DIR"/*.ppk; do
    if [ -f "$ppk" ]; then
        filename=$(basename "$ppk")
        echo "  - $filename" >> "$OUTPUT_DIR/install.yml"
        echo "Added $filename to manifest"
    fi
done

# 复制公钥到输出目录
cp "${KEY_NAME}.pub" "$OUTPUT_DIR/"

# 创建版本文件
echo "$VERSION" > "$OUTPUT_DIR/VERSION"

# 清理临时配置
rm -f "$OUTPUT_DIR/batch.yaml"

# 打包
echo "Creating zip package..."
cd "$OUTPUT_DIR"
zip -r "../$PACK_NAME" .
cd ..

# 清理临时目录
rm -rf "$OUTPUT_DIR"

echo ""
echo "=========================================="
echo "Build completed!"
echo "Output: $PACK_NAME"
ls -lh "$PACK_NAME"
echo ""
echo "Contents:"
unzip -l "$PACK_NAME"
echo "=========================================="
