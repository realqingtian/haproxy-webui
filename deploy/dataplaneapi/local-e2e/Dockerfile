# 本地端到端联调环境:单容器同时跑 HAProxy + dataplaneapi,模拟一台受管节点。
# 构建参数与 WebUI 后端部署形态无关,Mac(Apple Silicon)默认 arm64,x86 机器用 --build-arg DPAPI_ARCH=x86_64
#
# HAProxy 版本由 ALPINE_VERSION 决定:默认 3.19 → haproxy 2.8,与真实受管节点版本对齐;
# 需要 haproxy 3.x 时传 --build-arg ALPINE_VERSION=3(取 alpine latest 的 haproxy)。
#
# 启动(推荐,docker-compose.yml 带 healthcheck,集成/E2E 测试共用):
#   docker compose up -d --wait
# 验证: curl -u dataplaneapi:demosecret localhost:5555/v3/info
#   (dataplaneapi 3.x 前缀是 /v3;2.x 才是 /v2)
# WebUI 注册实例地址(docker desktop 下后端容器访问宿主端口):
#   http://host.docker.internal:5555  用户 dataplaneapi  密码 demosecret
# HAProxy stats 页: http://localhost:8404/stats
ARG ALPINE_VERSION=3.19
FROM alpine:${ALPINE_VERSION}

RUN apk add --no-cache haproxy curl

ARG DPAPI_VERSION=3.4.3
ARG DPAPI_ARCH=arm64
ADD https://github.com/haproxytech/dataplaneapi/releases/download/v${DPAPI_VERSION}/dataplaneapi_${DPAPI_VERSION}_linux_${DPAPI_ARCH}.tar.gz /tmp/dpapi.tar.gz
RUN tar -xzf /tmp/dpapi.tar.gz -C /tmp \
    && install -m 0755 /tmp/dataplaneapi /usr/local/bin/dataplaneapi \
    && rm -rf /tmp/dpapi.tar.gz /tmp/dataplaneapi /tmp/LICENSE /tmp/CHANGELOG.md

COPY haproxy.cfg /etc/haproxy/haproxy.cfg
COPY start.sh /start.sh

EXPOSE 5555 8404

CMD ["/bin/sh", "/start.sh"]
