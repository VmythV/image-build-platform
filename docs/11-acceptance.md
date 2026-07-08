# MVP 验收清单

## 1. 基础启动

- `make test` 通过。
- `make build` 通过。
- `./bin/ibp-server --addr 127.0.0.1:8080 --static-dir web/dist` 可以启动。
- `GET /healthz` 返回 `status=ok`。
- 首次访问 Web 控制台可以初始化管理员。
- 初始化后可以登录、退出、重新登录。

## 2. Docker 部署

- `make docker-build VERSION=dev` 可以构建镜像。
- `docker compose up -d` 可以启动 SQLite 单容器版本。
- 容器 `/healthz` 健康检查通过。
- 不挂载 `/var/run/docker.sock` 时，本机构建检测应失败且错误可读。
- 挂载 `/var/run/docker.sock` 后，本机 Docker 主机检测成功。

## 3. 构建主机

- 默认 `Local Docker` 主机自动创建。
- 主机 Check 能显示架构、OS、Docker 版本和 BuildKit 状态。
- 可以添加 SSH 主机。
- SSH 主机 Check 能连接远端 Docker。
- 禁用主机后不参与调度。
- 主机并发计数在任务结束后回到正常值。

## 4. 镜像仓库

- 可以添加通用 Registry。
- 凭据保存后前端不回显密码。
- Registry Check 成功时状态为 `available`。
- 错误凭据 Check 失败且错误信息可读。
- 可以设置默认 push registry。

## 5. 镜像项目和版本图

- 可以创建 Java、Python、Node、MySQL 等不同镜像项目。
- 进入项目后能看到 Git-style 版本图。
- 可以从根节点创建子版本。
- 可以从任意节点创建分支。
- 点击节点能看到 Dockerfile、说明和构建入口。
- Dockerfile 校验 API 能识别缺少 `FROM` 的文件。

## 6. 构建任务

- 可以从版本节点创建构建任务。
- 任务进入 `queued`。
- Dispatch/Start 后进入 `building`。
- 构建成功后进入 `pushing`。
- 推送成功后进入 `push_success`。
- 构建失败进入 `build_failed`。
- 推送失败进入 `push_failed`。
- 取消任务能终止外部命令并进入 `canceled`。
- Retry 会创建新的任务并保留 `retry_of_task_id`。

## 7. 日志和产物

- 构建中可以 Stream Logs。
- 构建结束后可以 Load Logs。
- SSE 流在任务结束时发送最终状态。
- 推送成功后 Artifacts 页面出现镜像产物。
- 产物包含 image ref、tag、digest、架构、项目、仓库。
- 可以复制 `docker pull` 命令。

## 8. Settings、Help 和审计

- Dashboard 展示真实统计数据。
- Help 页面包含主机、Registry、日志、SSH、Docker socket 风险和失败排查。
- Settings 页面可以查看默认系统设置。
- admin 可以修改设置。
- 写操作会进入 Audit Logs。
- 非 admin 查看 Audit Logs 返回 forbidden。

## 9. 备份和恢复

- `make backup` 能生成备份包。
- SQLite 数据库、配置和数据目录包含在备份范围内。
- 使用备份恢复后服务能启动。
- 恢复后管理员能登录。
- 恢复后 Registry 凭据能解密。

## 10. 发布包

- `make release VERSION=v0.1.0` 能生成 tar.gz。
- 发布包包含：
  - `ibp-server`
  - `web/dist`
  - `config.example.yaml`
  - `docs`
  - `deploy/compose/sqlite.yml`
  - `deploy/compose/postgres.yml`
  - `deploy/systemd/image-build-platform.service`
  - `scripts`
- checksums 文件生成成功。

## 11. CI/CD

- push 到 `main` 会运行 CI。
- pull request 会运行 CI。
- CI 包含 `make test`。
- CI 包含 `make build`。
- CI 校验所有 compose 文件。
- CI 执行 Docker build smoke。
- tag `v*` 会创建或更新 GitHub Release。
- tag `v*` 会发布 Linux `amd64` 和 `arm64` 二进制包。
- tag `v*` 会发布 GHCR 多架构镜像。
