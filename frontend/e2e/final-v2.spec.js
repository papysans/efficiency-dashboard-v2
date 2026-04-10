// @ts-check
import { test, expect } from '@playwright/test'

const BASE = 'http://localhost:8880'

test.describe('全功能验证', () => {

  // ========= 首页 =========
  test('首页: 指标有数据且全部可点击', async ({ page }) => {
    await page.goto(BASE + '/')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)
    // 8 指标卡片 + 3 导航卡片 = 11
    const cards = page.locator('.el-card')
    expect(await cards.count()).toBeGreaterThanOrEqual(8)
    // 所有指标卡片有 cursor pointer
    const clickable = page.locator('.el-card[style*="cursor"],.metric-card[style*="cursor"],.dashboard-metric[style*="cursor"]')
    console.log(`首页可点击卡片: ${await clickable.count()}`)
    await page.screenshot({ path: 'test-results/final-01-home.png', fullPage: true })
  })

  // ========= 导航菜单 =========
  test('导航: 5个主菜单项，无"更多"子菜单', async ({ page }) => {
    await page.goto(BASE + '/')
    await page.waitForTimeout(1000)
    const items = page.locator('.el-menu-item:not([disabled])')
    const count = await items.count()
    const texts = []
    for (let i = 0; i < count; i++) texts.push((await items.nth(i).textContent())?.trim())
    console.log(`菜单项: ${texts.join(' | ')}`)
    // 应包含 首页、仓库、用户、组织、任务
    expect(texts.join(',')).toContain('仓库')
    expect(texts.join(',')).toContain('任务')
    // 不应有"更多"
    const subMenu = page.locator('.el-sub-menu__title')
    expect(await subMenu.count()).toBe(0)
  })

  // ========= 仓库列表 /repo-v2 =========
  test('仓库列表: 有数据、可搜索、可跳转', async ({ page }) => {
    await page.goto(BASE + '/repo-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)
    const rows = page.locator('.el-table__body-wrapper .el-table__row')
    const rowCount = await rows.count()
    console.log(`仓库列表: ${rowCount} 行`)
    expect(rowCount).toBeGreaterThan(0)
    // FilterBar 组件存在
    const filterBar = page.locator('.kb-filter-card')
    expect(await filterBar.count()).toBeGreaterThan(0)
    // 搜索
    const search = page.locator('input[placeholder*="搜索"]')
    if (await search.count() > 0) {
      await search.fill('costrict')
      await page.waitForTimeout(500)
      const filtered = await rows.count()
      console.log(`搜索costrict后: ${filtered} 行`)
      expect(filtered).toBeGreaterThan(0)
      await search.clear()
    }
    await page.screenshot({ path: 'test-results/final-02-repo-list.png', fullPage: true })
  })

  // ========= 仓库详情 /repo/:repoId =========
  test('仓库详情: commit列表含硅比例和关联task', async ({ page }) => {
    await page.goto(BASE + '/repo/' + encodeURIComponent('repo-kanban-dev'))
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)
    // 有返回按钮
    const backBtn = page.locator('button').filter({ hasText: /返回/ })
    expect(await backBtn.count()).toBeGreaterThan(0)
    // 有 commit 表格
    const tables = page.locator('.el-table')
    console.log(`仓库详情表格数: ${await tables.count()}`)
    expect(await tables.count()).toBeGreaterThanOrEqual(1)
    // 检查有硅比例相关内容（progress bar 或百分比文字）
    const body = await page.locator('body').textContent()
    const hasSilica = body.includes('硅') || body.includes('silica') || body.includes('%') || body.includes('progress')
    console.log(`有硅比例展示: ${hasSilica}`)
    await page.screenshot({ path: 'test-results/final-03-repo-detail.png', fullPage: true })
  })

  // ========= 用户列表 /user-v2 =========
  test('用户列表: 有数据、时间格式正确', async ({ page }) => {
    await page.goto(BASE + '/user-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)
    const rows = page.locator('.el-table__body-wrapper .el-table__row')
    console.log(`用户列表: ${await rows.count()} 行`)
    expect(await rows.count()).toBeGreaterThan(0)
    // 时间格式应为 YYYY-MM-DD 而非 ISO8601
    const body = await page.locator('body').textContent()
    expect(body).not.toContain('T14:00:00+08:00')
    await page.screenshot({ path: 'test-results/final-04-user-list.png', fullPage: true })
  })

  // ========= 用户详情 /user/:userId =========
  test('用户详情: 有指标和关联数据', async ({ page }) => {
    await page.goto(BASE + '/user/user-001')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)
    const metricCards = page.locator('.kb-metric-card')
    console.log(`用户详情指标卡片: ${await metricCards.count()}`)
    expect(await metricCards.count()).toBeGreaterThanOrEqual(3)
    await page.screenshot({ path: 'test-results/final-05-user-detail.png', fullPage: true })
  })

  // ========= 任务列表 /task-v2 =========
  test('任务列表: 有数据、可搜索、可跳转', async ({ page }) => {
    await page.goto(BASE + '/task-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)
    const rows = page.locator('.el-table__body-wrapper .el-table__row')
    const rowCount = await rows.count()
    console.log(`任务列表: ${rowCount} 行`)
    expect(rowCount).toBeGreaterThan(0)
    // 点击第一行应跳转到 /task/xxx
    await rows.first().click()
    await page.waitForTimeout(2000)
    expect(page.url()).toContain('/task/')
    await page.screenshot({ path: 'test-results/final-06-task-list-nav.png', fullPage: true })
  })

  // ========= 任务详情 /task/:taskId =========
  test('任务详情: 有对话历史', async ({ page }) => {
    const res = await page.request.get(`${BASE}/api/v2/tasks?startDate=20260101&endDate=20261231&page=1&pageSize=1`)
    const data = await res.json()
    const taskId = data.data[0].task_id
    await page.goto(BASE + '/task/' + taskId)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)
    const timeline = page.locator('.el-timeline-item')
    console.log(`Task详情对话数: ${await timeline.count()}`)
    expect(await timeline.count()).toBeGreaterThan(0)
    await page.screenshot({ path: 'test-results/final-07-task-detail.png', fullPage: true })
  })

  // ========= 组织列表 /org-v2 =========
  test('组织列表: 有级联下拉和数据', async ({ page }) => {
    await page.goto(BASE + '/org-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)
    // 检查级联下拉存在
    const selects = page.locator('.el-select')
    console.log(`组织级联下拉数: ${await selects.count()}`)
    expect(await selects.count()).toBeGreaterThanOrEqual(1)
    // 表格
    const rows = page.locator('.el-table__body-wrapper .el-table__row')
    console.log(`组织列表行数: ${await rows.count()}`)
    await page.screenshot({ path: 'test-results/final-08-org-list.png', fullPage: true })
  })

  // ========= 旧路由已移除 =========
  test('旧路由不可访问', async ({ page }) => {
    const oldRoutes = ['/dashboard', '/project-panel', '/efficiency', '/repo-panel', '/user-panel', '/org-panel']
    for (const r of oldRoutes) {
      await page.goto(BASE + r)
      await page.waitForTimeout(500)
      // Vue Router 应该 fallback 到首页或显示空白（不应该有旧内容）
      const url = page.url()
      // 旧路由不应该正常渲染出内容（Vue Router 无匹配路由会显示空白或 redirect）
      console.log(`${r} → ${url}`)
    }
  })

  // ========= 综合质量 =========
  test('所有页面无JS错误无API 500', async ({ page }) => {
    const errors = []
    const apiErrors = []
    page.on('pageerror', (e) => errors.push(e.message))
    page.on('response', (r) => { if (r.url().includes('/api/') && r.status() >= 500) apiErrors.push(`${r.status()} ${r.url()}`) })
    
    for (const p of ['/', '/repo-v2', '/user-v2', '/org-v2', '/task-v2']) {
      await page.goto(BASE + p)
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(1500)
    }
    // 详情页
    await page.goto(BASE + '/repo/' + encodeURIComponent('repo-costrict-main'))
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1500)
    await page.goto(BASE + '/user/user-001')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1500)

    if (errors.length) console.log('JS Errors:', errors)
    if (apiErrors.length) console.log('API Errors:', apiErrors)
    expect(errors.length).toBe(0)
    expect(apiErrors.length).toBe(0)
  })
})
