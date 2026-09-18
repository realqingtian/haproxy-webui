# v0.8 本地 keepalived 双节点验证环境(仅开发/验证用,勿用于生产):
# 单容器 = 一台受管节点:haproxy + keepalived(unicast VRRP)+ sshd(服务管理 SSH 链路)+ dataplaneapi。
# 与 local-e2e 分离:那边保持"单节点最小环境",这边加 keepalived 与 sshd。
#
# 用法:docker compose -f deploy/dev-keepalived/docker-compose.yml up -d --build --wait
# 验证:docker exec kvrrp-lb1 ip -o -4 addr show | grep 172.28.255.100   (MASTER 持 VIP)
#       docker exec kvrrp-lb2 keepalived --version
# failover 演示:docker exec kvrrp-lb1 keepalived --config-test ... 或直接停 lb1 的 keepalived:
#       docker exec kvrrp-lb1 pkill keepalived && sleep 3 && docker exec kvrrp-lb2 ip -o -4 addr | grep .100
# WebUI 注册(指向宿主端口):http://localhost:5557 / http://localhost:5558
#   凭据 dataplaneapi / demosecret;SSH 配置:宿主 127.0.0.1:2222/2223,用户 root / 密码 devroot
ARG ALPINE_VERSION=3.19
FROM alpine:${ALPINE_VERSION}

RUN apk add --no-cache haproxy keepalived curl openssh && \
    ssh-keygen -A && \
    echo 'root:devroot' | chpasswd && \
    sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config && \
    sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config

ARG DPAPI_VERSION=3.4.3
ARG DPAPI_ARCH=arm64
ADD https://github.com/haproxytech/dataplaneapi/releases/download/v${DPAPI_VERSION}/dataplaneapi_${DPAPI_VERSION}_linux_${DPAPI_ARCH}.tar.gz /tmp/dpapi.tar.gz
RUN tar -xzf /tmp/dpapi.tar.gz -C /tmp \
    && install -m 0755 /tmp/dataplaneapi /usr/local/bin/dataplaneapi \
    && rm -rf /tmp/dpapi.tar.gz /tmp/dataplaneapi /tmp/LICENSE /tmp/CHANGELOG.md

COPY haproxy.cfg /etc/haproxy/haproxy.cfg
COPY start.sh /start.sh

EXPOSE 5555 8404 22

CMD ["/bin/sh", "/start.sh"]
