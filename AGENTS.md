# AGENTS.md — AI 协作规范

本文件约束 AI 助手在本仓库中的行为。开始工作前先读一遍;与本文件冲突的口头习惯约定,以本文件为准。

## 项目速览

- HAProxy 管理平台:Go BFF(`backend/`,Gin + GORM + SQLite)+ React 前端(`frontend/`,Vite + shadcn/ui + Tailwind v4),通过 HAProxy 官方 dataplaneapi(`/v3`)管理节点。
- `PLAN.md` 是进度与决策的唯一真源:完成任务后把对应项 `[ ]` 改为 `[x]` 并附日期与验证方式;未完成项保持不动。
- `docs/ROADMAP.md` 是后续迭代的规划文档:规划期只改 ROADMAP,开工时把对应期任务搬进 PLAN.md 作为进度真源。
- 工具链:前端用 bun,后端用 go(1.27+)。本地开发:`go run ./cmd/server`(:8080)+ `bun run dev`(:5173,/api 代理到 8080)。
- 测试分三层:`make test`(单测 + 进程内集成,无 Docker 依赖)/ `make test-integration`(起 local-e2e 容器跑真实 dataplaneapi)/ `make e2e`(Playwright 冒烟,自动编排容器 + 独立 DB 后端 + vite)。
- 文档 lint:改完 Markdown 跑 `git ls-files '*.md' | xargs bunx markdownlint-cli`(只查入库文件;通配符写法会误扫 node_modules),配置见 `.markdownlint.json`,保持零告警。

## Git 提交规范(必须遵守)

**完整规范见 [docs/git-conventions.md](docs/git-conventions.md),所有 git 操作必须遵守,违反视为未完成工作。**核心红线:

- Message 一律英文,Conventional Commits 格式:`type(scope): subject`
- **改动默认停留在工作区,不 commit、不 push**;完成修改后主动汇报改动范围,等用户审查
- 仅在用户明确说「提交」时 commit(仅本地);说「提交到 GitHub」「push」时才推送
- 提交前过构建检查:`go build ./... && go vet ./...`、`bun run build`
- 禁止入库:`deploy/server.local.env`、`.env`、`backend/data/`、任何 `.pem`(已 gitignore,勿 force add)

## 文档同步(必须遵守)

**任务完成 = 代码 + 文档一起交付。** 每完成一项任务,必须在同一批次内更新所有受影响的文档,
文档没同步视为任务未完成:

| 变更类型 | 必须更新的文档 |
| --- | --- |
| 计划任务完成 / 状态变化 | `PLAN.md`(复选框 + 日期与验证方式)、`docs/ROADMAP.md`(如该项在路线图中) |
| 新增 / 变更功能、接口、环境变量 | `README.md`(功能表、上手指南、环境变量表) |
| 新增 / 变更部署产物(脚本、service、清单) | `deploy/` 下对应文档与 `README.md` 部署章节 |
| 新增 / 变更目录结构、协作规范 | `README.md` 目录树、`AGENTS.md` / `docs/` 对应文档 |

提交前自检:浏览一遍代码 diff,逐条确认上表;发现遗漏先补文档再提交。文档更新可以与代码
同一个提交,也可以是紧随的 `docs:` 提交(见 docs/git-conventions.md)。

## 其他行为约束

- 对仓库结构与接口不确定时,先读 `PLAN.md` 与相关源码,不要凭猜测修改。
- 涉及真实 HAProxy 节点的操作(配置变更、reload、回滚)属于高风险动作:连接信息只从 `deploy/server.local.env` 读取(gitignored),并确认用户已授权。
- 依赖升级与新增依赖:先说明用途与影响,征得同意再改 `go.mod` / `package.json`。
- 后续新增的协作规范文档统一放 `docs/` 下,并在本文件登记入口。
