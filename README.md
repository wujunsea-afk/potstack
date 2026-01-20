# PotStack

[![Release](https://img.shields.io/github/v/release/wujunsea-afk/potstack)](https://github.com/wujunsea-afk/potstack/releases)
[![Build Status](https://img.shields.io/github/actions/workflow/status/wujunsea-afk/potstack/release.yml)](https://github.com/wujunsea-afk/potstack/actions)
[![License](https://img.shields.io/github/license/wujunsea-afk/potstack)](LICENSE)

PotStack 是一个跨平台的集成后端服务（支持 Windows / Linux），旨在提供零依赖的 Git 托管、应用沙箱运行及自动编排能力。

## 核心特性

- **单进程架构 (AIO)**: 整合了路由转发、Git 引擎、沙箱管理和 Loader 编排。
- **三端口架构**: 业务端口、管理端口、内部端口分离，安全灵活。
- **Pure Go Git 引擎**: 基于 `go-git` 实现，无需安装 Git 客户端。
- **动态路由**: 支持 exe 和 static 两种沙箱类型，自动路由刷新。
- **HTTPS 自动续签**: 支持 Let's Encrypt / ZeroSSL 自动证书管理（HTTP-01 / DNS-01）。
- **Docker 集成**: 支持拉取 Docker 镜像作为沙箱运行环境。

## 快速开始

### 1. 下载安装

前往 [GitHub Releases](https://github.com/wujunsea-afk/potstack/releases) 下载对应平台的发布包：

- **Linux**: `potstack-vX.Y.Z-linux-amd64.tar.gz`
- **Windows**: `potstack-vX.Y.Z-windows-amd64.zip`

### 2. 部署

解压后，目录包含以下核心文件：
- `potstack` (或 `potstack.exe`)
- `potstack-base.zip`: 包含对应平台的基础组件
- `https.yaml.example`: HTTPS 配置示例

设置数据目录并启动：

```bash
export POTSTACK_DATA_DIR="./data"
export POTSTACK_HTTP_PORT=61080

./potstack
```

首次启动会自动解压 `potstack-base.zip` 并初始化系统仓库。

### 3. 配置说明

| 环境变量 | 默认值 | 说明 |
|---------|--------|------|
| `POTSTACK_DATA_DIR` | `data` | 数据根目录 |
| `POTSTACK_HTTP_PORT` | `61080` | 业务端口 (HTTPS/HTTP) |
| `POTSTACK_ADMIN_PORT` | `61081` | 管理端口 (HTTPS/HTTP) |
| `POTSTACK_TOKEN` | 无 | 系统鉴权令牌 |

> 内部端口默认为 `61082`。

## 库引用 (Go Library)

PotStack 设计为可嵌入的 Go 库。

### 安装

```bash
go get github.com/wujunsea-afk/potstack
```

> **注意**: 由于模块名称仍为 `potstack`，需要在 `go.mod` 中添加 replace 指令：

```go
require (
    potstack v0.0.0
)

replace potstack => github.com/wujunsea-afk/potstack v1.0.0
```

### 使用示例

```go
package main

import "potstack"

func main() {
    // 使用默认配置启动
    potstack.Run()
}
```

## 开发与构建

### 自动化发布

本项目包含自动化发布脚本，可一键发布到 GitHub：

```bash
# 需要配置 POTSTACK_RELEASE_KEY Secret
./deploy_to_github.sh
```

此脚本会自动：
1. 提交代码并推送到 GitHub
2. 打 `vX.Y.Z` 标签
3. 触发 GitHub Actions 进行跨平台构建和发布

### 手动构建

**构建二进制**:
```bash
go build -o potstack .
```

**构建基础包 (Base Pack)**:
```bash
# Linux
echo "1" | ./build_base_pack.sh

# Windows
echo "2" | ./build_base_pack.sh
```

## 文档资源

- [API 接口文档](docs/user/API.md)
- [部署指南](docs/user/DEPLOYMENT.md)
- [HTTPS 证书管理](docs/user/HTTPS.md)
- [Loader 设计文档](docs/dev/LOADER.md)

## 许可证

[Apache 2.0](LICENSE)
