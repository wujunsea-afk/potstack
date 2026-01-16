# Keeper 模块设计文档

Keeper 模块负责沙箱进程的生命周期管理，包括启动、停止、监控和自动重启。

## 文件结构

```
internal/keeper/
├── service.go        # 核心管理器实现
├── process_windows.go # Windows 进程管理
└── process_unix.go    # Unix 进程管理
```

## 核心类型

### SandboxManager 结构体

```go
type SandboxManager struct {
    RepoRoot         string                   // 仓库根目录
    PotProvider      PotProvider              // 沙箱提供者接口
    Router           *router.Router           // 路由器引用
    runningInstances map[string]*Instance     // 运行中的实例
    mu               sync.RWMutex             // 读写锁
    stopChan         chan struct{}            // 停止信号
}
```

### PotProvider 接口

```go
type PotProvider interface {
    GetInstalledPots() []PotURI
}
```

由 Loader 模块实现，返回已安装的沙箱列表。

### PotURI 结构体

```go
type PotURI struct {
    Org  string
    Name string
}
```

## 主要方法

### NewManager

```go
func NewManager(repoRoot string, r *router.Router) *SandboxManager
```

创建沙箱管理器实例。

### StartKeeper

```go
func (s *SandboxManager) StartKeeper()
```

启动 Keeper 主循环。

**处理流程**：
1. 执行初始 `reconcile()`
2. 每 30 秒执行 `monitor()`
3. 响应 `stopChan` 停止信号

### reconcile

```go
func (s *SandboxManager) reconcile()
```

确保所有沙箱处于期望状态。

**处理逻辑**：
```
遍历所有已安装的沙箱
├─ 从 Git 读取 pot.yml
├─ Type = static
│   └─ 调用 refreshRoute（刷新路由）
└─ Type = exe
    ├─ 无 run.yml: createRuntime → Start
    ├─ TargetStatus = running
    │   ├─ 未运行: Start
    │   └─ 已运行: refreshRoute
    └─ TargetStatus = stopped
        └─ 正在运行: Stop
```

### createRuntime

```go
func (s *SandboxManager) createRuntime(org, name string) error
```

准备沙箱运行环境。

**处理流程**：
1. 清空 `program/` 目录
2. 从 bare 仓库克隆代码
3. 删除 `.git` 目录

### Start

```go
func (s *SandboxManager) Start(org, name string) error
```

启动 `exe` 类型沙箱进程。

**处理流程**：
1. 从 Git 读取 `pot.yml` 验证类型
2. 分配空闲端口
3. 准备环境变量（内置 + 用户自定义）
4. 启动进程
5. 保存 `run.yml`
6. 启动 `watchProcess` goroutine
7. 调用 `refreshRoute`

**内置环境变量**：

| 变量 | 说明 |
|------|------|
| `DATA_PATH` | 沙箱数据目录 |
| `PROGRAM_PATH` | 程序代码目录 |
| `LOG_PATH` | 日志目录 |
| `POTSTACK_BASE_URL` | 主服务内部地址 |
| `SU_SERVER_ADDR` | 监听地址 |

### Stop

```go
func (s *SandboxManager) Stop(org, name string) error
```

停止沙箱进程。

**处理流程**：
1. 终止进程（Windows: Job Object，Unix: Kill）
2. 从 `runningInstances` 移除
3. 更新 `run.yml` 状态
4. 调用 `refreshRoute`

### refreshRoute

```go
func (s *SandboxManager) refreshRoute(org, name string)
```

调用 Router 刷新接口更新路由。

```go
url := fmt.Sprintf("http://localhost:%s/pot/potstack/router/refresh", config.InternalPort)
```

### watchProcess

```go
func (s *SandboxManager) watchProcess(key string, cmd *JobCmd)
```

监控进程状态，自动重启。

**处理流程**：
1. 等待进程退出
2. 从 `runningInstances` 移除
3. 检查 `run.yml` 的 `TargetStatus`
4. 如果是 `running`，等待 1 秒后重启

### SignalUpdate

```go
func (s *SandboxManager) SignalUpdate(org, name string)
```

Loader 更新通知，重新部署沙箱。

**处理流程**：
1. 调用 `createRuntime` 更新代码
2. 调用 `Stop` 停止进程
3. 调用 `Start` 重启进程

## 配置文件

### run.yml

位置：`{repo}.git/data/faaspot/run.yml`

```yaml
target_status: running  # running / stopped
runtime:
  port: 61234
  pid: 12345
```

## 目录结构

```
{org}/{name}.git/
└── data/
    └── faaspot/
        ├── program/      # 代码检出目录
        ├── data/         # 沙箱数据目录
        ├── log/          # 日志目录
        └── run.yml       # 运行状态
```

## 进程管理

### Windows (process_windows.go)

使用 Job Object 管理子进程：
- 创建 Job Object
- 将进程添加到 Job
- 终止时杀死整个 Job

### Unix (process_unix.go)

使用进程组管理：
- 设置 `Setpgid`
- 终止时发送 `SIGKILL` 到进程组

## 线程安全

使用 `sync.RWMutex` 保护 `runningInstances`：
- 读取使用 `RLock`
- 写入使用 `Lock`

## 依赖关系

```
keeper
  ├── internal/router (Router, refreshRoute)
  ├── internal/models (PotConfig, RunConfig)
  ├── internal/git (ReadPotYml)
  └── config (InternalPort, RepoDir)
```

## 端口分配

使用 `GetFreePort()` 函数动态分配端口：

```go
func GetFreePort() (int, error) {
    listener, err := net.Listen("tcp", ":0")
    if err != nil {
        return 0, err
    }
    defer listener.Close()
    return listener.Addr().(*net.TCPAddr).Port, nil
}
```
