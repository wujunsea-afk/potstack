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
VERSION=$(date +%Y%m%d%H%M%S)
POTPACKER="./potpacker"
KEY_NAME="potstack_release"

# 平台选择
echo "=========================================="
echo "  PotStack Base Pack Builder"
echo "=========================================="
echo ""
echo "Select target platform:"
echo "  1) linux"
echo "  2) windows"
echo ""
read -p "Enter choice [1-2]: " choice

case $choice in
    1) PLATFORM="linux" ;;
    2) PLATFORM="windows" ;;
    *) echo "Invalid choice"; exit 1 ;;
esac

PACK_NAME="potstack-base.zip"

echo ""
echo "Building PotStack Base Pack v${VERSION} for ${PLATFORM}..."

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

# 组件处理函数
process_item() {
    local item="$1"
    if [ -d "$item" ]; then
        # 目录 → 调用 potpacker 构建
        local name=$(basename "$item")
        local output_file="$OUTPUT_DIR/${name}.ppk"
        
        # 检查重复
        if [ -f "$output_file" ]; then
            echo "Error: Duplicate PPK name: ${name}.ppk"
            exit 1
        fi
        
        echo "------------------------------------------"
        echo "Building component: $name"
        
        if ! "$POTPACKER" -p "$item" -k "${KEY_NAME}.key" -o "$output_file"; then
            echo "Error: Failed to pack '$name'"
            exit 1
        fi
        
        echo "Built: $output_file"
        FOUND_COUNT=$((FOUND_COUNT + 1))
    elif [[ "$item" == *.ppk ]]; then
        # PPK 文件 → 直接复制
        local filename=$(basename "$item")
        local output_file="$OUTPUT_DIR/$filename"
        
        # 检查重复
        if [ -f "$output_file" ]; then
            echo "Error: Duplicate PPK name: $filename"
            exit 1
        fi
        
        echo "------------------------------------------"
        echo "Copying PPK: $filename"
        
        cp "$item" "$OUTPUT_DIR/"
        
        echo "Copied: $output_file"
        FOUND_COUNT=$((FOUND_COUNT + 1))
    fi
}

COMPONENTS_ROOT="components"
FOUND_COUNT=0

# 处理平台专用目录
if [ -d "$COMPONENTS_ROOT/$PLATFORM" ]; then
    echo ""
    echo "=== Processing $PLATFORM components ==="
    for item in "$COMPONENTS_ROOT/$PLATFORM"/*; do
        [ -e "$item" ] && process_item "$item"
    done
else
    echo "Error: Components directory '$COMPONENTS_ROOT/$PLATFORM' not found!"
    exit 1
fi

if [ "$FOUND_COUNT" -eq 0 ]; then
    echo "Error: No components or PPK files found in '$COMPONENTS_ROOT/$PLATFORM'!"
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
