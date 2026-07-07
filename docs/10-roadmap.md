# 开发路线图

## 1. 路线图目标

路线图用于把当前需求和设计文档拆成可以执行的实现阶段。每个阶段都应产生可运行、可验证的增量，而不是只堆代码。

总体目标：

- 先搭起可运行骨架。
- 再打通用户、数据库、主机和仓库。
- 再实现镜像项目、版本图和 Dockerfile。
- 最后完成构建执行、日志、产物和发布。

## 2. 版本规划

### 2.1 v0.1.0 MVP

目标：

- 单实例私有化镜像构建平台。
- 本机 Docker socket 构建。
- SSH 远程构建。
- 通用 Docker Registry 推送。
- Git-style 版本图。
- SSE 实时日志。
- 二进制和 Docker 部署。

### 2.2 v0.2.0 增强版

候选能力：

- Agent 模式。
- Git 构建上下文。
- buildx 多架构构建。
- 项目级权限。
- Webhook。
- 更完整用户管理。

### 2.3 v0.3.0 安全和治理

候选能力：

- OIDC/LDAP。
- API token。
- 镜像漏洞扫描。
- SBOM。
- 策略审批。
- KMS/Vault。

## 3. MVP 里程碑

### M0 文档和架构基线

状态：已完成。

产物：

- `docs/01-requirements.md`
- `docs/02-architecture.md`
- `docs/03-mvp.md`
- `docs/04-ui-design.md`
- `docs/05-database-design.md`
- `docs/06-api.md`
- `docs/07-build-execution.md`
- `docs/08-security.md`
- `docs/09-deployment.md`
- `docs/10-roadmap.md`

完成标准：

- MVP 范围明确。
- 技术栈明确。
- 数据库、API、构建执行、安全和部署边界明确。

### M1 项目脚手架

目标：

- 建立可运行的前后端工程。

任务：

- 初始化 Go module。
- 建立 `cmd/ibp-server`。
- 建立 `internal` 模块目录。
- 建立 Vite React TypeScript 前端。
- 接入 shadcn/ui、Tailwind、lucide。
- 配置 Makefile。
- 配置基础 Dockerfile。
- 配置 GitHub Actions 基础 CI。
- 后端托管前端静态资源。
- 提供 `/healthz`。

完成标准：

- `make dev` 可以启动前后端开发环境。
- `make build` 可以构建前端和后端。
- 浏览器可以打开空 Dashboard。
- `/healthz` 返回 ok。

### M2 配置、数据库和迁移

目标：

- 平台可以读取配置，初始化数据库并执行迁移。

任务：

- 实现配置加载。
- 实现 SQLite 数据库连接。
- 预留 PostgreSQL driver。
- 实现 migration runner。
- 创建核心表。
- 实现 repository 基础结构。
- 实现统一 ID 生成。
- 实现统一时间处理。

完成标准：

- 首次启动自动创建数据库。
- 重启后数据保留。
- 迁移记录可查询。
- 配置文件和环境变量生效。

### M3 认证、初始化和 RBAC

目标：

- 后台具备基础安全入口。

任务：

- 首次管理员初始化。
- 登录。
- 退出。
- 当前用户接口。
- session 存储。
- 密码哈希。
- RBAC middleware。
- 登录页和初始化页。
- 前端登录态处理。

完成标准：

- 未登录访问后台跳转登录。
- 初始化管理员后可登录。
- viewer、maintainer、admin 权限有基础区分。
- session 过期后回到登录页。

### M4 构建主机管理

目标：

- 管理本机和 SSH 构建主机。

任务：

- BuildHost 表和 repository。
- 主机 CRUD API。
- Local Docker 检测。
- SSH 连接检测。
- 远程 Docker 检测。
- 主机状态展示。
- 主机列表和表单。
- Docker socket 风险提示。

完成标准：

- 可以检测本机 Docker。
- 可以添加 SSH 主机并检测 Docker。
- 可以看到架构、Docker 版本和状态。
- 禁用主机后不参与调度。

### M5 镜像仓库管理

目标：

- 管理可拉取和可推送的镜像仓库。

任务：

- Credential 加密存储。
- Registry 表和 repository。
- 仓库 CRUD API。
- 仓库登录检测。
- 默认推送仓库。
- 仓库列表和表单。
- 凭据脱敏展示。

完成标准：

- 可以添加 Harbor 或通用 Registry。
- 正确凭据检测成功。
- 错误凭据检测失败且错误可读。
- 前端不回显密码或 token。

### M6 镜像项目、版本和分支

目标：

- 支撑不同镜像项目的独立版本线。

任务：

- ImageProject 表和 API。
- ImageBranch 表和 API。
- ImageVersionNode 表和 API。
- 创建项目时初始化 main 分支。
- 起始镜像字段和展示。
- 项目列表和筛选。
- 版本节点详情。
- 创建子版本。
- 创建分支。

完成标准：

- Java、Python、Node.js 等项目独立展示。
- 创建项目后可以进入版本图。
- 从节点创建子版本后图数据正确。
- 从节点创建分支后图数据正确。

### M7 前端版本图和 Dockerfile 编辑

目标：

- 提供核心镜像版本工作台。

任务：

- 接入 `@xyflow/react`。
- 实现 Git-style 节点图。
- 实现节点详情抽屉。
- 实现版本图筛选。
- 实现 Monaco Dockerfile 编辑器。
- 实现 Dockerfile 校验 API。
- 实现表单生成 Dockerfile。
- 实现 Dockerfile diff。

完成标准：

- 版本图可缩放、拖拽、点击节点。
- 节点只展示少量信息。
- 详情面板展示 Dockerfile 和构建记录入口。
- Dockerfile 可编辑、保存和校验。

### M8 构建任务和调度

目标：

- 构建任务可以创建、排队和调度。

任务：

- BuildTask 表和 API。
- 构建任务状态机。
- 数据库队列。
- 调度器。
- 主机并发计数。
- 构建任务列表。
- 构建任务详情。
- 取消和重试接口。

完成标准：

- 从版本节点可以创建构建任务。
- 任务能进入队列。
- 调度器选择正确架构主机。
- 状态流转准确。
- 取消和重试可用。

### M9 本机 Docker 构建

目标：

- 打通本机构建闭环。

任务：

- 构建上下文生成。
- LocalDockerExecutor。
- Docker CLI 参数生成。
- `docker build` 执行。
- `docker image inspect`。
- 本地临时 Docker config。
- 构建失败错误分类。
- 构建后清理。

完成标准：

- 使用二进制部署可本机构建最小镜像。
- Docker 部署挂 socket 后可本机构建。
- Dockerfile 错误能显示为 `build_failed`。
- 超时和取消能终止构建。

### M10 SSH 远程构建

目标：

- 打通 SSH 远程构建闭环。

任务：

- SSH client 封装。
- SFTP 上传上下文。
- 远程临时目录管理。
- 远程 `docker build`。
- 远程清理。
- SSH 失败错误分类。

完成标准：

- 可以在远程 Linux 主机构建镜像。
- SSH 连接失败显示可读错误。
- 上传失败显示可读错误。
- 任务结束后远程临时目录被清理。

### M11 实时日志

目标：

- 构建过程可实时观察。

任务：

- LogSink。
- 日志文件写入。
- 日志序号。
- 日志脱敏。
- SSE endpoint。
- 前端日志查看器。
- 自动滚动、暂停、搜索、下载。

完成标准：

- 构建中前端实时显示日志。
- SSE 断线可重连。
- 构建完成后历史日志可查看。
- 日志下载可用。

### M12 推送和产物管理

目标：

- 构建成功后推送镜像并记录产物。

任务：

- `docker login --password-stdin`。
- `docker tag`。
- `docker push`。
- digest 提取。
- ImageArtifact 表和 API。
- artifact push events。
- 产物列表和详情。
- 复制 pull 命令。
- 重新推送。

完成标准：

- 构建成功后推送到指定仓库。
- 推送失败状态为 `push_failed`。
- 产物记录包含 image ref、tag、digest、架构。
- 可以复制 `docker pull` 命令。

### M13 Dashboard、设置、帮助和审计

目标：

- 平台具备完整后台体验。

任务：

- Dashboard summary API。
- Settings API。
- 审计日志记录。
- 审计日志列表。
- Help 页面。
- 构建失败排查说明。
- Docker socket 风险说明。

完成标准：

- Dashboard 能看到任务、主机、仓库概览。
- 设置可调整全局并发和保留天数。
- 关键操作进入审计日志。
- 用户能在 Help 找到主机、仓库、架构和失败排查说明。

### M14 打包、部署和验收

目标：

- 发布可运行 MVP。

任务：

- 前端生产构建。
- 后端内嵌前端资源。
- 二进制构建。
- Docker 镜像构建。
- docker compose 示例。
- 配置文件示例。
- systemd 示例。
- README 快速启动。
- 手工验收场景。

完成标准：

- 二进制部署可运行。
- Docker 部署可运行。
- 本机构建闭环通过。
- SSH 远程构建闭环通过。
- 推送失败和构建失败场景可排查。
- release 包含 checksums。

## 4. 建议实现顺序

严格顺序：

1. M1 项目脚手架。
2. M2 配置、数据库和迁移。
3. M3 认证、初始化和 RBAC。
4. M4 构建主机管理。
5. M5 镜像仓库管理。
6. M6 镜像项目、版本和分支。
7. M7 前端版本图和 Dockerfile 编辑。
8. M8 构建任务和调度。
9. M9 本机 Docker 构建。
10. M11 实时日志。
11. M12 推送和产物管理。
12. M10 SSH 远程构建。
13. M13 Dashboard、设置、帮助和审计。
14. M14 打包、部署和验收。

说明：

- M10 可以和 M9 后半段并行，但建议先完成本机构建。
- M11 日志也可以从 M9 开始同步开发。
- M12 推送依赖 M5 仓库和 M9 构建。

## 5. 测试计划

### 5.1 后端测试

必须覆盖：

- 配置加载。
- 数据库迁移。
- 密码哈希。
- session。
- RBAC。
- 凭据加密和解密。
- 主机检测结果解析。
- 仓库检测结果解析。
- Dockerfile 校验。
- 镜像引用拼装。
- 构建任务状态机。
- 主机调度策略。
- Docker 命令参数生成。
- 日志脱敏。

### 5.2 前端测试

建议覆盖：

- 登录页。
- 初始化页。
- 主机表单校验。
- 仓库表单校验。
- 镜像项目创建。
- 版本图节点点击。
- Dockerfile 编辑保存。
- 构建任务创建。
- 日志查看器状态。

### 5.3 端到端测试

MVP 至少覆盖：

- 初始化管理员并登录。
- 添加本机构建主机。
- 添加仓库。
- 创建镜像项目。
- 创建版本节点。
- 发起构建。
- 查看日志。
- 查看产物。

## 6. 发布检查清单

发布 v0.1.0 前必须确认：

- 所有 MVP 验收场景通过。
- 数据库迁移从空库可成功执行。
- 旧版本升级迁移可成功执行，如果已有旧版本。
- 二进制包可启动。
- Docker 镜像可启动。
- Docker socket 风险提示存在。
- 凭据不明文存储。
- 构建日志不泄漏测试凭据。
- README 有快速启动说明。
- release notes 说明已知限制。

## 7. 风险和缓解

### 7.1 构建执行复杂度高

风险：

- Docker CLI、SSH、日志、取消和清理交织，容易出现边界问题。

缓解：

- 先完成本机构建。
- Executor 接口稳定后再扩展 SSH。
- 状态机和命令参数生成必须有单元测试。

### 7.2 Docker socket 权限高

风险：

- Docker 部署挂 socket 后容器可控制宿主机 Docker。

缓解：

- UI 和文档强提示。
- 推荐生产使用专用远程构建主机。
- 后续实现 Agent。

### 7.3 版本图交互复杂

风险：

- 图布局和节点详情容易拖慢 MVP。

缓解：

- MVP 只做清晰节点连线和详情抽屉。
- 不做复杂合并、跨项目依赖图和自动布局高级能力。

### 7.4 SQLite 到 PostgreSQL 差异

风险：

- JSON、时间和布尔字段存在差异。

缓解：

- Repository 层屏蔽差异。
- 表结构避免复杂数据库特性。
- 早期加入 PostgreSQL 集成测试。

## 8. v0.1.0 完成定义

v0.1.0 完成需要满足：

- `docs/03-mvp.md` 的完成定义全部满足。
- `docs/09-deployment.md` 的部署验收全部满足。
- 本机构建和 SSH 构建各至少通过一次完整闭环。
- 构建失败、推送失败、取消和超时场景可验证。
- 用户可以从后台完成主机、仓库、项目、版本、构建、日志、产物全流程。
- GitHub Release 提供二进制包、Docker 镜像和部署说明。
