# Git 提交规范

本文档是本仓库 git 提交的完整规范,AI 助手与协作者**必须遵守**(入口约定见根目录 [AGENTS.md](../AGENTS.md))。

## 1. Message 语言

提交标题、正文、页脚一律使用**英文**,禁止中文、拼音、无意义字符。

## 2. 格式:Conventional Commits

```text
<type>(<scope>): <subject>

[optional body]

[optional footer(s)]
```

### type(必选,小写)

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

### scope(可选,小写)

`backend` / `frontend` / `deploy` / `docs` / `deps`。跨多范围时省略 scope。

### subject 规则

- 英文祈使语气、首字母小写:`add`、`fix`,不是 `added`、`Added`
- 结尾不加句号,整行(含 type/scope)不超过 72 字符
- 描述"这个提交做了什么",而不是"我改了哪些文件"

### body(可选)

解释动机与影响的"为什么",每行不超过 72 字符;简单改动省略。

### 破坏性变更

type 后加 `!`(如 `feat!:`),并在页脚写 `BREAKING CHANGE: <说明>`。

## 3. 示例(本项目真实语境)

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

## 4. 提交工作流(先审查,后提交)

1. **一次提交一个逻辑变更**,不把不相关的修改混在一起;顺手改的无关内容单独提交或还原。
2. **提交前必须过构建检查**:`backend` 下 `go build ./... && go vet ./...`;`frontend` 下 `bun run build`(含 tsc)。检查不通过不提交。
3. **改动默认停留在工作区,不 commit、不 push**。完成一轮修改后,主动向用户汇报改动范围
   (`git status --short` + 每个文件改了什么、为什么),等用户审查;用户要求看细节时提供
   `git diff` / `git diff --stat`。
4. **只有用户明确要求时才提交**:用户说「提交」「commit」时执行 commit(仅本地);用户说
   「提交到 GitHub」「push」时才 commit + push。用户审查后提出修改意见的,先改完再等指令,
   不要自作主张提交。
5. **文档与代码同票**:任务完成时必须同步更新所有受影响的文档(PLAN.md / ROADMAP / README /
   deploy 文档等,对照见 AGENTS.md「文档同步」一节),在同一提交或紧随的 `docs:` 提交里完成;
   文档没同步视为任务未完成。
6. 不要给提交附加 AI 署名、Co-Authored-By 等页脚,除非用户要求。
7. 不 amend 已推送的提交、不 force push、不删除远程分支,除非用户明确要求。

## 5. 禁止入库的文件

以下文件已在 .gitignore 排除,**保持排除,勿 force add**;若发现被暂存,先移出暂存区再提交:

- `deploy/server.local.env` — 服务器连接与凭据
- `.env` — 运行时密钥
- `backend/data/` — SQLite 数据库(含加密凭据与快照)
- 任何 `.pem` 私钥
