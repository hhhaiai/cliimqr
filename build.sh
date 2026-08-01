#!/bin/bash
# build.sh — cliimqr 一键构建脚本
#
# 用法:
#   ./build.sh                    # 默认版本号，使用 Docker 构建
#   ./build.sh --local            # 本地交叉编译 + 打包（无需 Docker）
#   ./build.sh 1.3                # 指定版本号
#   ./build.sh 1.3 2              # 指定版本号和构建号
#
# 产物在 release/ 目录

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# 确定版本号
if [ $# -ge 1 ] && [ "$1" != "--local" ]; then
  VERSION="$1"
elif [ $# -ge 2 ] && [ "$2" != "--local" ]; then
  VERSION="$2"
else
  GIT_TAG=$(git describe --tags --abbrev=0 2>/dev/null || true)
  if [ -n "$GIT_TAG" ]; then
    VERSION="${GIT_TAG#v}"
  else
    VERSION=$(grep 'appVersion =' main.go | sed 's/.*"\(.*\)".*/\1/')
    [ -z "$VERSION" ] && VERSION="1.2"
  fi
fi

BUILD="${2:-1}"
BUILD_DATE="$(date -u +%Y%m%d%H%M%S)"
USE_LOCAL=false

for arg in "$@"; do
  [ "$arg" = "--local" ] && USE_LOCAL=true
done

mkdir -p release

echo "=========================================="
echo "  cliimqr 构建"
echo "  版本: ${VERSION}"
echo "  构建: ${BUILD}"
echo "  模式: $([ "$USE_LOCAL" = true ] && echo "本地编译" || echo "Docker")"
echo "=========================================="
echo ""

if [ "$USE_LOCAL" = true ]; then
  # ---- 本地编译模式 ----
  echo "==> 交叉编译所有平台..."
  for os in linux darwin windows; do
    for arch in amd64 arm64 arm 386; do
      case "${os}-${arch}" in
        darwin-arm|darwin-386|windows-arm) continue ;;
      esac
      ext=""; [ "$os" = "windows" ] && ext=".exe"
      echo -n "  ${os}/${arch}... "
      GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build \
        -trimpath -buildvcs=false \
        -ldflags="-s -w -X main.appVersion=${VERSION}" \
        -o "release/cliimqr-${os}-${arch}${ext}" . && echo "OK" || echo "FAIL"
    done
  done

  echo ""
  echo "==> 打包发行产物..."

  # 架构名映射: goarch → vscode-style (tar.gz/zip名)
vscode_arch_name() {
  case "$1" in
    amd64) echo "x64" ;;
    386)   echo "x86" ;;
    arm64) echo "arm64" ;;
    arm)   echo "armhf" ;;
  esac
}

  PLATFORMS=("linux:amd64" "linux:386" "linux:arm64" "linux:arm"
             "darwin:amd64" "darwin:arm64"
             "windows:amd64" "windows:386" "windows:arm64")

  for entry in "${PLATFORMS[@]}"; do
    os="${entry%%:*}"
    arch="${entry##*:}"
    vscode_arch=$(vscode_arch_name "$arch")
    ext=""; [ "$os" = "windows" ] && ext=".exe"
    src="release/cliimqr-${os}-${arch}${ext}"

    [ ! -f "$src" ] && { echo "  [SKIP] ${os}/${arch}"; continue; }

    tmpdir="$(mktemp -d)"
    cp "$src" "${tmpdir}/cliimqr${ext}"
    chmod +x "${tmpdir}/cliimqr${ext}"

    if [ "$os" = "windows" ]; then
      archive="cliimqr-win32-${vscode_arch}-${VERSION}.zip"
      (cd "$tmpdir" && zip -q "${SCRIPT_DIR}/release/${archive}" "cliimqr${ext}")
      echo "  [ZIP]  ${archive}"
    elif [ "$os" = "darwin" ]; then
      archive="cliimqr-darwin-${vscode_arch}-${VERSION}.tar.gz"
      tar czf "release/${archive}" -C "$tmpdir" "cliimqr${ext}"
      echo "  [TAR]  ${archive}"
    else
      archive="cliimqr-linux-${vscode_arch}-${VERSION}.tar.gz"
      tar czf "release/${archive}" -C "$tmpdir" "cliimqr${ext}"
      echo "  [TAR]  ${archive}"
    fi
    rm -rf "$tmpdir"
    rm "$src"  # 删除裸二进制
  done

  echo ""
  echo "==> Checksums..."
  cd release
  sha256sum cliimqr-* > "cliimqr-${VERSION}-checksums.txt" 2>/dev/null || \
    shasum -a 256 cliimqr-* > "cliimqr-${VERSION}-checksums.txt"
  cd "$SCRIPT_DIR"

else
  # ---- Docker 构建模式 ----
  echo "==> 构建 Docker 镜像..."
  docker build \
    --build-arg "VERSION=${VERSION}" \
    --build-arg "BUILD=${BUILD}" \
    --build-arg "BUILD_DATE=${BUILD_DATE}" \
    -t cliimqr-builder .

  echo ""
  echo "==> 打包发行产物..."
  docker run --rm \
    -v "$(pwd)/release:/out" \
    -e "VERSION=${VERSION}" \
    -e "BUILD=${BUILD}" \
    -e "BUILD_DATE=${BUILD_DATE}" \
    cliimqr-builder
fi

echo ""
echo "=========================================="
echo "  构建完成！"
echo "  产物目录: $(pwd)/release/"
echo "=========================================="
ls -lh release/