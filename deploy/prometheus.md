# Prometheus 指标接入指引

HAProxy 自带 Prometheus 导出器(2.0+),无需额外 exporter 进程。

## 1. 节点侧:开启 prometheus-exporter

在受管节点的 haproxy.cfg 中(或直接复用现有的 stats 监听)加一个使用
prometheus-exporter 服务的 frontend:

```text
listen stats
    bind *:8404
    stats enable
    stats uri /stats
    http-request use-service prometheus-exporter if { path /metrics }
```

通过 WebUI 的 M3 配置管理也能改(原始配置 tab 或事务接口),保存后会自动 reload。
建议安全组把 8404 限制为仅 Prometheus 服务器可达。

## 2. Prometheus 侧:抓取配置

```yaml
scrape_configs:
  - job_name: haproxy
    metrics_path: /metrics
    static_configs:
      - targets: ["38.92.9.158:8404"]
```

## 3. 常用指标

| 指标前缀 | 含义 |
| --- | --- |
| `haproxy_frontend_http_requests_total` | 前端请求总量 |
| `haproxy_frontend_current_sessions` | 前端当前连接 |
| `haproxy_backend_servers` | 后端服务器状态(1=UP,按 state 标签) |
| `haproxy_server_response_time_average_seconds` | 响应时间均值 |
| `haproxy_backend_http_responses_total` | 按 code 标签的响应计数(4xx/5xx 告警常用) |

## 4. 告警规则建议

- 后端不可用:`haproxy_backend_up == 0` 或某 backend 的 UP server 数为 0
- 5xx 突增:`rate(haproxy_backend_http_responses_total{code="5xx"}[5m])` 超阈值
- 队列堆积:`haproxy_backend_current_queue > 0` 持续 5 分钟
