# 部署设计

## 1. 部署目标

平台必须支持两种部署形态：

- 二进制部署：下载可执行文件和配置文件后直接运行。
- Docker 部署：通过容器运行平台服务。

MVP 目标：

- 单实例部署。
- 前端静态资源由后端服务托管。
- 默认 SQLite，生产可配置 PostgreSQL。
- 支持本机构建和 SSH 远程构建。
- 支持健康检查、配置文件、数据目录、日志目录、备份和升级说明。

MVP 不支持：

- 高可用多实例。
- Kubernetes Helm Chart。
- 外部对象存储。
- 分布式队列。
- 自动数据库主从。

## 2. 部署产物

### 2.1 二进制发布包

建议发布包结构：

```text
image-build-platform_{version}_{os}_{arch}.tar.gz
└── image-build-platform
    ├── ibp-server
    ├── config.example.yaml
    ├── README.md
    ├── LICENSE
    └── scripts
        ├── install-systemd.sh
        └── backup.sh
```

说明：

- `ibp-server` 是 Go 后端二进制。
- 前端静态资源优先内嵌到二进制。
- `config.example.yaml` 是默认配置示例。
- `scripts` 提供可选运维脚本，不作为运行必需。

### 2.2 Docker 镜像

Docker 镜像包含：

- `ibp-server`。
- 内嵌前端静态资源。
- Docker CLI。
- OpenSSH client。
- CA certificates。
- timezone data。

Docker 镜像不包含：

- Docker daemon。
- 内置 Registry。
- 数据库服务。

## 3. 运行依赖

### 3.1 二进制部署依赖

必需：

- Linux amd64 或 arm64。
- 可写数据目录。
- 可用端口，默认 `8080`。

如需本机构建：

- 已安装 Docker CLI。
- 后端运行用户可访问 Docker daemon。

如需 SSH 构建：

- 后端服务可访问远程主机 SSH 端口。
- 远程主机已安装 Docker。
- 远程构建用户有 Docker 权限。

### 3.2 Docker 部署依赖

必需：

- Docker Engine。
- 可写数据卷。

如需本机构建：

- 挂载宿主机 Docker socket：`/var/run/docker.sock`。

如只使用 SSH 远程构建：

- 不需要挂载 Docker socket。

## 4. 配置文件

默认配置文件路径：

```text
config.yaml
```

也可以通过参数指定：

```bash
./ibp-server --config /etc/image-build-platform/config.yaml
```

建议配置：

```yaml
server:
  addr: "0.0.0.0:8080"
  public_url: "http://localhost:8080"
  allowed_origins: []

database:
  driver: "sqlite"
  dsn: "data/app.db"

security:
  secret_key: "change-me-use-a-long-random-secret"
  session_ttl: "24h"
  secure_cookie: false
  csrf_enabled: true

storage:
  data_dir: "data"
  log_dir: "data/logs"
  context_dir: "data/contexts"
  tmp_dir: "data/tmp"
  backup_dir: "data/backups"

build:
  default_timeout: "1h"
  max_global_concurrency: 2
  enable_buildkit: true

logs:
  retention_days: 30

contexts:
  retention_days: 7
```

生产要求：

- `security.secret_key` 必须修改。
- 使用 HTTPS 时 `security.secure_cookie` 必须设为 `true`。
- 数据目录不能放在临时目录。

## 5. 环境变量

环境变量优先级高于配置文件。

建议支持：

```text
IBP_CONFIG
IBP_SERVER_ADDR
IBP_PUBLIC_URL
IBP_DATABASE_DRIVER
IBP_DATABASE_DSN
IBP_SECURITY_SECRET_KEY
IBP_DATA_DIR
IBP_LOG_DIR
IBP_CONTEXT_DIR
IBP_MAX_GLOBAL_CONCURRENCY
```

Docker 部署建议至少配置：

```text
IBP_SECURITY_SECRET_KEY
IBP_PUBLIC_URL
```

## 6. 数据目录

默认数据目录：

```text
data
├── app.db
├── contexts
├── logs
│   └── builds
├── tmp
└── backups
```

权限要求：

- 仅平台运行用户可读写。
- 不作为静态文件目录暴露。
- Docker 部署时使用持久化 volume。

## 7. 二进制部署

### 7.1 快速启动

```bash
tar -xzf image-build-platform_0.1.0_linux_amd64.tar.gz
cd image-build-platform
cp config.example.yaml config.yaml
./ibp-server --config config.yaml
```

访问：

```text
http://localhost:8080
```

首次访问时进入管理员初始化页面。

### 7.2 systemd 部署

建议目录：

```text
/opt/image-build-platform
├── ibp-server
├── config.yaml
└── data
```

systemd 示例：

```ini
[Unit]
Description=Image Build Platform
After=network.target

[Service]
Type=simple
User=ibp
Group=ibp
WorkingDirectory=/opt/image-build-platform
ExecStart=/opt/image-build-platform/ibp-server --config /opt/image-build-platform/config.yaml
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

启动：

```bash
sudo systemctl daemon-reload
sudo systemctl enable image-build-platform
sudo systemctl start image-build-platform
```

### 7.3 本机构建权限

如果二进制部署需要本机构建：

```bash
sudo usermod -aG docker ibp
sudo systemctl restart image-build-platform
```

注意：

- 加入 `docker` 组意味着该用户具备较高宿主机控制权限。
- 生产环境建议使用专用构建主机，而不是平台服务所在主机。

## 8. Docker 部署

### 8.1 仅使用 SSH 远程构建

不挂载 Docker socket：

```bash
docker run -d \
  --name image-build-platform \
  -p 8080:8080 \
  -e IBP_SECURITY_SECRET_KEY="change-me-use-a-long-random-secret" \
  -v ibp-data:/app/data \
  ghcr.io/vmythv/image-build-platform:latest
```

这种模式下：

- 平台容器不能访问宿主机 Docker。
- 需要添加 SSH 构建主机后才能构建。

### 8.2 使用宿主机 Docker 构建

挂载 Docker socket：

```bash
docker run -d \
  --name image-build-platform \
  -p 8080:8080 \
  -e IBP_SECURITY_SECRET_KEY="change-me-use-a-long-random-secret" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ibp-data:/app/data \
  ghcr.io/vmythv/image-build-platform:latest
```

风险：

- 挂载 Docker socket 后，平台容器可以控制宿主机 Docker daemon。
- 该模式只建议在可信环境使用。

### 8.3 docker compose

SQLite 版本：

```yaml
services:
  image-build-platform:
    image: ghcr.io/vmythv/image-build-platform:latest
    container_name: image-build-platform
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      IBP_PUBLIC_URL: "http://localhost:8080"
      IBP_SECURITY_SECRET_KEY: "change-me-use-a-long-random-secret"
      IBP_DATABASE_DRIVER: "sqlite"
      IBP_DATABASE_DSN: "data/app.db"
    volumes:
      - ibp-data:/app/data
      # Enable this only when local host Docker builds are required.
      # - /var/run/docker.sock:/var/run/docker.sock

volumes:
  ibp-data:
```

PostgreSQL 版本：

```yaml
services:
  image-build-platform:
    image: ghcr.io/vmythv/image-build-platform:latest
    container_name: image-build-platform
    restart: unless-stopped
    depends_on:
      - postgres
    ports:
      - "8080:8080"
    environment:
      IBP_PUBLIC_URL: "http://localhost:8080"
      IBP_SECURITY_SECRET_KEY: "change-me-use-a-long-random-secret"
      IBP_DATABASE_DRIVER: "postgres"
      IBP_DATABASE_DSN: "postgres://ibp:ibp-password@postgres:5432/ibp?sslmode=disable"
    volumes:
      - ibp-data:/app/data

  postgres:
    image: postgres:17
    restart: unless-stopped
    environment:
      POSTGRES_DB: ibp
      POSTGRES_USER: ibp
      POSTGRES_PASSWORD: ibp-password
    volumes:
      - ibp-postgres:/var/lib/postgresql/data

volumes:
  ibp-data:
  ibp-postgres:
```

## 9. PostgreSQL 配置

数据库 DSN 示例：

```text
postgres://ibp:password@127.0.0.1:5432/ibp?sslmode=disable
```

生产建议：

- 使用独立数据库用户。
- 限制数据库用户只访问平台数据库。
- 开启数据库备份。
- 跨主机访问时启用 TLS。

## 10. 反向代理和 HTTPS

生产环境建议使用 Nginx、Caddy 或 Traefik 终止 HTTPS。

Nginx 示例：

```nginx
server {
    listen 443 ssl http2;
    server_name ibp.example.com;

    ssl_certificate /etc/nginx/certs/ibp.crt;
    ssl_certificate_key /etc/nginx/certs/ibp.key;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /api/v1/build-tasks/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_buffering off;
        proxy_read_timeout 3600s;
    }
}
```

SSE 注意事项：

- 对日志流接口关闭代理缓冲。
- 增大 read timeout。
- 不要压缩事件流。

## 11. 健康检查

接口：

```text
GET /healthz
```

成功响应：

```json
{
  "status": "ok",
  "time": "2026-07-07T10:00:00Z",
  "version": "0.1.0"
}
```

Docker healthcheck 建议：

```dockerfile
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
```

## 12. 日志

服务日志：

- 输出到 stdout。
- systemd 或 Docker 负责采集。
- 建议使用结构化 JSON 日志，后续实现可选。

构建日志：

```text
data/logs/builds/{taskID}.log
```

日志保留：

- 默认 30 天。
- 可在系统设置中配置。

## 13. 备份

### 13.1 需要备份的内容

必须备份：

- 数据库。
- 配置文件。
- `security.secret_key`。

建议备份：

- 构建日志。
- 构建上下文。

可不备份：

- 临时目录。
- 任务临时 Docker config。

### 13.2 SQLite 备份

停止服务后备份最简单：

```bash
systemctl stop image-build-platform
tar -czf ibp-backup-$(date +%Y%m%d%H%M%S).tar.gz config.yaml data
systemctl start image-build-platform
```

在线备份后续可以通过 SQLite backup API 实现。

### 13.3 PostgreSQL 备份

```bash
pg_dump "$IBP_DATABASE_DSN" > ibp-$(date +%Y%m%d%H%M%S).sql
```

### 13.4 备份安全

- 备份文件包含凭据密文和可能敏感的构建日志。
- 备份必须加密保存。
- 丢失 `security.secret_key` 后无法解密已有凭据。

## 14. 恢复

SQLite 恢复：

```bash
systemctl stop image-build-platform
tar -xzf ibp-backup-20260707100000.tar.gz -C /opt/image-build-platform
systemctl start image-build-platform
```

恢复后检查：

- 服务能启动。
- 管理员能登录。
- 凭据能正常解密。
- 主机检测正常。
- 仓库检测正常。

## 15. 升级

### 15.1 二进制升级

步骤：

1. 阅读 release notes。
2. 备份数据库、配置和 secret key。
3. 停止服务。
4. 替换 `ibp-server`。
5. 启动服务。
6. 检查 `/healthz`。
7. 登录后台检查主机、仓库和构建任务。

示例：

```bash
sudo systemctl stop image-build-platform
cp ibp-server /opt/image-build-platform/ibp-server
sudo systemctl start image-build-platform
curl http://127.0.0.1:8080/healthz
```

### 15.2 Docker 升级

```bash
docker compose pull
docker compose up -d
```

升级前仍需备份数据卷和配置。

### 15.3 数据库迁移

应用启动时自动执行迁移。

要求：

- 迁移前检查数据库备份。
- 迁移失败时服务启动失败并输出明确错误。
- 不允许在 dirty migration 状态下继续运行。

## 16. 降级

MVP 不承诺自动降级。

如果需要降级：

- 使用升级前备份恢复数据库。
- 使用旧版本二进制或镜像。
- 确认配置文件兼容。

## 17. 运维任务

定期任务：

- 清理过期 session。
- 清理过期构建日志。
- 清理过期构建上下文。
- 清理临时目录。
- 刷新主机状态，MVP 可手动触发。
- 刷新仓库状态，MVP 可手动触发。

## 18. 发布流程

建议发布产物：

- Linux amd64 二进制包。
- Linux arm64 二进制包。
- Docker 镜像 amd64。
- Docker 镜像 arm64。
- docker compose 示例。
- checksums。

版本号：

```text
v0.1.0
```

镜像 tag：

```text
ghcr.io/vmythv/image-build-platform:v0.1.0
ghcr.io/vmythv/image-build-platform:latest
```

## 19. 部署验收

MVP 部署验收必须通过：

- 二进制启动成功。
- Docker 启动成功。
- 首次管理员初始化成功。
- SQLite 默认数据库可用。
- PostgreSQL 配置可用，若 MVP 实现 PostgreSQL。
- `/healthz` 返回 ok。
- Docker 部署不挂 socket 时，本机构建显示不可用。
- Docker 部署挂 socket 后，本机构建检测成功。
- SSH 远程主机检测成功。
- 构建日志能写入数据目录。
- 备份和恢复流程可执行。

