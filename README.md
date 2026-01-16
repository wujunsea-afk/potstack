# PotStack (沙箱集成栈)

PotStack 是一个跨平台的集成后端服务（支持 Windows / Linux），旨在提供零依赖的 Git 托管、应用沙箱运行及自动编排能力。

## 核心特性

- **单进程架构 (AIO)**: 整合了路由转发、Git 引擎、沙箱管理和 Loader 编排。
- **三端口架构**: 业务端口、管理端口、内部端口分离，安全灵活。
- **Pure Go Git 引擎**: 基于 `go-git` 实现，无需安装 Git 客户端。
- **动态路由**: 支持 exe 和 static 两种沙箱类型，自动路由刷新。
- **HTTPS 自动续签**: 支持 Let's Encrypt / ZeroSSL 自动证书管理。
- **SQLite 数据库**: 轻量级数据存储。

## 快速开始

### 环境变量

| 环境变量 | 默认值 | 说明 |
|---------|--------|------|
| `POTSTACK_DATA_DIR` | `data` | 数据根目录 |
| `POTSTACK_HTTP_PORT` | `61080` | 业务端口 |
| `POTSTACK_ADMIN_PORT` | `61081` | 管理端口 |
| `POTSTACK_TOKEN` | 无 | 鉴权令牌 |

> **注意**: 内部端口固定为 `61082`，不可配置。

### 端口架构

| 端口 | 默认值 | 用途 | HTTPS |
|------|--------|------|-------|
| 业务端口 | 61080 | `/web`, `/api`, `/cdn`, `/health` | ✅ |
| 管理端口 | 61081 | `/admin`, `/health` | ✅ |
| 内部端口 | 61082 | `/pot`, `/repo`, `/refresh`, `/health` | ❌ |

### 目录结构

```
部署目录/
├── potstack.exe              # 主程序
├── https.yaml.example        # HTTPS 配置模板
├── start.bat                 # Windows 启动脚本
└── start.sh                  # Linux 启动脚本

$POTSTACK_DATA_DIR/           # 例如 ./data/
├── https.yaml                 # HTTPS 配置（首次启动自动创建）
├── certs/                     # 证书目录
│   ├── cert.pem
│   └── key.pem
├── log/
│   └── potstack.log
└── repo/                      # 仓库根目录
    ├── potstack/              # 系统仓库
    │   ├── keeper.git/
    │   ├── loader.git/
    │   └── repo.git/
    │       └── data/
    │           └── potstack.db  ← SQLite 数据库
    └── {org}/
        └── {name}.git/
            └── data/faaspot/
                ├── program/      # 代码检出
                ├── data/         # 沙箱数据
                ├── log/          # 日志目录
                └── run.yml       # 运行状态
```

## 路由说明

### 业务端口 (61080)

| 路由 | 说明 |
|------|------|
| `/cdn/*path` | 静态资源 CDN |
| `/web/:org/:name/*path` | 沙箱 Web 路由（去掉 `/{org}/{name}`，保留 `/web`） |
| `/api/:org/:name/*path` | 沙箱 API 路由（去掉 `/{org}/{name}`，保留 `/api`） |
| `/health` | 健康检查 |

### 管理端口 (61081)

| 路由 | 说明 |
|------|------|
| `/admin/:org/:name/*path` | 沙箱管理路由（去掉 `/{org}/{name}`，保留 `/admin`） |
| `/health` | 健康检查 |

### 内部端口 (61082)

| 路由 | 说明 |
|------|------|
| `/pot/:org/:name/*path` | 沙箱内部路由（去掉 `/pot/{org}/{name}`） |
| `/repo/:owner/:reponame/*` | Git Smart HTTP（无认证） |
| `/pot/potstack/router/refresh` | 刷新沙箱路由 |
| `/health` | 健康检查 |

## 沙箱配置

### pot.yml 结构

```yaml
title: "My Sandbox"
version: "1.0.0"
owner: "org-name"
potname: "sandbox-name"

# 沙箱类型：exe 或 static
type: "exe"

# static 类型专用：静态文件根目录
root: "public"

# exe 类型专用：环境变量
env:
  - name: APP_MODE
    value: "dev"
```

### 沙箱类型

| 类型 | 说明 |
|------|------|
| `exe` | 可执行程序沙箱，Keeper 管理进程生命周期 |
| `static` | 静态文件沙箱，直接从 Git 服务文件 |

### 内置环境变量（exe 类型）

| 变量 | 说明 |
|------|------|
| `DATA_PATH` | 沙箱数据目录 |
| `PROGRAM_PATH` | 程序代码目录 |
| `LOG_PATH` | 日志目录 |
| `POTSTACK_BASE_URL` | 主服务内部地址 |

### HTTPS 配置

#### 配置文件

- **模板文件**: `https.yaml.example`（与程序同目录）
- **配置文件**: `$DATA_DIR/https.yaml`（运行时使用，支持热重载）

#### 模式一：纯 HTTP（默认）

```yaml
mode: http
```

#### 模式二：手动证书

```yaml
mode: https
acme:
  enabled: false
```

将证书放到 `$DATA_DIR/certs/cert.pem` 和 `key.pem`。

#### 模式三：自动续签（HTTP-01）

需要 80 端口可访问，中国大陆需域名备案。

```yaml
mode: https
acme:
  enabled: true
  domain: git.example.com
  challenge: http-01
  http:
    port: 80
```

#### 模式四：自动续签（DNS-01，推荐）

无需开放端口，无需备案，支持内网。

```yaml
mode: https
acme:
  enabled: true
  domain: git.example.com
  challenge: dns-01
  dns:
    provider: tencentcloud
    credentials:
      secret_id: "your-secret-id"
      secret_key: "your-secret-key"
```

### 支持的 DNS 提供商

| 提供商 | provider 值 | 凭证配置 |
|--------|-------------|---------|
| 腾讯云 | `tencentcloud` | `secret_id`, `secret_key` |
| 阿里云 | `alidns` | `access_key_id`, `access_key_secret` |
| Cloudflare | `cloudflare` | `api_token` |

## 编译打包

```bash
# 编译
./build.sh

# 打包（含基础组件）
./build_base_pack.sh
```

构建产物：
- `potstack`: Linux 可执行文件
- `potstack.exe`: Windows 可执行文件
- `potstack-base.zip`: 完整部署包

## 技术文档

### 用户文档

- [API 接口文档](docs/user/API.md)
- [部署指南](docs/user/DEPLOYMENT.md)
- [HTTPS 证书管理](docs/user/HTTPS.md)

### 开发者文档

- [架构设计](docs/dev/ARCHITECTURE.md)
- [开发指南](docs/dev/DEVELOPMENT.md)
- [Loader 模块设计](docs/dev/LOADER.md)
