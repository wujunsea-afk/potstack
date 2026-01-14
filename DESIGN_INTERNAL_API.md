# 内部 API/Service 层重构方案 v2

## 1. 背景与目标
当前 PotStack 的业务逻辑（如创建用户、创建仓库、添加协作者）紧耦合在 `internet/api` 包的 HTTP Handler 中。通过 HTTP Loopback（自我调用）进行内部通信效率低下且增加了系统复杂性。

目标是将核心业务逻辑从 HTTP Layer 剥离，封装为独立的 **Service Layer**。

## 2. 架构设计

采用经典的三层架构：
1.  **Handler Layer (`internal/api`)**: 负责 HTTP 协议处理（参数绑定、验证、HTTP 响应）。
2.  **Service Layer (`internal/service`)**: 负责核心业务流程、事务管理、权限检查。
3.  **Data/Repo Layer (`internal/db`, `internal/git`)**: 负责底层数据存储和原子操作。

### 调用流向
- **External Request**: `HTTP Client` -> `Gin Handler` -> `Service` -> `DB/FS`
- **Internal Call (e.g., Loader)**: `Loader` -> `Service` -> `DB/FS`

## 3. 详细设计

### 3.1 Service 接口定义

将在 `internal/service` 包中定义核心服务接口。

**IUserService**:
```go
type IUserService interface {
    // CreateUser 创建用户（包含目录创建和 DB 记录）
    CreateUser(ctx context.Context, username, email, password string) (*model.User, error)
    // DeleteUser 删除用户
    DeleteUser(ctx context.Context, username string) error
    // GetUser 获取用户
    GetUser(ctx context.Context, username string) (*model.User, error)
}
```

**IRepoService**:
```go
type IRepoService interface {
    // 仓库管理
    CreateRepo(ctx context.Context, owner, name string) (*model.Repo, error)
    DeleteRepo(ctx context.Context, owner, name string) error
    GetRepo(ctx context.Context, owner, name string) (*model.Repo, error)
    
    // 协作者管理
    AddCollaborator(ctx context.Context, owner, repo, collaborator, permission string) error
    RemoveCollaborator(ctx context.Context, owner, repo, collaborator string) error
    ListCollaborators(ctx context.Context, owner, repo string) ([]*model.CollaboratorResponse, error)
    IsCollaborator(ctx context.Context, owner, repo, user string) (bool, error)
}
```

### 3.2 Service 实现示例

逻辑将从 `internal/api/*.go` 迁移到 `internal/service/*.go`。

例如 `RepoService.AddCollaborator`:
```go
func (s *RepoService) AddCollaborator(ctx context.Context, owner, repoName, collaboratorName, permission string) error {
    repo, err := s.repoStore.GetRepositoryByOwnerAndName(owner, repoName)
    if err != nil { return err }
    if repo == nil { return ErrRepoNotFound }

    user, err := s.userStore.GetOrCreateUser(collaboratorName, "")
    // ...
    return s.repoStore.AddCollaborator(repo.ID, user.ID, permission)
}
```

### 3.3 Handler 重构

Handler 将只负责 HTTP 转换。

```go
func AddCollaboratorHandler(svc service.IRepoService) gin.HandlerFunc {
    return func(c *gin.Context) {
        // ... Bind JSON ...
        err := svc.AddCollaborator(c.Request.Context(), owner, repo, collaborator, opt.Permission)
        if err != nil {
            if errors.Is(err, service.ErrRepoNotFound) {
                c.JSON(404, ...)
            } else {
                c.JSON(500, ...)
            }
            return
        }
        c.Status(204)
    }
}
```

## 4. 实施步骤

1.  **创建 `internal/service` 包**。
    - 定义 `errors.go`: 标准业务错误。
    - 定义 `interfaces.go`: Service 接口。
2.  **实现 `UserService`**: 迁移 `internal/api/user.go` 逻辑。
3.  **实现 `RepoService`**: 迁移 `internal/api/repo.go` 和 `internal/api/collaborator.go` 逻辑。
4.  **重构 Handlers**: 修改 `internal/api` 中的 Handler 以调用 Service (构造函数注入)。
5.  **依赖组装**:
    - 修改 `main.go` 初始化 Services。
    - 将 Services 注入到 `api.Server` (或者直接传递给路由注册函数)。
    - 将 Services 注入到 `Loader`。
6.  **Loader 改造**: Loader 调用 Service 替代 HTTP Request。

## 5. 优势
- **性能**: 内部调用零网络开销（无序列化/反序列化，无 TCP 握手）。
- **类型安全**: 编译期检查参数类型。
- **可测试性**: Service 层可以独立进行单元测试，不依赖 HTTP Server。
- **一致性**: 无论通过 API 还是 Loader，业务规则完全一致。
