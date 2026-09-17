# HAProxy WebUI 开发计划与进度

> **维护约定**:任务完成后把对应项 `[ ]` 改为 `[x]`,未完成的不动;新任务追加到对应里程碑下。
> 每个里程碑的顺序即建议实施顺序,允许跨项并行。每期开工时,从 `docs/ROADMAP.md` 把对应任务搬入本文件追加里程碑段。

**当前状态**:M1–M5 已全部完成(2026-09-17),进度与决策存档见 [docs/PLAN-M1-M5.md](docs/PLAN-M1-M5.md)。
下一期 **v0.5 体验与质量** 尚未开工,规划见 [docs/ROADMAP.md](docs/ROADMAP.md);开工时将其任务搬入本文件。

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
