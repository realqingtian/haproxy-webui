#!/bin/sh
# 容器内启动脚本:后台起 haproxy(master-worker,pid 写入 pidfile),再以守护循环跑 dataplaneapi。
#
# 坑点记录(实测定论,dataplaneapi v3.4.3):
# 1. dataplaneapi 启动时会试跑一次 reload-cmd;
# 2. 不能用 pidof haproxy 定位进程——会同时命中 master 和 worker,USR2 打死 worker 导致 dataplaneapi 启动失败(exit 1 且无日志);
#    必须用 -p pidfile 只对 master 发信号,reload/restart 统一走 doreload 辅助脚本;
# 3. 必须等 haproxy 的 runtime socket(/var/run/haproxy.sock)就绪后再启动 dataplaneapi:
#    只等 pidfile 不够,socket 未就绪时 dataplaneapi 会静默退出(exit 1 无日志);
# 4. 重试循环记录退出码到 /var/log/dpapi.log,便于排查。
echo "container haproxy version: $(haproxy -v | head -1)" # 便于确认与真实节点的版本对齐情况
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

mkdir -p /var/log
(
    n=0
    while true; do
        n=$((n + 1))
        echo "$(date '+%F %T') starting dataplaneapi (attempt $n)" >> /var/log/dpapi.log
        dataplaneapi --config-file /etc/haproxy/haproxy.cfg --userlist dataplaneapi \
            --host 0.0.0.0 --port 5555 --haproxy-bin /usr/sbin/haproxy \
            --reload-cmd "doreload" \
            --restart-cmd "doreload" < /dev/null >> /var/log/dpapi.log 2>&1
        echo "$(date '+%F %T') dataplaneapi exited code $?" >> /var/log/dpapi.log
        sleep 3
    done
) &

wait
