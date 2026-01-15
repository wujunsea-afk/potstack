# Loader 模块实现文档

## 一、概述

Loader 模块负责 PotStack 的预处理逻辑，包括：
1. 初始化系统仓库
2. 解压并推送基础组件
3. 初始化 SQLite 数据库

---

## 二、系统仓库

Loader 需要创建以下系统仓库：

| 仓库 | 用途 |
|------|------|
| `potstack/keeper.git` | 沙箱守护进程 |
| `potstack/loader.git` | Loader 程序 |
| `potstack/repo.git` | 核心仓库 + SQLite 数据库 |

### 目录结构

```
$DATA_DIR/repo/potstack/
├── keeper.git/
│   ├── HEAD
│   ├── objects/
│   └── refs/
├── loader.git/
│   ├── HEAD
│   ├── objects/
│   └── refs/
└── repo.git/
    ├── HEAD
    ├── objects/
    ├── refs/
    └── data/
        └── potstack.db    ← SQLite 数据库
```

---

## 三、数据库设计

### 3.1 数据库位置

```
$DATA_DIR/repo/potstack/repo.git/data/potstack.db
```

### 3.2 表结构

#### user 表（用户）

```sql
CREATE TABLE IF NOT EXISTS user (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    username    TEXT NOT NULL UNIQUE,
    email       TEXT DEFAULT '',
    full_name   TEXT DEFAULT '',
    avatar_url  TEXT DEFAULT '',
    is_admin    INTEGER DEFAULT 0,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_username ON user(username);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| username | TEXT | 用户名（唯一） |
| email | TEXT | 邮箱 |
| full_name | TEXT | 全名 |
| avatar_url | TEXT | 头像 URL |
| is_admin | INTEGER | 是否管理员 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

#### repository 表（仓库）

```sql
CREATE TABLE IF NOT EXISTS repository (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id    INTEGER NOT NULL,
    name        TEXT NOT NULL,
    full_name   TEXT NOT NULL,
    description TEXT DEFAULT '',
    is_private  INTEGER DEFAULT 0,
    uuid        TEXT DEFAULT '',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (owner_id) REFERENCES user(id) ON DELETE CASCADE,
    UNIQUE(owner_id, name)
);

CREATE INDEX idx_repository_owner_id ON repository(owner_id);
CREATE INDEX idx_repository_full_name ON repository(full_name);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| owner_id | INTEGER | 所有者 ID（外键） |
| name | TEXT | 仓库名 |
| full_name | TEXT | 完整名称（owner/name） |
| description | TEXT | 描述 |
| is_private | INTEGER | 是否私有 |
| uuid | TEXT | 仓库 UUID |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

#### collaborator 表（协作者）

```sql
CREATE TABLE IF NOT EXISTS collaborator (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id     INTEGER NOT NULL,
    user_id     INTEGER NOT NULL,
    permission  TEXT DEFAULT 'write',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (repo_id) REFERENCES repository(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE,
    UNIQUE(repo_id, user_id)
);

CREATE INDEX idx_collaborator_repo_id ON collaborator(repo_id);
CREATE INDEX idx_collaborator_user_id ON collaborator(user_id);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| repo_id | INTEGER | 仓库 ID（外键） |
| user_id | INTEGER | 用户 ID（外键） |
| permission | TEXT | 权限（read/write/admin） |
| created_at | DATETIME | 创建时间 |

### 3.3 权限说明

| 权限 | admin | push | pull |
|------|-------|------|------|
| read | false | false | true |
| write | false | true | true |
| admin | true | true | true |

---

## 四、API 接口

### 4.1 用户管理

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/admin/users` | 创建用户 |
| DELETE | `/api/v1/admin/users/:username` | 删除用户 |

### 4.2 仓库管理

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/admin/users/:username/repos` | 创建仓库 |
| GET | `/api/v1/repos/:owner/:repo` | 获取仓库信息 |
| DELETE | `/api/v1/repos/:owner/:repo` | 删除仓库 |

### 4.3 协作者管理（Gogs 兼容）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/repos/:owner/:repo/collaborators` | 列出协作者 |
| GET | `/api/v1/repos/:owner/:repo/collaborators/:collaborator` | 判断是否为协作者 |
| PUT | `/api/v1/repos/:owner/:repo/collaborators/:collaborator` | 添加协作者 |
| DELETE | `/api/v1/repos/:owner/:repo/collaborators/:collaborator` | 移除协作者 |

---

## 五、协作者 API 详细格式

### 5.1 添加协作者

```
PUT /api/v1/repos/:owner/:repo/collaborators/:collaborator
```

**请求：**
```json
{
  "permission": "write"
}
```

**响应：**
```
Status: 204 No Content
```

### 5.2 列出协作者

```
GET /api/v1/repos/:owner/:repo/collaborators
```

**响应：**
```json
[
  {
    "id": 3,
    "username": "user1",
    "login": "user1",
    "full_name": "",
    "email": "user1@user.com",
    "avatar_url": "",
    "permissions": {
      "admin": false,
      "push": true,
      "pull": true
    }
  }
]
```

### 5.3 判断是否为协作者

```
GET /api/v1/repos/:owner/:repo/collaborators/:collaborator
```

**响应：**
- 是协作者：`204 No Content`
- 不是协作者：`404 Not Found`

### 5.4 移除协作者

```
DELETE /api/v1/repos/:owner/:repo/collaborators/:collaborator
```

**响应：**
```
Status: 204 No Content
```

---

## 六、Loader 初始化流程

```
┌─────────────────────────────────────────────────────────────────┐
│                    Loader 初始化流程                             │
└─────────────────────────────────────────────────────────────────┘

  1. 检查 PotStack 服务/数据层是否可用
     (内部检查)

  2. 创建系统用户 (调用 UserService)
     Username: "potstack"
     Email: "system@potstack.local"

  3. 创建系统仓库 (调用 RepoService)
     potstack/keeper
     potstack/loader
     potstack/repo

  4. 部署组件
     4.1 解压 potstack-base.zip
         ├── install.yml
         ├── potstack.ppk
         └── VERSION

     4.2 读取 install.yml
         packages:
           - potstack.ppk

     4.3 对于每个 ppk 文件：
         ├── 解压 potstack.ppk
         │   └── potstack/
         │       ├── keeper/
         │       ├── loader/
         │       └── repo/
         │
         ├── 遍历 owner/potname 目录
         ├── 确保用户和仓库存在
         └── 推送到 owner/potname.git

  5. 初始化完成
```

### 6.1 potstack-base.zip 结构

> **自动分发**: 系统启动时，会自动检查数据目录（`$DATA_DIR`）下是否存在 `potstack-base.zip`。如果不存在，Loader 会尝试从程序运行目录（Executable Path）自动复制该文件，实现开箱即用。

```
potstack-base.zip
├── install.yml       # 安装清单
├── potstack.ppk      # 组件包（由 potpacker 生成）
└── VERSION           # 版本号
```

### 6.2 install.yml 格式

```yaml
# PotStack Base Pack Install Manifest
version: "20260114144608"

packages:
  - potstack.ppk
```

### 6.3 ppk 文件结构

ppk 是 **具有自定义 Header 的 ZIP 归档文件**，由 `potpacker` 工具生成。

**格式:**
```
[Magic Bytes: POTP]  (4 bytes)
[Version: 0x01]      (1 byte)
[Sig Algo: 0x01]     (1 byte)
[Content Len]        (8 bytes, BigEndian)
[Signature]          (64 bytes, ED25519)
[Compressed Data]    (Zip Content)
```

**Content (Zip) 结构:**
```
potstack/              # owner
    ├── keeper/        # potname
    │   └── pot.yml
    ├── loader/
    │   └── pot.yml
    └── repo/
        └── pot.yml
```

---

## 七、文件结构

```
potstack/
├── internal/
│   ├── api/
│   │   ├── user.go            # 用户管理
│   │   ├── repo.go            # 仓库管理
│   │   ├── collaborator.go    # 协作者管理（新增）
│   │   └── types.go           # 数据类型
│   ├── db/
│   │   ├── db.go              # SQLite 连接 + 初始化
│   │   ├── user.go            # 用户 DAO
│   │   ├── repo.go            # 仓库 DAO
│   │   └── collaborator.go    # 协作者 DAO
│   ├── loader/
│   │   └── loader.go          # Loader 逻辑
│   └── ...
├── build_base_pack.sh         # 打包脚本
└── ...
```

---

## 八、实现任务清单

| 序号 | 任务 | 状态 |
|------|------|------|
| 1 | 创建 db 模块 | ✅ 已完成 |
| 2 | 实现用户 DAO | ✅ 已完成 |
| 3 | 实现仓库 DAO | ✅ 已完成 |
| 4 | 实现协作者 DAO | ✅ 已完成 |
| 5 | 调整 user.go（删除组织接口） | ✅ 已完成 |
| 6 | 调整 repo.go（数据库操作） | ✅ 已完成 |
| 7 | 新增 collaborator.go | ✅ 已完成 |
| 8 | 调整 main.go 路由 | ✅ 已完成 |
| 9 | 创建 loader 模块 | ✅ 已完成 |
| 10 | 实现 build_base_pack.sh | ✅ 已完成 |
| 11 | 更新 README.md | ✅ 已完成 |

---

## 九、依赖

需要添加 SQLite 驱动：

```bash
go get github.com/mattn/go-sqlite3
```

