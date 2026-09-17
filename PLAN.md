# HAProxy WebUI 开发计划与进度

> **维护约定**:任务完成后把对应项 `[ ]` 改为 `[x]`,未完成的不动;新任务追加到对应里程碑下。
> 每个里程碑的顺序即建议实施顺序,允许跨项并行。

## 技术决策记录

| 决策点 | 结论 | 说明 |
|---|---|---|
| 管理通道 | HAProxy 官方 Data Plane API(dataplaneapi sidecar) | 不自研 haproxy.cfg 解析器,与 HAProxy Enterprise GUI 同底座;要求 HAProxy ≥ 1.9,建议 2.6+ |
| BFF 语言 | Go + Gin | 单二进制部署,CGO 关闭 |
| 存储 | SQLite(GORM + 纯 Go 驱动) | 仅元数据:实例 / 用户 / 审计 |
| 前端 | React 19 + Vite + TS + shadcn/ui(radix)+ Tailwind v4 + TanStack Query | 包管理用 bun |
| 认证 | JWT(Bearer,24h)+ bcrypt | 后续接 OIDC(M4 之后) |
| RBAC | admin / operator / viewer | viewer 只读;写操作(含实例管理)需 operator+;用户管理需 admin |
| 集群支持 | 单机与集群统一为"多实例"模型 | 一台 HAProxy = 一个实例;keepalived 主备展示归入 M4 |
| 部署 | docker compose + systemd 裸机,k8s 后续扩展 | 前端由 nginx 托管并反代 /api |

---

## M1 骨架与部署(已全部完成)

- [x] 前端脚手架(bun create vite,React 19 + Vite 8 + TS 6)
- [x] 后端骨架:cmd/server + internal/{config,model,database,auth,api,dataplane}
- [x] 数据模型与迁移:users / instances / audit_log;首次启动种子管理员
- [x] JWT 登录 + auth 中间件 + RequireRole RBAC(实例写操作限 operator+,审计查询限 admin)
- [x] 实例 CRUD 接口(/api/instances)+ dataplaneapi 连通性探测(/api/instances/:id/test)
- [x] 审计日志:登录成败、实例变更自动记录
- [x] 前端接入 Tailwind CSS v4 + shadcn/ui、路径别名 `@/`、/api dev proxy
- [x] 前端页面:登录页、应用布局(侧边导航 + 用户菜单)、仪表盘占位
- [x] 后端验证:go build / go vet 通过;health 与 login 冒烟测试通过
- [x] 前端验证:tsc -b && vite build 通过
- [x] 部署产物编写:docker-compose.yml、backend/frontend Dockerfile、nginx 配置、systemd unit、dataplaneapi 节点侧 install.sh + haproxy.cfg 片段
- [x] docker compose 整包构建与启动验证(2026-09-17 API 级:双镜像构建通过,nginx→后端代理、登录、实例创建 401/201、连通性测试 502 均符合预期;浏览器端手工走查待做)
- [x] dataplaneapi 本地联调环境(deploy/dataplaneapi/local-e2e:单容器 HAProxy 3.4 + dataplaneapi v3.4.3,含启动坑位处理)
- [x] dataplaneapi 节点侧实机安装验证(2026-09-17 雨云 38.92.9.158,Ubuntu 24.04 x86_64:install.sh 全流程跑通,HAProxy 2.8.16 + dataplaneapi v3.4.3 systemd 服务 active,server 本机 /v3/info 通、错误凭据 401;安全组未开 5555,本机经 SSH 隧道 `make tunnel` 访问)
- [x] 端到端联调:注册实例 → 连通性测试打到真实 dataplaneapi(2026-09-17 实例经 API 注册(实例管理页属 M2),连通性测试返回 ok + v3.4.3;整链 nginx→BFF→SQLite 凭据→Basic Auth→dataplaneapi)
- [x] 浏览器端手工走查(2026-09-17:未登录重定向、错误密码提示、登录进仪表盘、布局截图核对、登出、登出后路由保护;走查中发现并修复登录 401 被误判为会话过期的 bug)

## M2 只读展示 + 运行时控制(2026-09-17 已全部完成,真实节点 rainyun-rcs 验证)

- [x] 实例管理页:列表 / 新增 / 编辑 / 删除 / 连通性测试(浏览器走查通过:测试连接、删除对话框均验证)
- [x] 配置只读展示:frontends(含 binds)/ backends / servers 清单 + haproxy.cfg 原文 tab(15s 自动刷新)
- [x] 运行时操作:server 状态切换(ready 上线 / maint 维护 / drain 排空)+ weight 调整(0-256)
- [x] 运行时操作入口:后端代理 dataplaneapi runtime 接口,UI 明示「运行时操作,重启后失效」语义,全部写入审计日志
- [x] stats 仪表盘:汇总卡(连接/速率/会话/流量)+ 前端/后端/服务器三张表,10s 自动刷新(native stats 接口)
- [x] 权限联动:前端 canWrite 隐藏/禁用写控件 + 后端 RequireRole 兜底(view1 viewer 账号已实测:写操作 403、用户管理 403、读与审计正常)

## M3 配置管理(2026-09-17 已全部完成,真实节点 rainyun-rcs 验证)

- [x] transaction 封装:开事务(带版本乐观锁)→ 修改 → 提交触发校验与优雅 reload;失败自动放弃事务
- [x] 可视化编辑:backend 新建/删除、server 增/改/删、frontend 新建/删除/默认后端修改、bind 增/删、ACL 添加/删除(整组)
- [x] 保存流程 UI:对话框内字段级变更(新建/编辑表单即变更内容)→ 提交后 toast 展示 reload-id → reload 状态查询接口
- [x] 并发保护:事务基于版本号开启,冲突时报错提示刷新;回滚同样带版本校验
- [x] 配置版本历史列表与一键回滚:每次提交/回滚/同步自动存快照(SQLite),UI 列表 + 查看原文 + 回滚(最新快照不提供回滚)
- [x] 配置漂移处理:「从服务器同步」按钮重建基线快照

## M4 平台化(2026-09-17 完成核心项,其余注明原因)

- [x] 用户管理页:用户 CRUD、角色分配(admin)、重置密码;保护规则:不能删自己/降级或删除最后一个 admin;改密接口 + 用户菜单「修改密码」
- [x] 审计日志查询页:操作类型下拉 + 用户名过滤,30s 自动刷新(所有登录用户可见)
- [x] 实例凭据加密存储:AES-256-GCM,密钥 HAPROXY_WEBUI_ENCRYPTION_KEY(缺省从 JWT secret 派生,有警告);兼容历史明文,实例编辑时自动迁移;DB 落库验证 enc: 密文
- [x] 配置模板:HTTP / TCP 负载均衡一键生成(frontend+bind+backend+服务器组,单事务提交);实测 mode tcp 经 frontend POST 可写入
- [x] Prometheus 指标接入:部署指引 deploy/prometheus.md(节点 prometheus-exporter 配置、抓取配置、常用指标与告警规则建议)
- [ ] keepalived 主备集群视角展示(需节点实际部署 keepalived 环境,暂缓)
- [ ] OIDC / SSO 登录(需外部 IdP 接入条件,暂缓;JWT 体系已预留)
- [ ] k8s 部署清单(当前部署形态 compose + systemd 已满足,待有 k8s 环境后补充)

---

## 已知风险与注意事项

1. **运行时 vs 持久化**:运行时上下线重启即失效,M2 的 UI 必须明示该语义,持久化改动一律走配置 + reload。
2. **配置漂移**:绕过 UI 的手工修改会导致状态不一致,依赖 M3 的「重新同步」+ diff。
3. **dataplaneapi 安全**:5555 端口 = 负载均衡器控制权,只允许 WebUI 后端所在网络访问,严禁公网。
4. **JWT 密钥**:默认 dev 密钥仅限本地,生产必须通过环境变量覆盖(compose 中已强制校验)。
5. **dataplaneapi 版本差异**:3.x 的 API 前缀是 `/v3`(2.x 为 `/v2`),探活为 `/v3/info`;release 资产命名中 64 位 x86 是 `x86_64`(amd64 只有包管理器格式)。BFF 客户端已对齐 v3。
6. **前端 401 语义**:登录接口的 401(密码错误)与其它接口的 401(会话过期)必须区分,api.ts 已通过 `authRedirect` 选项处理,新增登录类接口时注意沿用。
7. **dataplaneapi 以 root 运行**:它需要重写 haproxy.cfg、执行 systemctl reload、读 root 属主的 runtime socket;M4 考虑普通用户 + polkit/sudo 白名单加固。

## 历史决策备忘

- 2026-09-17:方案选型定为「dataplaneapi + 自建前端」(B 路线),放弃 Roxy-WI 直接复用与纯 SSH 自研路线。
- 2026-09-17:BFF 选 Go;UI 选 shadcn/ui;确认多用户 RBAC;部署先 compose + systemd。
- 2026-09-17:M1 联调发现三处问题并修复——dataplaneapi 3.x 前缀 /v3、release 资产 x86_64 命名、前端登录 401 误判;本地联调容器的启动坑位已记录在 local-e2e/start.sh 注释。
- 2026-09-17:实机安装发现 dataplaneapi systemd 服务需以 root 运行(写 cfg / systemctl reload / 读 runtime socket),已改 service 文件并列入 M4 加固;雨云服务器安全组不开 5555,WebUI 经 SSH 隧道(make tunnel)访问,连接信息存 gitignored 的 deploy/server.local.env。M1 至此全部完成。
- 2026-09-17:M2 完成。直连取代隧道(安全组已放通 5555,tunnel 保留为备用)。M2 实测补充的 v3 字段差异:配置模型无 mode 字段、server.check 为 "enabled"/"disabled" 字符串、runtime server 名字字段为 name(规范示例写 server_name)、runtime weight 为 JSON 数字(stats/native 不带参数即返回全部对象)。
- 2026-09-17:M3 完成。实测补充:事务内的配置写/删操作返回 202(提交时生效)而非 201/204,postJSON/deleteJSON 已放宽;raw 回滚走 POST /configuration/raw?version=N(自带乐观校验);bind 创建必须带 name。浏览器走查全流程:添加服务器→快照→删除→回滚恢复→清理,均通过。
- 2026-09-17:M4 核心完成。compose 容器停止(应用户要求),开发验证切本地进程(go run :8080 + vite :5173),数据从容器卷迁出至 backend/data/(已 gitignore)。凭据加密:key 缺省从 JWT secret 派生,老明文在实例编辑时自动迁移。RBAC 实测:operator 可写、viewer 全只读 403、用户管理仅 admin。keepalived/OIDC/k8s 三项因依赖外部环境暂缓,条件具备后补充。
