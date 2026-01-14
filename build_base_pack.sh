#!/bin/bash
# build_base_pack.sh - 打包 PotStack 组件包
#
# 用法: ./build_base_pack.sh
# 输出: potstack-base.zip
#
# 说明: 使用 potpacker 生成 .ppk 文件，然后打包成 zip

set -e

# 配置
OUTPUT_DIR="dist"
PACK_NAME="potstack-base.zip"
VERSION=$(date +%Y%m%d%H%M%S)
POTPACKER="./potpacker"  # potpacker 工具路径
KEY_NAME="potstack_release"

# 定义要打包的组件（可扩展）
# 格式: "源目录:输出文件名"
PACKAGES=(
    "components/potstack:potstack.ppk"
    # 后续可添加更多，如:
    # "components/myapp:myapp.ppk"
)

echo "Building PotStack Base Pack v${VERSION}..."

# 检查 potpacker 工具
if [ ! -x "$POTPACKER" ]; then
    echo "Error: potpacker not found or not executable at $POTPACKER"
    exit 1
fi

# 1. 检查/生成密钥对
if [ ! -f "${KEY_NAME}.key" ]; then
    echo "Generating signing key pair..."
    "$POTPACKER" keygen -o "$KEY_NAME"
fi

# 清理并创建目录
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# 初始化 install.yml
cat > "$OUTPUT_DIR/install.yml" << EOF
# PotStack Base Pack Install Manifest
# 生成时间: $(date -Iseconds)
version: "$VERSION"

# 需要安装的 ppk 包列表
packages:
EOF

# 生成 ppk 文件并更新 install.yml
for pkg in "${PACKAGES[@]}"; do
    SRC="${pkg%%:*}"
    OUT="${pkg##*:}"
    
    if [ -d "$SRC" ]; then
        echo "Packing $SRC -> $OUT"
        
        # 执行 potpacker 并检查结果 (添加 -k 参数)
        if ! "$POTPACKER" -p "$SRC" -o "$OUTPUT_DIR/$OUT" -k "${KEY_NAME}.key"; then
            echo "Error: Failed to pack $SRC"
            exit 1
        fi
        
        # 验证输出文件存在
        if [ ! -f "$OUTPUT_DIR/$OUT" ]; then
            echo "Error: Expected output file $OUTPUT_DIR/$OUT not found"
            exit 1
        fi
        
        # 添加到 install.yml
        echo "  - $OUT" >> "$OUTPUT_DIR/install.yml"
        
        echo "  ✓ $OUT created successfully"
    else
        echo "Warning: $SRC not found, skipping"
    fi
done

# 复制公钥到输出目录
cp "${KEY_NAME}.pub" "$OUTPUT_DIR/"

# 创建版本文件
echo "$VERSION" > "$OUTPUT_DIR/VERSION"

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
