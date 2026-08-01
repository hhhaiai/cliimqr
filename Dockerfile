# cliimqr 多平台发布构建
# 交叉编译所有平台二进制，打包为 tar.gz/zip/deb/rpm
#
# 用法:
#   docker build --build-arg VERSION=1.2 --build-arg BUILD=1 -t cliimqr-builder .
#   docker run --rm -v "$PWD/release:/out" cliimqr-builder
#
# 或使用 build.sh 一键构建:
#   ./build.sh [version]

# ------------------------------------------------
# Stage 1: 交叉编译 Go 二进制
# ------------------------------------------------
FROM golang:1.24-alpine3.21 AS builder

RUN apk add --no-cache git

ARG VERSION=1.2
WORKDIR /src
COPY . .

# 交叉编译所有平台组合
RUN set -e; \
    for os in linux darwin windows; do \
      for arch in amd64 arm64 arm 386; do \
        case "${os}-${arch}" in \
          darwin-arm|darwin-386|windows-arm) continue ;; \
        esac; \
        ext=""; [ "$os" = "windows" ] && ext=".exe"; \
        echo "==> Building cliimqr-${os}-${arch}${ext}"; \
        GOOS="${os}" GOARCH="${arch}" go build \
          -trimpath -buildvcs=false \
          -ldflags="-s -w -X main.appVersion=${VERSION}" \
          -o "/build/cliimqr-${os}-${arch}${ext}" .; \
      done; \
    done; \
    echo "==> All builds complete"; \
    ls -la /build/

# ------------------------------------------------
# Stage 2: 打包为发行产物
# ------------------------------------------------
FROM alpine:3.21 AS packager

RUN apk add --no-cache tar zip bash coreutils binutils rpm

# 安装 nfpm (Go 静态二进制，无需依赖)
ARG NFPM_VERSION=2.41.1
RUN wget -q "https://github.com/goreleaser/nfpm/releases/download/v${NFPM_VERSION}/nfpm_${NFPM_VERSION}_Linux_x86_64.tar.gz" \
    -O /tmp/nfpm.tar.gz && \
    tar xzf /tmp/nfpm.tar.gz -C /usr/local/bin/ nfpm && \
    chmod +x /usr/local/bin/nfpm && \
    rm /tmp/nfpm.tar.gz

ARG VERSION=1.2
ARG BUILD=1
ARG BUILD_DATE=$(date -u +%Y%m%d%H%M%S)
WORKDIR /out

# 复制所有编译好的二进制
COPY --from=builder /build/ /build/

# 复制打包脚本
COPY docker-package.sh /docker-package.sh
RUN chmod +x /docker-package.sh

ENTRYPOINT ["/docker-package.sh"]