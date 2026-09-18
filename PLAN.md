# HAProxy WebUI 项目全景(状态 · 决策 · 进度 · 规划)

> **本文件是项目唯一的进度真源与规划文档**:现状、里程碑总览、技术决策、各期明细、
> 后续规划、技术债与已知风险都在这里。
> 2026-09-18 文档整理:原 docs/PLAN-M1-M5.md(M1–M5 归档)、docs/ROADMAP.md(路线图)、
> docs/quality-cleanup.md(清理批次)已并入本文件并删除,docs/ 目录不再保留;
> 协作行为与 Git 提交规范独立在根目录 AGENTS.md;使用者文档见 README.md。
> **维护约定**:任务完成后把对应条目 `[ ]` 改为 `[x]` 并附日期与验证方式,未完成的不动;
> 新需求先进「五、后续规划」池,与用户确认范围并开工时,再升格为「四、各期明细」下的里程碑小节。

## 一、当前状态(先看这里)

- **已交付**:M1–M5、v0.5、v0.6 完成并推送 GitHub(最新 61ff6aa);
  v0.7(服务管理与配置体验)、v0.8(keepalived 集群视角)于 2026-09-18 完成开发、
  测试与真机 / 本地环境验收,改动在工作区待用户审查后提交。
- **功能范围**:多实例 HAProxy 可视化管理(配置事务编辑 + 一次 reload + 快照回滚)、
  运行时上下线与权重、监控大盘与集群 VRRP 视角、SSL 证书管理、dataplaneapi 服务管理
  (远程重启)、告警通知(飞书 / 钉钉 / 企业微信)、定时巡检、RBAC + 会话安全 + 审计、
  OIDC / SSO、k8s 部署清单。
- **质量门禁**:`make test`(单测 + 进程内集成,无 Docker 依赖)、`make test-integration`
  (真实 dataplaneapi 容器)、`make e2e`(Playwright 冒烟)三层全绿;markdownlint 零告警。
- **已排期待开发**(2026-09-18 用户确认):v0.10 Runtime maps 在线编辑 →
  v0.11 实时化(stats SSE 推送 + 日志尾部)→ v0.12 移动端适配与多语言,
  详见「四、各期明细」对应小节;需求池其余项未排期。

## 二、里程碑总览

| 周期 | 主题 | 状态 | 关键交付 |
| --- | --- | --- | --- |
| M1 | 骨架与部署 | ✅ 2026-09-17 | 前后端骨架、JWT + RBAC、实例 CRUD、全套部署产物、节点实机安装验证 |
| M2 | 只读展示 + 运行时控制 | ✅ 2026-09-17 | 配置与原文展示、server 上下线 / 权重、stats 仪表盘 |
| M3 | 配置管理 | ✅ 2026-09-17 | 事务化编辑、版本快照与一键回滚、漂移同步 |
| M4 | 平台化 | ✅ 2026-09-17 | 用户 / 审计页、凭据加密、配置模板、OIDC、k8s 清单、集群分组第一阶段 |
| M5 | M4 收尾批次 | ✅ 2026-09-17 | dataplaneapi 脱离 root(dpapi + sudo 白名单,实机验证) |
| v0.5 | 体验与质量 | ✅ 2026-09-17 | 批量暂存区(一次 reload)、diff 预览、三层测试体系、多实例体验、暗色 / 分页 / 分包 |
| v0.6 | 运维与可观测 | ✅ 2026-09-17 | 定时巡检、告警通知、Prometheus 探测、会话吊销、审计增强、独立加密密钥 |
| v0.7 | 服务管理与配置体验 | ✅ 2026-09-18(待提交) | dataplaneapi 服务管理(SSH)、配置搜索与跳转、SSL 证书管理;真机验收通过并修复节点存量问题 |
| v0.8 | keepalived 集群视角 | ✅ 2026-09-18(待提交) | VRRP 真实状态探测 + 监控总览集群视图;本地双容器验证环境,failover 实测 |

## 三、技术决策记录(现行有效)

| 决策点 | 结论 | 说明 |
| --- | --- | --- |
| 管理通道 | HAProxy 官方 Data Plane API(dataplaneapi sidecar) | 不自研 haproxy.cfg 解析器,与 HAProxy Enterprise GUI 同底座;要求 HAProxy ≥ 1.9,建议 2.6+(2026-09-17 选型,放弃 Roxy-WI 复用与纯 SSH 自研路线) |
| BFF 语言 | Go + Gin | 单二进制部署,CGO 关闭 |
| 存储 | SQLite(GORM + 纯 Go 驱动) | 仅元数据:实例 / 用户 / 审计 |
| 前端 | React 19 + Vite + TS + shadcn/ui(radix)+ Tailwind v4 + TanStack Query | 包管理用 bun |
| 认证 | JWT(Bearer,24h)+ bcrypt;OIDC/SSO(Authorization Code + PKCE,M5 起已交付) | 本地账号登录可配置开关 |
| RBAC | admin / operator / viewer | viewer 只读;写操作(含实例管理)需 operator+;用户管理需 admin |
| 集群支持 | 单机与集群统一为"多实例"模型 | 一台 HAProxy = 一个实例;VRRP 真实状态探测已于 v0.8 接入 |
| 节点服务管理 | BFF 经 SSH 执行 systemctl(v0.7) | 不引入节点侧 agent;x/crypto/ssh 与既有 bcrypt 同模块;重启依赖节点 sudo 免密白名单 |
| 部署 | docker compose + systemd 裸机 + k8s(kustomize,M5 起已交付) | 前端由 nginx 托管并反代 /api |

## 四、各期明细

### M1–M5(2026-09-17 一天内完成;完整过程存档见 git 历史)

- **M1 骨架与部署**:前后端骨架与数据模型(users / instances / audit_log)、JWT + RBAC、
  实例 CRUD 与连通性探测、审计、全套部署产物(compose / systemd / nginx / 节点侧
  install.sh + cfg 片段)、local-e2e 单容器联调环境;雨云节点实机安装验证
  (HAProxy 2.8.16 + dataplaneapi v3.4.3)。走查中修复登录 401 被误判为会话过期的问题。
- **M2 只读展示 + 运行时控制**:实例管理页、配置与原文展示(15s 刷新)、server 运行时
  上下线 / 权重(UI 明示「重启后失效」并写审计)、stats 仪表盘(10s 刷新);
  RBAC 前后端联动实测(viewer 全只读)。直连取代 SSH 隧道(安全组放通 5555)。
- **M3 配置管理**:事务封装(版本乐观锁)、可视化编辑(backend / server / frontend / bind /
  ACL)、每次提交自动快照 + 一键回滚、「从服务器同步」重建基线处理漂移。
- **M4 平台化**:用户管理与审计查询页、实例凭据 AES-256-GCM 加密(历史明文自动迁移)、
  配置模板(HTTP / TCP 一键生成)、Prometheus 接入指引;keepalived 降级第一阶段
  (集群分组模型与 UI)、OIDC / SSO(本地 dex 完整浏览器验证)、k8s kustomize 清单
  (本机 kind 实测)。compose 容器停止、数据迁至 backend/data/ 应用户要求。
- **M5 收尾批次**:dataplaneapi 脱离 root(专用用户 dpapi + sudo 白名单仅 reload/restart
  haproxy,雨云实机回归);独立质量清理批次(原 docs/quality-cleanup.md,已并入本段):
  审计写入去重、dataplane transport 归一、SQLite WAL + busy_timeout、登录限流
  (1 分钟 5 次失败锁 1 分钟)、make test/check 目标、STRICT 严格生产模式。
- **早期踩坑备忘**(实测得来,接新版本节点时注意):dataplaneapi 3.x 前缀 /v3(探活
  /v3/info);release 资产 64 位 x86 命名是 x86_64;事务内写删返回 202(提交才生效);
  bind 创建必须带 name;runtime 字段差异(weight JSON 数字、server 名字段为 name、
  check 为 enabled/disabled 字符串);v3.4.3 不支持 backend ACL 写入(405,UI 已下线);
  local-e2e 容器多次 USR2 后有多代进程残留、runtime 可能打到旧代(仅容器坑,真机无)。

### v0.5 体验与质量(2026-09-17,纯本地可完成)

> 目标:把已有能力打磨到"敢给团队日常用"的水平,不新增依赖环境。任务来源:路线图 v0.5 段。

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

### v0.6 运维与可观测(2026-09-17)

> 目标:从"管理配置"扩展到"运维保障",具备告警与巡检能力。任务来源:路线图 v0.6 段。

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

**验收**(路线图):人为制造 reload 失败能收到通知;改密后旧 token 失效——均达成
(2026-09-17 进程内断言 + 2026-09-18 真机走查)。注:当时 reload 失败的根因之一是节点 unit 的
NoNewPrivileges=true 存量问题,v0.7 真机验收时定位并修复(见 v0.7 节)。

### v0.7 服务管理与配置体验(2026-09-18)

> 目标:补齐节点侧运维闭环(dataplaneapi 服务管理),并从需求池提入两项纯本地可完成的功能。
> 范围决策:keepalived 集群视角因节点未部署 keepalived,后经用户同意以本地 Docker 环境立项为 v0.8。

- [x] dataplaneapi 服务管理:实例可选配置 SSH 连接(地址 / 端口 / 用户 / 密码或私钥,凭据走
      既有加密存储),经 SSH 执行 systemctl 查询 dataplaneapi 服务状态(active/inactive/启动时间)
      与远程重启。systemd 接口方案选 SSH:不引入节点侧 agent,与 M5-4 的 sudo 白名单同思路,
      x/crypto/ssh 属既有直接依赖 golang.org/x/crypto 同模块,零新增模块依赖;unit 名可配且
      只允许安全字符集,命令由白名单模板拼装防注入;前端实例监控页「服务管理」卡片
      (状态展示登录可见,重启 operator+,审计留痕)
      (2026-09-18 完成:internal/systemd 包(ValidUnit 白名单 + show 输出解析 + 错误消息
       提取含 stdout 回退);Instance 增 SSH 六字段(端口/unit 缺省 22/dataplaneapi,凭据
       EncryptStored 加密;SSHPort 迁移带 default:0,否则存量 SQLite 加 NOT NULL 列失败);
       GET /:id/service 登录可读、POST /:id/service/restart operator+ +审计 service.restart;
       常见失败译为可操作 hint(sudo 未免密 → NOPASSWD 白名单指引);
       测试:systemd 单测 + 进程内集成(自建 in-process SSH 服务器,覆盖状态查询 / 重启 /
       viewer 403 / sudo 免密未配置 hint / unit 注入 400 / 未配置态);
       前端监控页服务管理卡片(重启确认对话框注明 dataplaneapi 中断不影响数据面)+
       实例对话框 SSH 配置区)
- [x] 配置搜索与跳转:配置页按 frontend / backend / server 名称即时过滤定位,raw 视图
      按关键字跳转并高亮命中行
      (2026-09-18 完成:配置页新增常驻搜索框;backend 卡片按名称或服务器名/地址过滤
       (仅保留命中服务器行)、frontend 行按名称/默认后端/监听过滤、无匹配空态;新增
       RawConfigView 组件:行号 + 命中行高亮 + 命中计数 + 上一个/下一个容器内定位
       (避免整页滚动);E2E 断言过滤与原文命中计数)
- [x] SSL 证书管理:基于 dataplaneapi storage ssl_certificates 接口,证书列表 / 上传 / 查看 /
      删除;配置页新增「证书」页签;local-e2e 容器补 ssl 证书目录以支持集成验证
      (2026-09-18 完成:实测 3.4.3 需显式 --ssl-certs-dir 才注册路由、上传为 multipart
       file_upload、删除默认触发 reload(202+Reload-Id)、列表不解析元数据需逐个补查
       ——BFF 已封装补全;名称白名单防路径穿越、PEM 本地预检、128KB 上限;上传不触发
       reload,删除默认触发并让「引用未删 → reload 失败」立即暴露(前端删除对话框检查
       raw 配置引用数并警示);证书页签含过期着色(30 天内黄 / 已过期红)、元数据详情、
       viewer 只读;dataplaneapi storage 接口不提供内容读取,「查看」为元数据视图;
       测试:进程内集成 + 容器级集成 TestContainerSSLCertificates;
       local-e2e start.sh 与 deploy/dataplaneapi/dataplaneapi.service 均默认启用证书目录)
- [x] 收尾:make test / test-integration / e2e 全绿;README 文档同步;真机走查重启 dataplaneapi
      (2026-09-18 全部完成,用户授权后执行:自动化部分——make test 六包全绿、
       make test-integration 两用例通过(haproxy 2.8 容器)、make e2e 3 passed(新增 v0.7 冒烟:
       搜索过滤/原文跳转/证书上传查看删除/服务管理未配置态,global-setup 补 --build 保证镜像
       随源码重建)、go vet + 前端构建 + markdownlint 零告警;真机走查——
       ①服务状态展示:实例配 SSH(root + 私钥)后 GET /service 返回 active/running/启动时间;
       ②远程重启:POST /service/restart 实测节点启动时间 09-17 09:31 → 09-18 03:54,共用 3 次
       (含 unit 变更后再次重启)均成功;
       ③证书链路:上传 → 列表含解析元数据(subject/有效期/序列号)→ 删除,reload succeeded,
       ssl 目录清空、haproxy.cfg md5 前后一致(零残留),审计 cert.upload / cert.delete /
       service.restart 齐全;
       **验收发现并修复存量问题**:节点 unit 残留 M4 时期的 NoNewPrivileges=true,与 M5-4 的
       dpapi sudo 白名单冲突——dataplaneapi 进程内发起的 reload 自 M5-4 起一直失败
       (9/17 验收的 reload 失败被误归因于端口占用);已推送仓库版 unit
       (NoNewPrivileges=false + --ssl-certs-dir)到节点并经 WebUI 重启生效,实测 reload
       succeeded。遗留改进:证书删除触发的 reload 结果暂无后台监视(仅返回 reloadId),
       后续可接入 reload 失败告警)

### v0.8 keepalived 集群视角(2026-09-18,本地 Docker 验证)

> 目标:补齐 keepalived 主备集群的 VRRP 真实状态探测(VIP 归属、主备角色一览)。
> 环境决策:真实节点无 keepalived,经用户同意在本地 Docker 搭建双节点环境(unicast VRRP)
> 作为验证环境(deploy/dev-keepalived/)。

- [x] 验证环境:deploy/dev-keepalived/ 双容器(lb1/lb2,alpine + haproxy + keepalived +
      openssh + dataplaneapi),固定 IP + unicast VRRP(容器网络无组播),sshd 供服务管理
      SSH 链路;compose 端口与 dpapi-e2e 错开(5557/5558)
      (2026-09-18 完成:环境一次搭成,实测 MASTER 持 VIP 172.28.255.100、BACKUP 无 VIP;
       docker stop lb1 → lb2 约 4s 接管 VIP,docker start lb1 → 高优先级抢占回切,均符合预期)
- [x] 后端探测:systemd 包增 KeepalivedStatus,角色推导 master/backup/fault;
      GET /api/clusters/:id/vrrp 并发探测组内实例(未配 SSH 的实例标注),登录可读
      (2026-09-18 完成:实现用 pgrep + ip addr 双命令——不依赖 systemd,容器 / alpine
       节点同样可用;VIP 只在 Go 侧与 ip 输出精确比对(IPv4-mapped 兼容),不进命令串;
       并发探测,未配 SSH / SSH 失败的节点单独标注不互相影响;单测覆盖 VIP 匹配矩阵,
       集成测试(in-process SSH)覆盖 主/备/故障/未配 SSH 四态;真实验证环境实测与
       failover 翻转一致)
- [x] 前端:监控总览页集群分组内嵌 VRRP 状态(VIP + 各节点主/备/故障徽标 + 手动刷新)
      (2026-09-18 完成:VrrpStrip 组件,主绿/备蓝/故障红,15s 轮询 + 刷新按钮,
       未配 SSH 节点显示「未配置 SSH」;浏览器实测渲染与 failover 数据翻转正常)
- [x] 收尾:make test 全绿 + 新用例;README 同步;本地环境实测主备状态与 failover
      (2026-09-18 完成:make test 八包全绿;README 功能表增「集群视角」行 +
       「keepalived 验证环境」使用说明 + 目录树;markdownlint 零告警)

## 五、后续规划(需求池,未排期)

> 有价值但未排期的需求,开工前需确认范围。当前为空:
> 原五项(Runtime maps、实时推送、移动端适配、i18n、日志尾部)经用户 2026-09-18 确认
> 已排期为 v0.10–v0.12;v0.7/v0.8 验收遗留三项改进已于 v0.9 完成。

## 六、已知技术债(已全部偿清)

| 债 | 影响 | 偿还记录 |
| --- | --- | --- |
| ~~dataplaneapi 以 root 运行~~ | — | M5-4(2026-09-17):专用用户 dpapi + sudo 白名单,雨云节点实机验证 |
| ~~实例凭据加密密钥缺省派生自 JWT secret~~ | — | v0.6(2026-09-17):ENCRYPTION_KEY 非空时密钥仅由其派生,compose 必填,启动自动迁移历史密文 |
| ~~前端单 chunk >500kB~~ | — | v0.5(2026-09-17):路由懒加载,入口 448kB 无警告 |
| ~~local-e2e 容器内 haproxy 为 3.4、真实节点为 2.8~~ | — | v0.6(2026-09-17):镜像默认 alpine 3.19(haproxy 2.8.16)对齐真实节点 |

## v0.9 可靠性收尾(2026-09-18 开工)

> 目标:清掉 v0.7/v0.8 验收遗留的三个改进项,补一份转生产安全清单。任务来源:遗留改进 +
> 已知风险 3 / 8,范围为用户确认的「按建议完善」。

- [x] 证书删除 reload 接入后台监视(2026-09-18 完成):watchReloadStatus 改为接收完整
      失败描述,证书删除拿到 Reload-Id 后走同一监视器——进程内全链路测试(fake 返回 failed →
      webhook 收到「证书删除 fail.pem 后 reload 失败」+ 审计 reload.failed)
- [x] SSH host key 指纹校验(2026-09-18 完成):systemd 包 Fingerprint(格式对齐
      ssh-keygen -lf)+ HostKeyCallback 钉扎;实例增 SSHHostKey 字段(非密钥,明文可见),
      空 = 首连信任并自动回写(TOFU),非空 = 不匹配拒绝连接(提示重装 / 中间人,可重置重录);
      ConfigFromInstance / Hint 移入 systemd 包供 api 与 scheduler 共用;
      测试:TOFU 回写 / 钉扎匹配 / 指纹不符 502+hint(in-process SSH 服务器抽出为
      internal/systemd/systemdtest 供 api 与 scheduler 共用);前端实例对话框展示指纹与重置
- [x] keepalived 主备切换告警(2026-09-18 完成):scheduler 新增 VRRP 巡检(与连通性监控同周期,
      仅探测配置了 SSH 且已入组的实例),角色相对上次已知状态变化即边沿告警
      (notify 新增 KindVRRPChange「主备切换」+ 审计 vrrp.change);首轮静默建基线,
      探测失败记 unknown 不覆盖已知角色(失联由连通性告警覆盖);
      测试:diff 纯函数单测 + 全链路集成(fake SSH 双节点翻转 → webhook 一次 + 审计含
      「lb1: 主 → 备 / lb2: 备 → 主」,角色不变不重复告警)
- [x] README 转生产检查清单(2026-09-18 完成):admin 密码、5555 收紧、HTTPS、
      SSH 最小授权、数据备份、告警链路验证六项自查;告警功能行补主备切换,服务管理行补指纹
- [x] 收尾:make test / test-integration / e2e 全绿;PLAN / README 同步
      (2026-09-18 完成:make test 八包全绿、test-integration 两用例通过、e2e 3 passed、
       vet + 前端构建 + markdownlint 零告警)

## v0.10 Runtime maps 在线编辑(2026-09-18 排期)

> 来源:需求池。目标:在线管理 HAProxy maps(灰度名单 / 域名分流等),条目增删改即时生效并可持久化。

- [x] 探测与基建(2026-09-18 完成):实测 3.4.3——storage maps 默认可用(免 flag,
       /etc/haproxy/maps);runtime maps 全套 CRUD 需配置引用才注册;
       **条目 PUT/DELETE 需按 key 定位(GET 返回的指针 id 节点不认),force_sync=true
       即时同步节点文件**;local-e2e 镜像内置被 demo 前端引用的 hosts.map
- [x] 后端(2026-09-18 完成):dataplane/maps.go 客户端(runtime 列表 / 条目 / 增删改 +
       storage 列表 / 内容 / 上传,transport 抽出 postMultipart 与证书上传共用);
       handler_maps.go 合并视图(runtime 生效 + storage 未引用,active 标记)、
       key/value 白名单校验(无空白引号 ≤256)、未生效 map 转 400 + 可操作 hint;
       读登录可读、写 operator+,审计 map.entry.add/set/delete + map.upload;
       fake dataplane 补 maps 全套端点;进程内 + 容器级集成测试
       (增删改后经 storage 内容接口断言 force_sync 落盘)
- [x] 前端(2026-09-18 完成):配置页「Maps」页签——map 列表(生效状态徽标)、
       条目表 + 添加行、行内编辑值 / 删除(确认即时生效语义)、文件内容查看、
       上传对话框;未生效 map 仅可查看内容并展示指引
- [x] 收尾(2026-09-18 完成):make test 八包全绿、test-integration 三用例通过
       (新增 TestContainerMapsFlow)、e2e 3 passed(Maps 条目增删步骤)、
       vet + 前端构建 + markdownlint 零告警;README 功能表增「Runtime Maps」行,
       service 单元 ExecStartPre 补 /etc/haproxy/maps 目录

## v0.11 实时化:stats 推送与日志尾部(2026-09-18 排期)

> 来源:需求池。目标:stats 替代 10s 轮询;提供节点 haproxy 日志实时尾部查看。
> 技术决策:用 SSE(Server-Sent Events)而非 WebSocket——本场景全部为服务端单向推送,
> SSE 零新依赖且 Bearer 鉴权头天然可用(EventSource 不支持自定义头,前端用 fetch 流式
> 读取 + 自动重连);日志尾部复用 v0.7 的 SSH 通道执行 tail -f 流式转发。

- [x] SSE 基建(2026-09-18 完成):`GET /api/instances/:id/stats/stream` 每 5s 拉
      dataplaneapi 全量 stats 推 JSON 事件(text/event-stream + Flush;拉取失败输出注释行
      保持连接,由连通性告警兜底);客户端断开(请求 ctx 取消)即结束
- [x] stats 页接入 SSE(2026-09-18 完成):useStatsStream hook 以 fetch 流式读取
      (Bearer 头天然可用),解析 data 帧更新数据;连续失败 3 次自动回落 10s 轮询
      (useQuery enabled 联动),页头显示「实时推送中 / 已回落轮询」状态
- [x] 日志尾部(2026-09-18 完成):Instance 增 LogPath(可选,默认 /var/log/haproxy.log);
      systemd 包抽出不带响应体的 dial() 并新增 TailStream(SSH tail -n 200 -f 逐行回调,
      ctx 取消即断连);`GET /:id/logs/stream` SSE 逐行 JSON 推送(需 SSH,登录可读,
      路径白名单校验防注入);前端「日志」页签(暂停 / 清屏 / 关键字过滤 / 重连,
      缓冲上限 2000 行);local-e2e 容器补 busybox syslogd + /dev/log 目标 + sshd
      (2222 端口)使日志落文件、链路可测
- [x] 收尾(2026-09-18 完成):make test 八包全绿(stats 流 / 日志流进程内测试 +
       未配置 400 + 非法路径 400)、test-integration 四用例通过(新增容器级日志流:
       经 SSH tail 断言就绪标记行)、e2e 4 passed(新增日志尾部冒烟:浏览器全链路)、
       vet + 前端构建 + markdownlint 零告警;README 功能表同步

## v0.12 体验覆盖:移动端适配与多语言(2026-09-18 排期)

> 来源:需求池。目标:窄屏可用 + 界面多语言。i18n 选 react-i18next(zh 默认,en 抽取),
> 前端新增依赖会在实施时说明。

- [x] 移动端 / 窄屏适配(2026-09-18 完成):md 以下侧边栏改为汉堡按钮唤起的抽屉导航
      (点遮罩 / 导航后自动关闭);主内容区内边距响应式(p-3 / sm:p-4 / md:p-6);
      用户名在窄屏仅显示头像;Table 组件自带横向滚动、Dialog 自带窄屏宽度限制,均无需改动;
      375px 视口浏览器走查仪表盘 / 实例管理 / 抽屉交互
- [x] 多语言 i18n(2026-09-18 完成):react-i18next 接入(zh 默认、偏好持久化 localStorage),
      顶栏语言切换(中文 / English)实时生效;**全部页面与组件的用户可见文案抽取完毕**
      (src/i18n/zh.ts + en.ts 约 300 键:nav / common / layout / login / dashboard /
      instances / config / stats / certs / maps / logs / service / monitoring / audit /
      users / alerts / oidc / vrrp / dlg / staging / template / revisions;
      en 缺失键回落中文)。设计决策:后端接口错误文案与审计摘要(config-ops)保持中文
- [x] 收尾(2026-09-18 完成):make test 八包全绿、vet、前端构建、E2E 4 passed、
      markdownlint 零告警;README 体验行更新为完整多语言覆盖
- [ ] 收尾:测试 + e2e 适配 + README / PLAN 同步

## 七、已知风险与注意事项(仍然有效)

1. **运行时 vs 持久化**:运行时上下线重启即失效,UI 必须明示该语义,持久化改动一律走配置 + reload。
2. **配置漂移**:绕过 UI 的手工修改会导致状态不一致,依赖「从服务器同步」重建基线快照。
3. **dataplaneapi 5555 端口**:安全组已放行公网直连(2026-09-17,开发验证期,用户知情接受的临时状态)。
   dataplaneapi 有 Basic Auth 且已非 root 运行(M5-4),但该端口为**明文 HTTP**——凭据可被链路窃听,且无频控。
   转生产前应收紧安全组来源 IP 或配 TLS。
4. **JWT 密钥**:默认 dev 密钥仅限本地,生产必须通过环境变量覆盖(compose 中已强制校验)。
5. **dataplaneapi 版本差异**:3.x 的 API 前缀是 `/v3`(2.x 为 `/v2`),探活为 `/v3/info`;release 资产命名中
   64 位 x86 是 `x86_64`(amd64 只有包管理器格式)。BFF 客户端已对齐 v3,接入新版本节点时注意回归。
6. **前端 401 语义**:登录接口的 401(密码错误)与其它接口的 401(会话过期)必须区分,api.ts 已通过
   `authRedirect` 选项处理,新增登录类接口(如 OIDC 回调)时注意沿用。
7. **实例凭据加密密钥**:v0.6 起生产(compose)强制要求独立的 HAPROXY_WEBUI_ENCRYPTION_KEY;
   独立密钥生效后密钥不再依赖 JWT secret(轮换 JWT secret 不影响凭据),首次启动自动迁移历史密文;
   本地开发不设置时仍回落旧派生并告警,STRICT 模式拒绝启动。
8. **服务管理 SSH(v0.7/v0.9)安全边界**:SSH 密码 / 私钥与实例凭据同机制加密存储;
   host key 自 v0.9 起采用 TOFU + 钉扎(首连自动记录、此后不匹配拒绝)——残余风险窗口仅在
   首次连接(可与节点控制台核对指纹后录入);远程重启要求节点侧 sudo NOPASSWD 白名单,
   建议限定到具体 unit 的 systemctl restart。
9. **存量 SQLite 库迁移**:对已有数据的表加 NOT NULL 列必须带 default(如 Instance.SSHPort 的
   `default:0`),否则 AutoMigrate 报 Cannot add a NOT NULL column;新列默认值语义在代码侧兜底。
