# HAProxy WebUI 开发计划与进度

> **维护约定**:任务完成后把对应项 `[ ]` 改为 `[x]`,未完成的不动;新任务追加到对应里程碑下。
> 每个里程碑的顺序即建议实施顺序,允许跨项并行。每期开工时,从 `docs/ROADMAP.md` 把对应任务搬入本文件追加里程碑段。

## 技术决策记录

| 决策点 | 结论 | 说明 |
| --- | --- | --- |
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

**当前状态**:M1–M5、v0.5 与 v0.6 均已完成(2026-09-17/18,含雨云节点真机验收;M1–M5 存档见
[docs/PLAN-M1-M5.md](docs/PLAN-M1-M5.md))。v0.6 代码与文档已推送 GitHub(80bcee5..4062090);
真机验收期间顺带确认:定时巡检已在真实节点抓到漂移快照(revision #18,source=scheduled)。
下一期规划见 docs/ROADMAP.md 需求池与技术债表。

## v0.6 运维与可观测(2026-09-17 开工)

> 目标:从"管理配置"扩展到"运维保障",具备告警与巡检能力。任务来源:docs/ROADMAP.md v0.6 段,
> 实施顺序即下列顺序。

- [x] 基础设施:优雅停机(`http.Server` + signal)+ `Setting` 设置表与 `GET/PUT /api/settings`
      (admin),快照周期 / 探测周期 / 告警冷却存 DB,改完即生效无需重启
      (2026-09-17 T1 完成:main.go 改 http.Server + NotifyContext 优雅停机;scheduler 15s tick
       骨架;settings 包 upsert/默认值回落/巡检状态 JSON 存取,单测覆盖往返;
       GET settings 登录可读、PUT 仅 admin,后端 build/vet/test 全绿)
- [x] 会话管理:JWT 刷新与主动吊销——`User.TokenVersion` + Claims.Ver,中间件逐请求校验用户存在
      且版本一致;改密 / 重置密码 / 删除用户 / 强制下线即吊销全部旧 token;
      `POST /api/auth/refresh` 滑动续期(前端余量 <12h 自动刷新);用户页「强制下线」
      (2026-09-17 T2 完成:中间件逐请求 DB 校验,角色以 DB 实时为准——改角色无需重登立即生效
       (集成测试断言);改密后旧 token 401、重置 / 删除 / 强制下线同效(均测试覆盖);
       前端余量 <12h 自动刷新(挂载 + 30 分钟周期)、改密成功即回登录页、用户页强制下线按钮;
       前后端 build / test 全绿)
- [x] 审计日志增强:`from`/`to` 时间过滤(CreatedAt 加索引)+ CSV 导出(同过滤条件,上限 1 万行,
      UTF-8 BOM);前端 datetime-local 起止输入 + 导出按钮
      (2026-09-17 T3 完成:内联 handler 迁至 handler_audit.go,from/to 接受 RFC3339 与
       datetime-local 两种精度,非法值 400(测试覆盖);CSV 带 BOM、含表头与全部过滤条件
       (测试覆盖);前端时间范围 + 清除按钮 + 导出按钮(apiDownload 鉴权下载),
       操作类型下拉补强制下线 / reload 失败)
- [x] Prometheus 探测:实例可选 `MetricsURL`(空则按 BaseURL host 推导 :8404),GET /metrics 探活;
      `GET /api/instances/:id/metrics-probe`;实例监控页探测卡片 + 未接入指引
      (2026-09-17 T4 完成:地址推导规则含显式基地址 / 已带 /metrics / 推导失败四类
       (单测覆盖);probe 接口返回 ok/url/detail/hint,HTTP 200 且内容含 haproxy 指标才算可达
       (集成测试正反例覆盖);实例监控页探测卡片(结果缓存 1 分钟 + 手动重测),
       实例对话框新增「Metrics 地址(可选)」)
- [x] 配置快照定时抓取:`ConfigRevision.Source`(manual/sync/scheduled)+ Drifted;scheduler 周期拉
      raw 与最近快照比对,仅漂移或无基线时落新快照(防膨胀),巡检状态(每实例 lastRun/结果)写
      Setting;RevisionsTab 来源列 + 漂移徽标 + 巡检状态
      (2026-09-17 T5 完成:CaptureRevision 与 gin 解耦供 handler/巡检共用;巡检三阶段
       基线/无变化不落/漂移落快照打标 + 节点不可达记 error(单测覆盖);巡检状态聚合 JSON 落
       Setting 并经 /api/settings 透出;快照列表新增来源列与漂移徽标,版本历史页顶部显示巡检状态)
- [x] 告警通知:`AlertChannel`(飞书 / 钉钉 / 企业微信,text 消息)+ `internal/notify` fan-out +
      测试发送接口;触发源:reload 失败(提交后后台轮询 ReloadStatus 至终态)、连通性探测失败、
      backend 全 DOWN——均边沿触发 + 恢复通知 + 冷却时间;新导航页「告警与巡检」(admin)
      (2026-09-17 T6 完成:三种渠道 payload 形状单测;fan-out 只发启用渠道、单渠道失败不影响
       其他(单测);监控边沿触发测试覆盖首轮静默 / 翻转告警 / 恢复告警 / 节点失联恢复;
       reload 失败进程内全链路测试(fake 返回 failed → webhook 收到推送 + 审计 reload.failed);
       「告警与巡检」页含渠道 CRUD / 测试发送 / 巡检周期设置 / 巡检状态表,导航仅 admin 可见;
       顺带修复 gorm default:true 吞掉显式 false 的隐患(Instance/AlertChannel.Enabled 去 default 标签))
- [x] 技术债:独立加密密钥(ENCRYPTION_KEY 非空时密钥仅由其派生,启动时旧钥密文自动迁移重加密,
      EncryptStored 失败不再静默降级明文,compose 密钥必填);local-e2e 镜像提供 haproxy 2.8
      (alpine 3.19,build-arg 可切回最新)
      (2026-09-17 T7 完成:独立密钥下轮换 JWT secret 凭据不受影响、TryMigrateCiphertext 三态与
       DB 级迁移幂等(单测覆盖);EncryptStored 改返回错误由 handler 500 兜底;docker-compose 的
       ENCRYPTION_KEY 改 :? 必填;local-e2e 默认 alpine 3.19 = HAProxy 2.8.16,容器级集成测试
       在 2.8 下全链路通过,ALPINE_VERSION=3 可切回最新 haproxy)
- [x] 收尾:fake dataplaneapi 补 reloads/:id(reloads 状态可注入 failed);测试补齐(吊销 / 刷新 /
      审计过滤 / CSV / settings RBAC / 渠道 payload / 巡检漂移 / 边沿告警 / probe / 密钥迁移 /
      reload 失败全链路);make test / test-integration / e2e 全绿;README / ROADMAP / .env.example
      / compose 文档同步
      (2026-09-17 收尾完成:后端 go test 六包全绿,make test-integration 在 haproxy 2.8 容器下通过,
       Playwright 两条冒烟 2 passed(核心链路 + 告警与巡检);文档四处同步完毕)
- [x] 真机验收:雨云节点人为制造 reload 失败收通知、改密后旧 token 失效走查
      (2026-09-18 完成:本地后端连真实库启动(旧派生密钥兼容路径正常),飞书渠道测试发送
       code:0;真实节点提交绑定 0.0.0.0:5555(已占用端口)的前端——事务提交成功而 reload
       必然失败,后台监视器轮询确认 failed,审计落 reload.failed,告警推送飞书成功
       (sent via feishu-rainyun);随即回滚基线快照,节点配置零残留。改密走查用临时 operator
       账号:改密前旧 token 200,自改密码后旧 token 401、新密码登录 200,临时账号已删。
       过程中发现并修复渠道测试接口的 nil-pointer panic(map 字面量两侧表达式都会求值),
       已补强测试覆盖渠道测试发送路径)

**验收**(ROADMAP):人为制造 reload 失败能收到通知;改密后旧 token 失效。

## 已知风险与注意事项(仍然有效)

1. **运行时 vs 持久化**:运行时上下线重启即失效,UI 必须明示该语义,持久化改动一律走配置 + reload。
2. **配置漂移**:绕过 UI 的手工修改会导致状态不一致,依赖「从服务器同步」重建基线快照。
3. **dataplaneapi 5555 端口**:安全组已放行公网直连(2026-09-17,开发验证期,用户知情接受的临时状态)。
   dataplaneapi 有 Basic Auth 且已非 root 运行(M5-4),但该端口为**明文 HTTP**——凭据可被链路窃听,且无频控。
   转生产前应收紧安全组来源 IP 或配 TLS。
4. **JWT 密钥**:默认 dev 密钥仅限本地,生产必须通过环境变量覆盖(compose 中已强制校验)。
5. **dataplaneapi 版本差异**:3.x 的 API 前缀是 `/v3`(2.x 为 `/v2`),探活为 `/v3/info`;release 资产命名中 64 位 x86 是 `x86_64`(amd64 只有包管理器格式)。BFF 客户端已对齐 v3,接入新版本节点时注意回归。
6. **前端 401 语义**:登录接口的 401(密码错误)与其它接口的 401(会话过期)必须区分,api.ts 已通过 `authRedirect` 选项处理,新增登录类接口(如 OIDC 回调)时注意沿用。
7. **实例凭据加密密钥**:v0.6 起生产(compose)强制要求独立的 HAPROXY_WEBUI_ENCRYPTION_KEY;
   独立密钥生效后密钥不再依赖 JWT secret(轮换 JWT secret 不影响凭据),首次启动自动迁移历史密文;
   本地开发不设置时仍回落旧派生并告警,STRICT 模式拒绝启动。
