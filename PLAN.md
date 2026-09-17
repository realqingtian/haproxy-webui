# HAProxy WebUI 开发计划与进度

> **维护约定**:任务完成后把对应项 `[ ]` 改为 `[x]`,未完成的不动;新任务追加到对应里程碑下。
> 每个里程碑的顺序即建议实施顺序,允许跨项并行。每期开工时,从 `docs/ROADMAP.md` 把对应任务搬入本文件追加里程碑段。

## 技术决策记录

| 决策点 | 结论 | 说明 |
|---|---|---|
| 管理通道 | HAProxy 官方 Data Plane API(dataplaneapi sidecar) | 不自研 haproxy.cfg 解析器,与 HAProxy Enterprise GUI 同底座;要求 HAProxy ≥ 1.9,建议 2.6+ |
| BFF 语言 | Go + Gin | 单二进制部署,CGO 关闭 |
| 存储 | SQLite(GORM + 纯 Go 驱动) | 仅元数据:实例 / 用户 / 审计 |
| 前端 | React 19 + Vite + TS + shadcn/ui(radix)+ Tailwind v4 + TanStack Query | 包管理用 bun |
| 认证 | JWT(Bearer,24h)+ bcrypt;OIDC/SSO(Authorization Code + PKCE,M5 起) | 本地账号登录可配置开关 |
| RBAC | admin / operator / viewer | viewer 只读;写操作(含实例管理)需 operator+;用户管理需 admin |
| 集群支持 | 单机与集群统一为"多实例"模型 | 一台 HAProxy = 一个实例;keepalived 主备展示归入集群分组(M5-3 第一阶段) |
| 部署 | docker compose + systemd 裸机 + k8s(kustomize,M5 起) | 前端由 nginx 托管并反代 /api |

## v0.5 体验与质量(2026-09-17 开工,纯本地可完成)

> 目标:把已有能力打磨到"敢给团队日常用"的水平,不新增依赖环境。任务来源:docs/ROADMAP.md v0.5 段。

- [x] 配置批量暂存区(staging)(2026-09-17 v0.5 完成,真机 rainyun-rcs 走查):编辑操作跨对话框累积
      进「待提交清单」(常驻入口显示条数,支持逐条移除/一键清空),一次事务批量应用——实测 3 操作与
      2 操作各只产生 1 条快照、1 次 reload;失败路径实测:任一操作失败整体回滚、清单保留可改后重提。
      附带修复:AclDialog 关闭按钮误为 type=submit(空字段会提交空操作);发现 dataplaneapi v3.4.3
      不支持 backend ACL 写入(405),UI 已移除 backend 卡片的 ACL 入口(frontend ACL 不受影响);
      后端 note 超 500 字节按 rune 边界截断,防 AuditLog.Detail 静默截断(附单测)
- [x] 文本级 diff 预览(2026-09-17 v0.5 完成,真机走查):暂存面板内嵌「diff 预览」——后端
      `POST /config/preview` 开事务执行暂存操作后读取事务内 raw 再放弃事务(不 reload、不落盘、
      不产生快照);前端用 diff(jsdiff)渲染统一 diff(新增绿/删除红,未变段折叠)。
      实测:预览正确显示新增 frontend/backend 行与 _md5hash/_version 头变更,预览后快照数不变,
      提交后节点 raw 与预览一致;后端 preview 不 commit 只 abort(集成测试断言 commits/aborts 计数)
- [x] 后端自动化测试(2026-09-17 v0.5 完成):dataplane 客户端单测 17 例
      (transport 状态码语义 GET200/POST201|202/DELETE202|204/401 映射、事务生命周期含 Reload-Id 头、
      /v3 字段差异回归:weight 数字/字符串、server name 字段、stats native;请求形状断言)+
      BFF 进程内集成测试(fake dataplaneapi):登录→实例注册(凭据加密)→apply 单事务→快照/回滚→
      失败中止→viewer 403→审计;另有容器级集成(make test-integration,真实 dataplaneapi v3.4.3,
      容器不可达自动 skip)。纯 stdlib,零新依赖
- [x] E2E 冒烟测试(Playwright)(2026-09-17 v0.5 完成):frontend/e2e/smoke.spec.ts 覆盖
      登录→实例增删→连通性→配置读取→暂存提交(1 次 reload)→运行时上下线→模板创建(4 操作 1 事务)→
      回滚;playwright.config 自动编排 local-e2e 容器 + 独立临时 DB 后端 + vite dev;
      make e2e 实测 1 passed(3.2s)。走查产出修正:模板对话框 Label 未关联 htmlFor(用 placeholder 定位)
- [x] 多实例体验(2026-09-17 v0.5 完成,双实例实测:rainyun-rcs + local-e2e 容器):侧边栏「配置管理」
      多实例时展开实例选择器(附健康点 + 实例管理入口),单实例维持直跳;新增「监控总览」页
      /monitoring——按集群分组的实例行(健康 / dataplaneapi 版本 / 服务器 UP-DOWN / 请求速率 / 当前连接),
      顶部全实例聚合卡,行点击下钻单实例监控;stats 按实例并发拉取复用 ['instance-stats'] 缓存。
      SummaryCard / StatusBadge 提取为共享组件复用
- [x] 小项打包(2026-09-17 v0.5 完成):暗色模式(next-themes 三态切换,浏览器实测深浅切换与持久化)、
      列表分页(手写 Pagination 组件:审计日志 50/页实测 82 条→2 页、stats 服务器表 20/页、≤1 页自动隐藏)、
      前端分包(5 个重页面路由懒加载,入口 chunk 561.5kB→448.2kB,500kB 构建警告消失,vite 零额外配置)

**验收**(2026-09-17 全部达成):连续多处配置修改只触发 1 次 reload(真机实测 3 操作/2 操作各 1 条快照 1 次
reload,E2E 断言单条 reload);`go test ./...` 进 `make test`(容器级用例进 `make test-integration`,
无 Docker 自动跳过)、E2E 冒烟进 `make e2e`(1 passed);两个实例时选择器正常工作(rainyun-rcs + local-e2e
容器实测,选择器带健康点直达配置页)。

**当前状态**:M1–M5 与 v0.5 已全部完成(2026-09-17,M1–M5 存档见
[docs/PLAN-M1-M5.md](docs/PLAN-M1-M5.md));下一期规划在 [docs/ROADMAP.md](docs/ROADMAP.md),
开工时把任务搬入本文件。

## 已知风险与注意事项(仍然有效)

1. **运行时 vs 持久化**:运行时上下线重启即失效,UI 必须明示该语义,持久化改动一律走配置 + reload。
2. **配置漂移**:绕过 UI 的手工修改会导致状态不一致,依赖「从服务器同步」重建基线快照。
3. **dataplaneapi 5555 端口**:安全组已放行公网直连(2026-09-17,开发验证期,用户知情接受的临时状态)。
   dataplaneapi 有 Basic Auth 且已非 root 运行(M5-4),但该端口为**明文 HTTP**——凭据可被链路窃听,且无频控。
   转生产前应收紧安全组来源 IP 或配 TLS。
4. **JWT 密钥**:默认 dev 密钥仅限本地,生产必须通过环境变量覆盖(compose 中已强制校验)。
5. **dataplaneapi 版本差异**:3.x 的 API 前缀是 `/v3`(2.x 为 `/v2`),探活为 `/v3/info`;release 资产命名中 64 位 x86 是 `x86_64`(amd64 只有包管理器格式)。BFF 客户端已对齐 v3,接入新版本节点时注意回归。
6. **前端 401 语义**:登录接口的 401(密码错误)与其它接口的 401(会话过期)必须区分,api.ts 已通过 `authRedirect` 选项处理,新增登录类接口(如 OIDC 回调)时注意沿用。
7. **实例凭据加密密钥**:缺省从 JWT secret 派生,换 JWT secret 会导致历史密文不可解;生产应显式设置 HAPROXY_WEBUI_ENCRYPTION_KEY(ROADMAP 技术债:v0.6 独立密钥必填校验)。
