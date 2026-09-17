# HAProxy WebUI

一套自托管的 HAProxy 可视化管理平台。不用再 SSH 到服务器手改 `haproxy.cfg`,在浏览器里即可完成日常运维:**查看与修改负载均衡配置、上下线后端服务器、调整权重、监控流量、回滚误操作**——每一步都有权限控制和审计记录。

底层基于 HAProxy 官方 [Data Plane API](https://github.com/haproxytech/dataplaneapi)(dataplaneapi),配置的校验与优雅 reload 由官方实现,不自研配置解析器。支持管理一台或多台 HAProxy(单机、集群统一为"多实例"模型)。

## 功能总览

| 模块 | 你可以做什么 |
|---|---|
| 实例管理 | 注册多台 HAProxy 节点,一键连通性测试 |
| 配置管理 | 可视化管理 frontend / backend / server / bind / ACL;每次保存自动校验、优雅 reload、记录版本快照 |
| 运行时控制 | 服务器上线 / 维护 / 排空、调整权重——即时生效,不中断现有连接 |
| 模板创建 | HTTP / TCP 负载均衡常用场景一键生成(frontend + backend + 服务器组) |
| 版本历史 | 配置快照列表、查看历史原文、一键回滚(带并发保护);支持"从服务器同步"消除手工修改造成的漂移 |
| 监控 | QPS / 连接数 / 流量 / 状态码实时大盘,10 秒自动刷新 |
| 用户与权限 | 三种角色:管理员 / 操作员 / 只读;实例凭据 AES 加密存储 |
| 审计日志 | 谁在什么时候做了什么,全部可追溯 |

## 快速开始(Docker Compose)

前置:一台装好 Docker 的机器,以及至少一台已安装 [dataplaneapi](#haproxy-节点接入) 的 HAProxy 节点。

```bash
git clone git@github.com:realqingtian/haproxy-webui.git
cd haproxy-webui

# 1. 准备环境变量(必须设置 JWT 密钥)
cp .env.example .env
vim .env   # 修改 HAPROXY_WEBUI_JWT_SECRET,建议 openssl rand -hex 32

# 2. 启动
docker compose up -d --build

# 3. 访问 http://<主机IP>:8081,默认账号 admin / admin123
```

> **首次登录后请立即修改密码**(右上角头像 → 修改密码),并在「用户与权限」里为其他同事创建独立账号。

## 上手指南

第一次使用的推荐路径:

1. **接入节点**:进入「实例管理」→ 添加实例,填入节点上 dataplaneapi 的地址(通常 `http://<节点IP>:5555`)和凭据,点「测试」确认连通。
2. **查看现状**:点实例行的「配置」,可以浏览该节点的 frontend / backend / server 全量清单和配置原文;「监控」页查看实时流量。
3. **日常运维**:
   - 临时摘除一台后端:配置页中把该服务器的管理状态切为「维护」或「排空」——即时生效,重启后失效;
   - 新增一套负载均衡:「从模板创建」选 HTTP 或 TCP,填名称、监听端口、服务器列表,一次提交;
   - 加减服务器、调整健康检查:在对应 backend 卡片里操作。
4. **改错了?**:「版本历史」里每次保存都有快照,选一个时间点一键回滚。
5. **团队协作**:管理员在「用户与权限」创建账号并分配角色;所有操作自动进入「审计日志」。

**角色权限**

| 角色 | 权限 |
|---|---|
| 管理员 admin | 用户管理 + 全部操作 |
| 操作员 operator | 实例、配置、运行时的写操作 |
| 只读 viewer | 仅查看,所有写操作不可见 |

## 目录结构

```
haproxy-webui/
├── frontend/                      # 前端(React 19 + Vite + TS + shadcn/ui + Tailwind v4,bun 管理)
│   ├── src/
│   │   ├── components/
│   │   │   ├── config/            # 配置管理组件:编辑对话框 / 版本历史 / 配置模板
│   │   │   ├── layout/            # 应用布局(侧边导航 / 顶栏 / 修改密码)
│   │   │   └── ui/                # shadcn/ui 基础组件
│   │   ├── lib/                   # API 客户端(JWT 注入)、格式化工具
│   │   ├── pages/                 # 页面:仪表盘 / 实例 / 配置 / 监控 / 审计 / 用户 / 登录
│   │   ├── App.tsx                # 路由
│   │   └── main.tsx               # 入口
│   ├── Dockerfile                 # 前端镜像:bun 构建 + nginx 托管
│   └── nginx.conf                 # 容器内 nginx:静态托管 + /api 反代后端
├── backend/                       # 后端 BFF(Go + Gin + GORM + SQLite)
│   ├── cmd/server/                # 程序入口
│   ├── internal/
│   │   ├── api/                   # 路由与各模块 handler(auth / 实例 / 配置 / 用户 / 审计)
│   │   ├── auth/                  # JWT 签发校验 + RBAC 角色中间件
│   │   ├── config/                # 环境变量配置
│   │   ├── cryptoutil/            # 实例凭据 AES-256-GCM 加解密
│   │   ├── database/              # SQLite 连接 / 自动迁移 / 种子管理员
│   │   ├── dataplane/             # dataplaneapi 客户端(配置 / 事务 / 运行时 / stats)
│   │   └── model/                 # 数据模型(用户 / 实例 / 审计 / 配置快照)
│   └── Dockerfile                 # 后端镜像:多阶段构建,纯静态二进制
├── deploy/
│   ├── dataplaneapi/              # HAProxy 节点侧:一键安装脚本 / systemd / 配置片段
│   │   └── local-e2e/             # 本地联调环境(单容器 HAProxy + dataplaneapi)
│   ├── nginx/                     # 裸机部署的 nginx 站点配置
│   ├── systemd/                   # 后端 systemd 服务单元
│   ├── prometheus.md              # Prometheus 指标接入指引
│   └── tunnel.sh                  # SSH 隧道备用方案(不开 5555 端口时,make tunnel)
├── docker-compose.yml             # 整包编排(后端 + 前端 nginx)
├── Makefile                       # dev / build / docker-up / tunnel 快捷命令
├── AGENTS.md                      # AI 协作规范(提交规范等)
└── PLAN.md                        # 开发进度与决策记录
```

## 本地开发

```bash
# 终端 1:后端(:8080,首次启动自动建库,种子账号 admin/admin123)
cd backend && go run ./cmd/server

# 终端 2:前端(:5173,/api 自动代理到 8080)
cd frontend && bun install && bun run dev
```

构建检查:`backend` 下 `go build ./... && go vet ./...`;`frontend` 下 `bun run build`。

## 生产部署

### 方式一:Docker Compose(推荐)

见上文快速开始。数据(SQLite)存放在命名卷 `backend-data` 中,升级镜像不丢数据。

### 方式二:systemd 裸机

1. `make build` 产出 `backend/haproxy-webui` 二进制与 `frontend/dist/` 静态文件
2. 后端服务:按 `deploy/systemd/haproxy-webui.service` 头部注释安装
3. 前端托管:参考 `deploy/nginx/haproxy-webui.conf` 配置站点(静态文件 + `/api` 反代 8080)

### HAProxy 节点接入

每台要被管理的 HAProxy 节点:

```bash
sudo ./deploy/dataplaneapi/install.sh   # 下载二进制 + 安装 systemd 服务
# 将 deploy/dataplaneapi/haproxy.cfg.snippet 合并进 /etc/haproxy/haproxy.cfg
sudo systemctl enable --now dataplaneapi
```

> **安全要求**:dataplaneapi 端口(5555)等同于负载均衡器的完全控制权。
> - 最简单:在云安全组中把 5555 限制为仅 WebUI 服务器可达,**不要对公网开放**;
> - 不开端口:使用 SSH 隧道,服务器信息填入 gitignored 的 `deploy/server.local.env` 后执行 `make tunnel`,实例地址填 `http://host.docker.internal:5555`。

### 监控接入

HAProxy 自带 Prometheus 导出器,按 [deploy/prometheus.md](deploy/prometheus.md) 两步接入,附常用指标与告警规则建议。

## 环境变量(后端)

| 变量 | 默认 | 说明 |
|---|---|---|
| `HAPROXY_WEBUI_PORT` | `8080` | 监听端口 |
| `HAPROXY_WEBUI_DB` | `./data/haproxy-webui.db` | SQLite 路径 |
| `HAPROXY_WEBUI_JWT_SECRET` | dev 默认值(**生产必改**) | JWT 签名密钥,`openssl rand -hex 32` |
| `HAPROXY_WEBUI_ENCRYPTION_KEY` | 从 JWT secret 派生 | 实例凭据加密密钥,建议独立设置 |
| `HAPROXY_WEBUI_STRICT` | `false` | 严格生产模式:`true` 时未自定义 JWT secret / 加密密钥则拒绝启动 |
| `HAPROXY_WEBUI_ADMIN_USER` | `admin` | 首次启动种子管理员 |
| `HAPROXY_WEBUI_ADMIN_PASSWORD` | `admin123` | 首次启动种子管理员密码 |

## 常见问题

**实例测试连接报 401?** dataplaneapi 的 userlist 凭据与 WebUI 中填写的不一致,检查节点 `haproxy.cfg` 中 `userlist dataplaneapi` 的配置。

**运行时改了服务器状态,reload 后又回去了?** 这是设计行为:上下线 / 排空 / 权重是运行时操作,即时生效但不写配置文件;需要永久生效请在配置管理中修改服务器定义。

**配置被人在服务器上手工改过,界面显示不准?** 配置管理 → 版本历史 → 「从服务器同步」,以当前实际配置重建基线。

**HAProxy 版本有要求吗?** 需要 ≥ 1.9 且节点上运行 dataplaneapi(建议 HAProxy 2.6+、dataplaneapi 3.x,本平台按 v3 接口开发)。

## 相关文档

- [PLAN.md](PLAN.md) — 开发进度、里程碑与决策记录
- [docs/ROADMAP.md](docs/ROADMAP.md) — 后续迭代路线图
- [AGENTS.md](AGENTS.md) — AI 协作与提交规范
- [deploy/prometheus.md](deploy/prometheus.md) — 监控接入
- [deploy/dataplaneapi/](deploy/dataplaneapi/) — 节点侧部署产物与本地联调环境
