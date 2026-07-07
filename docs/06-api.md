# API 设计

## 1. API 目标

API 为前端管理界面和后续开放能力提供统一后端契约。MVP 只提供同源前端使用的 REST API，不提供公开 API token。

设计目标：

- 契约稳定，方便前端并行开发。
- 错误结构统一，便于展示和排查。
- 分页、筛选、排序规则统一。
- 构建日志通过 SSE 实时推送。
- 敏感字段不返回明文。

## 2. 基础约定

### 2.1 Base URL

```text
/api/v1
```

健康检查不带版本：

```text
/healthz
```

### 2.2 Content Type

请求：

```text
Content-Type: application/json
```

响应：

```text
Content-Type: application/json
```

SSE 响应：

```text
Content-Type: text/event-stream
```

### 2.3 认证

MVP 使用服务端 session：

- 登录成功后后端设置 HttpOnly Cookie。
- Cookie 建议名称：`ibp_session`。
- Cookie 属性：`HttpOnly`、`SameSite=Lax`。
- HTTPS 部署时启用 `Secure`。
- 前端通过 `GET /api/v1/auth/me` 获取当前用户。

未登录响应：

```json
{
  "error": {
    "code": "UNAUTHENTICATED",
    "message": "Authentication is required.",
    "details": null
  }
}
```

### 2.4 统一成功响应

对象响应：

```json
{
  "data": {}
}
```

列表响应：

```json
{
  "data": [],
  "pagination": {
    "page": 1,
    "pageSize": 20,
    "total": 100
  }
}
```

操作成功但无需返回对象：

```json
{
  "data": {
    "success": true
  }
}
```

### 2.5 统一错误响应

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "Validation failed.",
    "details": {
      "fields": [
        {
          "field": "name",
          "message": "Name is required."
        }
      ]
    }
  }
}
```

常用错误码：

| HTTP | code | 说明 |
| --- | --- | --- |
| 400 | BAD_REQUEST | 请求格式错误 |
| 400 | VALIDATION_FAILED | 字段校验失败 |
| 401 | UNAUTHENTICATED | 未登录 |
| 403 | FORBIDDEN | 无权限 |
| 404 | NOT_FOUND | 资源不存在 |
| 409 | CONFLICT | 资源冲突 |
| 422 | INVALID_STATE | 状态不允许 |
| 500 | INTERNAL_ERROR | 服务端错误 |
| 502 | EXTERNAL_COMMAND_FAILED | Docker/SSH/Registry 命令失败 |
| 504 | OPERATION_TIMEOUT | 操作超时 |

### 2.6 分页

请求参数：

```text
page=1
pageSize=20
```

规则：

- `page` 从 1 开始。
- `pageSize` 默认 20。
- `pageSize` 最大 100。

### 2.7 排序

请求参数：

```text
sort=createdAt:desc
```

允许多字段时：

```text
sort=status:asc,createdAt:desc
```

后端只接受白名单字段。

### 2.8 时间格式

所有时间使用 ISO 8601 UTC：

```text
2026-07-07T10:00:00Z
```

### 2.9 ID 格式

所有 ID 使用字符串。

示例：

```text
01JZP8CEH8ASJ2VK72WY8V9M5B
```

## 3. 通用 DTO

### 3.1 User

```json
{
  "id": "user_1",
  "username": "admin",
  "displayName": "Administrator",
  "role": "admin",
  "status": "active",
  "lastLoginAt": "2026-07-07T10:00:00Z",
  "createdAt": "2026-07-07T10:00:00Z",
  "updatedAt": "2026-07-07T10:00:00Z"
}
```

### 3.2 BuildHost

```json
{
  "id": "host_1",
  "name": "Local Docker",
  "connectionType": "local_docker",
  "address": null,
  "port": null,
  "username": null,
  "dockerEndpoint": "/var/run/docker.sock",
  "architecture": "amd64",
  "os": "linux",
  "dockerVersion": "27.0.0",
  "buildkitSupported": true,
  "labels": ["local", "amd64"],
  "maxConcurrency": 2,
  "currentRunning": 0,
  "status": "online",
  "lastCheckedAt": "2026-07-07T10:00:00Z",
  "lastError": null,
  "createdAt": "2026-07-07T10:00:00Z",
  "updatedAt": "2026-07-07T10:00:00Z"
}
```

### 3.3 Registry

```json
{
  "id": "reg_1",
  "name": "Internal Harbor",
  "type": "harbor",
  "endpoint": "harbor.example.com",
  "namespace": "platform",
  "allowPull": true,
  "allowPush": true,
  "isDefaultPull": false,
  "isDefaultPush": true,
  "tlsVerify": true,
  "insecureHttp": false,
  "status": "available",
  "lastCheckedAt": "2026-07-07T10:00:00Z",
  "lastError": null,
  "createdAt": "2026-07-07T10:00:00Z",
  "updatedAt": "2026-07-07T10:00:00Z"
}
```

### 3.4 ImageProject

```json
{
  "id": "project_1",
  "name": "Java Runtime",
  "imageType": "java",
  "imageName": "java-runtime",
  "namespace": "platform",
  "rootImageRef": "eclipse-temurin:17",
  "rootImageSource": "external_image",
  "defaultRegistryId": "reg_1",
  "defaultArchitecture": "amd64",
  "labels": ["java", "jdk17"],
  "description": "Base Java runtime image.",
  "status": "active",
  "ownerId": "user_1",
  "latestVersionNodeId": "node_1",
  "createdAt": "2026-07-07T10:00:00Z",
  "updatedAt": "2026-07-07T10:00:00Z"
}
```

### 3.5 VersionGraph

```json
{
  "project": {
    "id": "project_1",
    "name": "Java Runtime",
    "rootImageRef": "eclipse-temurin:17"
  },
  "nodes": [
    {
      "id": "node_1",
      "version": "jdk17-v1",
      "branchId": "branch_main",
      "branchName": "main",
      "status": "active",
      "latestBuildStatus": "push_success",
      "latestImageTag": "jdk17-v1",
      "createdAt": "2026-07-07T10:00:00Z",
      "createdBy": "admin"
    }
  ],
  "edges": [
    {
      "id": "edge_node_1_node_2",
      "from": "node_1",
      "to": "node_2",
      "type": "parent"
    }
  ],
  "branches": [
    {
      "id": "branch_main",
      "name": "main",
      "status": "active",
      "headNodeId": "node_1"
    }
  ]
}
```

### 3.6 BuildTask

```json
{
  "id": "task_1",
  "projectId": "project_1",
  "versionNodeId": "node_1",
  "hostId": "host_1",
  "registryId": "reg_1",
  "imageName": "platform/java-runtime",
  "imageTag": "jdk17-v1",
  "imageRef": "harbor.example.com/platform/java-runtime:jdk17-v1",
  "architecture": "amd64",
  "status": "building",
  "errorCode": null,
  "errorMessage": null,
  "queuedAt": "2026-07-07T10:00:00Z",
  "startedAt": "2026-07-07T10:00:05Z",
  "finishedAt": null,
  "durationSeconds": null,
  "createdBy": "user_1",
  "createdAt": "2026-07-07T10:00:00Z",
  "updatedAt": "2026-07-07T10:00:10Z"
}
```

### 3.7 ImageArtifact

```json
{
  "id": "artifact_1",
  "buildTaskId": "task_1",
  "projectId": "project_1",
  "versionNodeId": "node_1",
  "registryId": "reg_1",
  "imageRef": "harbor.example.com/platform/java-runtime:jdk17-v1",
  "imageId": "sha256:...",
  "digest": "sha256:...",
  "tag": "jdk17-v1",
  "architecture": "amd64",
  "sizeBytes": 250000000,
  "status": "available",
  "pushed": true,
  "pushedAt": "2026-07-07T10:10:00Z",
  "deprecated": false,
  "createdAt": "2026-07-07T10:10:00Z"
}
```

## 4. Auth API

### 4.1 检查系统初始化状态

```text
GET /api/v1/setup/status
```

响应：

```json
{
  "data": {
    "initialized": false
  }
}
```

### 4.2 初始化管理员

```text
POST /api/v1/setup/admin
```

请求：

```json
{
  "username": "admin",
  "password": "ChangeMe123!",
  "displayName": "Administrator"
}
```

说明：

- 仅在系统未初始化时允许调用。
- 初始化完成后再次调用返回 `409 CONFLICT`。

### 4.3 登录

```text
POST /api/v1/auth/login
```

请求：

```json
{
  "username": "admin",
  "password": "ChangeMe123!"
}
```

响应：

```json
{
  "data": {
    "user": {
      "id": "user_1",
      "username": "admin",
      "displayName": "Administrator",
      "role": "admin"
    }
  }
}
```

### 4.4 退出

```text
POST /api/v1/auth/logout
```

### 4.5 当前用户

```text
GET /api/v1/auth/me
```

## 5. Build Hosts API

### 5.1 主机列表

```text
GET /api/v1/build-hosts?status=online&architecture=amd64&page=1&pageSize=20
```

响应：

```json
{
  "data": [],
  "pagination": {
    "page": 1,
    "pageSize": 20,
    "total": 0
  }
}
```

### 5.2 创建主机

```text
POST /api/v1/build-hosts
```

Local Docker 请求：

```json
{
  "name": "Local Docker",
  "connectionType": "local_docker",
  "dockerEndpoint": "/var/run/docker.sock",
  "maxConcurrency": 2,
  "labels": ["local", "amd64"]
}
```

SSH 请求：

```json
{
  "name": "ARM Builder",
  "connectionType": "ssh",
  "address": "192.168.1.20",
  "port": 22,
  "username": "builder",
  "sshPrivateKey": "-----BEGIN OPENSSH PRIVATE KEY-----...",
  "dockerCommand": "docker",
  "maxConcurrency": 1,
  "labels": ["arm64"]
}
```

响应返回脱敏后的 BuildHost，不返回私钥。

### 5.3 主机详情

```text
GET /api/v1/build-hosts/{id}
```

### 5.4 更新主机

```text
PUT /api/v1/build-hosts/{id}
```

说明：

- SSH 私钥为空时表示不更新凭据。
- 如果传入新私钥，则覆盖原凭据。

### 5.5 删除主机

```text
DELETE /api/v1/build-hosts/{id}
```

### 5.6 检测主机

```text
POST /api/v1/build-hosts/{id}/check
```

响应：

```json
{
  "data": {
    "status": "online",
    "architecture": "amd64",
    "os": "linux",
    "dockerVersion": "27.0.0",
    "buildkitSupported": true,
    "diskFreeBytes": 100000000000,
    "checks": [
      {
        "name": "docker",
        "status": "success",
        "message": "Docker is available."
      }
    ],
    "error": null
  }
}
```

### 5.7 启用或禁用主机

```text
POST /api/v1/build-hosts/{id}/enable
POST /api/v1/build-hosts/{id}/disable
```

## 6. Registries API

### 6.1 仓库列表

```text
GET /api/v1/registries?status=available&page=1&pageSize=20
```

### 6.2 创建仓库

```text
POST /api/v1/registries
```

请求：

```json
{
  "name": "Internal Harbor",
  "type": "harbor",
  "endpoint": "harbor.example.com",
  "namespace": "platform",
  "username": "robot$builder",
  "password": "secret",
  "allowPull": true,
  "allowPush": true,
  "isDefaultPush": true,
  "tlsVerify": true,
  "insecureHttp": false
}
```

### 6.3 仓库详情

```text
GET /api/v1/registries/{id}
```

### 6.4 更新仓库

```text
PUT /api/v1/registries/{id}
```

说明：

- 密码或 token 为空时表示不更新凭据。
- 更新后不返回明文凭据。

### 6.5 删除仓库

```text
DELETE /api/v1/registries/{id}
```

### 6.6 检测仓库

```text
POST /api/v1/registries/{id}/check
```

请求可选：

```json
{
  "testPullImage": "alpine:3.20"
}
```

响应：

```json
{
  "data": {
    "status": "available",
    "login": {
      "status": "success",
      "message": "Login succeeded."
    },
    "pull": {
      "status": "skipped",
      "message": "No test image provided."
    },
    "error": null
  }
}
```

## 7. Image Projects API

### 7.1 项目列表

```text
GET /api/v1/image-projects?imageType=java&keyword=runtime&page=1&pageSize=20
```

支持筛选：

- `imageType`
- `status`
- `architecture`
- `ownerId`
- `keyword`
- `label`

### 7.2 创建项目

```text
POST /api/v1/image-projects
```

请求：

```json
{
  "name": "Java Runtime",
  "imageType": "java",
  "imageName": "java-runtime",
  "namespace": "platform",
  "rootImageRef": "eclipse-temurin:17",
  "rootImageSource": "external_image",
  "defaultRegistryId": "reg_1",
  "defaultArchitecture": "amd64",
  "labels": ["java", "jdk17"],
  "description": "Base Java runtime image."
}
```

创建项目后，后端应自动创建 `main` 分支和初始版本节点。

### 7.3 项目详情

```text
GET /api/v1/image-projects/{id}
```

### 7.4 更新项目

```text
PUT /api/v1/image-projects/{id}
```

### 7.5 归档项目

```text
POST /api/v1/image-projects/{id}/archive
```

### 7.6 版本图

```text
GET /api/v1/image-projects/{id}/graph?branch=main&status=active
```

返回 `VersionGraph`。

## 8. Branches and Version Nodes API

### 8.1 分支列表

```text
GET /api/v1/image-projects/{projectId}/branches
```

### 8.2 创建分支

```text
POST /api/v1/image-projects/{projectId}/branches
```

请求：

```json
{
  "name": "jdk21",
  "startNodeId": "node_1",
  "description": "Java 21 branch."
}
```

### 8.3 归档分支

```text
POST /api/v1/image-projects/{projectId}/branches/{branchId}/archive
```

### 8.4 创建版本节点

```text
POST /api/v1/image-projects/{projectId}/version-nodes
```

请求：

```json
{
  "branchId": "branch_main",
  "parentNodeId": "node_1",
  "version": "jdk17-v2",
  "dockerfile": "FROM eclipse-temurin:17\nRUN apt-get update\n",
  "formConfigSnapshot": null,
  "description": "Install base packages."
}
```

### 8.5 节点详情

```text
GET /api/v1/image-projects/{projectId}/version-nodes/{nodeId}
```

响应包含：

- 节点基础信息。
- Dockerfile。
- 表单配置快照。
- 父节点。
- 分支。
- 最近构建任务。
- 最近镜像产物。

### 8.6 更新节点

```text
PUT /api/v1/image-projects/{projectId}/version-nodes/{nodeId}
```

### 8.7 比较节点 Dockerfile

```text
GET /api/v1/image-projects/{projectId}/version-nodes/{leftNodeId}/diff/{rightNodeId}
```

响应：

```json
{
  "data": {
    "leftNodeId": "node_1",
    "rightNodeId": "node_2",
    "leftDockerfile": "FROM ubuntu:24.04\n",
    "rightDockerfile": "FROM ubuntu:24.04\nRUN apt-get update\n",
    "unifiedDiff": "--- node_1\n+++ node_2\n@@ ..."
  }
}
```

### 8.8 保存图布局

```text
PUT /api/v1/image-projects/{projectId}/graph/layout
```

请求：

```json
{
  "positions": [
    {
      "nodeId": "node_1",
      "x": 120,
      "y": 80
    }
  ]
}
```

## 9. Dockerfile Form API

### 9.1 从表单生成 Dockerfile

```text
POST /api/v1/dockerfile/generate
```

请求：

```json
{
  "baseImage": "ubuntu:24.04",
  "environment": {
    "APP_ENV": "production"
  },
  "workdir": "/app",
  "packages": ["curl", "ca-certificates"],
  "copy": [
    {
      "source": "./app",
      "target": "/app"
    }
  ],
  "expose": [8080],
  "cmd": ["./server"],
  "entrypoint": [],
  "args": {
    "VERSION": "latest"
  },
  "labels": {
    "org.opencontainers.image.source": "image-build-platform"
  }
}
```

响应：

```json
{
  "data": {
    "dockerfile": "FROM ubuntu:24.04\n..."
  }
}
```

### 9.2 校验 Dockerfile

```text
POST /api/v1/dockerfile/validate
```

请求：

```json
{
  "dockerfile": "FROM ubuntu:24.04\n"
}
```

响应：

```json
{
  "data": {
    "valid": true,
    "warnings": [],
    "errors": []
  }
}
```

## 10. Build Tasks API

### 10.1 任务列表

```text
GET /api/v1/build-tasks?status=building&projectId=project_1&page=1&pageSize=20
```

支持筛选：

- `status`
- `projectId`
- `versionNodeId`
- `hostId`
- `registryId`
- `architecture`
- `createdBy`
- `createdFrom`
- `createdTo`

### 10.2 创建构建任务

```text
POST /api/v1/build-tasks
```

请求：

```json
{
  "projectId": "project_1",
  "versionNodeId": "node_1",
  "hostStrategy": {
    "type": "auto",
    "hostId": null,
    "architecture": "amd64",
    "labels": ["amd64"]
  },
  "registryId": "reg_1",
  "imageTag": "jdk17-v1",
  "buildArgs": {
    "VERSION": "1.0.0"
  },
  "buildOptions": {
    "noCache": false,
    "pull": true,
    "timeoutSeconds": 3600,
    "autoPush": true
  }
}
```

响应：

```json
{
  "data": {
    "id": "task_1",
    "status": "queued"
  }
}
```

### 10.3 任务详情

```text
GET /api/v1/build-tasks/{id}
```

### 10.4 取消任务

```text
POST /api/v1/build-tasks/{id}/cancel
```

规则：

- `queued`、`preparing_context`、`dispatching`、`building`、`pushing` 可尝试取消。
- 已完成任务返回 `422 INVALID_STATE`。

### 10.5 重试任务

```text
POST /api/v1/build-tasks/{id}/retry
```

响应返回新任务 ID：

```json
{
  "data": {
    "id": "task_2",
    "retryOfTaskId": "task_1",
    "status": "queued"
  }
}
```

### 10.6 历史日志

```text
GET /api/v1/build-tasks/{id}/logs?fromSeq=0&limit=500
```

响应：

```json
{
  "data": {
    "lines": [
      {
        "seq": 1,
        "time": "2026-07-07T10:00:00Z",
        "source": "system",
        "level": "info",
        "message": "Build task queued."
      }
    ],
    "nextSeq": 2,
    "complete": false
  }
}
```

### 10.7 实时日志 SSE

```text
GET /api/v1/build-tasks/{id}/logs/stream?fromSeq=0
```

SSE 事件：

```text
event: log
data: {"seq":1,"time":"2026-07-07T10:00:00Z","source":"docker-build","level":"info","message":"Step 1/4 : FROM ubuntu:24.04"}
```

状态事件：

```text
event: status
data: {"taskId":"task_1","status":"building","time":"2026-07-07T10:00:00Z"}
```

结束事件：

```text
event: complete
data: {"taskId":"task_1","status":"push_success","time":"2026-07-07T10:10:00Z"}
```

错误事件：

```text
event: error
data: {"code":"LOG_STREAM_ERROR","message":"Failed to read log stream."}
```

前端重连：

- 前端保存最后收到的 `seq`。
- 断线重连时带上 `fromSeq`。
- 后端先补发历史日志，再继续推送实时日志。

### 10.8 下载日志

```text
GET /api/v1/build-tasks/{id}/logs/download
```

响应：

```text
Content-Type: text/plain
Content-Disposition: attachment; filename="build-task-task_1.log"
```

## 11. Artifacts API

### 11.1 产物列表

```text
GET /api/v1/artifacts?projectId=project_1&status=available&page=1&pageSize=20
```

支持筛选：

- `projectId`
- `versionNodeId`
- `registryId`
- `architecture`
- `status`
- `pushed`
- `deprecated`

### 11.2 产物详情

```text
GET /api/v1/artifacts/{id}
```

### 11.3 复制拉取命令

```text
GET /api/v1/artifacts/{id}/pull-command
```

响应：

```json
{
  "data": {
    "command": "docker pull harbor.example.com/platform/java-runtime:jdk17-v1"
  }
}
```

### 11.4 重新推送

```text
POST /api/v1/artifacts/{id}/repush
```

请求：

```json
{
  "registryId": "reg_1"
}
```

### 11.5 归档产物

```text
POST /api/v1/artifacts/{id}/archive
```

### 11.6 标记废弃

```text
POST /api/v1/artifacts/{id}/deprecate
```

## 12. Audit Logs API

### 12.1 审计日志列表

```text
GET /api/v1/audit-logs?action=create_build_task&resourceType=build_task&page=1&pageSize=20
```

支持筛选：

- `actorId`
- `action`
- `resourceType`
- `resourceId`
- `createdFrom`
- `createdTo`

## 13. Settings API

### 13.1 获取设置

```text
GET /api/v1/settings
```

响应：

```json
{
  "data": {
    "platform": {
      "name": "Image Build Platform",
      "publicUrl": "http://localhost:8080"
    },
    "build": {
      "maxGlobalConcurrency": 2,
      "defaultTimeoutSeconds": 3600,
      "enableBuildkit": true
    },
    "logs": {
      "retentionDays": 30
    },
    "contexts": {
      "retentionDays": 7
    }
  }
}
```

### 13.2 更新设置

```text
PUT /api/v1/settings
```

请求：

```json
{
  "build": {
    "maxGlobalConcurrency": 4,
    "defaultTimeoutSeconds": 7200,
    "enableBuildkit": true
  },
  "logs": {
    "retentionDays": 60
  }
}
```

## 14. Dashboard API

### 14.1 概览

```text
GET /api/v1/dashboard/summary
```

响应：

```json
{
  "data": {
    "buildTasks": {
      "running": 1,
      "queued": 2,
      "successToday": 8,
      "failedToday": 1
    },
    "buildHosts": {
      "online": 2,
      "offline": 1,
      "disabled": 0
    },
    "registries": {
      "available": 1,
      "unavailable": 0
    },
    "recentFailures": []
  }
}
```

## 15. Help API

MVP 帮助内容可以前端静态维护，不一定需要 API。

如需后端返回：

```text
GET /api/v1/help/topics
GET /api/v1/help/topics/{slug}
```

## 16. Health API

### 16.1 健康检查

```text
GET /healthz
```

响应：

```json
{
  "status": "ok",
  "time": "2026-07-07T10:00:00Z",
  "version": "0.1.0"
}
```

## 17. 权限矩阵

| API 类别 | admin | maintainer | viewer |
| --- | --- | --- | --- |
| setup | yes | no | no |
| auth/me | yes | yes | yes |
| build-hosts read | yes | yes | yes |
| build-hosts write | yes | no | no |
| registries read | yes | yes | yes |
| registries write | yes | no | no |
| image-projects read | yes | yes | yes |
| image-projects write | yes | yes | no |
| version-nodes write | yes | yes | no |
| build-tasks read | yes | yes | yes |
| build-tasks create | yes | yes | no |
| build-tasks cancel | yes | yes | no |
| artifacts read | yes | yes | yes |
| artifacts repush | yes | yes | no |
| settings read | yes | no | no |
| settings write | yes | no | no |
| audit-logs read | yes | no | no |

## 18. 待实现时确认

1. 是否为所有写请求增加 CSRF token。
2. 是否生成 OpenAPI 3.1 文件并作为前端类型来源。
3. 是否使用统一 request ID 并返回 `X-Request-ID`。
4. 是否在 MVP 中提供用户管理 API，还是只支持初始化管理员。

