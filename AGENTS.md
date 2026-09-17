# AGENTS.md — AI 协作规范

本文件约束 AI 助手在本仓库中的行为。开始工作前先读一遍;与本文件冲突的口头习惯约定,以本文件为准。

## 项目速览

- HAProxy 管理平台:Go BFF(`backend/`,Gin + GORM + SQLite)+ React 前端(`frontend/`,Vite + shadcn/ui + Tailwind v4),通过 HAProxy 官方 dataplaneapi(`/v3`)管理节点。
- `PLAN.md` 是进度与决策的唯一真源:完成任务后把对应项 `[ ]` 改为 `[x]` 并附日期与验证方式;未完成项保持不动。
- 工具链:前端用 bun,后端用 go(1.27+)。本地开发:`go run ./cmd/server`(:8080)+ `bun run dev`(:5173,/api 代理到 8080)。

## Git 提交规范(必须遵守)

### 1. Message 一律使用英文

提交标题、正文、页脚全部用英文,禁止中文、拼音、无意义字符。

### 2. 格式:Conventional Commits

```
<type>(<scope>): <subject>

[optional body]

[optional footer(s)]
```

**type(必选,小写)**

| type | 用途 |
|---|---|
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

**subject 规则**
- 英文祈使语气、首字母小写:`add`、`fix`,不是 `added`、`Added`
- 结尾不加句号,整行(含 type/scope)不超过 72 字符
- 描述"这个提交做了什么",而不是"我改了哪些文件"

**body(可选)**:解释动机与影响的"为什么",每行不超过 72 字符;简单改动省略。

**破坏性变更**:type 后加 `!`(如 `feat!:`),并在页脚写 `BREAKING CHANGE: <说明>`。

### 3. 示例(本项目真实语境)

```
feat(backend): add transaction-based config apply API
fix(frontend): distinguish login 401 from session expiry
fix(backend): accept 202 responses from dataplaneapi tx operations
docs(plan): mark M3 milestone as completed
refactor(backend): extract dataplane request helpers into shared funcs
chore(deps): bump tanstack/react-query to 5.103
deploy: add prometheus exporter guide for haproxy nodes
```

反例(禁止):

```
update            ← 无信息量
修复了一个bug      ← 非英文
M3 finished       ← 非祈使语气、缺 type
feat: added new stuff to backend and frontend and also fixed some things  ← 混杂多个逻辑变更
```

### 4. 提交工作流规则

1. **一次提交一个逻辑变更**,不把不相关的修改混在一起;顺手改的无关内容单独提交或还原。
2. **提交前必须过构建检查**:`backend` 下 `go build ./... && go vet ./...`;`frontend` 下 `bun run build`(含 tsc)。检查不通过不提交。
3. **禁止入库的文件**(已在 .gitignore,保持排除,勿 force add):
   - `deploy/server.local.env` — 服务器连接与凭据
   - `.env` — 运行时密钥
   - `backend/data/` — SQLite 数据库(含加密凭据与快照)
   - 任何 `.pem` 私钥
   若发现密文/凭据被暂存,先移出暂存区再提交。
4. **PLAN.md 进度与代码变更同票**:完成计划任务时,同一提交或紧随的 `docs(plan):` 提交里更新对应复选框。
5. 默认**只在用户明确要求时**执行 commit;push 必须用户明确要求。不要给提交附加 AI 署名、Co-Authored-By 等页脚,除非用户要求。
6. 不 amend 已推送的提交、不 force push、不删除远程分支,除非用户明确要求。

## 其他行为约束

- 对仓库结构与接口不确定时,先读 `PLAN.md` 与相关源码,不要凭猜测修改。
- 涉及真实 HAProxy 节点的操作(配置变更、reload、回滚)属于高风险动作:连接信息只从 `deploy/server.local.env` 读取(gitignored),并确认用户已授权。
- 依赖升级与新增依赖:先说明用途与影响,征得同意再改 `go.mod` / `package.json`。
