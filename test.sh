#!/bin/bash
set -e

echo "Starting tests..."

echo "1. Testing API Layer (User/Repo Management)..."
go test ./internal/api/... -v

echo "✅ All tests passed!"
