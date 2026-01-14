# PotStack API 文档

## 1. 认证 (Authentication)

所有受保护的接口均需要 Token 认证。支持两种方式：

### 方式一：HTTP Header

```
Authorization: token YOUR_TOKEN
```

### 方式二：HTTP Basic Auth

```
Authorization: Basic base64(TOKEN:)
```

**示例:**
```bash
# Header 方式
curl -H "Authorization: token MySecretToken" http://localhost:61080/api/v1/repos/user/repo

# Basic Auth 方式
curl -u "MySecretToken:" http://localhost:61080/api/v1/repos/user/repo
```

---

## 2. 用户管理

### 创建用户

- **URL**: `POST /api/v1/admin/users`
- **认证**: 需要
- **说明**: 创建新用户，同时在文件系统中创建用户目录和数据库记录

**请求参数:**
| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | 是 | 用户名 |
| email | string | 否 | 邮箱 |

**响应示例:**
```json
{
  "id": 1,
  "username": "zhangsan",
  "email": "zhangsan@example.com"
}
```

**curl 示例:**
```bash
curl -X POST http://localhost:61080/api/v1/admin/users \
  -H "Authorization: token MySecretToken" \
  -H "Content-Type: application/json" \
  -d '{"username": "zhangsan", "email": "zhangsan@example.com"}'
```

---

### 删除用户

- **URL**: `DELETE /api/v1/admin/users/:username`
- **认证**: 需要
- **说明**: 删除用户及其所有仓库（物理删除）

**响应:** `204 No Content`

**curl 示例:**
```bash
curl -X DELETE http://localhost:61080/api/v1/admin/users/zhangsan \
  -H "Authorization: token MySecretToken"
```

---

## 3. 仓库管理

### 创建仓库

- **URL**: `POST /api/v1/admin/users/:username/repos`
- **认证**: 需要
- **说明**: 在指定用户下创建 Bare Git 仓库

**请求参数:**
| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 仓库名称 |
| description | string | 否 | 描述 |
| private | boolean | 否 | 是否私有 |

**响应示例:**
```json
{
  "id": 1,
  "owner": {
    "id": 1,
    "username": "zhangsan",
    "email": ""
  },
  "name": "myproject",
  "full_name": "zhangsan/myproject",
  "description": "",
  "private": false,
  "clone_url": "http://localhost:61080/zhangsan/myproject.git",
  "uuid": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

**curl 示例:**
```bash
curl -X POST http://localhost:61080/api/v1/admin/users/zhangsan/repos \
  -H "Authorization: token MySecretToken" \
  -H "Content-Type: application/json" \
  -d '{"name": "myproject", "description": "My project"}'
```

---

### 获取仓库信息

- **URL**: `GET /api/v1/repos/:owner/:repo`
- **认证**: 需要
- **说明**: 获取仓库元数据

**响应:** 同创建仓库返回结构

**curl 示例:**
```bash
curl http://localhost:61080/api/v1/repos/zhangsan/myproject \
  -H "Authorization: token MySecretToken"
```

---

### 删除仓库

- **URL**: `DELETE /api/v1/repos/:owner/:repo`
- **认证**: 需要
- **说明**: 物理删除仓库

**响应:** `204 No Content`

**curl 示例:**
```bash
curl -X DELETE http://localhost:61080/api/v1/repos/zhangsan/myproject \
  -H "Authorization: token MySecretToken"
```

---

## 4. 协作者管理（Gogs 兼容）

### 列出协作者

- **URL**: `GET /api/v1/repos/:owner/:repo/collaborators`
- **认证**: 需要
- **说明**: 获取仓库的所有协作者

**响应示例:**
```json
[
  {
    "id": 2,
    "username": "lisi",
    "login": "lisi",
    "full_name": "李四",
    "email": "lisi@example.com",
    "avatar_url": "",
    "permissions": {
      "admin": false,
      "push": true,
      "pull": true
    }
  }
]
```

**curl 示例:**
```bash
curl http://localhost:61080/api/v1/repos/zhangsan/myproject/collaborators \
  -H "Authorization: token MySecretToken"
```

---

### 判断是否为协作者

- **URL**: `GET /api/v1/repos/:owner/:repo/collaborators/:collaborator`
- **认证**: 需要
- **说明**: 检查用户是否为仓库协作者

**响应:**
- 是协作者: `204 No Content`
- 不是协作者: `404 Not Found`

**curl 示例:**
```bash
curl http://localhost:61080/api/v1/repos/zhangsan/myproject/collaborators/lisi \
  -H "Authorization: token MySecretToken"
```

---

### 添加协作者

- **URL**: `PUT /api/v1/repos/:owner/:repo/collaborators/:collaborator`
- **认证**: 需要
- **说明**: 添加用户为仓库协作者

**请求参数:**
| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| permission | string | 否 | 权限级别: read/write/admin，默认 write |

**权限说明:**
| 权限 | admin | push | pull |
|------|-------|------|------|
| read | false | false | true |
| write | false | true | true |
| admin | true | true | true |

**响应:** `204 No Content`

**curl 示例:**
```bash
curl -X PUT http://localhost:61080/api/v1/repos/zhangsan/myproject/collaborators/lisi \
  -H "Authorization: token MySecretToken" \
  -H "Content-Type: application/json" \
  -d '{"permission": "write"}'
```

---

### 移除协作者

- **URL**: `DELETE /api/v1/repos/:owner/:repo/collaborators/:collaborator`
- **认证**: 需要
- **说明**: 移除仓库协作者

**响应:** `204 No Content`

**curl 示例:**
```bash
curl -X DELETE http://localhost:61080/api/v1/repos/zhangsan/myproject/collaborators/lisi \
  -H "Authorization: token MySecretToken"
```

---

## 5. 证书管理

### 查询证书信息

- **URL**: `GET /api/v1/admin/certs/info`
- **认证**: 需要
- **说明**: 获取当前 HTTPS 证书的详细信息

**响应示例:**
```json
{
  "domain": ["tongwen.chat"],
  "issuer": "R3",
  "not_before": "2026-01-13T07:30:45Z",
  "not_after": "2026-04-13T07:30:44Z",
  "remaining_days": 89,
  "needs_renewal": false
}
```

**curl 示例:**
```bash
curl http://localhost:61080/api/v1/admin/certs/info \
  -H "Authorization: token MySecretToken"
```

---

### 强制续签证书

- **URL**: `POST /api/v1/admin/certs/renew`
- **认证**: 需要
- **说明**: 立即触发证书续签，续签前会备份当前证书

**响应示例:**
```json
{
  "success": true,
  "message": "Certificate renewed successfully",
  "archived_to": "data/certs/archive/20260114-100000"
}
```

**curl 示例:**
```bash
curl -X POST http://localhost:61080/api/v1/admin/certs/renew \
  -H "Authorization: token MySecretToken"
```

**注意事项:**
- 续签前会自动备份当前证书到 `data/certs/archive/` 目录
- 如果 ACME 未启用，将返回错误
- HTTP-01 模式由 autocert 自动管理，不支持手动续签

---

## 6. Git 仓库操作（go-git）

PotStack 基于 [go-git](https://github.com/go-git/go-git) 实现 Git 功能，建议使用 go-git 库直接操作仓库。

### 仓库 URL 格式

```
http://<host>:<port>/<owner>/<repo>.git
```

示例：`http://localhost:61080/zhangsan/myproject.git`

### Clone 仓库

```go
import (
    "github.com/go-git/go-git/v5"
    "github.com/go-git/go-git/v5/plumbing/transport/http"
)

func cloneRepo() error {
    _, err := git.PlainClone("/path/to/local", false, &git.CloneOptions{
        URL: "http://localhost:61080/zhangsan/myproject.git",
        Auth: &http.BasicAuth{
            Username: "token", // 可以是任意值
            Password: "YOUR_TOKEN",
        },
    })
    return err
}
```

### Push 代码

```go
func pushRepo() error {
    repo, err := git.PlainOpen("/path/to/local")
    if err != nil {
        return err
    }

    w, err := repo.Worktree()
    if err != nil {
        return err
    }

    // 添加文件
    _, err = w.Add(".")
    if err != nil {
        return err
    }

    // 提交
    _, err = w.Commit("Initial commit", &git.CommitOptions{})
    if err != nil {
        return err
    }

    // 推送
    err = repo.Push(&git.PushOptions{
        Auth: &http.BasicAuth{
            Username: "token",
            Password: "YOUR_TOKEN",
        },
    })
    return err
}
```

### Pull 代码

```go
func pullRepo() error {
    repo, err := git.PlainOpen("/path/to/local")
    if err != nil {
        return err
    }

    w, err := repo.Worktree()
    if err != nil {
        return err
    }

    err = w.Pull(&git.PullOptions{
        Auth: &http.BasicAuth{
            Username: "token",
            Password: "YOUR_TOKEN",
        },
    })
    return err
}
```

### 支持的操作

| 操作 | go-git 方法 | 支持状态 |
|------|------------|---------|
| Clone | `git.PlainClone()` | ✅ |
| Push | `repo.Push()` | ✅ |
| Pull | `worktree.Pull()` | ✅ |
| Fetch | `repo.Fetch()` | ✅ |
| Commit | `worktree.Commit()` | ✅ |
| 浅克隆 | `CloneOptions.Depth` | ⚠️ 部分支持 |
| LFS | - | ❌ 不支持 |

### 注意事项

- 建议使用 go-git 库而非 git 命令行操作仓库
- 认证使用 HTTP Basic Auth，Username 可以是任意值，Password 为 Token
- 大仓库性能可能较慢

---

## 7. 系统接口

### 健康检查

- **URL**: `GET /health`
- **认证**: 无需
- **说明**: 返回服务运行状态

**响应示例:**
```json
{
  "status": "UP",
  "service": "potstack"
}
```

---

## 8. 资源路由

### 通用资源访问

- **URL**: `GET /uri/*path`
- **认证**: 需要
- **说明**: 访问 `$REPO_ROOT` 下的文件

**示例:**
```bash
curl http://localhost:61080/uri/zhangsan/myrepo/README.md \
  -H "Authorization: token MySecretToken"
```

### CDN 资源访问

- **URL**: `GET /cdn/*path`
- **认证**: 无需
- **说明**: 访问 `$REPO_ROOT/biz.cdn` 下的静态资源

**示例:**
```bash
curl http://localhost:61080/cdn/logo.png
```

### Web 托管（预留）

- **URL**: `ANY /web/*path`
- **说明**: 映射沙箱内部 Web 服务（暂未实现）

### ATT 操作（预留）

- **URL**: `ANY /att/*path`
- **认证**: 需要
- **说明**: ATT 业务扩展（暂未实现）

---

## 8. 错误响应

所有错误返回统一格式：

```json
{
  "error": "error message"
}
```

### 常见错误码

| 状态码 | 说明 |
|--------|------|
| 400 | 请求参数错误 |
| 401 | 未认证 |
| 404 | 资源不存在 |
| 409 | 资源冲突（如用户/仓库已存在） |
| 500 | 服务器内部错误 |
