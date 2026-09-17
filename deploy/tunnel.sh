#!/usr/bin/env bash
# 建立本机到受管节点的 SSH 隧道:本机 5555 → 服务器 127.0.0.1:5555
# 用途:安全组不开放 5555 时,WebUI(本机 Docker)经此隧道访问 dataplaneapi。
# 服务器信息从 deploy/server.local.env 读取(已 gitignore)。
set -euo pipefail
cd "$(dirname "$0")/.."

if [ ! -f deploy/server.local.env ]; then
    echo "缺少 deploy/server.local.env,请按该文件格式填写服务器信息(勿提交 Git)"
    exit 1
fi
set -a
# shellcheck disable=SC1091
source deploy/server.local.env
set +a

echo ">> tunnel: localhost:5555 -> ${SERVER_USER}@${SERVER_HOST}:5555 (Ctrl-C 断开)"
exec ssh -N \
    -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o ExitOnForwardFailure=yes \
    -i "$SERVER_SSH_KEY" -p "$SERVER_SSH_PORT" \
    -L 5555:127.0.0.1:5555 "$SERVER_USER@$SERVER_HOST"
