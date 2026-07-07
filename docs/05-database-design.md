# 数据库设计

## 1. 设计目标

数据库需要支撑镜像构建平台的一期能力：

- 用户、登录会话和基础角色权限。
- 构建主机和镜像仓库管理。
- 镜像项目、起始镜像、版本节点和分支图。
- 构建任务状态机。
- 镜像产物和推送记录。
- 凭据加密存储。
- 审计日志和系统设置。

一期默认使用 SQLite，生产环境可切换 PostgreSQL。表结构设计需要尽量保持两者兼容。

## 2. 通用约定

### 2.1 ID

所有业务主键使用字符串 ID，推荐 ULID 或 UUID v7。

原因：

- 前端和日志中可直接引用。
- SQLite 和 PostgreSQL 都容易支持。
- 后续拆分 Agent 或多实例时更容易避免自增 ID 冲突。

字段约定：

```text
id TEXT PRIMARY KEY
```

### 2.2 时间

所有时间统一使用 UTC。

字段约定：

```text
created_at TIMESTAMP NOT NULL
updated_at TIMESTAMP NOT NULL
deleted_at TIMESTAMP NULL
```

API 返回 ISO 8601 字符串，例如：

```text
2026-07-07T10:00:00Z
```

### 2.3 软删除

核心业务表默认采用软删除：

- `users`
- `build_hosts`
- `registries`
- `image_projects`
- `image_branches`
- `image_version_nodes`
- `image_artifacts`

构建任务、审计日志、会话记录不做软删除：

- `build_tasks` 保留历史。
- `audit_logs` 不允许业务删除。
- `sessions` 可以物理删除过期记录。

### 2.4 JSON 字段

SQLite 使用 `TEXT` 保存 JSON 字符串。PostgreSQL 可使用 `JSONB`。

代码层通过 repository 隐藏差异。文档中统一写作 `JSON`。

适合 JSON 的字段：

- 标签数组。
- 构建参数。
- 表单配置快照。
- 检测结果详情。
- 前端图节点布局偏好。

### 2.5 布尔值

SQLite 使用 `INTEGER` 表示布尔值，`0` 为 false，`1` 为 true。PostgreSQL 使用 `BOOLEAN`。

文档中统一写作 `BOOLEAN`。

### 2.6 命名

- 表名使用复数 snake_case。
- 字段使用 snake_case。
- 外键字段使用 `{table_singular}_id`。
- 状态字段统一命名为 `status`。
- 创建者字段统一命名为 `created_by`。

## 3. 类型映射

| 逻辑类型 | SQLite | PostgreSQL |
| --- | --- | --- |
| ID | TEXT | TEXT |
| string | TEXT | TEXT |
| integer | INTEGER | INTEGER |
| boolean | INTEGER | BOOLEAN |
| timestamp | TEXT 或 DATETIME | TIMESTAMPTZ |
| json | TEXT | JSONB |
| encrypted text | TEXT | TEXT |

## 4. 枚举

### 4.1 用户角色

```text
admin
maintainer
viewer
```

### 4.2 凭据类型

```text
ssh_private_key
password
registry_token
registry_password
```

### 4.3 构建主机连接方式

```text
local_docker
ssh
docker_api
agent
```

MVP 只实现：

```text
local_docker
ssh
```

### 4.4 构建主机状态

```text
unknown
online
offline
unavailable
busy
disabled
```

### 4.5 镜像仓库类型

```text
generic
harbor
docker_hub
aliyun
tencent_cloud
```

一期按 `generic` 能力实现兼容，其他类型主要用于 UI 展示和默认字段提示。

### 4.6 镜像仓库状态

```text
unknown
available
unavailable
disabled
```

### 4.7 镜像类型

```text
java
python
nodejs
mysql
base_os
database
middleware
other
```

### 4.8 起始镜像来源

```text
external_image
internal_artifact
version_node
```

MVP 主要实现 `external_image`。

### 4.9 分支状态

```text
active
archived
```

### 4.10 版本节点状态

```text
draft
active
archived
```

构建状态不直接存为节点状态，节点最新构建状态通过最近构建任务或缓存字段展示。

### 4.11 构建任务状态

```text
created
queued
preparing_context
dispatching
building
build_success
pushing
push_success
preparing_context_failed
dispatch_failed
build_failed
push_failed
canceled
timeout
```

### 4.12 镜像产物状态

```text
available
push_failed
deprecated
archived
deleted_local
```

### 4.13 审计动作

```text
login
logout
create_user
update_user
create_build_host
update_build_host
delete_build_host
check_build_host
create_registry
update_registry
delete_registry
check_registry
create_image_project
update_image_project
create_version_node
update_version_node
create_branch
archive_branch
create_build_task
cancel_build_task
retry_build_task
repush_artifact
update_settings
```

## 5. 表关系概览

```text
users
  ├── sessions
  ├── image_projects.owner_id
  ├── image_version_nodes.created_by
  ├── build_tasks.created_by
  └── audit_logs.actor_id

credentials
  ├── build_hosts.credential_id
  └── registries.credential_id

image_projects
  ├── image_branches
  ├── image_version_nodes
  ├── build_tasks
  └── image_artifacts

image_version_nodes
  ├── image_version_nodes.parent_node_id
  ├── build_tasks
  └── image_artifacts

build_hosts
  └── build_tasks

registries
  ├── build_tasks
  └── image_artifacts

build_tasks
  ├── image_artifacts
  └── artifact_push_events
```

## 6. 表结构

### 6.1 users

用户表。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| id | TEXT | PK | 用户 ID |
| username | TEXT | NOT NULL UNIQUE | 登录名 |
| display_name | TEXT | NOT NULL | 显示名 |
| password_hash | TEXT | NOT NULL | 密码哈希 |
| role | TEXT | NOT NULL | `admin`、`maintainer`、`viewer` |
| status | TEXT | NOT NULL | `active`、`disabled` |
| last_login_at | TIMESTAMP | NULL | 最近登录时间 |
| created_at | TIMESTAMP | NOT NULL | 创建时间 |
| updated_at | TIMESTAMP | NOT NULL | 更新时间 |
| deleted_at | TIMESTAMP | NULL | 删除时间 |

索引：

```text
idx_users_username
idx_users_role
idx_users_status
```

约束：

- `username` 全局唯一。
- 至少保留一个 active admin，业务层保证。

### 6.2 sessions

登录会话表。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| id | TEXT | PK | session ID |
| user_id | TEXT | FK users.id | 用户 ID |
| token_hash | TEXT | NOT NULL UNIQUE | cookie token 哈希 |
| user_agent | TEXT | NULL | User-Agent |
| ip_address | TEXT | NULL | 登录 IP |
| expires_at | TIMESTAMP | NOT NULL | 过期时间 |
| created_at | TIMESTAMP | NOT NULL | 创建时间 |
| last_seen_at | TIMESTAMP | NULL | 最近访问时间 |

索引：

```text
idx_sessions_user_id
idx_sessions_expires_at
idx_sessions_token_hash
```

清理策略：

- 定期删除 `expires_at < now()` 的记录。

### 6.3 credentials

凭据表。只保存加密后的密文，不保存明文。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| id | TEXT | PK | 凭据 ID |
| type | TEXT | NOT NULL | 凭据类型 |
| name | TEXT | NOT NULL | 内部显示名 |
| encrypted_value | TEXT | NOT NULL | 加密值 |
| encryption_version | INTEGER | NOT NULL | 加密版本 |
| fingerprint | TEXT | NULL | SSH 公钥指纹或 token 指纹 |
| created_by | TEXT | FK users.id | 创建人 |
| created_at | TIMESTAMP | NOT NULL | 创建时间 |
| updated_at | TIMESTAMP | NOT NULL | 更新时间 |

索引：

```text
idx_credentials_type
idx_credentials_created_by
```

安全要求：

- API 不返回 `encrypted_value`。
- 更新凭据时只允许覆盖，不允许回显。
- 密钥轮换时通过 `encryption_version` 支持后续迁移。

### 6.4 build_hosts

构建主机表。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| id | TEXT | PK | 主机 ID |
| name | TEXT | NOT NULL | 主机名称 |
| connection_type | TEXT | NOT NULL | `local_docker`、`ssh` 等 |
| address | TEXT | NULL | SSH 地址，本机可为空 |
| port | INTEGER | NULL | SSH 端口 |
| username | TEXT | NULL | SSH 用户 |
| credential_id | TEXT | FK credentials.id NULL | SSH 凭据 |
| docker_endpoint | TEXT | NULL | Docker socket 或 endpoint |
| docker_command | TEXT | NULL | Docker 命令路径，默认 `docker` |
| architecture | TEXT | NULL | `amd64`、`arm64` 等 |
| os | TEXT | NULL | 操作系统 |
| docker_version | TEXT | NULL | Docker 版本 |
| buildkit_supported | BOOLEAN | NOT NULL | 是否支持 BuildKit |
| labels | JSON | NOT NULL | 标签数组 |
| max_concurrency | INTEGER | NOT NULL | 主机并发上限 |
| current_running | INTEGER | NOT NULL | 当前运行任务数缓存 |
| status | TEXT | NOT NULL | 主机状态 |
| last_checked_at | TIMESTAMP | NULL | 最近检测时间 |
| last_check_result | JSON | NULL | 最近检测详情 |
| last_error | TEXT | NULL | 最近错误摘要 |
| created_by | TEXT | FK users.id | 创建人 |
| created_at | TIMESTAMP | NOT NULL | 创建时间 |
| updated_at | TIMESTAMP | NOT NULL | 更新时间 |
| deleted_at | TIMESTAMP | NULL | 删除时间 |

索引：

```text
idx_build_hosts_connection_type
idx_build_hosts_architecture
idx_build_hosts_status
idx_build_hosts_deleted_at
```

约束：

- `max_concurrency >= 1`。
- `local_docker` 主机最多一个默认启用主机，业务层保证。

### 6.5 registries

镜像仓库表。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| id | TEXT | PK | 仓库 ID |
| name | TEXT | NOT NULL | 仓库名称 |
| type | TEXT | NOT NULL | 仓库类型 |
| endpoint | TEXT | NOT NULL | Registry 地址 |
| namespace | TEXT | NULL | 默认命名空间 |
| region | TEXT | NULL | 云厂商地域 |
| credential_id | TEXT | FK credentials.id NULL | 仓库凭据 |
| allow_pull | BOOLEAN | NOT NULL | 是否允许拉取 |
| allow_push | BOOLEAN | NOT NULL | 是否允许推送 |
| is_default_pull | BOOLEAN | NOT NULL | 默认拉取仓库 |
| is_default_push | BOOLEAN | NOT NULL | 默认推送仓库 |
| tls_verify | BOOLEAN | NOT NULL | 是否校验 TLS |
| insecure_http | BOOLEAN | NOT NULL | 是否允许 HTTP |
| status | TEXT | NOT NULL | 仓库状态 |
| last_checked_at | TIMESTAMP | NULL | 最近检测时间 |
| last_check_result | JSON | NULL | 最近检测详情 |
| last_error | TEXT | NULL | 最近错误摘要 |
| created_by | TEXT | FK users.id | 创建人 |
| created_at | TIMESTAMP | NOT NULL | 创建时间 |
| updated_at | TIMESTAMP | NOT NULL | 更新时间 |
| deleted_at | TIMESTAMP | NULL | 删除时间 |

索引：

```text
idx_registries_type
idx_registries_endpoint
idx_registries_status
idx_registries_default_push
idx_registries_deleted_at
```

约束：

- 同一 `endpoint + namespace` 不应重复，业务层提示。
- 默认推送仓库只能有一个，业务层保证。

### 6.6 image_projects

镜像项目表。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| id | TEXT | PK | 项目 ID |
| name | TEXT | NOT NULL | 项目名称 |
| image_type | TEXT | NOT NULL | 镜像类型 |
| image_name | TEXT | NOT NULL | 镜像名称 |
| namespace | TEXT | NULL | 命名空间 |
| root_image_ref | TEXT | NOT NULL | 起始镜像引用 |
| root_image_source | TEXT | NOT NULL | 起始镜像来源 |
| source_project_id | TEXT | FK image_projects.id NULL | 来源项目 |
| source_version_node_id | TEXT | FK image_version_nodes.id NULL | 来源版本节点 |
| default_registry_id | TEXT | FK registries.id NULL | 默认仓库 |
| default_architecture | TEXT | NOT NULL | 默认架构 |
| labels | JSON | NOT NULL | 标签数组 |
| description | TEXT | NULL | 描述 |
| status | TEXT | NOT NULL | `active`、`archived` |
| owner_id | TEXT | FK users.id | 维护人 |
| latest_version_node_id | TEXT | FK image_version_nodes.id NULL | 最新节点缓存 |
| latest_build_task_id | TEXT | FK build_tasks.id NULL | 最近构建缓存 |
| created_at | TIMESTAMP | NOT NULL | 创建时间 |
| updated_at | TIMESTAMP | NOT NULL | 更新时间 |
| deleted_at | TIMESTAMP | NULL | 删除时间 |

索引：

```text
idx_image_projects_name
idx_image_projects_image_type
idx_image_projects_image_name
idx_image_projects_owner_id
idx_image_projects_status
idx_image_projects_deleted_at
```

约束：

- 同一 `namespace + image_name` 建议唯一，业务层可允许高级用户覆盖。
- `root_image_source = version_node` 时必须有 `source_project_id` 和 `source_version_node_id`。

### 6.7 image_branches

镜像项目分支表。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| id | TEXT | PK | 分支 ID |
| project_id | TEXT | FK image_projects.id | 项目 ID |
| name | TEXT | NOT NULL | 分支名称 |
| start_node_id | TEXT | FK image_version_nodes.id NULL | 起始节点 |
| head_node_id | TEXT | FK image_version_nodes.id NULL | 当前最新节点 |
| description | TEXT | NULL | 分支说明 |
| status | TEXT | NOT NULL | `active`、`archived` |
| created_by | TEXT | FK users.id | 创建人 |
| created_at | TIMESTAMP | NOT NULL | 创建时间 |
| updated_at | TIMESTAMP | NOT NULL | 更新时间 |
| deleted_at | TIMESTAMP | NULL | 删除时间 |

索引：

```text
idx_image_branches_project_id
idx_image_branches_project_name
idx_image_branches_status
```

约束：

- 同一项目内分支名称唯一。
- 每个项目创建时默认创建 `main` 分支。

### 6.8 image_version_nodes

镜像版本节点表。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| id | TEXT | PK | 节点 ID |
| project_id | TEXT | FK image_projects.id | 项目 ID |
| branch_id | TEXT | FK image_branches.id | 分支 ID |
| parent_node_id | TEXT | FK image_version_nodes.id NULL | 父节点 |
| version | TEXT | NOT NULL | 版本号或 tag |
| dockerfile | TEXT | NOT NULL | Dockerfile 内容 |
| dockerfile_hash | TEXT | NOT NULL | Dockerfile 内容哈希 |
| form_config_snapshot | JSON | NULL | 表单配置快照 |
| build_context_ref | TEXT | NULL | 构建上下文引用 |
| description | TEXT | NULL | 说明 |
| status | TEXT | NOT NULL | 节点状态 |
| latest_build_task_id | TEXT | FK build_tasks.id NULL | 最近构建缓存 |
| latest_artifact_id | TEXT | FK image_artifacts.id NULL | 最近产物缓存 |
| graph_position | JSON | NULL | 前端图布局缓存 |
| created_by | TEXT | FK users.id | 创建人 |
| created_at | TIMESTAMP | NOT NULL | 创建时间 |
| updated_at | TIMESTAMP | NOT NULL | 更新时间 |
| deleted_at | TIMESTAMP | NULL | 删除时间 |

索引：

```text
idx_image_version_nodes_project_id
idx_image_version_nodes_branch_id
idx_image_version_nodes_parent_node_id
idx_image_version_nodes_project_version
idx_image_version_nodes_status
```

约束：

- 同一项目内 `version` 建议唯一。
- `parent_node_id` 不能指向其他项目的节点。
- 不能形成环，业务层保证。

### 6.9 build_tasks

构建任务表。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| id | TEXT | PK | 任务 ID |
| project_id | TEXT | FK image_projects.id | 项目 ID |
| version_node_id | TEXT | FK image_version_nodes.id | 版本节点 |
| retry_of_task_id | TEXT | FK build_tasks.id NULL | 重试来源 |
| host_id | TEXT | FK build_hosts.id NULL | 实际构建主机 |
| requested_host_id | TEXT | FK build_hosts.id NULL | 用户指定主机 |
| registry_id | TEXT | FK registries.id | 目标仓库 |
| image_name | TEXT | NOT NULL | 镜像名称 |
| image_tag | TEXT | NOT NULL | 镜像 tag |
| image_ref | TEXT | NOT NULL | 完整镜像引用 |
| architecture | TEXT | NOT NULL | 架构 |
| dockerfile_snapshot | TEXT | NOT NULL | 构建时 Dockerfile 快照 |
| dockerfile_hash | TEXT | NOT NULL | Dockerfile 哈希 |
| build_context_ref | TEXT | NULL | 构建上下文引用 |
| build_args | JSON | NULL | build args |
| build_options | JSON | NULL | no-cache、pull、timeout 等 |
| scheduler_reason | TEXT | NULL | 调度说明 |
| status | TEXT | NOT NULL | 任务状态 |
| error_code | TEXT | NULL | 错误码 |
| error_message | TEXT | NULL | 错误摘要 |
| log_path | TEXT | NULL | 日志文件路径 |
| queued_at | TIMESTAMP | NULL | 入队时间 |
| started_at | TIMESTAMP | NULL | 开始时间 |
| build_started_at | TIMESTAMP | NULL | 构建开始 |
| build_finished_at | TIMESTAMP | NULL | 构建结束 |
| push_started_at | TIMESTAMP | NULL | 推送开始 |
| finished_at | TIMESTAMP | NULL | 任务结束 |
| duration_seconds | INTEGER | NULL | 总耗时 |
| created_by | TEXT | FK users.id | 创建人 |
| created_at | TIMESTAMP | NOT NULL | 创建时间 |
| updated_at | TIMESTAMP | NOT NULL | 更新时间 |

索引：

```text
idx_build_tasks_project_id
idx_build_tasks_version_node_id
idx_build_tasks_host_id
idx_build_tasks_registry_id
idx_build_tasks_status
idx_build_tasks_created_by
idx_build_tasks_created_at
idx_build_tasks_retry_of_task_id
```

约束：

- 推送失败使用 `push_failed`，不能覆盖为 `build_failed`。
- 任务状态只允许按状态机流转，service 层保证。

### 6.10 image_artifacts

镜像产物表。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| id | TEXT | PK | 产物 ID |
| build_task_id | TEXT | FK build_tasks.id | 构建任务 |
| project_id | TEXT | FK image_projects.id | 项目 ID |
| version_node_id | TEXT | FK image_version_nodes.id | 版本节点 |
| registry_id | TEXT | FK registries.id | 仓库 ID |
| image_ref | TEXT | NOT NULL | 完整镜像引用 |
| image_id | TEXT | NULL | Docker image ID |
| digest | TEXT | NULL | 镜像 digest |
| tag | TEXT | NOT NULL | tag |
| architecture | TEXT | NOT NULL | 架构 |
| size_bytes | INTEGER | NULL | 镜像大小 |
| status | TEXT | NOT NULL | 产物状态 |
| pushed | BOOLEAN | NOT NULL | 是否推送成功 |
| pushed_at | TIMESTAMP | NULL | 推送时间 |
| deprecated | BOOLEAN | NOT NULL | 是否废弃 |
| created_at | TIMESTAMP | NOT NULL | 创建时间 |
| updated_at | TIMESTAMP | NOT NULL | 更新时间 |
| deleted_at | TIMESTAMP | NULL | 删除时间 |

索引：

```text
idx_image_artifacts_build_task_id
idx_image_artifacts_project_id
idx_image_artifacts_version_node_id
idx_image_artifacts_registry_id
idx_image_artifacts_image_ref
idx_image_artifacts_digest
idx_image_artifacts_status
```

约束：

- 同一 `registry_id + image_ref + digest` 不应重复。

### 6.11 artifact_push_events

产物推送记录表，用于记录初次推送和重新推送。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| id | TEXT | PK | 推送事件 ID |
| artifact_id | TEXT | FK image_artifacts.id | 产物 ID |
| build_task_id | TEXT | FK build_tasks.id NULL | 关联任务 |
| registry_id | TEXT | FK registries.id | 目标仓库 |
| status | TEXT | NOT NULL | `success`、`failed` |
| error_message | TEXT | NULL | 错误摘要 |
| started_at | TIMESTAMP | NOT NULL | 开始时间 |
| finished_at | TIMESTAMP | NULL | 结束时间 |
| created_by | TEXT | FK users.id | 操作人 |
| created_at | TIMESTAMP | NOT NULL | 创建时间 |

索引：

```text
idx_artifact_push_events_artifact_id
idx_artifact_push_events_registry_id
idx_artifact_push_events_created_at
```

### 6.12 audit_logs

审计日志表。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| id | TEXT | PK | 审计 ID |
| actor_id | TEXT | FK users.id NULL | 操作人 |
| actor_name | TEXT | NULL | 操作人名称快照 |
| action | TEXT | NOT NULL | 动作 |
| resource_type | TEXT | NOT NULL | 资源类型 |
| resource_id | TEXT | NULL | 资源 ID |
| resource_name | TEXT | NULL | 资源名称快照 |
| ip_address | TEXT | NULL | IP |
| user_agent | TEXT | NULL | User-Agent |
| request_id | TEXT | NULL | 请求 ID |
| detail | JSON | NULL | 详情 |
| created_at | TIMESTAMP | NOT NULL | 创建时间 |

索引：

```text
idx_audit_logs_actor_id
idx_audit_logs_action
idx_audit_logs_resource
idx_audit_logs_created_at
idx_audit_logs_request_id
```

审计日志不允许业务删除。

### 6.13 system_settings

系统设置表。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| key | TEXT | PK | 设置键 |
| value | JSON | NOT NULL | 设置值 |
| value_type | TEXT | NOT NULL | `string`、`number`、`boolean`、`object` |
| description | TEXT | NULL | 描述 |
| updated_by | TEXT | FK users.id NULL | 更新人 |
| updated_at | TIMESTAMP | NOT NULL | 更新时间 |

建议设置项：

```text
platform.name
platform.public_url
build.max_global_concurrency
build.default_timeout_seconds
build.enable_buildkit
logs.retention_days
contexts.retention_days
security.session_ttl_seconds
```

## 7. 删除策略

### 7.1 构建主机

- 如果主机已被构建任务引用，只能软删除。
- 被软删除的主机不参与调度。
- 历史任务仍展示主机名称快照或引用。

### 7.2 镜像仓库

- 如果仓库已被任务或产物引用，只能软删除。
- 被软删除的仓库不允许作为新任务目标。

### 7.3 镜像项目

- MVP 不提供硬删除。
- 归档项目后，不允许新建版本或构建。
- 历史任务和产物仍可查看。

### 7.4 版本节点

- MVP 不提供删除版本节点。
- 支持归档。

### 7.5 构建任务

- 不删除构建任务。
- 可以按保留策略清理日志文件，但任务记录保留。

## 8. 迁移策略

迁移文件命名：

```text
000001_init.up.sql
000001_init.down.sql
000002_add_artifact_push_events.up.sql
000002_add_artifact_push_events.down.sql
```

要求：

- 每次 schema 变更必须有迁移文件。
- 应用启动时自动执行未应用迁移。
- 迁移记录保存到 `schema_migrations`。
- 生产环境升级前必须提示备份数据库。

`schema_migrations`：

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| version | INTEGER | PK | 迁移版本 |
| dirty | BOOLEAN | NOT NULL | 是否失败中断 |
| applied_at | TIMESTAMP | NOT NULL | 应用时间 |

## 9. 数据保留策略

MVP 默认：

- 构建任务记录永久保留。
- 审计日志保留 180 天，管理员可配置。
- 构建日志保留 30 天，管理员可配置。
- 构建上下文保留 7 天，管理员可配置。
- 过期 session 每天清理。
- 临时 Docker 配置任务结束后立即清理。

## 10. 待实现时确认

进入编码前需要最终确认：

1. ID 使用 ULID 还是 UUID v7。
2. Go 数据库访问层使用 `database/sql` + sqlc，还是 `database/sql` + 手写 repository。
3. SQLite 时间字段使用 TEXT ISO 字符串还是 DATETIME。
4. PostgreSQL 是否作为 MVP 自动化测试的一部分。

