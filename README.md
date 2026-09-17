# HAProxy WebUI

自建的 HAProxy 管理前端:Go BFF 后端 + React 前端,通过 HAProxy 官方
[Data Plane API](https://github.com/haproxytech/dataplaneapi) 管理一台或多台 HAProxy
(单机 / 集群均可,每个节点注册一个实例)。

架构与路线图见 [PLAN.md](./PLAN.md)。

## 技术栈

| 层 | 选型 |
|---|---|
| 前端 | React 19 + Vite + TypeScript + shadcn/ui + Tailwind CSS v4 + TanStack Query |
| 后端 (BFF) | Go + Gin + GORM + SQLite(纯 Go 驱动,无 CGO) |
| 认证 | JWT(Bearer)+ RBAC:admin / operator / viewer |
| 节点通道 | HTTP Basic Auth → 每台 HAProxy 上的 dataplaneapi |

## 目录结构

```
frontend/          React 前端(bun 管理)
backend/           Go BFF 后端
  cmd/server/        入口
  internal/config/     环境变量配置
  internal/model/      User / Instance / AuditLog
  internal/database/   SQLite + 迁移 + 种子管理员
  internal/auth/       JWT + RBAC 中间件
  internal/api/        路由与 handler
  internal/dataplane/  dataplaneapi 客户端
deploy/            部署产物(systemd / nginx / dataplaneapi 节点侧)
docker-compose.yml 整包编排(backend + frontend/nginx)
```

## 本地开发

```bash
# 1. 启动后端(默认 :8080,首次启动自动建库并创建 admin/admin123)
make dev-backend

# 2. 另开终端启动前端(:5173,/api 自动代理到 8080)
make dev-frontend
```

登录后请尽快在后续版本的用户管理中修改默认密码。

## 部署

### Docker Compose

```bash
cp .env.example .env       # 修改 JWT_SECRET(必填)与管理员密码
docker compose up -d --build
# 访问 http://<host>:8081
```

### systemd 裸机

1. `make build` 得到 `backend/haproxy-webui` 二进制与 `frontend/dist/`
2. 按 `deploy/systemd/haproxy-webui.service` 文件头注释安装后端服务
3. 前端静态文件由系统 nginx 托管:参考 `deploy/nginx/haproxy-webui.conf`

### HAProxy 节点侧(dataplaneapi)

每台受管 HAProxy 节点执行:

```bash
sudo ./deploy/dataplaneapi/install.sh            # 下载二进制并装 systemd 服务(已实机验证)
# 按提示把 deploy/dataplaneapi/haproxy.cfg.snippet 合并进 haproxy.cfg
sudo systemctl enable --now dataplaneapi
```

然后在 WebUI「HAProxy 实例」(M2)中注册该节点:地址填
`http://<节点IP>:5555`,凭据为 snippet 中的 userlist 用户。

#### 网络接入:不开公网 5555 的推荐做法

dataplaneapi 端口 5555 等同于负载均衡器控制权,**不要对公网开放**。两种接法:

- **SSH 隧道(当前采用,适合单机自用)**:服务器信息填入 gitignored 的
  `deploy/server.local.env`(参考同目录 server.local.env 结构),然后 `make tunnel`,
  WebUI 中实例地址填 `http://host.docker.internal:5555`;
- **安全组白名单**:多节点/长期使用,在云安全组把 5555 限制为仅 WebUI 所在出口 IP 可达。

## 环境变量(后端)

| 变量 | 默认 | 说明 |
|---|---|---|
| `HAPROXY_WEBUI_PORT` | `8080` | 监听端口 |
| `HAPROXY_WEBUI_DB` | `./data/haproxy-webui.db` | SQLite 路径 |
| `HAPROXY_WEBUI_JWT_SECRET` | dev 默认值(**生产必改**) | JWT 签名密钥 |
| `HAPROXY_WEBUI_ADMIN_USER` | `admin` | 首次启动种子管理员用户名 |
| `HAPROXY_WEBUI_ADMIN_PASSWORD` | `admin123` | 首次启动种子管理员密码 |

开发进度与任务清单见 [PLAN.md](./PLAN.md)。
