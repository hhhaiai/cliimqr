#!/bin/bash
# docker-package.sh — 在 Docker 容器内运行，生成所有发行产物
# 使用 nfpm 打包 deb/rpm，手动打包 tar.gz/zip
set -euo pipefail

VERSION="${VERSION:-1.2}"
BUILD="${BUILD:-1}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y%m%d%H%M%S)}"

echo "==> Packaging cliimqr ${VERSION} (build ${BUILD})"

# 架构名映射: goarch → vscode-style (tar.gz/zip名)
declare -A VSCODE_ARCH
VSCODE_ARCH[amd64]="x64"
VSCODE_ARCH[386]="x86"
VSCODE_ARCH[arm64]="arm64"
VSCODE_ARCH[arm]="armhf"

# 架构名映射: goarch → deb arch
declare -A DEB_ARCH
DEB_ARCH[amd64]="amd64"
DEB_ARCH[386]="i386"
DEB_ARCH[arm64]="arm64"
DEB_ARCH[arm]="armhf"

# 架构名映射: goarch → rpm arch
declare -A RPM_ARCH
RPM_ARCH[amd64]="x86_64"
RPM_ARCH[386]="i386"
RPM_ARCH[arm64]="aarch64"
RPM_ARCH[arm]="armv7hl"

# 所有平台组合
PLATFORMS=(
  "linux:amd64"
  "linux:386"
  "linux:arm64"
  "linux:arm"
  "darwin:amd64"
  "darwin:arm64"
  "windows:amd64"
  "windows:386"
  "windows:arm64"
)

# ---- 清理 ----
rm -rf /out/*

# ---- 1. tar.gz 和 zip 包 ----
echo ""
echo "============================================"
echo "  Stage 1: tar.gz / zip 归档"
echo "============================================"

for entry in "${PLATFORMS[@]}"; do
  os="${entry%%:*}"
  arch="${entry##*:}"

  vscode_arch="${VSCODE_ARCH[$arch]}"
  ext=""
  [ "$os" = "windows" ] && ext=".exe"

  binary_name="cliimqr${ext}"
  src="/build/cliimqr-${os}-${arch}${ext}"

  if [ ! -f "$src" ]; then
    echo "  [SKIP] ${os}/${arch} — 二进制不存在"
    continue
  fi

  tmpdir="$(mktemp -d)"
  cp "$src" "${tmpdir}/${binary_name}"
  chmod +x "${tmpdir}/${binary_name}"

  if [ "$os" = "windows" ]; then
    archive_name="cliimqr-win32-${vscode_arch}-${VERSION}.zip"
    (cd "$tmpdir" && zip -q "/out/${archive_name}" "${binary_name}")
    echo "  [ZIP]  ${archive_name}"
  elif [ "$os" = "darwin" ]; then
    archive_name="cliimqr-darwin-${vscode_arch}-${VERSION}.tar.gz"
    tar czf "/out/${archive_name}" -C "$tmpdir" "${binary_name}"
    echo "  [TAR]  ${archive_name}"
  else
    archive_name="cliimqr-linux-${vscode_arch}-${VERSION}.tar.gz"
    tar czf "/out/${archive_name}" -C "$tmpdir" "${binary_name}"
    echo "  [TAR]  ${archive_name}"
  fi

  rm -rf "$tmpdir"
done

# ---- 2. deb 包 ----
echo ""
echo "============================================"
echo "  Stage 2: deb 包"
echo "============================================"

for arch in amd64 386 arm64 arm; do
  src="/build/cliimqr-linux-${arch}"
  if [ ! -f "$src" ]; then
    echo "  [SKIP] linux/${arch} — 二进制不存在"
    continue
  fi

  deb_arch="${DEB_ARCH[$arch]}"
  pkg="cliimqr_${VERSION}-${BUILD}_${deb_arch}"
  tmpdir="$(mktemp -d)"

  # 构建 deb 目录结构
  debdir="${tmpdir}/deb"
  mkdir -p "${debdir}/DEBIAN"
  mkdir -p "${debdir}/usr/bin"
  cp "$src" "${debdir}/usr/bin/cliimqr"
  chmod 755 "${debdir}/usr/bin/cliimqr"

  # 计算安装大小（KB）
  inst_size=$(stat -c%s "$src" 2>/dev/null || stat -f%z "$src" 2>/dev/null)
  inst_size=$((inst_size / 1024))

  cat > "${debdir}/DEBIAN/control" <<EOF
Package: cliimqr
Version: ${VERSION}
Section: utils
Priority: optional
Architecture: ${deb_arch}
Installed-Size: ${inst_size}
Maintainer: SimiRouter <support@dwchainless.com>
Description: CLI tool for beautified QR codes
 A command-line tool that generates beautified QR codes
 via the cli.im web interface. Supports custom logos,
 colors, dot patterns, and eye shapes.
EOF

  # 手动构建 deb 包
  cd "$tmpdir"
  tar czf control.tar.gz -C deb/DEBIAN .
  tar czf data.tar.gz -C deb usr/
  echo "2.0" > debian-binary
  ar -rcs "/out/${pkg}.deb" debian-binary control.tar.gz data.tar.gz

  echo "  [DEB]  ${pkg}.deb"
  rm -rf "$tmpdir"
done

# ---- 3. rpm 包 ----
echo ""
echo "============================================"
echo "  Stage 3: rpm 包"
echo "============================================"

# 检查 rpmbuild 是否可用
RPMBUILD_AVAILABLE=false
command -v rpmbuild &>/dev/null && RPMBUILD_AVAILABLE=true

for arch in amd64 386 arm64 arm; do
  src="/build/cliimqr-linux-${arch}"
  if [ ! -f "$src" ]; then
    echo "  [SKIP] linux/${arch} — 二进制不存在"
    continue
  fi

  if [ "$RPMBUILD_AVAILABLE" = false ]; then
    echo "  [SKIP] linux/${arch} rpm — rpmbuild 不可用"
    continue
  fi

  rpm_arch="${RPM_ARCH[$arch]}"
  pkg="cliimqr-${VERSION}-${BUILD}.${rpm_arch}"
  tmpdir="$(mktemp -d)"

  # rpmbuild 需要特定目录结构
  rpmdir="${tmpdir}/rpmbuild"
  mkdir -p "${rpmdir}/BUILD" "${rpmdir}/RPMS" "${rpmdir}/SOURCES" "${rpmdir}/SPECS" "${rpmdir}/SRPMS"

  # 准备源码目录
  buildroot="${tmpdir}/cliimqr-${VERSION}"
  mkdir -p "${buildroot}/usr/bin"
  cp "$src" "${buildroot}/usr/bin/cliimqr"
  chmod 755 "${buildroot}/usr/bin/cliimqr"
  tar czf "${rpmdir}/SOURCES/cliimqr-${VERSION}.tar.gz" -C "$(dirname "$buildroot")" "cliimqr-${VERSION}"

  cat > "${rpmdir}/SPECS/cliimqr.spec" <<EOF
%define _topdir ${rpmdir}
%define _buildhost localhost
%define _target_cpu ${rpm_arch}

Name: cliimqr
Version: ${VERSION}
Release: ${BUILD}
Summary: CLI tool for beautified QR codes
License: MIT
URL: https://github.com/hhhaiai/cliimqr
BuildArch: ${rpm_arch}
AutoReqProv: no

%description
A command-line tool that generates beautified QR codes
via the cli.im web interface. Supports custom logos,
colors, dot patterns, and eye shapes.

%prep
%setup -q -n cliimqr-${VERSION}

%install
install -d %{buildroot}/usr/bin
install -m 755 usr/bin/cliimqr %{buildroot}/usr/bin/cliimqr

%files
/usr/bin/cliimqr

%clean
rm -rf %{buildroot}
EOF

  rpmbuild --define "_topdir ${rpmdir}" \
    --define "_buildhost localhost" \
    --define "_target_cpu ${rpm_arch}" \
    -bb "${rpmdir}/SPECS/cliimqr.spec" --quiet 2>/dev/null || true

  found_rpm=$(find "${rpmdir}/RPMS" -name "*.rpm" -type f 2>/dev/null | head -1)
  if [ -n "$found_rpm" ]; then
    cp "$found_rpm" "/out/${pkg}.rpm"
    echo "  [RPM]  ${pkg}.rpm"
  else
    echo "  [SKIP] linux/${arch} rpm — rpmbuild 失败"
  fi

  rm -rf "$tmpdir"
done

# ---- 4. checksums ----
echo ""
echo "============================================"
echo "  Stage 4: Checksums"
echo "============================================"

cd /out
sha256sum cliimqr-* > "cliimqr-${VERSION}-checksums.txt"
echo "  [SUM]  cliimqr-${VERSION}-checksums.txt"

echo ""
echo "============================================"
echo "  完成！产物清单:"
echo "============================================"
ls -lh /out/