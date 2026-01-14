# PotStack 开发指南

## 一、开发环境

### 1.1 前置要求

| 工具 | 版本要求 | 说明 |
|------|---------|------|
| Go | 1.21+ | 编程语言 |
| Git | 2.x | 版本控制 |
| Make | 可选 | 构建工具 |

### 1.2 获取代码

```bash
git clone https://github.com/your-org/potstack.git
cd potstack
```

### 1.3 安装依赖

```bash
go mod download
go mod tidy
```

---

## 二、项目结构

```
potstack/
├── main.go                      # 入口文件
├── go.mod                       # Go 模块定义
├── go.sum                       # 依赖锁定
│
├── config/                      # 配置模块
│   └── config.go                # 环境变量读取
│
├── internal/                    # 内部模块
│   ├── api/                     # API 处理器
│   │   ├── user.go              # 用户管理
│   │   ├── repo.go              # 仓库管理
│   │   ├── collaborator.go      # 协作者管理
│   │   └── models.go            # 数据模型
│   │
│   ├── auth/                    # 认证模块
│   │   └── middleware.go        # Token 认证中间件
│   │
│   ├── db/                      # 数据库模块
│   │   ├── db.go                # 连接管理
│   │   ├── user.go              # 用户 DAO
│   │   ├── repo.go              # 仓库 DAO
│   │   └── collaborator.go      # 协作者 DAO
│   │
│   ├── git/                     # Git 模块
│   │   └── http_server.go       # Git Smart HTTP
│   │
│   ├── https/                   # HTTPS 模块
│   │   ├── config.go            # HTTPS 配置
│   │   ├── manager.go           # 证书管理
│   │   ├── acme_client.go       # ACME 客户端
│   │   └── dns_provider.go      # DNS 提供商
│   │
│   ├── loader/                  # Loader 模块
│   │   └── loader.go            # 初始化逻辑
│   │
│   └── router/                  # 资源路由
│       └── processor.go         # 路由处理器
│
├── docs/                        # 文档
│   ├── API.md
│   ├── ARCHITECTURE.md
│   ├── DEPLOYMENT.md
│   └── DEVELOPMENT.md
│
├── dev/                         # 开发资料
│   ├── autocert.md              # HTTPS 技术文档
│   └── loader.md                # Loader 技术文档
│
├── build.sh                     # 编译脚本
└── build_base_pack.sh           # 打包脚本
```

---

## 三、编译构建

### 3.1 本地编译

```bash
# 编译 Linux 版本
go build -o potstack-linux .

# 编译 Windows 版本
GOOS=windows GOARCH=amd64 go build -o potstack.exe .
```

### 3.2 使用构建脚本

```bash
# 仅编译
./build.sh

# 编译并打包
./build_base_pack.sh
```

### 3.3 交叉编译

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o potstack-linux-amd64 .

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o potstack-linux-arm64 .

# Windows AMD64
GOOS=windows GOARCH=amd64 go build -o potstack-windows-amd64.exe .
```

---

## 四、本地运行

### 4.1 设置环境变量

```bash
export POTSTACK_REPO_ROOT=./testdata
export POTSTACK_HTTP_PORT=61080
export POTSTACK_TOKEN=dev-token
```

### 4.2 启动服务

```bash
go run main.go
```

### 4.3 测试 API

```bash
# 健康检查
curl http://localhost:61080/health

# 创建用户
curl -X POST http://localhost:61080/api/v1/admin/users \
  -H "Authorization: token dev-token" \
  -H "Content-Type: application/json" \
  -d '{"username": "testuser"}'

# 创建仓库
curl -X POST http://localhost:61080/api/v1/admin/users/testuser/repos \
  -H "Authorization: token dev-token" \
  -H "Content-Type: application/json" \
  -d '{"name": "testrepo"}'
```

---

## 五、代码规范

### 5.1 格式化

```bash
go fmt ./...
```

### 5.2 静态检查

```bash
go vet ./...
```

### 5.3 代码风格

- 使用 Go 官方代码风格
- 变量命名使用驼峰式
- 导出函数必须有注释
- 错误处理不使用 panic

### 5.4 目录规范

| 目录 | 用途 |
|------|------|
| `internal/` | 内部模块，不对外暴露 |
| `config/` | 全局配置 |
| `docs/` | 用户文档 |
| `dev/` | 开发技术文档 |

---

## 六、添加新功能

### 6.1 添加新 API

1. 在 `internal/api/` 创建处理函数
2. 在 `internal/db/` 添加 DAO 方法（如需要）
3. 在 `main.go` 注册路由
4. 更新 `docs/API.md`

**示例：添加用户列表接口**

```go
// internal/api/user.go
func ListUsersHandler(c *gin.Context) {
    users, err := db.ListUsers()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, users)
}
```

```go
// main.go
v1.GET("/admin/users", api.ListUsersHandler)
```

### 6.2 添加新 DNS 提供商

1. 在 `internal/https/dns_provider.go` 添加新函数
2. 在 `NewDNSProvider` switch 中注册
3. 更新 `https.yaml.example` 添加配置示例
4. 更新文档

**示例：添加 AWS Route53**

```go
// internal/https/dns_provider.go
case "route53":
    return newRoute53Provider(creds)

func newRoute53Provider(creds map[string]string) (challenge.Provider, error) {
    // 实现...
}
```

### 6.3 添加新数据表

1. 在 `internal/db/db.go` 的 `initTables()` 添加 CREATE TABLE
2. 创建新的 DAO 文件 `internal/db/xxx.go`
3. 创建新的 API 处理器

---

## 七、测试

### 7.1 单元测试

```bash
go test ./...
```

### 7.2 API 测试脚本

```bash
# Windows
test_api.bat

# Linux
./test.sh
```

### 7.3 手动测试

```bash
# 完整测试流程
TOKEN=dev-token

# 1. 创建用户
curl -X POST http://localhost:61080/api/v1/admin/users \
  -H "Authorization: token $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username": "alice"}'

# 2. 创建仓库
curl -X POST http://localhost:61080/api/v1/admin/users/alice/repos \
  -H "Authorization: token $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "myproject"}'

# 3. 添加协作者
curl -X PUT http://localhost:61080/api/v1/repos/alice/myproject/collaborators/bob \
  -H "Authorization: token $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"permission": "write"}'

# 4. 列出协作者
curl http://localhost:61080/api/v1/repos/alice/myproject/collaborators \
  -H "Authorization: token $TOKEN"

# 5. Git clone
git clone http://dev-token@localhost:61080/alice/myproject.git
```

---

## 八、调试

### 8.1 开启调试日志

```bash
export GIN_MODE=debug
go run main.go
```

### 8.2 使用 Delve 调试

```bash
# 安装
go install github.com/go-delve/delve/cmd/dlv@latest

# 调试
dlv debug .
```

### 8.3 常见问题

| 问题 | 解决方案 |
|------|---------|
| 依赖下载失败 | 设置 `GOPROXY=https://goproxy.cn,direct` |
| CGO 编译错误 | 使用 `CGO_ENABLED=0` |
| 端口冲突 | 修改 `POTSTACK_HTTP_PORT` |

---

## 九、发布流程

### 9.1 版本号规范

使用语义化版本：`vX.Y.Z`

- X: 主版本（不兼容的 API 变更）
- Y: 次版本（向后兼容的功能新增）
- Z: 修订版（向后兼容的问题修正）

### 9.2 发布步骤

```bash
# 1. 更新版本号
vim VERSION

# 2. 编译打包
./build_base_pack.sh

# 3. 创建 Git tag
git tag v1.0.0
git push origin v1.0.0

# 4. 上传发布包
# potstack-base.zip
```

---

## 十、贡献指南

### 10.1 提交规范

```
<type>: <subject>

<body>
```

**Type:**
- `feat`: 新功能
- `fix`: 修复
- `docs`: 文档
- `refactor`: 重构
- `test`: 测试
- `chore`: 杂项

**示例:**
```
feat: add collaborator API

Add Gogs-compatible collaborator management:
- List collaborators
- Add collaborator
- Remove collaborator
```

### 10.2 分支规范

| 分支 | 用途 |
|------|------|
| `main` | 稳定版本 |
| `develop` | 开发分支 |
| `feature/*` | 功能分支 |
| `fix/*` | 修复分支 |
