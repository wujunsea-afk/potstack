# PotStack 部署指南

## 一、系统要求

### 硬件要求

| 项目 | 最低配置 | 推荐配置 |
|------|---------|---------|
| CPU | 1 核 | 2 核 |
| 内存 | 512 MB | 1 GB |
| 磁盘 | 1 GB | 10 GB |

### 操作系统

- Windows 10/11, Windows Server 2016+
- Linux (Ubuntu 20.04+, CentOS 7+, Debian 10+)

---

## 二、快速部署

### 2.1 下载

从 Release 页面下载最新版本：

- `potstack-base.zip` - 完整部署包

### 2.2 解压

```bash
unzip potstack-base.zip -d potstack
cd potstack
```

### 2.3 配置

#### 方式一：环境变量

```bash
export POTSTACK_REPO_ROOT=data
export POTSTACK_HTTP_PORT=61080
export POTSTACK_TOKEN=your-secret-token
```

#### 方式二：使用启动脚本

**Linux:**
```bash
cp start.sh.example start.sh
vim start.sh  # 修改配置
chmod +x start.sh
```

**Windows:**
```cmd
copy start.bat.example start.bat
notepad start.bat  # 修改配置
```

### 2.4 启动

**Linux:**
```bash
./start.sh
# 或
./potstack-linux
```

**Windows:**
```cmd
start.bat
# 或
potstack.exe
```

---

## 三、目录结构

### 3.1 部署目录

```
potstack/
├── potstack.exe          # 主程序 (Windows)
├── potstack-linux        # 主程序 (Linux)
├── https.yaml.example    # HTTPS 配置模板
├── start.bat             # Windows 启动脚本
├── start.sh              # Linux 启动脚本
└── VERSION               # 版本号
```

### 3.2 数据目录

首次启动后会创建：

```
$REPO_ROOT/                # 例如 ./data/
├── https.yaml             # HTTPS 配置
├── certs/                 # 证书目录
│   ├── cert.pem
│   └── key.pem
├── log/
│   └── potstack.log       # 日志
└── repo/                  # 仓库目录
    ├── potstack/          # 系统仓库
    │   ├── keeper.git/
    │   ├── loader.git/
    │   └── repo.git/
    │       └── data/
    │           └── potstack.db  # 数据库
    └── user1/
        └── myrepo.git/
```

---

## 四、HTTPS 配置

### 4.1 纯 HTTP（默认）

无需配置，直接启动即可。

### 4.2 手动证书

1. 准备证书文件：
   - `cert.pem` - 证书
   - `key.pem` - 私钥

2. 放置到 `$REPO_ROOT/certs/` 目录

3. 修改 `$REPO_ROOT/https.yaml`：
   ```yaml
   mode: https
   acme:
     enabled: false
   ```

### 4.3 自动续签（DNS-01，推荐）

修改 `$REPO_ROOT/https.yaml`：

```yaml
mode: https
acme:
  enabled: true
  domain: git.example.com
  challenge: dns-01
  dns:
    provider: tencentcloud
    credentials:
      secret_id: "YOUR_SECRET_ID"
      secret_key: "YOUR_SECRET_KEY"
```

支持的 DNS 提供商：

| 提供商 | provider | 凭证 |
|--------|----------|------|
| 腾讯云 | `tencentcloud` | `secret_id`, `secret_key` |
| 阿里云 | `alidns` | `access_key_id`, `access_key_secret` |
| Cloudflare | `cloudflare` | `api_token` |

### 4.4 证书管理操作

#### 查看证书信息

```bash
curl -H "Authorization: token YOUR_TOKEN" \
     https://your-domain:61080/api/v1/admin/certs/info
```

响应示例：
```json
{
  "domain": ["your-domain.com"],
  "remaining_days": 89,
  "needs_renewal": false
}
```

#### 手动强制续签

```bash
curl -X POST -H "Authorization: token YOUR_TOKEN" \
     https://your-domain:61080/api/v1/admin/certs/renew
```

#### 证书备份位置

续签前会自动备份到：
```
$REPO_ROOT/certs/archive/YYYYMMDD-HHMMSS/
├── cert.pem
└── key.pem
```

---

## 五、Systemd 服务（Linux）

### 5.1 创建服务文件

```bash
sudo vim /etc/systemd/system/potstack.service
```

```ini
[Unit]
Description=PotStack Service
After=network.target

[Service]
Type=simple
User=potstack
WorkingDirectory=/opt/potstack
Environment="POTSTACK_REPO_ROOT=/opt/potstack/data"
Environment="POTSTACK_HTTP_PORT=61080"
Environment="POTSTACK_TOKEN=your-secret-token"
ExecStart=/opt/potstack/potstack-linux
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### 5.2 启用服务

```bash
sudo systemctl daemon-reload
sudo systemctl enable potstack
sudo systemctl start potstack
```

### 5.3 查看状态

```bash
sudo systemctl status potstack
sudo journalctl -u potstack -f
```

---

## 六、Docker 部署

### 6.1 Dockerfile

```dockerfile
FROM ubuntu:22.04

WORKDIR /app

COPY potstack-linux /app/potstack
COPY https.yaml.example /app/

RUN chmod +x /app/potstack

ENV POTSTACK_REPO_ROOT=/data
ENV POTSTACK_HTTP_PORT=61080

EXPOSE 61080

VOLUME ["/data"]

CMD ["/app/potstack"]
```

### 6.2 构建和运行

```bash
docker build -t potstack .

docker run -d \
  --name potstack \
  -p 61080:61080 \
  -e POTSTACK_TOKEN=your-secret-token \
  -v /path/to/data:/data \
  potstack
```

### 6.3 Docker Compose

```yaml
version: '3'
services:
  potstack:
    image: potstack
    ports:
      - "61080:61080"
    environment:
      - POTSTACK_TOKEN=your-secret-token
    volumes:
      - ./data:/data
    restart: always
```

---

## 七、初始化系统仓库

首次部署后，需要初始化系统仓库：

### 7.1 使用 Loader

```go
import "potstack/internal/loader"

cfg := &loader.Config{
    PotStackURL:  "http://localhost:61080",
    Token:        "your-secret-token",
    BasePackPath: "potstack-base.zip",
}

l := loader.New(cfg)
if err := l.Initialize(); err != nil {
    log.Fatal(err)
}
```

### 7.2 手动初始化

```bash
# 创建系统用户
curl -X POST http://localhost:61080/api/v1/admin/users \
  -H "Authorization: token YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username": "potstack"}'

# 创建系统仓库
curl -X POST http://localhost:61080/api/v1/admin/users/potstack/repos \
  -H "Authorization: token YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "keeper"}'

curl -X POST http://localhost:61080/api/v1/admin/users/potstack/repos \
  -H "Authorization: token YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "loader"}'

curl -X POST http://localhost:61080/api/v1/admin/users/potstack/repos \
  -H "Authorization: token YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "repo"}'
```

---

## 八、健康检查

```bash
curl http://localhost:61080/health
```

返回：
```json
{"status": "UP", "service": "potstack"}
```

---

## 九、故障排查

### 9.1 查看日志

```bash
tail -f $REPO_ROOT/log/potstack.log
```

### 9.2 常见问题

| 问题 | 原因 | 解决方案 |
|------|------|---------|
| 端口被占用 | 其他服务占用 | 修改 `POTSTACK_HTTP_PORT` |
| 权限错误 | 目录权限不足 | `chmod -R 755 $REPO_ROOT` |
| 数据库错误 | 数据库损坏 | 删除重建 `potstack.db` |
| 证书申请失败 | DNS 配置错误 | 检查 DNS 凭证和域名 |

### 9.3 调试模式

```bash
export GIN_MODE=debug
./potstack-linux
```
