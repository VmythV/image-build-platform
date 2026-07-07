# 镜像构建平台架构设计

## 1. 已确认技术决策

| 方向 | 决策 |
| --- | --- |
| 后端语言 | Go |
| 后端形态 | 模块化单体，后续可拆 Agent |
| HTTP 路由 | `net/http` + `chi` |
| API 风格 | REST API |
| 实时日志 | SSE |
| 前端框架 | React + TypeScript + Vite |
| 前端数据请求 | TanStack Query |
| UI 风格 | 国际化开源风格 |
| UI 组件 | shadcn/ui + Radix UI + Tailwind CSS + lucide-react |
| 版本图 | @xyflow/react，React Flow 当前包名 |
| 默认数据库 | SQLite |
| 生产数据库 | PostgreSQL 可选 |
| 部署形态 | 单二进制 + Docker 镜像 |
| 一期构建方式 | 本机 Docker socket + SSH 远程主机 |
| 二期构建方式 | Agent、buildx 多架构、Git 构建上下文 |

## 2. 总体架构

一期采用前后端同仓库、单后端进程部署：

```text
Browser
  |
  | HTTP REST / SSE
  v
Go Backend
  |-- API and Auth
  |-- Build Scheduler
  |-- Local Docker Executor
  |-- SSH Docker Executor
  |-- Log Stream Hub
  |-- Static Frontend Assets
  |
  |-- SQLite / PostgreSQL
  |-- Data Directory
        |-- build contexts
        |-- build logs
        |-- temp docker config
```

设计原则：

- 单体优先，模块边界清晰。
- 构建执行、日志、凭据和任务状态机是核心复杂度，优先保证可靠性。
- 不在一期引入 Redis、消息队列、Kubernetes 或微服务。
- 远程 Agent 作为二期能力，不影响一期本机和 SSH 构建闭环。
- 所有敏感凭据只在后端保存和使用，前端只展示脱敏状态。

## 3. 仓库结构

建议目录：

```text
image-build-platform
├── cmd
│   └── ibp-server
│       └── main.go
├── internal
│   ├── app
│   ├── server
│   ├── auth
│   ├── host
│   ├── registry
│   ├── imageproject
│   ├── build
│   ├── executor
│   ├── artifact
│   ├── logstream
│   ├── audit
│   ├── storage
│   └── config
├── web
│   ├── src
│   ├── public
│   └── package.json
├── docs
├── scripts
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```

说明：

- `cmd/ibp-server` 只负责启动、配置加载和依赖组装。
- `internal` 按业务领域拆分，避免把逻辑堆到 handler。
- `web` 是前端 SPA。
- 后端发布时将 `web/dist` 内嵌或打包到二进制旁边。

## 4. 后端架构

### 4.1 模块划分

```text
internal/server       HTTP server, route mounting, middleware
internal/auth         login, session, RBAC
internal/host         build host CRUD, detection, connection test
internal/registry     registry CRUD, credential usage, login test
internal/imageproject project, root image, version node, branch graph
internal/build        build task, scheduler, state machine
internal/executor     local docker and SSH docker execution
internal/artifact     image artifact records
internal/logstream    SSE stream, log writer, log reader
internal/audit        audit records
internal/storage      DB connection, repositories, migrations
internal/config       config file and env loading
```

每个业务模块建议包含：

```text
handler.go      HTTP handler
service.go      business logic
repository.go   persistence interface
model.go        domain model
dto.go          API request/response DTO
```

### 4.2 HTTP API

API 统一挂载在 `/api/v1`：

```text
POST   /api/v1/auth/login
POST   /api/v1/auth/logout
GET    /api/v1/auth/me

GET    /api/v1/build-hosts
POST   /api/v1/build-hosts
POST   /api/v1/build-hosts/{id}/check

GET    /api/v1/registries
POST   /api/v1/registries
POST   /api/v1/registries/{id}/check

GET    /api/v1/image-projects
POST   /api/v1/image-projects
GET    /api/v1/image-projects/{id}/graph
GET    /api/v1/image-projects/{id}/version-nodes/{nodeId}

POST   /api/v1/build-tasks
GET    /api/v1/build-tasks
GET    /api/v1/build-tasks/{id}
POST   /api/v1/build-tasks/{id}/cancel
POST   /api/v1/build-tasks/{id}/retry
GET    /api/v1/build-tasks/{id}/logs
GET    /api/v1/build-tasks/{id}/logs/stream

GET    /api/v1/artifacts
GET    /api/v1/artifacts/{id}
POST   /api/v1/artifacts/{id}/repush

GET    /api/v1/audit-logs
GET    /api/v1/settings
PUT    /api/v1/settings
GET    /healthz
```

`chi` 负责主路由和模块子路由挂载。每个模块只暴露自己的 route 注册函数，例如：

```text
server
  Mount /api/v1/build-hosts -> host.Router
  Mount /api/v1/registries  -> registry.Router
  Mount /api/v1/build-tasks -> build.Router
```

### 4.3 认证和权限

一期建议采用服务端 session：

- 登录成功后写入 HttpOnly Cookie。
- Session 存储在数据库中。
- 前端不直接持有明文 token。
- API token 作为二期能力。

基础角色：

- `admin`：系统管理员。
- `maintainer`：镜像维护者。
- `viewer`：只读查看者。

权限控制放在 service 层和路由 middleware 两处：

- middleware 处理是否登录、基础角色。
- service 层处理具体资源权限，例如某个镜像项目的维护权限。

### 4.4 数据库和迁移

默认 SQLite，生产可切 PostgreSQL。

建议做法：

- Repository 层隐藏数据库差异。
- 数据库表设计以 PostgreSQL 能力为上限，但避免一期依赖复杂特性。
- 迁移文件放在 `internal/storage/migrations`。
- SQLite 数据文件默认放在 `data/app.db`。
- 配置中通过 `database.driver` 和 `database.dsn` 切换数据库。

核心表：

- `users`
- `sessions`
- `credentials`
- `build_hosts`
- `registries`
- `image_projects`
- `image_branches`
- `image_version_nodes`
- `build_tasks`
- `image_artifacts`
- `audit_logs`
- `system_settings`

### 4.5 配置

配置来源优先级：

1. 命令行参数。
2. 环境变量。
3. 配置文件。
4. 默认值。

建议配置文件：

```yaml
server:
  addr: "0.0.0.0:8080"
  public_url: "http://localhost:8080"

database:
  driver: "sqlite"
  dsn: "data/app.db"

security:
  secret_key: ""
  session_ttl: "24h"

storage:
  data_dir: "data"
  log_dir: "data/logs"
  context_dir: "data/contexts"

build:
  default_timeout: "1h"
  max_global_concurrency: 2
  enable_buildkit: true
```

`security.secret_key` 必须用于凭据加密和 cookie 签名。生产环境不允许使用空值。

## 5. 构建执行架构

### 5.1 执行器接口

后端通过统一接口调用不同执行器：

```text
Executor
  Check(ctx, host) -> HostCheckResult
  Build(ctx, task, context) -> BuildResult
  Push(ctx, task, artifact, registry) -> PushResult
  Cancel(ctx, taskID)
```

一期执行器：

- `LocalDockerExecutor`
- `SSHDockerExecutor`

二期执行器：

- `AgentExecutor`
- `BuildxExecutor`

### 5.2 本机构建

本机构建指后端所在环境可以访问 Docker daemon：

- 二进制部署：后端直接使用运行机器上的 Docker。
- Docker 部署：通过挂载 `/var/run/docker.sock` 访问宿主机 Docker。

平台容器不内置 Docker daemon，只包含 Docker CLI 和必要客户端工具。容器部署时如需本机构建，需要显式挂载 Docker socket。

### 5.3 SSH 远程构建

SSH 构建流程：

1. 后端生成构建上下文目录。
2. 通过 SSH/SFTP 将 Dockerfile 和上下文上传到远程主机临时目录。
3. 在远程主机执行 `docker build`。
4. 实时读取 stdout/stderr 并写入构建日志。
5. 构建成功后执行 `docker tag`、`docker login`、`docker push`。
6. 清理远程临时目录和临时 Docker 配置。

远程主机要求：

- Linux。
- 已安装 Docker。
- 构建用户有 Docker 权限。
- 可通过 SSH 登录。
- 磁盘空间满足构建上下文和镜像层写入。

### 5.4 Docker CLI 使用约定

一期优先通过 Docker CLI 执行构建，而不是直接操作 Docker Engine API。

原因：

- 本机和 SSH 远程主机路径一致。
- 更容易兼容用户已有 Docker 配置和 BuildKit。
- 日志输出更接近用户直接执行命令时看到的内容。

安全约定：

- 仓库密码通过 `docker login --password-stdin` 输入。
- 每个构建任务使用独立临时 `DOCKER_CONFIG`。
- 任务结束后清理临时 `DOCKER_CONFIG`。
- 不把密码、token、SSH 私钥写入构建日志。

### 5.5 构建任务状态机

状态流转：

```text
created
  -> queued
  -> preparing_context
  -> dispatching
  -> building
  -> build_success
  -> pushing
  -> push_success

失败状态：
  preparing_context_failed
  dispatch_failed
  build_failed
  push_failed
  canceled
  timeout
```

数据库中保存状态和关键时间：

- `queued_at`
- `started_at`
- `build_started_at`
- `build_finished_at`
- `push_started_at`
- `finished_at`

### 5.6 调度策略

一期调度策略：

1. 过滤禁用或离线主机。
2. 过滤架构不匹配的主机。
3. 过滤标签不匹配的主机。
4. 过滤达到并发上限的主机。
5. 在剩余主机中选择当前运行任务最少的主机。

构建任务允许用户指定：

- 固定主机。
- 按架构自动选择。
- 按标签自动选择。

### 5.7 日志架构

构建日志采用文件存储 + SSE 实时推送：

- 日志文件：`data/logs/builds/{taskID}.log`
- 每一行带递增序号、时间、来源和级别。
- 后端写文件的同时推送到内存订阅者。
- 前端通过 SSE 订阅 `/api/v1/build-tasks/{id}/logs/stream`。
- 构建完成后，历史日志通过普通 HTTP 分页读取。

日志事件建议格式：

```json
{
  "seq": 128,
  "time": "2026-07-07T10:00:00Z",
  "source": "docker-build",
  "level": "info",
  "message": "Step 3/10 : RUN apt-get update"
}
```

## 6. 前端架构

### 6.1 应用结构

```text
web/src
├── app
│   ├── router.tsx
│   ├── query-client.ts
│   └── providers.tsx
├── pages
│   ├── dashboard
│   ├── build-hosts
│   ├── registries
│   ├── image-projects
│   ├── version-graph
│   ├── build-tasks
│   ├── artifacts
│   ├── settings
│   └── help
├── components
│   ├── ui
│   ├── layout
│   ├── forms
│   ├── dockerfile-editor
│   ├── log-viewer
│   └── version-graph
├── api
├── hooks
├── lib
├── styles
└── i18n
```

### 6.2 UI 组件策略

采用 shadcn/ui 的原因：

- 组件代码归项目所有，便于深度定制。
- 基于 Radix UI，适合构建可访问的后台组件。
- Tailwind CSS 适合快速建立统一设计 token。
- lucide-react 提供一致的图标体系。

设计风格：

- 后台管理系统以效率和扫描为主。
- 避免营销站式大面积 hero 和装饰图形。
- 列表、表格、抽屉、弹窗、详情页应保持信息密度。
- 版本图页面使用全屏工作台布局，不把图放进装饰卡片。

### 6.3 页面信息架构

一级导航：

- Dashboard
- Build Hosts
- Registries
- Image Projects
- Build Tasks
- Artifacts
- Settings
- Help

核心页面：

- `Build Hosts`：主机列表、连接检测、架构检测、Docker 能力检测。
- `Registries`：仓库列表、凭据配置、登录测试。
- `Image Projects`：镜像目录、类型筛选、起始镜像选择。
- `Version Graph`：Git-style 节点图、节点详情、版本操作。
- `Build Tasks`：任务队列、任务详情、实时日志。
- `Artifacts`：镜像产物列表、复制拉取命令、重新推送。

### 6.4 版本图界面

版本图采用 `@xyflow/react`。

布局：

```text
┌─────────────────────────────────────────────────────────────┐
│ Project selector / branch filter / status filter / actions  │
├───────────────┬───────────────────────────────┬─────────────┤
│ image list    │ Git-style version graph       │ detail      │
│ and filters   │ nodes and edges               │ drawer      │
└───────────────┴───────────────────────────────┴─────────────┘
```

图上只展示少量信息：

- version/tag
- branch
- build status
- created time

点击节点后，在右侧详情面板展示：

- Dockerfile。
- 表单配置快照。
- 版本说明。
- 来源节点。
- 构建记录。
- 镜像产物。
- 新建版本、创建分支、重新构建、比较差异等操作。

图数据由后端返回，前端只负责布局和交互：

```json
{
  "nodes": [
    {
      "id": "node_1",
      "version": "jdk17-v1",
      "branch": "main",
      "status": "success",
      "createdAt": "2026-07-07T10:00:00Z"
    }
  ],
  "edges": [
    {
      "id": "edge_1_2",
      "from": "node_1",
      "to": "node_2",
      "type": "parent"
    }
  ]
}
```

### 6.5 API 状态管理

TanStack Query 负责服务端状态：

- 列表查询。
- 详情查询。
- 创建、更新、删除后的缓存失效。
- 构建状态轮询。
- 主机和仓库检测结果刷新。

本地 UI 状态只保存：

- 当前选中的镜像项目。
- 当前选中的图节点。
- 抽屉打开状态。
- 筛选条件。
- 主题和语言。

本地状态不要保存后端业务数据的副本。

### 6.6 实时日志界面

构建日志通过 SSE 接入：

```text
GET /api/v1/build-tasks/{id}/logs/stream
```

前端行为：

- 打开构建任务详情时建立 SSE 连接。
- 断线后自动重连。
- 支持暂停自动滚动。
- 支持搜索、下载、复制。
- 构建完成后关闭 SSE，切换为历史日志读取。

长日志需要使用虚拟列表，避免 DOM 节点过多导致页面卡顿。

### 6.7 Dockerfile 编辑器

Dockerfile 编辑器建议使用 Monaco Editor：

- Dockerfile 语法高亮。
- 只读查看模式。
- 编辑模式。
- 差异对比模式。

表单构建模式和源码模式共享同一个版本节点：

- 表单模式生成 Dockerfile。
- 用户切换源码模式后可以自由编辑。
- 源码被手动修改后，前端提示可能无法完整反向还原到表单。

## 7. 数据和文件存储

默认数据目录：

```text
data
├── app.db
├── contexts
│   └── {taskID}
├── logs
│   └── builds
│       └── {taskID}.log
├── tmp
└── backups
```

存储策略：

- 数据库存储结构化业务数据。
- 构建日志存文件。
- 构建上下文存文件并按策略清理。
- 凭据密文存数据库。
- 临时 Docker 配置存 `data/tmp` 并在任务结束后删除。

## 8. 部署架构

### 8.1 二进制部署

二进制部署包包含：

- `ibp-server`
- 前端静态资源，内嵌或随包。
- 配置文件示例。
- systemd service 示例。

依赖：

- 如需本机构建，运行机器需要安装 Docker CLI 并可访问 Docker daemon。
- 如需 SSH 构建，需要后端可访问远程主机 SSH 端口。

### 8.2 Docker 部署

Docker 镜像包含：

- 后端二进制。
- 前端静态资源。
- Docker CLI。
- OpenSSH client。
- CA certificates。

Docker 镜像不包含 Docker daemon。

本机构建示例：

```bash
docker run -d \
  --name image-build-platform \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ibp-data:/app/data \
  image-build-platform:latest
```

不挂载 Docker socket 时，平台仍可通过 SSH 远程构建主机执行构建。

## 9. 一期实现边界

一期必须完成：

- Go 后端模块化单体。
- React 管理后台。
- 用户登录和基础 RBAC。
- SQLite 默认存储。
- 本机 Docker socket 构建。
- SSH 远程构建。
- 通用 Docker Registry 凭据配置和登录测试。
- 镜像项目、起始镜像、版本节点和分支。
- Git-style 版本图。
- Dockerfile 编辑。
- 简单表单生成 Dockerfile。
- 构建任务队列。
- SSE 实时日志。
- 构建成功后推送镜像。
- 镜像产物管理。
- 单二进制和 Docker 镜像发布。

一期不做：

- 多后端实例。
- Kubernetes 原生调度。
- Agent。
- buildx 多架构 manifest。
- 漏洞扫描。
- SBOM。
- Git 仓库构建上下文。
- 多租户。

## 10. 后续演进

### 10.1 Agent 模式

Agent 适合解决以下问题：

- 平台不直接保存高权限 SSH 私钥。
- 构建主机主动连接平台，便于穿透内网。
- 更可靠地采集主机状态和构建日志。
- 支持更大规模的构建主机池。

### 10.2 多架构构建

二期支持 buildx：

- 单任务构建多个平台。
- 推送 manifest list。
- 管理 builder instance。
- 支持 QEMU 检测。

### 10.3 Git 构建上下文

后续支持：

- Git URL。
- 分支、tag、commit。
- 子目录作为构建上下文。
- Git 凭据。
- Webhook 触发构建。
