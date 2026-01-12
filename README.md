# PotStack (沙箱集成栈)

PotStack 是一个为 Windows 环境设计的集成后端服务，旨在提供零依赖的 Git 托管、应用沙箱运行及自动编排能力。

## 核心特性

- **单进程架构 (AIO)**: 整合了路由转发、Git 引擎、沙箱管理和 Loader 编排。
- **Pure Go Git 引擎**: 基于 `go-git` 实现，无需在 Windows 上安装 Git 客户端。
- **Gogs 兼容 API**: 完美对接现有的 Loader 和自动化脚本。

## 快速开始

### 环境变量
- `POTSTACK_TOKEN`: 管理 API 和 Git 传输所需的鉴权令牌（Basic Auth）。
- `POTSTACK_REPO_ROOT`: 仓库物理存储根目录。
- `POTSTACK_HTTP_PORT`: HTTP 服务端口（默认：61080）。
- `POTSTACK_ENABLE_HTTPS`: 是否启用 HTTPS (值为 `true` 时启用)。
- `POTSTACK_CERT_FILE`: TLS 证书文件路径 (默认为 `./cert.pem`)。
- `POTSTACK_KEY_FILE`: TLS 私钥文件路径 (默认为 `./key.pem`)。

### 启用 HTTPS (开发环境)

1. **生成本地证书** (需要 [mkcert](https://github.com/FiloSottile/mkcert))

   ```bash
   # 生成证书和私钥
   mkcert -cert-file cert.pem -key-file key.pem localhost 127.0.0.1
   ```

2. **启动 HTTPS 服务**

   ```bash
   POTSTACK_ENABLE_HTTPS=true go run main.go
   ```

### 编译运行

为了方便同时生成调试用的 Linux 版本和发布用的 Windows 版本，项目提供了统一的构建脚本：

```bash
./build.sh
```

构建产物：
- `potstack-linux`: Linux 可执行文件 (用于开发环境调试)
- `potstack.exe`: Windows 可执行文件 (用于目标环境部署)

## API 路由说明

### 管理类 (API v1)
- `POST /api/v1/admin/users`: 创建用户。
- `POST /api/v1/admin/users/:username/repos`: 创建仓库。
- `POST /api/v1/admin/users/:username/orgs`: 创建组织。
- `DELETE /api/v1/repos/:owner/:repo`: 删除仓库。

### 资源类
- `/uri/*path`: 物理资源访问及托管。
- `/cdn/*path`: 静态资源 CDN 访问。
- `/:owner/:repo.git/*`: 全量 Git Smart HTTP 支持。
