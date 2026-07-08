# Image Build Platform: private image build orchestration

> Image Build Platform is a self-hosted platform for managing Docker build hosts, container registries, Dockerfile versions, build logs, and image artifacts from one web console.
>
> The MVP implementation now includes the core local/SSH build loop: define a Dockerfile, choose a build host, stream logs, push the resulting image to a configured registry, and track the image artifact.

[![status](https://img.shields.io/badge/status-mvp-green)](#roadmap)
[![docs](https://img.shields.io/badge/docs-requirements-green)](docs/01-requirements.md)
[![architecture](https://img.shields.io/badge/docs-architecture-green)](docs/02-architecture.md)
[![mvp](https://img.shields.io/badge/docs-mvp-green)](docs/03-mvp.md)
[![ui](https://img.shields.io/badge/docs-ui_design-green)](docs/04-ui-design.md)
[![database](https://img.shields.io/badge/docs-database-green)](docs/05-database-design.md)
[![api](https://img.shields.io/badge/docs-api-green)](docs/06-api.md)
[![build](https://img.shields.io/badge/docs-build_execution-green)](docs/07-build-execution.md)
[![security](https://img.shields.io/badge/docs-security-green)](docs/08-security.md)
[![deployment](https://img.shields.io/badge/docs-deployment-green)](docs/09-deployment.md)
[![roadmap](https://img.shields.io/badge/docs-roadmap-green)](docs/10-roadmap.md)
[![acceptance](https://img.shields.io/badge/docs-acceptance-green)](docs/11-acceptance.md)
[![deployment](https://img.shields.io/badge/deploy-binary%20%7C%20docker-lightgrey)](#deployment-model)

中文说明见本文档下方。Detailed requirements live in [docs/01-requirements.md](docs/01-requirements.md), architecture decisions live in [docs/02-architecture.md](docs/02-architecture.md), MVP scope lives in [docs/03-mvp.md](docs/03-mvp.md), UI design lives in [docs/04-ui-design.md](docs/04-ui-design.md), database design lives in [docs/05-database-design.md](docs/05-database-design.md), API design lives in [docs/06-api.md](docs/06-api.md), build execution design lives in [docs/07-build-execution.md](docs/07-build-execution.md), security design lives in [docs/08-security.md](docs/08-security.md), deployment design lives in [docs/09-deployment.md](docs/09-deployment.md), implementation roadmap lives in [docs/10-roadmap.md](docs/10-roadmap.md), and acceptance checks live in [docs/11-acceptance.md](docs/11-acceptance.md).

---

## What is Image Build Platform

Image Build Platform is designed for teams that need a private, auditable way to build and publish container images without wiring every workflow by hand.

It gives operators a management console for build hosts and registries, while giving image maintainers a versioned workspace for Dockerfiles, branches, build tasks, logs, and produced images. The platform can run as a downloaded binary or as a Docker container. When containerized, it can either talk to remote builders or access the host Docker daemon through an explicitly mounted Docker socket.

## Product Tour

### Build Host Management

- Use the local machine as the default builder.
- Add remote Linux builders through SSH.
- Detect host architecture, Docker availability, Docker version, BuildKit support, disk space, and connection health.
- Route builds by architecture such as `amd64` and `arm64`.
- Limit concurrency per host.

### Registry Management

- Add self-hosted Docker Registry, Harbor, Docker Hub, Alibaba Cloud, Tencent Cloud, or any Docker Registry v2 compatible endpoint.
- Store credentials securely and show only masked values in the console.
- Use registries for both base-image pulls and final-image pushes.
- Test registry login, pull access, and push access.

### Version And Branch Graph

- Select an image family or starting image first, such as Java, Python, Node.js, or MySQL.
- Start from a base image and evolve it through version nodes.
- Branch from any version to create a new image line.
- Visualize versions and branches as a Git-style node graph.
- Keep each node compact, then open any node to inspect its Dockerfile, generated form config, description, build history, and produced artifacts.
- Compare Dockerfile changes between two versions.

### Build Workspace

- Write any valid Dockerfile directly.
- Generate a Dockerfile from a guided form for common image changes.
- Choose target architecture, build host strategy, target registry, image name, and tag.
- Stream build logs in real time.
- Retry failed builds and inspect the failure reason.
- Push successful builds to the selected registry.

### Image Artifact Management

- List built image artifacts with tag, digest, architecture, source project, version node, build task, and registry.
- Copy pull commands.
- Rebuild or repush an artifact.
- Archive or mark outdated artifacts.

## Core Flow

1. Create a build host or use the default local host.
2. Add a registry for pulling base images and pushing results.
3. Create an image project.
4. Write a Dockerfile or generate one from the form builder.
5. Select architecture, builder, target registry, and image tag.
6. Submit the build task.
7. Watch logs, success state, failure reason, and push status in the admin console.
8. Find the final image in artifact management.

## Development Quick Start

Prerequisites:

- Go 1.25 or newer.
- Node.js 22 or newer.
- npm 10 or newer.

Install frontend dependencies:

```bash
make install
```

Create a local config file if you want to override defaults:

```bash
cp config.example.yaml config.yaml
```

Run the backend and frontend dev servers:

```bash
make dev
```

Build everything:

```bash
make build
```

Run tests and type checks:

```bash
make test
```

Run the built backend with the built frontend assets:

```bash
./bin/ibp-server --config config.yaml --addr 127.0.0.1:8080 --static-dir web/dist
```

Health check:

```bash
curl http://127.0.0.1:8080/healthz
```

On startup, the backend creates the configured data directories, opens the configured database, and applies embedded SQL migrations automatically. The default database is SQLite at `data/app.db`.

## Run With Docker Compose

From a source checkout, start the SQLite single-container mode:

```bash
docker compose up -d
```

From a source checkout, start PostgreSQL mode:

```bash
docker compose -f docker-compose.postgres.yml up -d
```

With a published image, use the deploy compose files:

```bash
docker compose -f deploy/compose/sqlite.yml up -d
docker compose -f deploy/compose/postgres.yml up -d
```

Then open:

```text
http://localhost:8080
```

If local host Docker builds are required in Docker deployment, mount the host socket by uncommenting the `/var/run/docker.sock` volume in the compose file. This gives the platform container control over the host Docker daemon; use it only in trusted environments.

Build the Docker image:

```bash
make docker-build VERSION=dev
```

## Release Package

Build a tarball release for the current OS and architecture:

```bash
make release VERSION=v0.1.0
```

Cross-build by setting `TARGET_OS` and `TARGET_ARCH`:

```bash
VERSION=v0.1.0 TARGET_OS=linux TARGET_ARCH=amd64 bash scripts/release.sh
VERSION=v0.1.0 TARGET_OS=linux TARGET_ARCH=arm64 bash scripts/release.sh
```

Release artifacts are written to `dist/release` with SHA-256 checksums.
Each package includes the server binary, built frontend assets, docs, deploy examples, and operations scripts.

Create a local backup:

```bash
make backup
```

## Deployment Model

### Binary

The platform should ship as a backend binary with bundled or adjacent frontend assets. The binary deployment mode is intended for simple private installations where the service runs directly on a Linux host with access to Docker.

### Docker

The platform should also ship as a Docker image. In this mode, builds can run on remote SSH builders, future agents, or the host Docker daemon when `/var/run/docker.sock` is mounted.

Mounting the host Docker socket gives the platform container high control over the host Docker daemon. Production deployments should prefer dedicated remote builders or a future agent model when possible.

## 中文概览

Image Build Platform 是一个私有化镜像构建平台，目标是把构建主机、镜像仓库、Dockerfile 版本、分支、构建任务、实时日志和镜像产物统一到一个后台管理系统里。

一期重点是跑通完整闭环：

- 本机构建和 SSH 远程构建主机。
- 通用 Docker Registry 管理。
- 镜像项目、版本节点和分支管理。
- Dockerfile 直接编辑和简单表单生成。
- 构建任务队列和实时日志。
- 构建成功后推送到指定仓库。
- 二进制部署和 Docker 部署。

完整需求见 [docs/01-requirements.md](docs/01-requirements.md)，架构设计见 [docs/02-architecture.md](docs/02-architecture.md)，MVP 范围见 [docs/03-mvp.md](docs/03-mvp.md)，前端界面设计见 [docs/04-ui-design.md](docs/04-ui-design.md)，数据库设计见 [docs/05-database-design.md](docs/05-database-design.md)，API 设计见 [docs/06-api.md](docs/06-api.md)，构建执行设计见 [docs/07-build-execution.md](docs/07-build-execution.md)，安全设计见 [docs/08-security.md](docs/08-security.md)，部署设计见 [docs/09-deployment.md](docs/09-deployment.md)，开发路线图见 [docs/10-roadmap.md](docs/10-roadmap.md)，MVP 验收清单见 [docs/11-acceptance.md](docs/11-acceptance.md)。

## Roadmap

- Requirements, architecture, and MVP scope.
- Backend API, persistence, authentication, RBAC, settings, and audit logs.
- Build host connection, local Docker detection, and SSH remote builds.
- Registry credential storage, login testing, image push, and artifact records.
- Dockerfile editor, form builder, validation, diff, and Git-style version graph.
- Build queue, scheduling, cancel/retry, log streaming, and artifact management.
- Binary and Docker packaging with release scripts.
- Remote agent mode.
- Multi-architecture buildx support.
- Git build context, SBOM, and vulnerability scanning.

## Repository Status

This repository contains the MVP implementation: Go backend, React/Vite frontend, configuration loading, database migrations, authentication, build hosts, registries, image projects, version graphs, build tasks, real-time logs, artifact records, settings, audit logs, Docker files, release scripts, and deployment documentation.
