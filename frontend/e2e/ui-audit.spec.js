// @ts-check
import { test, expect } from '@playwright/test'

const BASE_URL = 'http://localhost:8880'

// 逐页面审查 UI 内容，输出详细信息用于改进分析
test.describe('UI 逐页审查', () => {

  test('审查1: Dashboard 首页', async ({ page }) => {
    await page.goto(BASE_URL + '/')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 提取所有指标卡片的文字
    const cards = page.locator('.el-card')
    const cardCount = await cards.count()
    console.log(`\n=== Dashboard 首页 ===`)
    console.log(`卡片数: ${cardCount}`)
    for (let i = 0; i < cardCount; i++) {
      const text = (await cards.nth(i).textContent())?.trim().replace(/\s+/g, ' ')
      console.log(`  卡片${i}: ${text}`)
    }

    // 检查数值是否为0
    const bodyText = await page.locator('body').textContent()
    const zeroMetrics = bodyText.match(/\b0\b/g)
    console.log(`页面中出现"0"的次数: ${zeroMetrics?.length || 0}`)

    // 导航菜单
    const menuItems = page.locator('.el-menu-item, .el-sub-menu__title')
    const menuCount = await menuItems.count()
    const menuTexts = []
    for (let i = 0; i < menuCount; i++) {
      menuTexts.push((await menuItems.nth(i).textContent())?.trim())
    }
    console.log(`导航菜单: ${menuTexts.join(' | ')}`)
  })

  test('审查2: Project 视图 - 列表', async ({ page }) => {
    await page.goto(BASE_URL + '/project-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    console.log(`\n=== Project 视图 ===`)

    // 筛选区元素
    const filterRow = page.locator('.kb-filter-row, .kb-filter-card')
    console.log(`筛选区: ${await filterRow.count()} 个`)

    // 搜索框
    const searchInput = page.locator('input[placeholder*="搜索"]')
    console.log(`搜索框: ${await searchInput.count()} 个`)

    // 表格列头
    const headers = page.locator('.el-table__header-wrapper th .cell')
    const headerCount = await headers.count()
    const headerTexts = []
    for (let i = 0; i < headerCount; i++) {
      const t = (await headers.nth(i).textContent())?.trim()
      if (t) headerTexts.push(t)
    }
    console.log(`表头: ${headerTexts.join(' | ')}`)

    // 表格行数和第一行数据
    const rows = page.locator('.el-table__body-wrapper .el-table__row')
    const rowCount = await rows.count()
    console.log(`数据行数: ${rowCount}`)
    if (rowCount > 0) {
      const firstRowCells = rows.first().locator('td .cell')
      const cellCount = await firstRowCells.count()
      const cellTexts = []
      for (let i = 0; i < cellCount; i++) {
        cellTexts.push((await firstRowCells.nth(i).textContent())?.trim())
      }
      console.log(`首行数据: ${cellTexts.join(' | ')}`)
    }

    // 图表区
    const charts = page.locator('.kb-chart-container, canvas')
    console.log(`图表数: ${await charts.count()}`)

    // 分页
    const pagination = page.locator('.el-pagination')
    console.log(`分页: ${await pagination.count()} 个`)
  })

  test('审查3: Project 视图 - 详情页', async ({ page }) => {
    // 通过 API 获取第一个 project_id
    const res = await page.request.get(`${BASE_URL}/api/v2/projects?startDate=20260101&endDate=20261231`)
    const data = await res.json()
    const projectId = data.data[0].project_id || data.data[0].repo_id

    await page.goto(BASE_URL + '/project/' + encodeURIComponent(projectId))
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    console.log(`\n=== Project 详情页 (${projectId}) ===`)

    // el-descriptions 元信息
    const descriptions = page.locator('.el-descriptions')
    console.log(`el-descriptions: ${await descriptions.count()}`)

    // 表格（参与者、Task、Commit等）
    const tables = page.locator('.el-table')
    const tableCount = await tables.count()
    console.log(`表格总数: ${tableCount}`)

    // 人工调整按钮
    const manualBtn = page.locator('button, .el-button').filter({ hasText: /人工调整/ })
    console.log(`人工调整按钮: ${await manualBtn.count()}`)

    // 链接(跨页面导航)
    const links = page.locator('.el-link')
    console.log(`可点击链接: ${await links.count()}`)
  })

  test('审查4: User 视图 - 列表', async ({ page }) => {
    await page.goto(BASE_URL + '/user-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    console.log(`\n=== User 视图 ===`)

    const headers = page.locator('.el-table__header-wrapper th .cell')
    const headerCount = await headers.count()
    const headerTexts = []
    for (let i = 0; i < headerCount; i++) {
      const t = (await headers.nth(i).textContent())?.trim()
      if (t) headerTexts.push(t)
    }
    console.log(`表头: ${headerTexts.join(' | ')}`)

    const rows = page.locator('.el-table__body-wrapper .el-table__row')
    console.log(`数据行数: ${await rows.count()}`)

    if (await rows.count() > 0) {
      const firstRowCells = rows.first().locator('td .cell')
      const cellCount = await firstRowCells.count()
      const cellTexts = []
      for (let i = 0; i < cellCount; i++) {
        cellTexts.push((await firstRowCells.nth(i).textContent())?.trim())
      }
      console.log(`首行数据: ${cellTexts.join(' | ')}`)
    }
  })

  test('审查5: User 视图 - 详情页', async ({ page }) => {
    // 通过 API 获取第一个 user_id
    const res = await page.request.get(`${BASE_URL}/api/v2/users?startDate=20260101&endDate=20261231`)
    const data = await res.json()
    const userId = data.data[0].user_id

    await page.goto(BASE_URL + '/user/' + userId)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    console.log(`\n=== User 详情页 (${userId}) ===`)

    const metricCards = page.locator('.kb-metric-card')
    const metricCount = await metricCards.count()
    console.log(`指标卡片: ${metricCount}`)
    for (let i = 0; i < metricCount; i++) {
      const text = (await metricCards.nth(i).textContent())?.trim().replace(/\s+/g, ' ')
      console.log(`  ${text}`)
    }

    const tables = page.locator('.el-table')
    console.log(`表格数: ${await tables.count()}`)

    const links = page.locator('.el-link')
    console.log(`跨页面链接: ${await links.count()}`)

    const charts = page.locator('.kb-chart-container, canvas')
    console.log(`图表数: ${await charts.count()}`)
  })

  test('审查6: Org 视图', async ({ page }) => {
    await page.goto(BASE_URL + '/org-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    console.log(`\n=== Org 视图 ===`)

    const breadcrumb = page.locator('.el-breadcrumb__item')
    console.log(`面包屑层级: ${await breadcrumb.count()}`)

    const rows = page.locator('.el-table__body-wrapper .el-table__row')
    console.log(`组织行数: ${await rows.count()}`)

    if (await rows.count() > 0) {
      const firstRowCells = rows.first().locator('td .cell')
      const cellCount = await firstRowCells.count()
      const cellTexts = []
      for (let i = 0; i < cellCount; i++) {
        cellTexts.push((await firstRowCells.nth(i).textContent())?.trim())
      }
      console.log(`首行: ${cellTexts.join(' | ')}`)
    }
  })

  test('审查7: Task 详情页', async ({ page }) => {
    const res = await page.request.get(`${BASE_URL}/api/v2/tasks?startDate=20260101&endDate=20261231&page=1&pageSize=1`)
    const data = await res.json()
    const taskId = data.data[0].task_id

    await page.goto(BASE_URL + '/task/' + taskId)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    console.log(`\n=== Task 详情页 (${taskId}) ===`)

    // 元信息
    const descriptions = page.locator('.el-descriptions__cell')
    const descCount = await descriptions.count()
    console.log(`元信息字段: ${descCount}`)
    for (let i = 0; i < Math.min(descCount, 14); i++) {
      const text = (await descriptions.nth(i).textContent())?.trim().replace(/\s+/g, ' ')
      console.log(`  ${text}`)
    }

    // 对话历史
    const timelineItems = page.locator('.el-timeline-item')
    console.log(`对话条目: ${await timelineItems.count()}`)

    // 统计卡片
    const metricCards = page.locator('.kb-metric-card')
    console.log(`统计卡片: ${await metricCards.count()}`)
    for (let i = 0; i < await metricCards.count(); i++) {
      const text = (await metricCards.nth(i).textContent())?.trim().replace(/\s+/g, ' ')
      console.log(`  ${text}`)
    }

    // 链接
    const links = page.locator('.el-link')
    console.log(`跨页面链接: ${await links.count()}`)
  })

  test('审查8: 人工调整对话框', async ({ page }) => {
    // 通过 API 获取第一个 project_id
    const res = await page.request.get(`${BASE_URL}/api/v2/projects?startDate=20260101&endDate=20261231`)
    const data = await res.json()
    const projectId = data.data[0].project_id || data.data[0].repo_id

    await page.goto(BASE_URL + '/project/' + encodeURIComponent(projectId))
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    const adjustBtn = page.locator('button, .el-button').filter({ hasText: /人工调整/ })
    if (await adjustBtn.count() > 0) {
      await adjustBtn.first().click()
      await page.waitForTimeout(1000)

      console.log(`\n=== 人工调整对话框 ===`)
      const dialog = page.locator('.el-dialog')
      const dialogText = (await dialog.textContent())?.trim().replace(/\s+/g, ' ')
      console.log(`内容: ${dialogText?.substring(0, 300)}`)

      const formItems = dialog.locator('.el-form-item')
      console.log(`表单项: ${await formItems.count()}`)
    }
  })

  test('审查9: API 响应数据质量', async ({ page }) => {
    console.log(`\n=== API 数据质量检查 ===`)

    // Dashboard
    const dashRes = await page.request.get(`${BASE_URL}/api/v2/dashboard/summary`)
    const dash = await dashRes.json()
    console.log(`Dashboard: tasks=${dash.total_tasks} users=${dash.total_users} commits=${dash.total_commits} projects=${dash.total_projects}`)
    console.log(`  cost=${dash.total_cost} tokens=${dash.total_tokens} diff_lines=${dash.total_diff_lines} ai_days=${dash.total_ai_estimated_days}`)

    // Projects - 检查关联完整性
    const projRes = await page.request.get(`${BASE_URL}/api/v2/projects?startDate=20260101&endDate=20261231`)
    const projs = await projRes.json()
    console.log(`\nProjects (${projs.total}):`)
    for (const p of projs.data || []) {
      const taskIds = typeof p.task_ids === 'string' ? JSON.parse(p.task_ids || '[]') : (p.task_ids || [])
      const commitIds = typeof p.commit_ids === 'string' ? JSON.parse(p.commit_ids || '[]') : (p.commit_ids || [])
      console.log(`  ${p.repo_id}: tasks=${taskIds.length} commits=${commitIds.length} cost=${p.cost || p.total_cost || 0}`)
    }

    // Users 前5个
    const userRes = await page.request.get(`${BASE_URL}/api/v2/users?startDate=20260101&endDate=20261231&page=1&pageSize=5`)
    const users = await userRes.json()
    console.log(`\nUsers (${users.total}), 前5:`)
    for (const u of (users.data || []).slice(0, 5)) {
      console.log(`  ${u.user_name}: tasks=${u.task_count} commits=${u.commit_count || 0} projects=${u.project_count || 0} cost=${u.total_cost}`)
    }

    // Org
    const orgRes = await page.request.get(`${BASE_URL}/api/v2/orgs?level=org1&startDate=20260101&endDate=20261231`)
    const orgs = await orgRes.json()
    console.log(`\nOrgs (org1): ${orgs.data?.length || 0} entries`)
    for (const o of orgs.data || []) {
      console.log(`  ${o.org_name}: users=${o.user_count} tasks=${o.task_count} commits=${o.commit_count}`)
    }

    // 关联质量: project有多少真正有task/commit关联
    let withTasks = 0, withCommits = 0, withBoth = 0
    for (const p of projs.data || []) {
      const tids = typeof p.task_ids === 'string' ? JSON.parse(p.task_ids || '[]') : (p.task_ids || [])
      const cids = typeof p.commit_ids === 'string' ? JSON.parse(p.commit_ids || '[]') : (p.commit_ids || [])
      if (tids.length > 0) withTasks++
      if (cids.length > 0) withCommits++
      if (tids.length > 0 && cids.length > 0) withBoth++
    }
    console.log(`\n关联质量: ${projs.total}个project中, 有task关联=${withTasks}, 有commit关联=${withCommits}, 双向关联=${withBoth}`)
  })
})
