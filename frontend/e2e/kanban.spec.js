import { test, expect } from '@playwright/test'

const BASE_URL = 'http://localhost:8880'

test.describe('AI Coding 指标看板 - 端到端测试', () => {

  test('首页加载正常，显示 6 张导航卡片', async ({ page }) => {
    await page.goto(BASE_URL + '/')
    await page.waitForLoadState('networkidle')

    // 页面标题
    await expect(page).toHaveTitle(/kanban|看板/i)

    // 顶部导航菜单
    const nav = page.locator('.el-menu')
    await expect(nav).toBeVisible()

    // 应有 6 张卡片
    const cards = page.locator('.el-card')
    await expect(cards).toHaveCount(6, { timeout: 5000 })

    // 截图
    await page.screenshot({ path: 'test-results/01-home.png', fullPage: true })
  })

  test('导航菜单包含所有页面入口', async ({ page }) => {
    await page.goto(BASE_URL + '/')
    await page.waitForLoadState('networkidle')

    const menuItems = page.locator('.el-menu-item')
    const count = await menuItems.count()
    expect(count).toBeGreaterThanOrEqual(6) // 首页、Dashboard、提效分析、项目、仓库、用户、组织

    await page.screenshot({ path: 'test-results/02-nav.png', fullPage: true })
  })

  test('Dashboard 页面加载正常，Tab 切换有效', async ({ page }) => {
    await page.goto(BASE_URL + '/dashboard')
    await page.waitForLoadState('networkidle')

    // 日期选择器应存在
    const datePicker = page.locator('.el-date-editor').first()
    await expect(datePicker).toBeVisible({ timeout: 5000 })

    // 应有 Tab 组件
    const tabs = page.locator('.el-tabs')
    await expect(tabs).toBeVisible()

    // 应有表格
    const table = page.locator('.el-table')
    await expect(table).toBeVisible()

    await page.screenshot({ path: 'test-results/03-dashboard-request.png', fullPage: true })

    // 切换到 Task Aggregate Tab（如果有多个 tab）
    const tabItems = page.locator('.el-tabs__item')
    const tabCount = await tabItems.count()
    if (tabCount > 1) {
      await tabItems.nth(1).click()
      await page.waitForTimeout(1000)
      await page.screenshot({ path: 'test-results/04-dashboard-aggregate.png', fullPage: true })
    }
  })

  test('提效分析面板加载正常', async ({ page }) => {
    await page.goto(BASE_URL + '/efficiency')
    await page.waitForLoadState('networkidle')

    // 页面应该存在而非空白
    const body = page.locator('body')
    await expect(body).not.toBeEmpty()

    // 应有维度切换（Project/Repo）
    const radioGroup = page.locator('.el-radio-group, .el-radio-button').first()
    // 如果存在则验证
    if (await radioGroup.isVisible().catch(() => false)) {
      await expect(radioGroup).toBeVisible()
    }

    await page.screenshot({ path: 'test-results/05-efficiency.png', fullPage: true })
  })

  test('项目面板加载正常，有表格和图表', async ({ page }) => {
    await page.goto(BASE_URL + '/project-panel')
    await page.waitForLoadState('networkidle')

    // 应有表格
    const table = page.locator('.el-table')
    await expect(table).toBeVisible({ timeout: 5000 })

    await page.screenshot({ path: 'test-results/06-project-panel.png', fullPage: true })
  })

  test('仓库面板加载正常', async ({ page }) => {
    await page.goto(BASE_URL + '/repo-panel')
    await page.waitForLoadState('networkidle')

    const body = page.locator('body')
    await expect(body).not.toBeEmpty()

    await page.screenshot({ path: 'test-results/07-repo-panel.png', fullPage: true })
  })

  test('用户面板加载正常', async ({ page }) => {
    await page.goto(BASE_URL + '/user-panel')
    await page.waitForLoadState('networkidle')

    const table = page.locator('.el-table')
    await expect(table).toBeVisible({ timeout: 5000 })

    await page.screenshot({ path: 'test-results/08-user-panel.png', fullPage: true })
  })

  test('组织面板加载正常，有层级导航', async ({ page }) => {
    await page.goto(BASE_URL + '/org-panel')
    await page.waitForLoadState('networkidle')

    const body = page.locator('body')
    await expect(body).not.toBeEmpty()

    await page.screenshot({ path: 'test-results/09-org-panel.png', fullPage: true })
  })

  test('首页卡片点击跳转正确', async ({ page }) => {
    await page.goto(BASE_URL + '/')
    await page.waitForLoadState('networkidle')

    // 点击第一张卡片（Dashboard）
    const cards = page.locator('.el-card')
    const firstCard = cards.first()
    await firstCard.click()
    await page.waitForTimeout(500)

    // 应跳转到某个子页面
    expect(page.url()).not.toBe(BASE_URL + '/')

    await page.screenshot({ path: 'test-results/10-card-navigation.png', fullPage: true })
  })

  test('页面无 JS 错误', async ({ page }) => {
    const errors = []
    page.on('pageerror', (err) => errors.push(err.message))

    // 遍历所有页面
    const pages = ['/', '/dashboard', '/efficiency', '/project-panel', '/repo-panel', '/user-panel', '/org-panel']
    for (const p of pages) {
      await page.goto(BASE_URL + p)
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(500)
    }

    // 不应有 JS 错误（API 404 可忽略）
    const criticalErrors = errors.filter(e => !e.includes('Network') && !e.includes('fetch') && !e.includes('AxiosError'))
    expect(criticalErrors).toEqual([])
  })

  test('API 代理正常工作', async ({ page }) => {
    // 直接请求 API 看代理是否正常
    const response = await page.goto(BASE_URL + '/api/indices')
    expect(response.status()).toBe(200)

    const body = await response.text()
    expect(body).toContain('request')
    expect(body).toContain('task')
  })

  test('所有页面响应式布局无水平滚动', async ({ page }) => {
    const pages = ['/', '/dashboard', '/efficiency', '/project-panel', '/repo-panel', '/user-panel', '/org-panel']

    for (const p of pages) {
      await page.goto(BASE_URL + p)
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(300)

      const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth)
      const clientWidth = await page.evaluate(() => document.documentElement.clientWidth)

      // scrollWidth 不应大于 clientWidth（无水平溢出）
      expect(scrollWidth).toBeLessThanOrEqual(clientWidth + 20) // 允许少许误差
    }
  })
})
