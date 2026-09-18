#!/bin/sh
# keepalived 验证环境单节点启动脚本:sshd + haproxy + dataplaneapi + keepalived(unicast VRRP)。
# keepalived 参数由 compose 环境变量注入:
#   KV_SELF_IP / KV_PEER_IP / KV_PRIORITY / KV_STATE — VRRP 单播对端、优先级与初始角色。
#
# 坑点(与 local-e2e/start.sh 同源经验 + 本环境新增):
# 1. dataplaneapi 必须等 haproxy runtime socket 就绪后再启动,否则静默退出;
# 2. keepalived 用 unicast(容器桥接网络无组播),VIP 挂在 eth0;
#    需要 NET_ADMIN / NET_RAW capabilities(compose cap_add);
# 3. keepalived 以 -n(不 fork)后台运行,方便容器单进程组管理。

set -x

# ---- sshd(服务管理 SSH 链路:WebUI 实例 SSH 直连容器)----
/usr/sbin/sshd

# ---- haproxy ----
haproxy -W -db -p /var/run/haproxy.pid -f /etc/haproxy/haproxy.cfg &

i=0
until [ -S /var/run/haproxy.sock ] && [ -s /var/run/haproxy.pid ] || [ "$i" -ge 80 ]; do
    sleep 0.25
    i=$((i + 1))
done

cat > /usr/local/bin/doreload <<'EOF'
#!/bin/sh
kill -USR2 "$(head -1 /var/run/haproxy.pid)"
EOF
chmod +x /usr/local/bin/doreload

# ---- dataplaneapi(带证书存储,顺带验证证书链路)----
mkdir -p /var/log /etc/haproxy/ssl
(
    n=0
    while true; do
        n=$((n + 1))
        echo "$(date '+%F %T') starting dataplaneapi (attempt $n)" >> /var/log/dpapi.log
        dataplaneapi --config-file /etc/haproxy/haproxy.cfg --userlist dataplaneapi \
            --host 0.0.0.0 --port 5555 --haproxy-bin /usr/sbin/haproxy \
            --ssl-certs-dir /etc/haproxy/ssl \
            --reload-cmd "doreload" \
            --restart-cmd "doreload" < /dev/null >> /var/log/dpapi.log 2>&1
        echo "$(date '+%F %T') dataplaneapi exited code $?" >> /var/log/dpapi.log
        sleep 3
    done
) &

# ---- keepalived(unicast VRRP)----
mkdir -p /etc/keepalived
cat > /etc/keepalived/keepalived.conf <<EOF
global_defs {
    router_id kvrrp_${KV_SELF_IP}
}

vrrp_instance VI_1 {
    state ${KV_STATE}
    interface eth0
    virtual_router_id 51
    priority ${KV_PRIORITY}
    advert_int 1
    unicast_src_ip ${KV_SELF_IP}
    unicast_peer {
        ${KV_PEER_IP}
    }
    virtual_ipaddress {
        ${KV_VIP} dev eth0
    }
}
EOF

exec keepalived --dont-fork --log-detail
