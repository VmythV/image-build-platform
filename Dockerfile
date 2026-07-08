FROM --platform=$BUILDPLATFORM node:22-alpine AS web-builder

WORKDIR /src/web
COPY web/package*.json ./
COPY web/.npmrc ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS backend-builder

ARG VERSION=dev
ARG TARGETOS=linux
ARG TARGETARCH
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN GO111MODULE=on go mod download
COPY . .
COPY --from=web-builder /src/web/dist ./web/dist
RUN target_arch="${TARGETARCH:-$(go env GOARCH)}" \
    && GO111MODULE=on CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${target_arch}" go build \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/ibp-server ./cmd/ibp-server

FROM alpine:3.22

WORKDIR /app
RUN apk add --no-cache ca-certificates docker-cli openssh-client tzdata wget \
    && mkdir -p /app/data /app/web/dist

COPY --from=backend-builder /out/ibp-server /app/ibp-server
COPY --from=web-builder /src/web/dist /app/web/dist

ENV IBP_SERVER_ADDR=0.0.0.0:8080
ENV IBP_STATIC_DIR=/app/web/dist

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/app/ibp-server"]
