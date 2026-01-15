# SQLite CGO 问题与解决方案报告

## 1. 问题背景

在构建 PotStack 的 Windows 版本时，遇到了 CGO 相关的编译错误和运行时错误。由于 PotStack 目标是提供跨平台、易部署的单文件二进制，因此使用了 `CGO_ENABLED=0` 进行纯静态编译。

## 2. 问题现象

### 2.1 编译阶段
*   **现象**: 在 Linux 环境下执行 `build.sh` 交叉编译 Windows 版本时报错：
    ```
    gcc: error: unrecognized command-line option ‘-mthreads’
    ```
*   **原因**: 原有的 SQLite 驱动 (`github.com/mattn/go-sqlite3`) 必须依赖 GCC 进行 C 代码编译。在 Linux 上编译 Windows 目标需要安装 MinGW-w64 交叉工具链，而当前环境未安装。

### 2.2 运行阶段 (切换驱动初期)
*   **现象**: 尝试切换驱动后，程序抛出 Panic：
    ```
    panic: runtime error: invalid memory address or nil pointer dereference
    ...
    Fatal: failed to init database: failed to open database: sql: unknown driver "sqlite" (forgotten import?)
    ```
*   **原因**: 
    1.  数据库驱动未正确注册到 `database/sql`，导致 `sql.Open("sqlite", ...)` 失败。
    2.  `main.go` 中的错误处理机制仅打印 Warning 未退出，导致后续逻辑使用了未初始化的空指针 `db` 对象。

## 3. 解决方案

决定**彻底弃用 CGO 依赖**，切换到纯 Go 实现的 SQLite 驱动。

### 3.1 驱动选型
采用 **`github.com/glebarez/go-sqlite`**。
*   **类型**: Pure Go (无 CGO)。
*   **底层**: 基于 `modernc.org/sqlite` (将 sqlite C 源码转译为 Go)。
*   **接口**: 标准 `database/sql` 接口，驱动名为 `"sqlite"`。
*   **兼容性**: 完美支持 Windows/Linux/macOS 跨平台交叉编译。

### 3.2 代码变更

**1. 修改 `internal/db/db.go`**
替换导入路径：
```go
import (
    _ "github.com/glebarez/go-sqlite" // 替换 github.com/mattn/go-sqlite3
)

func Init(...) {
    // 使用 "sqlite" 作为驱动名
    db, err = sql.Open("sqlite", dbPath) 
    // ...
}
```

**2. 修改 `main.go`**
增加显式导入以确保驱动注册（解决 unknown driver 问题）：
```go
import (
    _ "github.com/glebarez/go-sqlite" // 强制注册驱动
)
```
优化错误处理，数据库初始化失败立即退出：
```go
if err := db.Init(config.RepoDir); err != nil {
    log.Fatalf("Fatal: failed to init database: %v", err)
}
```

**3. 修改 `build.sh`**
确保禁用 CGO：
```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ...
```

## 4. 验证结果
*   **构建**: Linux 和 Windows 版本均能在 Linux 环境下成功编译。
*   **运行**: 服务启动正常，数据库读写正常。
*   **依赖**: 最终产物无任何 DLL 依赖，完全静态链接。
