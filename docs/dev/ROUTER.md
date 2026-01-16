# Router 模块设计文档

Router 模块负责 PotStack 的动态路由管理，实现沙箱请求的路由注册、匹配和转发。

## 文件结构

```
internal/router/
├── router.go     # 核心路由器实现
└── refresh.go    # 路由刷新接口
```

## 核心类型

### Router 结构体

```go
type Router struct {
    RepoRoot      string                    // 仓库根目录
    pathRoutes    map[string]http.Handler   // 路径 -> Handler 映射
    sandboxRoutes map[string][]string       // 沙箱 -> 路由键列表
    mu            sync.RWMutex              // 读写锁
}
```

## 主要方法

### NewRouter

```go
func NewRouter(repoRoot string) *Router
```

创建新的路由器实例。

### ServeHTTP

```go
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request)
```

实现 `http.Handler` 接口，使用最长前缀匹配算法查找路由。

**匹配逻辑**：
1. 遍历所有已注册的路径前缀
2. 找到与请求路径匹配的最长前缀
3. 调用对应的 Handler 处理请求
4. 无匹配时返回 404

### RegisterStatic

```go
func (r *Router) RegisterStatic(org, name string, potCfg *models.PotConfig) error
```

注册 `static` 类型沙箱路由。

**处理流程**：
1. 清理旧路由
2. 创建 `resource.NewStaticHandler`（从 Git 读取静态文件）
3. 注册四个前缀路由

### RegisterExe

```go
func (r *Router) RegisterExe(org, name string) error
```

注册 `exe` 类型沙箱路由。

**处理流程**：
1. 清理旧路由
2. 读取 `run.yml` 获取端口
3. 创建 `httputil.NewSingleHostReverseProxy`
4. 注册四个前缀路由

### registerThreeRoutesInternal

```go
func (r *Router) registerThreeRoutesInternal(org, name string, handler http.Handler)
```

内部方法，为沙箱注册四个路由前缀：

| 路由前缀 | 路径转换规则 |
|----------|-------------|
| `/pot/{org}/{name}/*` | 去掉 `/pot/{org}/{name}` |
| `/api/{org}/{name}/*` | 去掉 `/{org}/{name}`，保留 `/api` |
| `/web/{org}/{name}/*` | 去掉 `/{org}/{name}`，保留 `/web` |
| `/admin/{org}/{name}/*` | 去掉 `/{org}/{name}`，保留 `/admin` |

### RemoveRoutes

```go
func (r *Router) RemoveRoutes(org, name string)
```

移除沙箱的所有已注册路由。

## 路径转换函数

### stripPrefixHandler

```go
func stripPrefixHandler(prefix string, handler http.Handler) http.Handler
```

移除整个前缀：
- `/pot/org/name/foo` → `/foo`

### stripOrgNameHandler

```go
func stripOrgNameHandler(org, name string, handler http.Handler) http.Handler
```

只移除 `/{org}/{name}` 部分：
- `/api/org/name/users` → `/api/users`
- `/web/org/name/index.html` → `/web/index.html`

## 刷新接口

### RefreshHandler

```go
func RefreshHandler(dynamicRouter *Router) gin.HandlerFunc
```

HTTP 接口：`POST /pot/potstack/router/refresh`

**请求体**：
```json
{
  "org": "test-org",
  "name": "test-app"
}
```

**处理流程**：
1. 从 Git 读取 `pot.yml`
2. 根据 `type` 调用 `RegisterStatic` 或 `RegisterExe`
3. 返回结果

**错误响应**：
- `400` - 请求格式错误
- `404` - `pot.yml` 不存在
- `500` - 注册失败

## 线程安全

Router 使用 `sync.RWMutex` 保证并发安全：
- `ServeHTTP` 使用读锁
- `RegisterStatic`、`RegisterExe`、`RemoveRoutes` 使用写锁

## 依赖关系

```
router
  ├── internal/models (PotConfig, RunConfig)
  ├── internal/resource (NewStaticHandler)
  ├── internal/git (ReadPotYml)
  └── config (RepoDir)
```
