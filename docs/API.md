# PotStack API 文档

## 1. 认证 (Authentication)
所有受保护的接口均需要 HTTP Basic Auth 认证。请使用环境变量 `POTSTACK_TOKEN` 配置的值作为用户名或密码。
如果没有配置 Token，默认允许所有访问（仅用于调试）。

**示例 Header:**
`Authorization: Basic <base64(POTSTACK_TOKEN)>`

---

## 2. API v1 接口详情

### 用户管理 (User Management)

#### 创建用户
- **URL**: `POST /api/v1/admin/users`
- **认证**: 需要 (Admin)
- **说明**: 创建一个新的代码托管用户。这将在文件系统中创建一个以用户名命名的目录。

##### 请求参数 (Request Body)
| 字段名 | 类型 | 是否必需 | 说明 |
| :--- | :--- | :--- | :--- |
| `username` | `string` | 是 | 要创建的用户名 |
| `email` | `string` | 否 | 用户的邮箱地址 |

##### 成功返回 (Success Response)
```json
{
    "id": 0,
    "username": "zhangsan",
    "email": "zhangsan@example.com"
}
```

**Curl 示例:**
```bash
curl -X POST http://localhost:61080/api/v1/admin/users \
  -u "MySecretToken:" \
  -H "Content-Type: application/json" \
  -d '{"username": "zhangsan", "email": "zhangsan@example.com"}'
```

---

#### 删除用户
- **URL**: `DELETE /api/v1/admin/users/:username`
- **认证**: 需要 (Admin)
- **说明**: 删除一个用户及其下属的所有仓库。这是一个物理删除操作。

##### 请求参数 (URL Parameters)
| 字段名 | 类型 | 说明 |
| :--- | :--- | :--- |
| `username` | `string` | 要删除的用户的名称 |

##### 成功返回 (Success Response)
- **Code**: `204 No Content`

**Curl 示例:**
```bash
curl -X DELETE http://localhost:61080/api/v1/admin/users/zhangsan \
  -u "MySecretToken:"
```

---
### 仓库管理 (Repository Management)

#### 创建仓库
- **URL**: `POST /api/v1/admin/users/:username/repos`
- **认证**: 需要 (Admin)
- **说明**: 在指定用户下创建一个新的 Bare Git 仓库。

##### 请求参数 (URL Parameters)
| 字段名 | 类型 | 说明 |
| :--- | :--- | :--- |
| `username` | `string` | 仓库所属的用户名 |

##### 请求参数 (Request Body)
| 字段名 | 类型 | 是否必需 | 说明 |
| :--- | :--- | :--- | :--- |
| `name` | `string` | 是 | 仓库名称 |
| `description`| `string` | 否 | 仓库描述 |
| `private` | `boolean`| 否 | 是否为私有仓库 (当前版本中此字段无实际作用) |

##### 成功返回 (Success Response)
```json
{
    "id": 0,
    "owner": {
        "id": 0,
        "username": "zhangsan",
        "email": ""
    },
    "name": "project-alpha",
    "full_name": "zhangsan/project-alpha",
    "description": "",
    "private": false,
    "clone_url": "http://localhost:61080/zhangsan/project-alpha.git",
    "uuid": "some-generated-uuid"
}
```

**Curl 示例:**
```bash
curl -X POST http://localhost:61080/api/v1/admin/users/zhangsan/repos \
  -u "MySecretToken:" \
  -H "Content-Type: application/json" \
  -d '{"name": "project-alpha"}'
```

---

#### 获取仓库信息
- **URL**: `GET /api/v1/repos/:owner/:repo`
- **认证**: 无需认证
- **说明**: 获取仓库的公开元数据。

##### 请求参数 (URL Parameters)
| 字段名 | 类型 | 说明 |
| :--- | :--- | :--- |
| `owner` | `string` | 仓库所属的用户名 |
| `repo` | `string` | 仓库名称 |

##### 成功返回 (Success Response)
返回一个与创建仓库时结构相同的 `Repository` 对象。
```json
{
    "id": 0,
    "owner": {
        "id": 0,
        "username": "zhangsan",
        "email": ""
    },
    "name": "project-alpha",
    "full_name": "zhangsan/project-alpha",
    "description": "",
    "private": false,
    "clone_url": "http://localhost:61080/zhangsan/project-alpha.git",
    "uuid": "some-generated-uuid"
}
```

**Curl 示例:**
```bash
curl http://localhost:61080/api/v1/repos/zhangsan/project-alpha
```

---

#### 删除仓库
- **URL**: `DELETE /api/v1/repos/:username/:reponame`
- **认证**: 需要 (Admin)
- **说明**: 物理删除一个仓库目录。

##### 请求参数 (URL Parameters)
| 字段名 | 类型 | 说明 |
| :--- | :--- | :--- |
| `username`| `string` | 仓库所属的用户名 |
| `reponame`| `string` | 要删除的仓库名称 |

##### 成功返回 (Success Response)
- **Code**: `204 No Content`

**Curl 示例:**
```bash
curl -X DELETE http://localhost:61080/api/v1/repos/zhangsan/project-alpha \
  -u "MySecretToken:"
```

### Git Smart HTTP

PotStack 原生支持标准的 Git 协议，可以直接使用 `git` 命令行工具进行操作。

- **URL**: `http://host:port/:owner/:repo.git`

**Git Clone 示例:**
```bash
git clone http://localhost:61080/zhangsan/project-alpha.git
```

**Git Push 示例:**
```bash
cd project-alpha
touch README.md
git add .
git commit -m "Initial commit"
git push origin master
```

### 系统健康检查 (System Health Check)
PotStack 提供健康检查接口，用于监控服务状态。

#### 健康检查
- **URL**: `GET /health`
- **认证**: 无需认证 (Public)
- **说明**: 返回服务运行状态。当服务正常运行时，返回 `{"status": "UP", "service": "potstack"}`。
- **Curl 示例**:
  ```bash
  curl http://localhost:61080/health
 
### 统一资源路由 (Unified Resource Routing)

PotStack 提供了一套统一的资源访问机制，用于直接访问仓库文件或 CDN 资源。

#### 通用资源访问 (Resource Access)
- **URL**: `GET /uri/*path`
- **认证**: 需要 (Basic Auth)
- **说明**: 直接访问 `POTSTACK_REPO_ROOT` 下的文件。
- **示例**: 
  `GET /uri/username/repo/README.md` 将读取 `<REPO_ROOT>/username/repo/README.md`

#### CDN 资源访问 (CDN Access)
- **URL**: `GET /cdn/*path`
- **认证**: **无需认证 (Public)**
- **说明**: 访问 `POTSTACK_REPO_ROOT/biz.cdn` 目录下的静态资源。
- **场景**: 用于托管前端静态文件或公共资源。
- **示例**:
  `GET /cdn/logo.png` 将读取 `<REPO_ROOT>/biz.cdn/logo.png`

#### Web 托管 (Web Hosting) - *(Preview)*
- **URL**: `ANY /web/*path`
- **说明**: 用于映射沙箱内部的 Web 服务（暂未实现，预留接口）。

#### ATT 操作 - *(Preview)*
- **URL**: `ANY /att/*path`
- **认证**: 需要 (Basic Auth)
- **说明**: 用于特定的 ATT 业务扩展操作（暂未实现，预留接口）。
