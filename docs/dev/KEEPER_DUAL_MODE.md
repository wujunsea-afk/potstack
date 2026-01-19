# Keeper 双模式架构设计

## 模式选择逻辑

```
检查 potstack/keeper.git
├─ 不存在 → 模式一（内置完整 Keeper）
└─ 存在 → 读取 pot.yml
    ├─ type != exe → 模式一
    └─ type == exe → 模式二（内置精简 + 外置 Keeper Pot）
```

---

## 外置 Keeper Pot 路由设计

外置 Keeper Pot 内部路由定义在 `/admin` 下：

| 内部定义 | 外部访问（管理端口） | 内部访问（内部端口） |
|----------|---------------------|---------------------|
| `/admin/` | `/admin/potstack/keeper/` | `/pot/potstack/keeper/admin/` |
| `/admin/pots` | `/admin/potstack/keeper/pots` | `/pot/potstack/keeper/admin/pots` |

**路径转换规则**：
- `/admin/potstack/keeper/*` → 去掉 `/potstack/keeper`，保留 `/admin/*`
- `/pot/potstack/keeper/*` → 去掉 `/pot/potstack/keeper`，变成 `/*`

---

## 模式一：内置 Keeper

**职责**：完整的沙箱生命周期管理
- reconcile 所有 Pot
- 启动/停止 Pot 进程
- 自动重启崩溃的 Pot
- 调用 Router 刷新路由

---

## 模式二：外置 Keeper

### 内置 Keeper（精简版）

**职责**：仅保证 keeper pot 运行
- 只管理 `potstack/keeper` 这一个 Pot
- 监控 keeper pot 崩溃并自动重启

### 外置 Keeper Pot

**职责**：完整的沙箱管理 + 管理界面
- 管理所有其他 Pot 的生命周期
- 提供管理界面：`/admin/potstack/keeper`
- 查看 Pot 状态、启停 Pot、日志查看等

---

## 代码实现

```go
type KeeperMode int

const (
    KeeperModeBuiltin  KeeperMode = iota // 模式一
    KeeperModeExternal                    // 模式二
)

func (s *SandboxManager) DetectMode() KeeperMode {
    var potCfg models.PotConfig
    err := git.ReadPotYml(s.RepoRoot, "potstack", "keeper", &potCfg)
    if err != nil || potCfg.Type != "exe" {
        return KeeperModeBuiltin
    }
    return KeeperModeExternal
}

func (s *SandboxManager) StartKeeper() {
    mode := s.DetectMode()
    
    if mode == KeeperModeBuiltin {
        log.Println("Keeper mode: builtin")
        s.runBuiltinKeeper()
    } else {
        log.Println("Keeper mode: external")
        s.runExternalKeeper()
    }
}

func (s *SandboxManager) runExternalKeeper() {
    for {
        s.ensureKeeperPotRunning()
        time.Sleep(10 * time.Second)
    }
}

func (s *SandboxManager) ensureKeeperPotRunning() {
    key := "potstack/keeper"
    s.mu.RLock()
    _, running := s.runningInstances[key]
    s.mu.RUnlock()
    
    if !running {
        log.Println("Starting keeper pot...")
        s.createRuntime("potstack", "keeper")
        s.Start("potstack", "keeper")
    }
}
```
