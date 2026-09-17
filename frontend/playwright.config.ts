import { defineConfig } from '@playwright/test'

// E2E 冒烟:local-e2e 容器(真实 HAProxy + dataplaneapi)+ 全新临时 DB 的后端 + vite dev。
// 三者由 webServer/globalSetup 自动编排;运行:`cd frontend && bunx playwright test`(或 make e2e)。
// 注意:8080/5173 端口需空闲(e2e 用独立 DB,不复用开发实例)。
const E2E_DB = '/tmp/haproxy-webui-e2e.db'

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  workers: 1,
  fullyParallel: false,
  reporter: 'list',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'retain-on-failure',
  },
  globalSetup: './e2e/global-setup.ts',
  webServer: [
    {
      // 独立临时 DB(每次运行前清空),种子账号 admin/admin123
      command: `sh -c 'rm -f ${E2E_DB} && HAPROXY_WEBUI_DB=${E2E_DB} go run ./cmd/server'`,
      url: 'http://localhost:8080/api/health',
      cwd: '../backend',
      reuseExistingServer: false,
      timeout: 120_000,
    },
    {
      command: 'bun run dev -- --port 5173 --strictPort',
      url: 'http://localhost:5173',
      reuseExistingServer: false,
      timeout: 60_000,
    },
  ],
})
