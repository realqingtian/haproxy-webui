#!/usr/bin/env bash
# 在 HAProxy 节点上安装 dataplaneapi(下载 GitHub release 二进制 + 安装 systemd unit)
# 用法: sudo ./install.sh [版本号,默认 latest]
# 注意:脚本未在实机全量验证,执行后请运行 `dataplaneapi --help` 核对参数名,并在 haproxy.cfg 中合并同目录 haproxy.cfg.snippet
set -euo pipefail

VERSION="${1:-latest}"
INSTALL_DIR=/usr/local/bin
REPO="haproxytech/dataplaneapi"

if [[ $VERSION == "latest" ]]; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep -o '"tag_name": *"[^"]*"' | cut -d'"' -f4)
fi
VERSION_NUM=${VERSION#v}
echo ">> installing dataplaneapi ${VERSION}"

ARCH=$(uname -m)
# 注意:release 资产命名中 64 位 x86 是 x86_64(amd64 只有 apk/deb/rpm 包),arm64 同名
case "$ARCH" in
    x86_64) ARCH_NAME=x86_64 ;;
    aarch64 | arm64) ARCH_NAME=arm64 ;;
    *) echo "unsupported arch: $ARCH"; exit 1 ;;
esac

URL="https://github.com/${REPO}/releases/download/${VERSION}/dataplaneapi_${VERSION_NUM}_linux_${ARCH_NAME}.tar.gz"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "$URL" -o "$TMP/dpapi.tar.gz"
tar -xzf "$TMP/dpapi.tar.gz" -C "$TMP"
install -m 0755 "$TMP/dataplaneapi" "$INSTALL_DIR/dataplaneapi"

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
install -m 0644 "$SCRIPT_DIR/dataplaneapi.service" /etc/systemd/system/dataplaneapi.service
systemctl daemon-reload
echo ">> done. 下一步:"
echo "   1) 在 /etc/haproxy/haproxy.cfg 合并 haproxy.cfg.snippet(改掉 CHANGE_ME)"
echo "   2) systemctl enable --now dataplaneapi"
