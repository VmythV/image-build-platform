# 安全设计

## 1. 安全目标

镜像构建平台会接触 Docker daemon、SSH 私钥、镜像仓库凭据和可执行任意命令的 Dockerfile。安全设计必须明确风险边界，并让实现默认遵循最小权限和可审计原则。

安全目标：

- 凭据不明文存储。
- 凭据不回显到前端。
- 凭据不进入构建日志。
- 高风险操作有权限控制和审计。
- Docker socket 风险必须在界面和文档中明确提示。
- 构建任务必须可取消、可超时、可追踪。
- 外部命令执行避免 shell 注入。

## 2. 信任边界

```text
Browser
  -> Web UI
  -> Go Backend
     -> Database
     -> Local filesystem
     -> Docker CLI / Docker daemon
     -> SSH remote hosts
     -> Registry endpoints
```

信任级别：

- 后端服务是受信任核心。
- 数据库和数据目录必须视为敏感资产。
- Docker daemon 具备宿主机高权限。
- SSH 远程主机具备执行 Docker 构建的权限。
- Dockerfile 内容不可信，必须按高风险输入处理。
- 镜像仓库和远程网络返回内容不可信。

## 3. 威胁模型

主要威胁：

- 未授权用户发起构建。
- 低权限用户修改主机或仓库凭据。
- 凭据在数据库、日志、API 响应或错误信息中泄漏。
- Dockerfile 执行恶意命令。
- 挂载 Docker socket 后平台容器控制宿主机 Docker。
- SSH 私钥泄漏后远程主机被接管。
- 命令参数拼接导致注入。
- 上传构建上下文路径穿越。
- 构建日志过大导致服务不可用。
- 长时间构建耗尽主机资源。
- 镜像推送到错误仓库或错误命名空间。

## 4. 认证

### 4.1 Session

MVP 使用服务端 session：

- Cookie 名称：`ibp_session`。
- Cookie 必须设置 `HttpOnly`。
- Cookie 必须设置 `SameSite=Lax`。
- HTTPS 部署时必须设置 `Secure`。
- session token 只在客户端保存随机值。
- 数据库只保存 session token hash。

### 4.2 密码

要求：

- 密码只保存强哈希。
- 推荐 Argon2id。
- 若实现成本需要降低，可先使用 bcrypt，并保留升级字段。
- 登录失败返回统一错误，避免枚举用户。
- 管理员初始化时必须校验密码强度。

密码强度 MVP 要求：

- 最少 10 个字符。
- 不能全为空白。
- 不能与用户名相同。

### 4.3 登录保护

MVP 应支持：

- 登录失败审计。
- 同一用户名短时间多次失败后短暂限流。
- session 过期。
- 主动退出。

后续支持：

- MFA。
- OIDC。
- LDAP。
- API token。

## 5. 授权

### 5.1 角色

MVP 角色：

```text
admin
maintainer
viewer
```

### 5.2 权限原则

- 只有 admin 能管理构建主机。
- 只有 admin 能管理镜像仓库和凭据。
- admin 和 maintainer 能管理镜像项目、版本、分支和构建任务。
- viewer 只能查看。
- settings 和 audit logs 只有 admin 可访问。

### 5.3 服务端强制授权

权限必须在后端校验，不能只依赖前端隐藏按钮。

校验位置：

- middleware 检查是否登录。
- route 层检查基础角色。
- service 层检查具体资源和状态。

## 6. 凭据安全

### 6.1 凭据范围

需要加密的凭据：

- SSH 私钥。
- SSH 密码，MVP 可不支持。
- 镜像仓库密码。
- 镜像仓库 token。
- 后续 Git token。

### 6.2 加密存储

凭据表只保存密文。

要求：

- 使用 authenticated encryption，例如 AES-GCM 或 XChaCha20-Poly1305。
- 每条凭据使用独立 nonce。
- 密钥来自 `security.secret_key` 或外部 secret 文件。
- 生产环境不允许使用空密钥。
- 密文中记录加密版本，支持后续轮换。

### 6.3 密钥管理

MVP 支持：

- 配置文件或环境变量提供 `security.secret_key`。
- 启动时校验密钥存在和长度。
- 密钥不写入日志。

后续支持：

- KMS。
- Vault。
- 密钥轮换工具。

### 6.4 API 脱敏

API 不返回：

- `encrypted_value`。
- SSH 私钥。
- 仓库密码。
- token 明文。

API 可以返回：

- 是否已配置凭据。
- SSH 公钥指纹。
- token 指纹。
- 最近更新时间。

### 6.5 凭据使用

使用规则：

- 只在执行需要时解密。
- 解密值只保留在内存中。
- 使用后尽快释放引用，Go 中不能保证强擦除，但要缩短生命周期。
- 不把凭据写入临时文件，除非必要。
- 仓库密码通过 stdin 传给 `docker login`。

## 7. Docker socket 风险

挂载 `/var/run/docker.sock` 到平台容器意味着平台容器可以控制宿主机 Docker daemon。实际风险接近宿主机 root 权限。

必须提示：

- Docker socket 只建议在可信内网或单机部署中使用。
- 生产环境优先使用专用远程构建主机或后续 Agent。
- 只有 admin 可以启用本机构建主机。
- Docker socket 未挂载时，本机构建应显示不可用，而不是静默失败。

界面提示位置：

- 新增 Local Docker 主机表单。
- Docker 部署帮助页。
- 主机检测失败详情。
- 部署文档。

## 8. SSH 安全

### 8.1 SSH 凭据

MVP 优先支持 SSH 私钥。

要求：

- 私钥加密存储。
- 私钥不回显。
- 私钥不写入日志。
- 私钥不上传到远程主机。
- 更新私钥时覆盖原凭据。

### 8.2 远程主机用户

推荐：

- 使用专用 `builder` 用户。
- 只授予 Docker 构建所需权限。
- 不复用 root 用户。
- 限制 SSH key 的使用范围。

现实限制：

- 如果用户属于 `docker` 组，通常可以控制 Docker daemon。
- 这同样是高权限能力，界面需要提示。

### 8.3 Host Key

MVP 可以先提供两种模式：

- `strict`：校验 known_hosts。
- `insecure_skip_verify`：跳过校验，仅用于测试。

默认建议：

- 二进制部署可以使用本机 known_hosts。
- Web 表单可后续支持录入 host key 指纹。

如果 MVP 为了降低复杂度选择跳过 host key 校验，必须在 UI 明确标注风险。

## 9. 镜像仓库安全

要求：

- 仓库密码和 token 加密存储。
- `docker login` 使用 `--password-stdin`。
- 每个构建任务使用独立临时 `DOCKER_CONFIG`。
- 任务结束后删除临时 `DOCKER_CONFIG`。
- 不在日志中打印 `docker login` 的 stdin。
- 不安全 HTTP 仓库必须有高风险提示。

推送保护：

- 目标镜像完整引用在提交构建前预览。
- 推送仓库必须是已启用且允许推送的仓库。
- viewer 不能触发推送。
- maintainer 只能触发构建和推送，不能修改仓库凭据。

## 10. Dockerfile 和构建安全

Dockerfile 可以执行任意命令。平台不应把 Dockerfile 视为安全脚本。

MVP 控制措施：

- 只有 admin 和 maintainer 可以创建和构建 Dockerfile。
- viewer 只读。
- 构建任务必须有超时。
- 构建主机必须有并发限制。
- 构建日志保留并可审计。
- 构建上下文大小必须限制。

不承诺：

- 阻止恶意 Dockerfile 对构建主机造成影响。
- 沙箱化 Docker build。
- 检测所有恶意行为。

建议：

- 生产环境使用专用构建主机。
- 不在平台服务宿主机上构建不可信 Dockerfile。
- 对不同信任级别的镜像使用不同构建主机标签。

## 11. 命令执行安全

规则：

- 不使用 shell 拼接命令。
- 使用参数数组调用外部命令。
- 用户输入必须作为参数值传递，不拼接成命令字符串。
- 远程 SSH 命令需要严格转义路径和参数。
- 文件路径只允许使用平台生成的目录。
- 不允许用户指定任意宿主机路径作为构建上下文。

需要校验的输入：

- 镜像 tag。
- registry endpoint。
- namespace。
- image name。
- build arg key。
- 构建上下文路径。
- SSH 地址和端口。

## 12. 构建上下文安全

上传压缩包必须防护：

- Zip Slip 路径穿越。
- 绝对路径。
- 超大文件。
- 超多文件。
- 符号链接逃逸。
- 特殊设备文件。

MVP 建议：

- 禁止符号链接。
- 禁止隐藏控制文件覆盖平台生成文件。
- 限制解压后大小。
- 限制文件数量。
- Dockerfile 由平台单独写入，不允许上传包覆盖。

## 13. 日志安全

日志风险：

- Docker 命令输出可能包含环境变量。
- 构建脚本可能 echo secret。
- 错误信息可能包含 token。

平台措施：

- 对已知敏感值做精确脱敏。
- 对 token-like 字符串做保守脱敏，作为后续增强。
- 命令展示时隐藏密码参数。
- 日志下载接口需要权限。
- 构建日志保留天数可配置。

限制：

- 如果用户在 Dockerfile 中主动打印未知 secret，平台无法保证完全识别。
- 高敏构建应避免把 secret 作为普通 build arg 传入。

## 14. Web 安全

### 14.1 CSRF

因为 MVP 使用 Cookie session，写接口需要考虑 CSRF。

建议：

- `SameSite=Lax` 是最低要求。
- 对所有非 GET 请求增加 CSRF token，建议在 MVP 实现。
- 如果不做 CSRF token，必须限制 CORS 并记录为安全债务。

### 14.2 CORS

默认不允许跨域。

只有明确配置 `server.allowed_origins` 时才启用跨域。

### 14.3 安全响应头

建议：

```text
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: no-referrer
Content-Security-Policy: default-src 'self'
```

开发环境可以放宽 CSP，生产环境应收紧。

## 15. 审计日志

必须审计：

- 登录成功。
- 登录失败。
- 退出登录。
- 创建、修改、删除构建主机。
- 检测构建主机。
- 创建、修改、删除镜像仓库。
- 检测镜像仓库。
- 创建镜像项目。
- 创建版本节点。
- 创建分支。
- 发起构建。
- 取消构建。
- 重试构建。
- 重新推送产物。
- 修改系统设置。

审计记录必须包含：

- 操作人。
- 动作。
- 资源类型。
- 资源 ID。
- IP。
- User-Agent。
- request ID。
- 时间。

审计记录不能包含：

- 密码。
- token。
- SSH 私钥。
- 未脱敏 build args。

## 16. 数据目录安全

数据目录包含：

- SQLite 数据库。
- 构建日志。
- 构建上下文。
- 临时 Docker 配置。
- 备份。

要求：

- 数据目录权限仅平台运行用户可读写。
- 备份文件视为敏感文件。
- Docker 部署时数据卷不能暴露给不可信容器。
- 日志目录不能被 Web 静态文件服务直接暴露。

## 17. 部署安全

### 17.1 二进制部署

建议：

- 使用专用系统用户运行服务。
- 使用 systemd 管理。
- 限制数据目录权限。
- 如果需要本机构建，明确该用户具备 Docker 权限。
- 配置反向代理 HTTPS。

### 17.2 Docker 部署

建议：

- 不以 privileged 模式运行平台容器。
- 只在确实需要本机构建时挂载 Docker socket。
- 数据卷单独挂载。
- 生产环境使用 HTTPS 反向代理。
- 配置非空 `security.secret_key`。

## 18. 备份和恢复

备份内容：

- 数据库。
- `data/contexts`，可选。
- `data/logs`，可选。
- 配置文件。
- 加密密钥。

重要规则：

- 没有加密密钥，凭据密文无法恢复。
- 备份必须加密保存。
- 恢复到新环境前确认 Docker socket 和 SSH 主机权限。

## 19. 安全验收清单

MVP 必须满足：

- 未登录不能访问 API。
- viewer 不能创建构建任务。
- maintainer 不能修改仓库凭据。
- SSH 私钥不回显。
- 仓库密码不回显。
- 数据库中凭据不是明文。
- 构建日志中不出现仓库密码。
- Docker socket 表单和帮助页有风险提示。
- 构建任务有超时。
- 构建任务可以取消。
- 上传上下文不能路径穿越。
- 所有外部命令不通过 shell 拼接执行。
- 审计日志记录关键操作。

## 20. 后续增强

- 项目级权限。
- OIDC/LDAP。
- API token。
- MFA。
- KMS/Vault。
- SSH host key 管理界面。
- BuildKit secret。
- 镜像漏洞扫描。
- SBOM。
- 策略引擎，例如禁止推送到生产仓库前未审批。
- Agent 主动连接，减少平台保存 SSH 私钥。

