APP_NAME := ibp-server
VERSION ?= dev
SERVER_ADDR ?= 0.0.0.0:8080
WEB_DIR := web
GO ?= go
NPM ?= npm

.PHONY: install dev dev-api dev-web build build-api build-web test fmt docker-build release backup restore clean

install:
	$(NPM) --prefix $(WEB_DIR) install

dev:
	$(MAKE) -j2 dev-api dev-web

dev-api:
	GO111MODULE=on $(GO) run ./cmd/ibp-server --addr $(SERVER_ADDR)

dev-web:
	$(NPM) --prefix $(WEB_DIR) run dev -- --host 0.0.0.0

build: build-web build-api

build-web:
	$(NPM) --prefix $(WEB_DIR) run build

build-api:
	mkdir -p bin
	GO111MODULE=on CGO_ENABLED=0 $(GO) build -ldflags "-X main.version=$(VERSION)" -o bin/$(APP_NAME) ./cmd/ibp-server

test:
	GO111MODULE=on $(GO) test ./...
	$(NPM) --prefix $(WEB_DIR) run typecheck

fmt:
	gofmt -w cmd internal

docker-build:
	docker build --build-arg VERSION=$(VERSION) -t image-build-platform:$(VERSION) .

release:
	VERSION=$(VERSION) bash scripts/release.sh

backup:
	bash scripts/backup.sh

restore:
	BACKUP_FILE="$(BACKUP_FILE)" APP_DIR="$(APP_DIR)" DATA_DIR="$(DATA_DIR)" CONFIG_FILE="$(CONFIG_FILE)" SAFETY_DIR="$(SAFETY_DIR)" bash scripts/restore.sh

clean:
	rm -rf bin dist $(WEB_DIR)/dist
