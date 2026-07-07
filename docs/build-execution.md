# 构建执行设计

## 1. 目标

构建执行模块负责把用户提交的 Dockerfile、构建上下文、目标架构、构建主机和镜像仓库配置转化为可追踪、可取消、可排错的构建任务。

MVP 目标：

- 支持本机 Docker socket 构建。
- 支持 SSH 远程 Docker 构建。
- 支持实时日志采集和 SSE 推送。
- 支持构建成功后推送镜像。
- 支持任务取消、超时和重试。
- 支持任务结束后的本地和远程临时资源清理。
- 支持敏感值脱敏，避免凭据进入日志。

MVP 不做：

- Agent。
- buildx 多架构 manifest。
- Kubernetes 调度。
- Docker API TCP 构建。
- Git 构建上下文。
- 分布式队列。

## 2. 核心组件

```text
Build API
  -> Build Service
  -> Scheduler
  -> Build Queue
  -> Executor
       |-- LocalDockerExecutor
       |-- SSHDockerExecutor
  -> Log Writer
  -> Artifact Service
  -> Audit Service
```

### 2.1 Build Service

职责：

- 校验构建请求。
- 创建构建任务。
- 保存 Dockerfile 快照和构建参数快照。
- 将任务放入队列。
- 控制任务取消和重试。
- 维护任务状态机。

### 2.2 Scheduler

职责：

- 根据架构、主机标签、主机状态、并发上限选择构建主机。
- 锁定构建主机并增加运行计数。
- 调度失败时写入 `dispatch_failed`。

### 2.3 Executor

职责：

- 检查目标主机 Docker 能力。
- 准备构建上下文。
- 执行 `docker build`。
- 执行 `docker tag`。
- 执行 `docker login`。
- 执行 `docker push`。
- 采集 stdout/stderr。
- 响应取消和超时。
- 清理临时目录和临时 Docker 配置。

### 2.4 Log Writer

职责：

- 接收构建过程中的结构化日志事件。
- 写入日志文件。
- 推送给 SSE 订阅者。
- 对敏感值进行脱敏。

## 3. Executor 接口

建议接口：

```go
type Executor interface {
    Check(ctx context.Context, host BuildHost) (HostCheckResult, error)
    Prepare(ctx context.Context, task BuildTask) (PreparedContext, error)
    Build(ctx context.Context, task BuildTask, prepared PreparedContext, logs LogSink) (BuildResult, error)
    Push(ctx context.Context, task BuildTask, result BuildResult, registry Registry, logs LogSink) (PushResult, error)
    Cleanup(ctx context.Context, prepared PreparedContext, logs LogSink) error
    Cancel(taskID string) error
}
```

接口要求：

- 所有方法必须支持 `context.Context`。
- `Cleanup` 必须尽力执行，不能因为主流程失败就跳过。
- `Build` 和 `Push` 必须持续写日志。
- 错误必须带阶段信息，便于映射任务状态。

## 4. 构建任务生命周期

### 4.1 状态流转

```text
created
  -> queued
  -> preparing_context
  -> dispatching
  -> building
  -> build_success
  -> pushing
  -> push_success
```

失败状态：

```text
preparing_context_failed
dispatch_failed
build_failed
push_failed
canceled
timeout
```

### 4.2 状态更新规则

- 状态更新必须在数据库事务中完成。
- 状态只能按允许路径流转。
- 推送失败必须标记为 `push_failed`，不能覆盖为 `build_failed`。
- 取消任务时，如果外部命令已启动，需要先取消进程，再更新状态。
- 超时任务标记为 `timeout`。
- 重试任务创建新任务，并记录 `retry_of_task_id`。

### 4.3 任务时间字段

任务执行过程中需要写入：

- `queued_at`
- `started_at`
- `build_started_at`
- `build_finished_at`
- `push_started_at`
- `finished_at`
- `duration_seconds`

## 5. 构建上下文

### 5.1 MVP 支持范围

MVP 支持两种构建上下文：

- 仅 Dockerfile，无额外文件。
- 用户上传压缩包作为构建上下文。

后续支持：

- Git 仓库。
- 模板上下文。
- 平台内文件集。

### 5.2 本地目录结构

```text
data/contexts/{taskID}
├── Dockerfile
├── context
│   └── ...
└── metadata.json
```

`metadata.json` 保存：

```json
{
  "taskId": "task_1",
  "dockerfileHash": "sha256:...",
  "source": "inline",
  "createdAt": "2026-07-07T10:00:00Z"
}
```

### 5.3 Dockerfile 写入规则

- Dockerfile 必须使用任务创建时的快照。
- 版本节点后续修改不影响已创建任务。
- Dockerfile 文件名固定为 `Dockerfile`。
- Dockerfile 内容需要以 UTF-8 保存。

### 5.4 上传压缩包规则

上传上下文必须限制：

- 文件大小。
- 解压后总大小。
- 文件数量。
- 单文件大小。
- 路径深度。

解压必须防止：

- 路径穿越，例如 `../`。
- 绝对路径。
- 覆盖 Dockerfile。
- 符号链接逃逸。
- 特殊设备文件。

MVP 可以先禁用符号链接。

### 5.5 清理策略

- 构建完成后按配置保留上下文。
- 默认保留 7 天。
- 手动重试时不复用旧目录，重新创建上下文。
- 清理任务只删除已结束任务的上下文。

## 6. Dockerfile 校验

MVP 构建前校验：

- Dockerfile 非空。
- 必须包含 `FROM`。
- 目标镜像 tag 合法。
- 构建参数 key 合法。
- 不允许空目标仓库。

不做完整 Dockerfile 语义解析。

校验失败：

- API 阶段失败：返回 `VALIDATION_FAILED`。
- 执行阶段发现异常：任务标记 `preparing_context_failed` 或 `build_failed`。

## 7. 本机构建

### 7.1 适用场景

本机构建用于：

- 二进制部署时访问当前机器 Docker。
- Docker 部署时访问挂载的宿主机 Docker socket。

平台容器不运行 Docker daemon。

### 7.2 环境检测

检测命令：

```bash
docker version
docker info --format '{{.Architecture}}'
docker info --format '{{json .}}'
```

检测内容：

- Docker CLI 是否存在。
- Docker daemon 是否可访问。
- 架构。
- 操作系统。
- Docker 版本。
- BuildKit 支持。
- 磁盘空间。

### 7.3 命令执行

后端使用 `exec.CommandContext` 调用 Docker CLI。

要求：

- 不通过 shell 拼接命令。
- 命令参数使用数组传递。
- stdout 和 stderr 都接入日志管道。
- 进程退出码需要记录。
- context 取消时终止进程。

## 8. SSH 远程构建

### 8.1 远程目录

远程任务目录：

```text
/tmp/ibp-builds/{taskID}
├── context
├── docker-config
└── scripts
```

要求：

- 目录名必须包含 taskID。
- 不使用用户输入直接拼接路径。
- 任务结束后删除远程目录。
- 清理失败需要记录 warning，不覆盖构建结果。

### 8.2 上传流程

1. 建立 SSH 连接。
2. 创建远程临时目录。
3. 上传 Dockerfile 和上下文。
4. 设置必要文件权限。
5. 执行远程 Docker 命令。

上传要求：

- 上传前计算本地文件 hash。
- 上传失败标记 `dispatch_failed`。
- 上传后可选校验远程文件大小。

### 8.3 SSH 执行要求

- 不把仓库密码写入远程脚本文件。
- 不把 SSH 私钥写入远程主机。
- 远程命令需要明确工作目录。
- 每个命令都要采集 stdout/stderr。
- SSH session 断开时任务标记失败或超时。

### 8.4 远程 Docker 配置

每个任务使用独立远程 `DOCKER_CONFIG`：

```text
/tmp/ibp-builds/{taskID}/docker-config
```

推送结束后删除。

## 9. Docker 命令设计

### 9.1 docker build

基础命令：

```bash
docker build \
  -f Dockerfile \
  -t {localImageRef} \
  --platform linux/{architecture} \
  {contextPath}
```

可选参数：

```text
--build-arg KEY=VALUE
--no-cache
--pull
```

BuildKit：

```text
DOCKER_BUILDKIT=1
```

规则：

- `--platform` 根据任务架构设置。
- 构建 tag 使用任务内部 tag，避免和最终远端引用混淆。
- build args 需要脱敏后再写日志。

### 9.2 docker tag

```bash
docker tag {localImageRef} {targetImageRef}
```

### 9.3 docker login

```bash
docker login {registryEndpoint} --username {username} --password-stdin
```

规则：

- 密码只通过 stdin 输入。
- 日志中不打印密码。
- 使用独立 `DOCKER_CONFIG`。

### 9.4 docker push

```bash
docker push {targetImageRef}
```

推送成功后提取：

- digest。
- 镜像引用。
- 推送耗时。

### 9.5 inspect

构建成功后执行：

```bash
docker image inspect {targetImageRef}
```

记录：

- image ID。
- size。
- architecture。
- created。

## 10. 镜像引用规则

目标镜像引用格式：

```text
{registryEndpoint}/{namespace}/{imageName}:{tag}
```

规则：

- `registryEndpoint` 来自仓库配置。
- `namespace` 优先使用构建请求，其次使用项目默认命名空间，其次使用仓库命名空间。
- `imageName` 来自镜像项目。
- `tag` 来自构建请求。

校验：

- tag 不能为空。
- tag 不允许包含空格。
- endpoint 不允许包含协议前缀，除非后续明确支持。
- insecure HTTP 仓库通过 Docker daemon 配置或临时配置处理，MVP 先提示用户需要在构建主机 Docker 中配好 insecure registry。

## 11. 日志采集

### 11.1 日志格式

每条日志写为结构化记录：

```json
{
  "seq": 128,
  "time": "2026-07-07T10:00:00Z",
  "source": "docker-build",
  "level": "info",
  "message": "Step 3/10 : RUN apt-get update"
}
```

`source` 可选值：

```text
system
scheduler
context
ssh
docker-build
docker-login
docker-push
cleanup
```

### 11.2 日志文件

路径：

```text
data/logs/builds/{taskID}.log
```

要求：

- 每行一条 JSON。
- `seq` 单调递增。
- 写文件和 SSE 推送使用同一条事件。
- 日志写入失败需要标记任务异常，但不能丢失进程退出处理。

### 11.3 脱敏

日志写入前必须脱敏：

- 仓库密码。
- token。
- SSH 私钥。
- build args 中标记为 secret 的值。
- 配置中的 secret key。

脱敏规则：

- 精确值替换为 `******`。
- 对长 token 也要处理部分截断泄漏。
- 命令展示时只展示安全参数。

## 12. 取消和超时

### 12.1 取消

用户可以取消：

- `queued`
- `preparing_context`
- `dispatching`
- `building`
- `pushing`

取消规则：

- 排队任务直接标记 `canceled`。
- 已启动本地进程时，通过 context 终止进程。
- SSH 远程命令需要关闭 session。
- 如果无法确认远程进程已终止，记录 warning，并提示用户检查远程主机。

### 12.2 超时

每个任务有超时时间：

- 用户请求指定。
- 未指定时使用系统默认值。

超时后：

- 取消当前上下文。
- 终止本地或远程命令。
- 标记 `timeout`。
- 执行清理。

## 13. 清理

### 13.1 必须清理的资源

本地：

- 临时 Docker config。
- 临时构建上下文，按保留策略。
- 临时上传缓存。

远程：

- `/tmp/ibp-builds/{taskID}`。
- 远程 Docker config。
- 远程脚本文件。

### 13.2 清理失败处理

- 清理失败写入日志。
- 清理失败不覆盖构建成功或推送成功状态。
- 清理失败需要在任务详情中展示 warning。
- 后续可以提供手动清理入口。

## 14. 调度和并发

### 14.1 全局并发

系统设置：

```text
build.max_global_concurrency
```

调度器不能超过全局并发。

### 14.2 主机并发

每个主机有：

```text
max_concurrency
current_running
```

任务开始时增加计数，任务结束时减少计数。

要求：

- 增减计数必须在事务或互斥逻辑中执行。
- 服务异常重启后，需要根据运行中任务恢复或修正计数。

### 14.3 主机选择

自动选择顺序：

1. 过滤 disabled/deleted 主机。
2. 过滤 offline/unavailable 主机。
3. 过滤架构不匹配主机。
4. 过滤标签不匹配主机。
5. 过滤已满并发主机。
6. 选择运行任务数最少的主机。
7. 相同运行数时选择最近成功检测时间最新的主机。

## 15. 错误分类

### 15.1 错误码

```text
CONTEXT_PREPARE_FAILED
HOST_NOT_AVAILABLE
HOST_ARCH_MISMATCH
SSH_CONNECT_FAILED
SSH_UPLOAD_FAILED
DOCKER_NOT_AVAILABLE
DOCKER_BUILD_FAILED
DOCKER_LOGIN_FAILED
DOCKER_PUSH_FAILED
DOCKER_INSPECT_FAILED
TASK_CANCELED
TASK_TIMEOUT
CLEANUP_FAILED
```

### 15.2 状态映射

| 错误 | 任务状态 |
| --- | --- |
| 上下文准备失败 | preparing_context_failed |
| 无可用主机 | dispatch_failed |
| SSH 连接失败 | dispatch_failed |
| 上传失败 | dispatch_failed |
| docker build 失败 | build_failed |
| docker login 失败 | push_failed |
| docker push 失败 | push_failed |
| 用户取消 | canceled |
| 超时 | timeout |

## 16. 重试

重试规则：

- 重试创建新任务。
- 新任务复制原任务的 Dockerfile 快照和构建参数。
- 新任务可以允许用户修改目标主机、tag、仓库和参数，MVP 可先不支持修改。
- 原任务保留。
- 新任务记录 `retry_of_task_id`。

## 17. 审计

需要记录审计：

- 创建构建任务。
- 取消构建任务。
- 重试构建任务。
- 构建完成后推送成功。
- 重新推送产物。

审计详情不记录敏感值。

## 18. 测试要求

### 18.1 单元测试

必须覆盖：

- 任务状态机。
- 主机调度策略。
- 镜像引用拼装。
- Docker 命令参数生成。
- 日志脱敏。
- 构建参数校验。

### 18.2 集成测试

建议覆盖：

- 本地 Docker 构建一个最小镜像。
- 构建失败状态。
- 推送失败状态。
- SSE 日志订阅。

### 18.3 手工验收

必须覆盖：

- Docker 部署挂载 socket 后本机构建成功。
- SSH 远程构建成功。
- SSH 连接失败可读。
- Dockerfile 错误可读。
- 仓库登录失败可读。
- 构建取消后状态正确。

