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

# 检查密钥是否存在，不存在则自动生成
if [ ! -f "${KEY_NAME}.key" ]; then
    echo "Signing key '${KEY_NAME}.key' not found. Generating new key..."
    "$POTPACKER" keygen -o "$KEY_NAME"
    if [ $? -ne 0 ]; then
        echo "Error: Failed to generate key"
        exit 1
    fi
    echo "Key generated: ${KEY_NAME}.key"
fi

# 清理并创建目录
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# 遍历 components 目录逐个打包
COMPONENTS_ROOT="components"
FOUND_COUNT=0

if [ -d "$COMPONENTS_ROOT" ]; then
    for dir in "$COMPONENTS_ROOT"/*; do
        if [ -d "$dir" ]; then
            owner=$(basename "$dir")
            echo "------------------------------------------"
            echo "Packing component owner: $owner"
            
            # 使用新版命令：单路径扫描打包
            # -p: 扫描路径 (例如 components/potstack)
            # -k: 密钥
            # -o: 输出文件 (例如 dist/potstack.ppk)
            output_file="$OUTPUT_DIR/${owner}.ppk"
            
            if ! "$POTPACKER" -p "$dir" -k "${KEY_NAME}.key" -o "$output_file"; then
                echo "Error: Failed to pack '$owner'"
                exit 1
            fi
            
            echo "Packed: $output_file"
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

# 创建版本文件
echo "$VERSION" > "$OUTPUT_DIR/VERSION"

# 清理临时配置


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
