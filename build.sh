#!/bin/bash
set -e

echo "Starting build process..."

# 1. Build for Linux (Host OS - for debugging)
echo "Building for Linux (amd64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o potstack-linux main.go
echo "✅ Linux binary: potstack-linux"

echo "Building for Windows (amd64) without console for release..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui" -o potstack.exe main.go
echo "✅ Windows binary (no console): potstack.exe"

if command -v upx &> /dev/null; then
  echo "Compressing with UPX..."
  # upx --best potstack.exe
  echo "✅ UPX Compression complete"
else
  echo "⚠️ UPX not found, skipping compression."
fi

echo "Build complete!"
ls -lh potstack-linux potstack.exe
