#!/bin/bash
set -e

echo "Starting tests..."

echo "1. Testing API Layer (User/Repo Management)..."
go test ./internal/api/... -v

echo ""
echo "2. Testing Loader (Key Pinning & Docker)..."
go test ./internal/loader/... -v 

echo ""
echo "3. Testing Docker Client..."
go test ./internal/docker/... -v

echo ""
echo "✅ All tests passed!"
