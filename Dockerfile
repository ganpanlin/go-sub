# syntax=docker/dockerfile:1
# 构建说明：依赖（含 mihomo 延迟测速内核）已固化在 vendor/ 目录，
# 构建阶段 -mod=vendor 离线编译，不需要访问 Go 模块代理。

FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -trimpath \
  -ldflags="-s -w -X go-sub/internal/version.Version=${VERSION} -X go-sub/internal/version.Commit=${COMMIT} -X go-sub/internal/version.BuildTime=${BUILD_TIME}" \
  -o /out/proxy-filter ./cmd/proxy-filter

FROM alpine:3.22

# ca-certificates: mihomo 真实延迟测速需要验证 TLS 证书
# tzdata: 时区支持
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /out/proxy-filter /app/proxy-filter
COPY frontend /app/frontend
COPY default-data /app/default-data
COPY scripts/docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh

EXPOSE 8080
VOLUME ["/app/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/api/health >/dev/null || exit 1

ENTRYPOINT ["/app/docker-entrypoint.sh"]
