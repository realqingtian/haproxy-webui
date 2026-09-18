# HAProxy WebUI

一套自托管的 HAProxy 可视化管理平台。不用再 SSH 到服务器手改 `haproxy.cfg`,在浏览器里即可完成日常运维:**查看与修改负载均衡配置、上下线后端服务器、调整权重、监控流量、回滚误操作**——每一步都有权限控制和审计记录。

底层基于 HAProxy 官方 [Data Plane API](https://github.com/haproxytech/dataplaneapi)(dataplaneapi),配置的校验与优雅 reload 由官方实现,不自研配置解析器。支持管理一台或多台 HAProxy(单机、集群统一为"多实例"模型)。

## 功能总览

| 模块 | 你可以做什么 |
| --- | --- |
| 实例管理 | 注册多台 HAProxy 节点,一键连通性测试 |
| 配置管理 | 可视化管理 frontend / backend / server / bind / ACL;编辑先进入待提交清单,可预览统一 diff,攒够后一次事务批量提交(只触发一次 reload),失败整体回滚;每次提交自动记录版本快照;支持按名称搜索过滤与配置原文高亮跳转 |
| 运行时控制 | 服务器上线 / 维护 / 排空、调整权重——即时生效,不中断现有连接 |
| 模板创建 | HTTP / TCP 负载均衡常用场景一键生成(frontend + backend + 服务器组) |
| SSL 证书管理 | 浏览节点证书目录:列表 / 上传 / 删除,自动解析主体与有效期并高亮临期、过期证书;删除前检查配置引用(需节点启用证书存储,见节点接入) |
| 服务管理 | 实例可选配置 SSH,实时查看 dataplaneapi 服务状态(active / PID / 启动时间)并远程重启(需 sudo 免密白名单;重启有确认对话框并记入审计);host key 指纹首次连接自动记录、此后钉扎校验,防节点伪装 |
| 版本历史 | 配置快照列表(标记手动 / 同步 / 定时巡检来源)、查看历史原文、一键回滚(带并发保护);支持"从服务器同步"消除漂移 |
| 定时巡检 | 后台按可配周期抓取节点配置,与最近快照比对,发现绕过 WebUI 的手工修改即落「漂移」快照并显著标出 |
| 监控 | 单实例 QPS / 连接数 / 流量 / 状态码实时大盘(10 秒刷新);监控总览页聚合全部实例健康与速率,按集群分组,点击下钻;实例监控页可探测节点 Prometheus /metrics 是否可访问并给接入指引 |
| 告警通知 | reload 失败(含证书删除触发)、节点连通性探测失败、backend 全部 DOWN、keepalived 主备切换 / 节点故障时推送飞书 / 钉钉 / 企业微信机器人 webhook(可配多渠道、可发测试消息;边沿触发 + 恢复通知 + 冷却防刷屏) |
| 集群视角 | keepalived 主备集群的 VRRP 真实状态:监控总览页按集群展示 VIP 与各节点主 / 备 / 故障角色(复用实例级 SSH 配置,节点跑 keepalived 即可,无新增部署依赖;本地可用 deploy/dev-keepalived/ 双容器环境验证) |
| 用户与权限 | 三种角色:管理员 / 操作员 / 只读;实例凭据 AES 加密存储;管理员可强制下线任意账号 |
| 会话安全 | JWT 滑动续期;改密 / 重置密码 / 删除用户 / 强制下线后全部旧会话立即失效;改角色即时生效 |
| 审计日志 | 谁在什么时候做了什么,全部可追溯;支持时间范围过滤与 CSV 导出 |
| 体验细节 | 深色 / 浅色 / 跟随系统主题切换;大列表分页(审计日志、服务器表);前端按路由分包按需加载 |

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
   - 新增一套负载均衡:「从模板创建」选 HTTP 或 TCP,填名称、监听端口、服务器列表;
   - 加减服务器、调整健康检查:在对应 backend 卡片里操作。
   - 以上编辑都不会立即生效:先进入「待提交清单」跨对话框累积,可逐条移除、diff 预览,
     确认后一次提交——单个事务应用、只触发一次 reload,任一步失败整体回滚。
4. **改错了?**:「版本历史」里每次提交都有快照,选一个时间点一键回滚。
5. **团队协作**:管理员在「用户与权限」创建账号并分配角色;所有操作自动进入「审计日志」。
6. **放着别管也不掉链**:管理员在「告警与巡检」里配置周期与群机器人 webhook——
   节点配置被人手工改动(漂移)会定期抓快照留痕,reload 失败 / 节点失联 / backend 全 DOWN 会推送到群里。

### 角色权限

| 角色 | 权限 |
| --- | --- |
| 管理员 admin | 用户管理 + 全部操作 |
| 操作员 operator | 实例、配置、运行时的写操作 |
| 只读 viewer | 仅查看,所有写操作不可见 |

## 目录结构

```text
haproxy-webui/
├── frontend/                      # 前端(React 19 + Vite + TS + shadcn/ui + Tailwind v4,bun 管理)
│   ├── src/
│   │   ├── components/
│   │   │   ├── config/            # 配置管理组件:编辑对话框 / 版本历史 / 证书管理 / 原文高亮搜索
│   │   │   ├── layout/            # 应用布局(侧边导航 / 顶栏 / 修改密码)
│   │   │   └── ui/                # shadcn/ui 基础组件
│   │   ├── lib/                   # API 客户端(JWT 注入)、格式化工具
│   │   ├── pages/                 # 页面:仪表盘 / 实例 / 配置 / 监控总览 / 单实例监控 / 审计 / 用户 / 告警与巡检 / 登录
│   │   ├── App.tsx                # 路由
│   │   └── main.tsx               # 入口
│   ├── Dockerfile                 # 前端镜像:bun 构建 + nginx 托管
│   ├── nginx.conf                 # 容器内 nginx:静态托管 + /api 反代后端
│   ├── playwright.config.ts       # E2E 编排:local-e2e 容器 + 独立 DB 后端 + vite dev
│   └── e2e/                       # Playwright 冒烟用例(核心链路)
├── backend/                       # 后端 BFF(Go + Gin + GORM + SQLite)
│   ├── cmd/server/                # 程序入口(优雅停机)
│   ├── internal/
│   │   ├── api/                   # 路由与各模块 handler(auth / 实例 / 配置 / 用户 / 审计 / 告警 / 设置)
│   │   ├── auth/                  # JWT 签发校验(含会话吊销)+ RBAC 角色中间件
│   │   ├── config/                # 环境变量配置
│   │   ├── cryptoutil/            # 实例凭据 AES-256-GCM 加解密(独立密钥 + 历史密文迁移)
│   │   ├── database/              # SQLite 连接 / 自动迁移 / 种子管理员
│   │   ├── dataplane/             # dataplaneapi 客户端(配置 / 事务 / 运行时 / stats)
│   │   ├── notify/                # 告警推送(飞书 / 钉钉 / 企业微信 webhook)
│   │   ├── scheduler/             # 后台定时任务(快照巡检 / 健康探测 / 告警触发)
│   │   ├── settings/              # 系统级键值配置(巡检周期等,运行时可改)
│   │   ├── systemd/               # SSH 远程 systemd 管理(dataplaneapi 服务状态查询 / 重启)
│   │   └── model/                 # 数据模型(用户 / 实例 / 审计 / 配置快照 / 设置 / 告警渠道)
│   └── Dockerfile                 # 后端镜像:多阶段构建,纯静态二进制
├── deploy/
│   ├── dataplaneapi/              # HAProxy 节点侧:一键安装脚本 / systemd / 配置片段
│   │   ├── local-e2e/             # 本地联调与测试环境(单容器 HAProxy + dataplaneapi,docker compose 化,集成/E2E 共用)
│   │   └── dev-keepalived/        # v0.8 本地 keepalived 双节点验证环境(unicast VRRP + sshd)
│   ├── nginx/                     # 裸机部署的 nginx 站点配置
│   ├── systemd/                   # 后端 systemd 服务单元
│   ├── prometheus.md              # Prometheus 指标接入指引
│   └── tunnel.sh                  # SSH 隧道备用方案(不开 5555 端口时,make tunnel)
├── docker-compose.yml             # 整包编排(后端 + 前端 nginx)
├── Makefile                       # dev / build / docker-up / tunnel 快捷命令
├── AGENTS.md                      # AI 协作与 Git 提交规范
└── PLAN.md                        # 项目全景唯一文档:状态 / 里程碑 / 决策 / 进度 / 规划
```

## 本地开发

```bash
# 终端 1:后端(:8080,首次启动自动建库,种子账号 admin/admin123)
cd backend && go run ./cmd/server

# 终端 2:前端(:5173,/api 自动代理到 8080)
cd frontend && bun install && bun run dev
```

构建检查:`backend` 下 `go build ./... && go vet ./...`;`frontend` 下 `bun run build`。

## 测试

```bash
make test               # 后端单测 + 进程内集成(无 Docker 依赖,always green)
make test-integration   # 容器级集成:起 local-e2e(真实 HAProxy + dataplaneapi v3)跑真实链路
make e2e                # Playwright E2E 冒烟:自动编排容器 + 独立 DB 后端 + vite dev,覆盖核心链路
```

E2E 首次运行需安装浏览器:`cd frontend && bunx playwright install chromium`;需要 Docker 运行,8080/5173 端口空闲。

### keepalived 验证环境(可选)

没有真实 keepalived 主备时,可用本地 Docker 双容器环境验证「集群视角」与证书 / 服务管理链路:

```bash
docker compose -f deploy/dev-keepalived/docker-compose.yml up -d --build --wait
# lb1(主)dataplaneapi http://localhost:5557,SSH 127.0.0.1:2222;lb2(备)5558 / 2223
# SSH 账号 root / devroot(仅开发环境);VIP 172.28.255.100 在主节点上
```

WebUI 侧:建集群(填 VIP 172.28.255.100)→ 注册两实例并填入上述 SSH 配置 →
「监控总览」集群卡片即可看到主 / 备角色;`docker exec kvrrp-lb1 pkill keepalived`
可演示 failover(备机接管,页面状态随之翻转)。

## 生产部署

### 方式一:Docker Compose(推荐)

见上文快速开始。数据(SQLite)存放在命名卷 `backend-data` 中,升级镜像不丢数据。

### 转生产检查清单

上线对外使用前,除 compose 已强制的两把密钥外,逐项自查:

- [ ] **修改 admin 密码**:右上角头像 → 修改密码(默认 admin123 仅限首次启动);
- [ ] **收紧 dataplaneapi 端口(5555)**:云安全组把来源 IP 限制为 WebUI 服务器地址,
      或改走 SSH 隧道 / TLS 前置——该端口等同负载均衡器完全控制权且为明文 HTTP;
- [ ] **启用 HTTPS**:前端 nginx 配置证书,登录凭据与配置内容不应明文过公网;
- [ ] **服务管理 SSH 最小授权**:专用账号 + sudoers 仅放行所需 `systemctl restart <unit>`;
- [ ] **数据备份**:定期备份 `backend-data` 卷(SQLite,含加密凭据与配置快照);
- [ ] **验证告警链路**:「告警与巡检」中配置群机器人并点「测试」。

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

可选能力需要在节点侧额外准备:

- **SSL 证书管理**(配置页「证书」页签):dataplaneapi 需以 `--ssl-certs-dir`
  指定证书目录——本仓库 `deploy/dataplaneapi/` 的 service 单元已默认启用
  (自动创建 `/etc/haproxy/ssl`);存量节点手工增加该参数并重启 dataplaneapi,
  未启用时该页签会展示修复指引。
- **服务管理**(实例监控页「服务管理」卡片):在 WebUI 实例设置中填写 SSH
  连接(地址 / 端口 / 用户 / 密码或私钥,均加密存储)。host key 指纹在首次连接时
  自动记录(TOFU),之后每次连接钉扎校验——节点重装 / 更换主机 key 后,在实例设置中
  「重置」指纹再重连即可。重启功能需为 SSH 用户配置 sudo 免密白名单(状态查询不需要):

  ```text
  sshuser ALL=(root) NOPASSWD: /usr/bin/systemctl restart dataplaneapi
  ```

> **安全要求**:dataplaneapi 端口(5555)等同于负载均衡器的完全控制权。
>
> - 最简单:在云安全组中把 5555 限制为仅 WebUI 服务器可达,**不要对公网开放**;
> - 不开端口:使用 SSH 隧道,服务器信息填入 gitignored 的 `deploy/server.local.env` 后执行 `make tunnel`,实例地址填 `http://host.docker.internal:5555`。

### 监控接入

HAProxy 自带 Prometheus 导出器,按 [deploy/prometheus.md](deploy/prometheus.md) 两步接入,附常用指标与告警规则建议。

## 环境变量(后端)

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `HAPROXY_WEBUI_PORT` | `8080` | 监听端口 |
| `HAPROXY_WEBUI_DB` | `./data/haproxy-webui.db` | SQLite 路径 |
| `HAPROXY_WEBUI_JWT_SECRET` | dev 默认值(**生产必改**) | JWT 签名密钥,`openssl rand -hex 32` |
| `HAPROXY_WEBUI_ENCRYPTION_KEY` | 从 JWT secret 派生(仅限本地开发) | 实例凭据加密密钥,**生产必填且须与 JWT secret 不同**(compose 已强制);换 JWT secret 不影响已配置独立密钥的部署;首次配置后启动会自动迁移历史密文 |
| `HAPROXY_WEBUI_STRICT` | `false` | 严格生产模式:`true` 时未自定义 JWT secret / 加密密钥则拒绝启动 |
| `HAPROXY_WEBUI_ADMIN_USER` | `admin` | 首次启动种子管理员 |
| `HAPROXY_WEBUI_ADMIN_PASSWORD` | `admin123` | 首次启动种子管理员密码 |

## 常见问题

**实例测试连接报 401?** dataplaneapi 的 userlist 凭据与 WebUI 中填写的不一致,检查节点 `haproxy.cfg` 中 `userlist dataplaneapi` 的配置。

**运行时改了服务器状态,reload 后又回去了?** 这是设计行为:上下线 / 排空 / 权重是运行时操作,即时生效但不写配置文件;需要永久生效请在配置管理中修改服务器定义。

**配置被人在服务器上手工改过,界面显示不准?** 配置管理 → 版本历史 → 「从服务器同步」,以当前实际配置重建基线。开启「告警与巡检」的快照巡检后,这类漂移会被定时任务自动发现并留下漂移快照。

**怎么知道节点 reload 失败了?** 在「告警与巡检」添加飞书 / 钉钉 / 企业微信机器人渠道并点「测试」;此后配置提交触发的 reload 一旦失败(后台轮询确认),会推送告警并写入审计日志(`reload.failed`)。

**改密 / 强制下线后旧会话还能用吗?** 不能。改密、管理员重置密码、删除用户、强制下线都会立即吊销该用户的全部 token;管理员改角色也即时生效,无需重新登录。

**HAProxy 版本有要求吗?** 需要 ≥ 1.9 且节点上运行 dataplaneapi(建议 HAProxy 2.6+、dataplaneapi 3.x,本平台按 v3 接口开发;本地测试环境默认以 HAProxy 2.8 运行,与常见生产版本对齐)。

## 相关文档

- [PLAN.md](PLAN.md) — 项目全景:当前状态 / 里程碑 / 技术决策 / 各期明细 / 后续规划 / 已知风险
- [AGENTS.md](AGENTS.md) — AI 协作与 Git 提交规范
- [deploy/prometheus.md](deploy/prometheus.md) — 监控接入
- [deploy/dataplaneapi/](deploy/dataplaneapi/) — 节点侧部署产物与本地联调环境
