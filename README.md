# PotStack (沙箱集成栈)

PotStack 是一个跨平台的集成后端服务（支持 Windows / Linux），旨在提供零依赖的 Git 托管、应用沙箱运行及自动编排能力。

## 核心特性

- **单进程架构 (AIO)**: 整合了路由转发、Git 引擎、沙箱管理和 Loader 编排。
- **Pure Go Git 引擎**: 基于 `go-git` 实现，无需在 Windows 上安装 Git 客户端。
- **Gogs 兼容 API**: 完美对接现有的 Loader 和自动化脚本。
- **HTTPS 自动续签**: 支持 Let's Encrypt / ZeroSSL 自动证书管理（HTTP-01 / DNS-01）。
- **SQLite 数据库**: 轻量级数据存储，支持用户、仓库和协作者管理。

## 快速开始

### 环境变量

| 环境变量 | 默认值 | 说明 |
|---------|--------|------|
| `POTSTACK_REPO_ROOT` | `data` | 数据根目录 |
| `POTSTACK_HTTP_PORT` | `61080` | HTTP/HTTPS 服务端口 |
| `POTSTACK_TOKEN` | 无 | 鉴权令牌（必填） |

### 目录结构

```
部署目录/
├── potstack.exe              # 主程序
├── https.yaml.example        # HTTPS 配置模板
├── start.bat                 # Windows 启动脚本
└── start.sh                  # Linux 启动脚本

$REPO_ROOT/                    # 例如 ./data/
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
    └── user1/
        └── myrepo.git/
```

### HTTPS 配置

#### 配置文件

- **模板文件**: `https.yaml.example`（与程序同目录）
- **配置文件**: `$REPO_ROOT/https.yaml`（运行时使用，支持热重载）

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

将证书放到 `$REPO_ROOT/certs/cert.pem` 和 `key.pem`。

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

### 编译打包

```bash
# 编译
./build.sh

# 打包（含基础组件）
./build_base_pack.sh
```

构建产物：
- `potstack-linux`: Linux 可执行文件
- `potstack.exe`: Windows 可执行文件
- `potstack-base.zip`: 完整部署包

## API 路由说明

### 用户管理

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/admin/users` | 创建用户 |
| DELETE | `/api/v1/admin/users/:username` | 删除用户 |

### 仓库管理

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/admin/users/:username/repos` | 创建仓库 |
| GET | `/api/v1/repos/:owner/:repo` | 获取仓库信息 |
| DELETE | `/api/v1/repos/:owner/:repo` | 删除仓库 |

### 协作者管理（Gogs 兼容）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/repos/:owner/:repo/collaborators` | 列出协作者 |
| GET | `/api/v1/repos/:owner/:repo/collaborators/:collaborator` | 判断是否为协作者 |
| PUT | `/api/v1/repos/:owner/:repo/collaborators/:collaborator` | 添加协作者 |
| DELETE | `/api/v1/repos/:owner/:repo/collaborators/:collaborator` | 移除协作者 |

### 资源路由

| 路由 | 说明 |
|------|------|
| `/uri/*path` | 物理资源访问 |
| `/cdn/*path` | 静态资源 CDN |
| `/web/*path` | Web 路径映射 |
| `/:owner/:repo.git/*` | Git Smart HTTP |
| `/health` | 健康检查 |

## 技术文档

### 用户文档

- [API 接口文档](docs/user/API.md)
- [部署指南](docs/user/DEPLOYMENT.md)
- [HTTPS 证书管理](docs/user/HTTPS.md)

### 开发者文档

- [架构设计](docs/dev/ARCHITECTURE.md)
- [开发指南](docs/dev/DEVELOPMENT.md)
- [Loader 模块设计](docs/dev/LOADER.md)
