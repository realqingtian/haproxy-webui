import { test, expect } from '@playwright/test'

// 核心链路冒烟:登录 → 实例增删 → 配置读取 → 暂存提交 → 运行时上下线 → 模板创建 → 回滚。
// 环境由 playwright.config.ts 编排:全新后端(admin/admin123)+ vite dev + local-e2e 容器。
// 容器配置在运行间持久,对象名带时间戳后缀避免冲突;实例在用例末尾删除,容器配置经回滚恢复。

const UNIQUE = String(Date.now() % 1_000_000)
const STG_BACKEND = `e2e_stg_${UNIQUE}`
const TPL_FRONTEND = `e2e_tpl_${UNIQUE}`
const TPL_BACKEND = `e2e_tpb_${UNIQUE}`

test('核心链路冒烟', async ({ page }) => {
  test.setTimeout(90_000)

  // ---- 登录 ----
  await page.goto('/login')
  await page.getByLabel('用户名').fill('admin')
  await page.getByLabel('密码').fill('admin123')
  await page.getByRole('button', { name: '登录' }).click()
  await expect(page).toHaveURL('/')

  // ---- 实例新增(指向 local-e2e 容器)----
  await page.getByRole('button', { name: '实例管理' }).click()
  await expect(page.getByRole('heading', { name: 'HAProxy 实例' })).toBeVisible()
  await page.getByRole('button', { name: '添加实例' }).click()
  await page.getByLabel('名称').fill('e2e-node')
  await page.getByLabel('地址', { exact: true }).fill('http://localhost:5555')
  await page.getByLabel('用户名').fill('dataplaneapi')
  await page.getByLabel('密码').fill('demosecret')
  await page.getByRole('button', { name: '保存' }).click()
  await expect(page.getByText('e2e-node')).toBeVisible()

  // 连通性测试
  const row = page.getByRole('row').filter({ hasText: 'e2e-node' })
  await row.getByRole('button', { name: '测试' }).click()
  await expect(page.getByText(/连接成功:dataplaneapi v3/)).toBeVisible({ timeout: 10_000 })

  // ---- 配置读取(单实例:侧边栏「配置管理」直跳)----
  await page.getByRole('button', { name: '配置管理' }).click()
  await expect(page.getByRole('heading', { name: /e2e-node · 配置管理/ })).toBeVisible()
  await expect(page.getByText('demo_app')).toBeVisible()

  // ---- 暂存提交:新建 backend → 待提交 → 一次 reload ----
  await page.getByRole('button', { name: '新建 backend' }).click()
  await page.getByRole('textbox').fill(STG_BACKEND)
  await page.getByRole('dialog').getByRole('button', { name: '保存' }).click()
  await expect(page.getByRole('button', { name: `待提交(1)` })).toBeVisible()

  await page.getByRole('button', { name: /待提交\(1\)/ }).click()
  await expect(page.getByRole('dialog').getByText(`创建 backend ${STG_BACKEND}`)).toBeVisible()
  await page.getByRole('dialog').getByRole('button', { name: /提交\(1 条/ }).click()
  await expect(page.getByText(/配置已保存并触发 reload/)).toBeVisible({ timeout: 15_000 })
  await expect(page.getByRole('button', { name: '待提交', exact: true })).toBeDisabled()

  // ---- 运行时上下线(demo_app 初始 server,即时生效不写配置)----
  const serverRow = page.getByRole('row').filter({ hasText: 's1' }).first()
  await serverRow.getByRole('combobox').click()
  await page.getByRole('option', { name: '维护' }).click()
  await expect(page.getByText(/已切换为「维护」/)).toBeVisible({ timeout: 10_000 })

  // ---- 模板创建(HTTP,单事务多操作)----
  await page.getByRole('button', { name: '从模板创建' }).click()
  await page.getByPlaceholder('web_front').fill(TPL_FRONTEND)
  await page.getByPlaceholder('80').fill('48765')
  await page.getByPlaceholder('app_pool').fill(TPL_BACKEND)
  // 第一行服务器(名称已预填 s1,填地址与端口)
  await page.getByPlaceholder('服务器地址').first().fill('127.0.0.1')
  await page.getByPlaceholder('端口').first().fill('8081')
  await page.getByRole('button', { name: '生成并保存' }).click()
  // 模板 = backend + server + frontend + bind 共 4 条操作
  await expect(page.getByRole('button', { name: `待提交(4)` })).toBeVisible()
  await page.getByRole('button', { name: /待提交\(4\)/ }).click()
  await page.getByRole('dialog').getByRole('button', { name: /提交\(4 条/ }).click()
  await expect(page.getByText(/配置已保存并触发 reload/)).toBeVisible({ timeout: 20_000 })
  await expect(page.getByText(`backend ${TPL_BACKEND}`)).toBeVisible({ timeout: 10_000 })

  // ---- 回滚到上一快照(模板生成前),模板对象随之消失 ----
  await page.getByRole('tab', { name: '版本历史' }).click()
  const rollbackBtns = page.getByRole('button', { name: '回滚' })
  await expect(rollbackBtns.first()).toBeVisible()
  await rollbackBtns.first().click()
  await page.getByRole('button', { name: '确认回滚' }).click()
  await expect(page.getByText(/回滚到快照 #\d+/).first()).toBeVisible({ timeout: 15_000 })
  await page.getByRole('tab', { name: '后端与服务器' }).click()
  await expect(page.getByText(`backend ${TPL_BACKEND}`)).toHaveCount(0, { timeout: 10_000 })

  // ---- 实例删除 ----
  await page.getByRole('button', { name: '实例管理' }).click()
  const delRow = page.getByRole('row').filter({ hasText: 'e2e-node' })
  await delRow.locator('button').last().click() // 行尾图标删除按钮
  await page.getByRole('dialog').getByRole('button', { name: '删除' }).click()
  await expect(page.getByText('实例已删除')).toBeVisible()
  await expect(page.getByRole('row').filter({ hasText: 'e2e-node' })).toHaveCount(0)
})

// v0.6:告警渠道 CRUD + 巡检设置保存(admin 专属页)
test('告警与巡检冒烟', async ({ page }) => {
  const CHANNEL = `e2e_hook_${UNIQUE}`

  await page.goto('/login')
  await page.getByLabel('用户名').fill('admin')
  await page.getByLabel('密码').fill('admin123')
  await page.getByRole('button', { name: '登录' }).click()
  await expect(page).toHaveURL('/')

  // ---- 渠道新增 ----
  await page.getByRole('button', { name: '告警与巡检' }).click()
  await expect(page.getByRole('heading', { name: '告警与巡检' })).toBeVisible()
  await page.getByRole('button', { name: '添加渠道' }).click()
  await page.getByLabel('名称').fill(CHANNEL)
  await page.getByLabel('Webhook 地址').fill('http://localhost:1/hook') // 不可达地址,仅测 CRUD
  await page.getByRole('dialog').getByRole('button', { name: '保存' }).click()
  await expect(page.getByText('渠道已创建')).toBeVisible()

  // ---- 测试发送(指向不可达地址,应报发送失败而非无响应)----
  const chRow = page.getByRole('row').filter({ hasText: CHANNEL })
  await chRow.getByRole('button', { name: `测试 ${CHANNEL}` }).click()
  await expect(page.getByText(/发送失败|测试消息已发送/).first()).toBeVisible({ timeout: 15_000 })

  // ---- 巡检设置保存(快照巡检设为 0 = 关闭,不影响其他用例)----
  await page.getByLabel('快照巡检周期(分钟,0 = 关闭)').fill('0')
  await page.getByLabel('健康探测周期(秒,≥10)').fill('30')
  await page.getByRole('button', { name: '保存设置' }).click()
  await expect(page.getByText(/巡检设置已保存/)).toBeVisible()

  // ---- 渠道删除 ----
  await chRow.getByRole('button', { name: `删除 ${CHANNEL}` }).click()
  await page.getByRole('dialog').getByRole('button', { name: '删除' }).click()
  await expect(page.getByText('渠道已删除')).toBeVisible()
  await expect(page.getByRole('row').filter({ hasText: CHANNEL })).toHaveCount(0)
})
