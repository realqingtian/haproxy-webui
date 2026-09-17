# 代码质量清理批次(2026-09-17 立项)

> 本文档是本轮清理的工作清单,完成后标记 `[x]` 并附验证方式。
> 来源:PLAN.md 风险区遗留项 + M1–M4 开发过程中记录的代码级优化点。

## A. 后端清理

- [x] A1 审计写入去重:合并到 `internal/api/audit.go`(`writeAudit` 指定用户 / `audit` 取当前登录用户),
      删除 `AuthHandler.audit` 方法与 handler_instance 内重复定义
- [x] A2 dataplane HTTP 助手归一:新建 `internal/dataplane/transport.go`
      (getJSON / postJSON / putJSON / deleteJSON / readText / decodeResponse),
      状态码约定注释集中维护;config.go、tx.go、client.go 均改为调用助手
- [x] A3 SQLite 开启 WAL + busy_timeout(5s):`database.Open` DSN 追加 pragma;
      验证 `PRAGMA journal_mode` 返回 wal
- [x] A4 登录接口限流:`internal/api/ratelimit.go` 滑动窗口(IP+用户名,1 分钟内失败 5 次锁 1 分钟),
      登录成功清零;实测连续 6 次错密码第 6 次返回 429
- [x] A5 Makefile 增加 `test`(`go test ./...`)与 `check`(test + vet + 前端构建)目标
- [x] A6 严格生产模式:`HAPROXY_WEBUI_STRICT=true` 时,JWT secret 为默认值或未设置独立加密密钥
      拒绝启动;实测 STRICT=true 且默认 secret 时启动即 fatal

## B. 前端清理

- [x] B1 仪表盘 `cacheUser()` 删除,统一使用 `lib/api.getCachedUser()`

## C. 小功能

- [x] C1 实例批量健康探测:`GET /api/health/instances`(登录即可)并发探测全部启用实例;
      仪表盘「HAProxy 实例」卡片改为显示每个实例的在线/不可达状态,30s 自动刷新
      (实测返回 rainyun-rcs ok=true + v3.4.3)
- [x] C2 补充基础单元测试:cryptoutil(往返/历史明文/空串/错密钥)、登录限流器(锁定/重置/隔离),
      `go test ./...` 全部通过

## D. 待用户配合(不在本轮代码范围)

- [ ] D1 dataplaneapi root 运行加固:节点上创建专用系统用户,配置 polkit/sudo 白名单仅允许
      其执行 `systemctl reload haproxy` / `restart haproxy`,随后把 dataplaneapi.service 的
      `User=root` 改为该用户并实机回归。需要用户在雨云节点配合执行命令。
