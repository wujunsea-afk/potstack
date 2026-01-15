# HTTPS 证书管理技术文档

## 概述

PotStack 支持三种 HTTPS 模式：
1. **手动证书**：用户自行管理证书文件
2. **HTTP-01 自动续签**：通过 80 端口验证，需要备案
3. **DNS-01 自动续签**：通过 DNS TXT 记录验证，无需备案（推荐）

---

## 一、配置文件

### 文件说明

| 文件 | 位置 | 说明 |
|------|------|------|
| `https.yaml.example` | 程序同目录 | 配置模板（安装包自带） |
| `https.yaml` | `$DATA_DIR/` | 实际配置（运行时使用） |

首次启动时，如果 `$DATA_DIR/https.yaml` 不存在，程序会自动从模板 `https.yaml.example` 复制。

配置支持热重载（约 30 秒生效），无需重启服务。

### 完整配置示例

```yaml
# 模式：http / https
mode: https

# ACME 自动证书配置
acme:
  # 是否启用自动续签
  enabled: true
  
  # 域名
  domain: git.example.com
  
  # 验证方式：http-01 / dns-01
  challenge: dns-01
  
  # HTTP-01 配置
  http:
    port: 80
  
  # DNS-01 配置
  dns:
    provider: tencentcloud  # tencentcloud / alidns / cloudflare
    credentials:
      secret_id: ""
      secret_key: ""
  
  # CA 目录列表
  directories:
    - https://acme-v02.api.letsencrypt.org/directory
    - https://acme.zerossl.com/v2/DV90
  
  # 重试策略
  retry_count: 3
  retry_delay_seconds: 5
  
  # 提前续签天数
  renew_before_days: 30
  
  # 联系邮箱
  email: ""
```

---

## 二、证书检查流程

```
                    ┌─────────────────────┐
                    │  启动/定时检查/API   │
                    └──────────┬──────────┘
                               ↓
                    ┌─────────────────────┐
                    │   读取当前证书       │
                    └──────────┬──────────┘
                               ↓
              ┌────────────────┴────────────────┐
              ↓                                 ↓
       证书不存在/损坏                    证书存在且可解析
              │                                 │
              ↓                                 ↓
         直接申请新证书              ┌──────────┴──────────┐
                                    ↓                     ↓
                              域名匹配？              域名不匹配
                                    │                     │
                              ┌─────┴─────┐               ↓
                              ↓           ↓          备份 + 重申
                           已过期      未过期
                              │           │
                              ↓           ↓
                         备份 + 重申   检查剩余天数
                                          │
                                    ┌─────┴─────┐
                                    ↓           ↓
                              < 30 天        > 30 天
                                    │           │
                                    ↓           ↓
                         后台备份 + 续签    正常使用
```

### 触发时机

| 时机 | 说明 |
|------|------|
| 启动时 | 程序启动时检查证书 |
| 定时检查 | 每 12 小时自动检查 |
| API 调用 | `POST /api/v1/admin/certs/renew` |

---

## 三、DNS-01 验证

### 优势

- ✅ 无需开放 80/443 端口
- ✅ 无需备案
- ✅ 支持内网服务器
- ✅ 支持通配符证书

### 支持的 DNS 提供商

| 提供商 | provider 值 | 凭证配置 |
|--------|-------------|---------|
| 腾讯云 | `tencentcloud` | `secret_id`, `secret_key` |
| 阿里云 | `alidns` | `access_key_id`, `access_key_secret` |
| Cloudflare | `cloudflare` | `api_token` |

### 工作流程

```
你的服务器                              Let's Encrypt
     │                                      │
     │ ① 申请证书 (git.example.com)         │
     │──────────────────────────────────────▶│
     │                                      │
     │ ② 返回挑战令牌                        │
     │◀──────────────────────────────────────│
     │                                      │
     │ ③ 通过 DNS API 添加 TXT 记录          │
     │    _acme-challenge.git.example.com   │
     │                                      │
     │ ④ 通知完成                            │
     │──────────────────────────────────────▶│
     │                                      │
     │ ⑤ Let's Encrypt 查询 DNS             │
     │   （不访问你的服务器）                 │
     │                                      │
     │ ⑥ 验证通过，签发证书                   │
     │◀──────────────────────────────────────│
     │                                      │
     ▼
  证书保存到 certs/cert.pem
```

---

## 四、HTTP-01 验证

### 适用场景

- 有备案的公网服务器
- 80 端口可访问

### 配置示例

```yaml
mode: https
acme:
  enabled: true
  domain: git.example.com
  challenge: http-01
  http:
    port: 80
```

---

## 五、手动证书

### 适用场景

- 开发环境
- 内网环境
- 已有证书

### 使用方式

1. 将证书放到 `$DATA_DIR/certs/`:
   - `cert.pem` - 证书文件
   - `key.pem` - 私钥文件

2. 配置 https.yaml:
```yaml
mode: https
acme:
  enabled: false
```

### 生成开发证书

使用 [mkcert](https://github.com/FiloSottile/mkcert):

```bash
mkcert -cert-file certs/cert.pem -key-file certs/key.pem localhost 127.0.0.1
```

---

## 六、证书热重载

证书文件更新后，服务会自动检测并重载（约 30 秒），无需重启。

---

## 七、故障排查

| 问题 | 可能原因 | 解决方案 |
|------|---------|---------|
| 证书申请失败 | 80 端口不通 | 检查防火墙，改用 DNS-01 |
| 域名验证失败 | DNS 未解析 | 检查域名解析 |
| DNS API 错误 | 凭证错误 | 检查 API 凭证 |
| 证书已过期 | 续签失败 | 检查日志，手动续签 |

---

## 八、CA 故障切换

配置多个 CA 目录，按优先级尝试：

```yaml
acme:
  directories:
    - https://acme-v02.api.letsencrypt.org/directory  # 首选
    - https://acme.zerossl.com/v2/DV90                # 备选
```

---

## 九、自动续签检查

### 检查机制

- **检查间隔**: 每 12 小时自动检查证书有效期
- **提前续签**: 距离到期不足 `renew_before_days`（默认 30 天）时触发续签
- **域名变更检测**: 配置域名改变时自动重申证书

### 续签流程

```
定时检查（每 12 小时）
    ↓
检查证书剩余有效期
    ├── > 30 天 → 无需操作
    └── < 30 天 → 触发续签
           ↓
       备份当前证书到 archive/
           ↓
       申请新证书
           ↓
       热重载（服务不中断）
```

---

## 十、证书管理 API

### 查看证书信息

```bash
GET /api/v1/admin/certs/info
```

响应：
```json
{
  "domain": ["git.example.com"],
  "issuer": "R3",
  "not_before": "2026-01-13T07:30:45Z",
  "not_after": "2026-04-13T07:30:44Z",
  "remaining_days": 89,
  "needs_renewal": false
}
```

### 强制续签

```bash
POST /api/v1/admin/certs/renew
```

响应：
```json
{
  "success": true,
  "message": "Certificate renewed successfully",
  "archived_to": "certs/archive/20260114-100000"
}
```

---

## 十一、证书存储结构

```
$DATA_DIR/certs/
├── cert.pem              # 当前证书
├── key.pem               # 当前私钥
├── acme_user.json        # ACME 账户信息
└── archive/              # 历史存档
    ├── 20260113-153045/
    │   ├── cert.pem
    │   └── key.pem
    └── 20260414-120000/
        ├── cert.pem
        └── key.pem
```

文件说明：
| 文件 | 说明 |
|------|------|
| `cert.pem` | 服务器证书（包含证书链） |
| `key.pem` | 私钥（权限应为 600） |
| `acme_user.json` | ACME 账户密钥和注册信息 |
| `archive/` | 续签前的证书备份 |
