# AGENTS.md — AI 协作规范

本文件约束 AI 助手在本仓库中的行为:协作流程、文档约定与 Git 提交规范**全部在这一份文件里**。
(2026-09-18 文档整理:原 docs/git-conventions.md 已并入本文件,docs/ 目录取消;
项目进度与规划统一在根目录 PLAN.md。)开始工作前先读一遍;与本文件冲突的口头习惯约定,以本文件为准。

## 项目速览

- HAProxy 管理平台:Go BFF(`backend/`,Gin + GORM + SQLite)+ React 前端(`frontend/`,Vite + shadcn/ui + Tailwind v4),通过 HAProxy 官方 dataplaneapi(`/v3`)管理节点。
- `PLAN.md` 是项目全景的唯一文档:现状 / 里程碑总览 / 技术决策 / 各期明细 / 后续规划(需求池)/ 技术债 / 已知风险都在其中。任务完成后把对应条目 `[ ]` 改为 `[x]` 并附日期与验证方式;新需求先进「后续规划」池,经用户确认排期后升格为里程碑小节;未完成项保持不动。
- 工具链:前端用 bun,后端用 go(1.27+)。本地开发:`go run ./cmd/server`(:8080)+ `bun run dev`(:5173,/api 代理到 8080)。
- 测试分三层:`make test`(单测 + 进程内集成,无 Docker 依赖)/ `make test-integration`(起 local-e2e 容器跑真实 dataplaneapi)/ `make e2e`(Playwright 冒烟,自动编排容器 + 独立 DB 后端 + vite)。
- 文档 lint:改完 Markdown 跑 `git ls-files '*.md' | xargs bunx markdownlint-cli`(只查入库文件;通配符写法会误扫 node_modules),配置见 `.markdownlint.json`,保持零告警。

## 文档结构(必须遵守)

| 文档 | 读者 | 内容 |
| --- | --- | --- |
| `README.md` | 使用者 | 功能总览、上手指南、部署、环境变量、FAQ |
| `PLAN.md` | 维护者 | 项目全景唯一文档:状态 / 里程碑 / 决策 / 各期明细 / 后续规划 / 风险 |
| `AGENTS.md`(本文件) | AI 与协作者 | 协作流程、文档约定、Git 提交规范 |

规范类内容统一更新本文件对应章节,不再另建独立规范文档、不再使用 docs/ 目录。

## Git 提交规范(必须遵守,违反视为未完成工作)

### Message 语言与格式

提交标题、正文、页脚一律使用**英文**,禁止中文、拼音、无意义字符。格式为 Conventional Commits:

```text
<type>(<scope>): <subject>

[optional body]

[optional footer(s)]
```

**type(必选,小写)**:

| type | 用途 |
| --- | --- |
| feat | 新功能 |
| fix | 缺陷修复 |
| docs | 仅文档(含 PLAN.md 进度更新) |
| refactor | 重构,无行为变化 |
| perf | 性能优化 |
| test | 测试 |
| chore | 构建、依赖、工具链等杂项 |
| ci | CI 流水线 |
| style | 格式调整(空白、格式化) |

**scope(可选,小写)**:`backend` / `frontend` / `deploy` / `docs` / `deps`。跨多范围时省略 scope。

**subject 规则**:英文祈使语气、首字母小写(`add`、`fix`,不是 `added`);结尾不加句号;整行(含 type/scope)不超过 72 字符;描述"这个提交做了什么",而不是"我改了哪些文件"。

**body(可选)**:解释动机与影响的"为什么",每行不超过 72 字符;简单改动省略。

**破坏性变更**:type 后加 `!`(如 `feat!:`),并在页脚写 `BREAKING CHANGE: <说明>`。

### 示例(本项目真实语境)

```text
feat(backend): add transaction-based config apply API
fix(frontend): distinguish login 401 from session expiry
fix(backend): accept 202 responses from dataplaneapi tx operations
docs(plan): mark M3 milestone as completed
refactor(backend): extract dataplane request helpers into shared funcs
chore(deps): bump tanstack/react-query to 5.103
deploy: add prometheus exporter guide for haproxy nodes
```

反例(禁止):

```text
update            ← 无信息量
修复了一个bug      ← 非英文
M3 finished       ← 非祈使语气、缺 type
feat: added new stuff to backend and frontend and also fixed some things  ← 混杂多个逻辑变更
```

### 提交工作流(先审查,后提交)

1. **一次提交一个逻辑变更**,不把不相关的修改混在一起;顺手改的无关内容单独提交或还原。
2. **提交前必须过构建检查**:`backend` 下 `go build ./... && go vet ./...`;`frontend` 下 `bun run build`(含 tsc)。检查不通过不提交。
3. **改动默认停留在工作区,不 commit、不 push**。完成一轮修改后,主动向用户汇报改动范围
   (`git status --short` + 每个文件改了什么、为什么),等用户审查;用户要求看细节时提供
   `git diff` / `git diff --stat`。
4. **只有用户明确要求时才提交**:用户说「提交」「commit」时执行 commit(仅本地);用户说
   「提交到 GitHub」「push」时才 commit + push。用户审查后提出修改意见的,先改完再等指令,
   不要自作主张提交。
5. **文档与代码同票**:任务完成时必须同步更新所有受影响的文档(对照下节「文档同步」),
   在同一提交或紧随的 `docs:` 提交里完成;文档没同步视为任务未完成。
6. 不要给提交附加 AI 署名、Co-Authored-By 等页脚,除非用户要求。
7. 不 amend 已推送的提交、不 force push、不删除远程分支,除非用户明确要求。

### 禁止入库的文件

以下文件已在 .gitignore 排除,**保持排除,勿 force add**;若发现被暂存,先移出暂存区再提交:

- `deploy/server.local.env` — 服务器连接与凭据
- `.env` — 运行时密钥
- `backend/data/` — SQLite 数据库(含加密凭据与快照)
- 任何 `.pem` 私钥

## 文档同步(必须遵守)

**任务完成 = 代码 + 文档一起交付。** 每完成一项任务,必须在同一批次内更新所有受影响的文档,
文档没同步视为任务未完成:

| 变更类型 | 必须更新的文档 |
| --- | --- |
| 计划任务完成 / 状态变化 | `PLAN.md`(复选框 + 日期与验证方式) |
| 新增 / 变更功能、接口、环境变量 | `README.md`(功能表、上手指南、环境变量表) |
| 新增 / 变更部署产物(脚本、service、清单) | `deploy/` 下对应文档与 `README.md` 部署章节 |
| 协作规范变更 | `AGENTS.md`(本文件) |

提交前自检:浏览一遍代码 diff,逐条确认上表;发现遗漏先补文档再提交。文档更新可以与代码
同一个提交,也可以是紧随的 `docs:` 提交。

## 其他行为约束

- 对仓库结构与接口不确定时,先读 `PLAN.md` 与相关源码,不要凭猜测修改。
- 涉及真实 HAProxy 节点的操作(配置变更、reload、重启服务、回滚)属于高风险动作:连接信息只从
  `deploy/server.local.env` 读取(gitignored),并确认用户已授权。
- 依赖升级与新增依赖:先说明用途与影响,征得同意再改 `go.mod` / `package.json`。
