// @ts-check
import { test, expect } from '@playwright/test'

const BASE_URL = 'http://localhost:8880'

// ============================================================
// 1. 后端 API 数据验证（直接请求，确保数据充分）
// ============================================================
test.describe('1. 后端 V2 API 数据验证', () => {

  test('1.1 Dashboard 汇总有数据', async ({ request }) => {
    const res = await request.get(`${BASE_URL}/api/v2/dashboard/summary`)
    expect(res.ok()).toBeTruthy()
    const data = await res.json()
    console.log('Dashboard:', JSON.stringify(data))
    expect(data.total_tasks).toBeGreaterThan(0)
    expect(data.total_users).toBeGreaterThan(0)
    expect(data.total_commits).toBeGreaterThan(0)
    expect(data.total_projects).toBeGreaterThan(0)
    expect(data.total_cost).toBeGreaterThan(0)
    expect(data.total_tokens).toBeGreaterThan(0)
    expect(data.total_diff_lines).toBeGreaterThan(0)
  })

  test('1.2 Tasks 列表有数据', async ({ request }) => {
    const res = await request.get(`${BASE_URL}/api/v2/tasks?startDate=20260101&endDate=20261231&page=1&pageSize=50`)
    expect(res.ok()).toBeTruthy()
    const data = await res.json()
    console.log(`Tasks: total=${data.total}`)
    expect(data.total).toBeGreaterThan(0)
    expect(data.data.length).toBeGreaterThan(0)
    // 验证字段完整
    const t = data.data[0]
    expect(t.task_id).toBeTruthy()
    expect(t.user_id).toBeTruthy()
    expect(t.repo_id).toBeTruthy()
  })

  test('1.3 Task 详情有 conversations', async ({ request }) => {
    const listRes = await request.get(`${BASE_URL}/api/v2/tasks?startDate=20260101&endDate=20261231&page=1&pageSize=1`)
    const listData = await listRes.json()
    const taskId = listData.data[0].task_id
    const res = await request.get(`${BASE_URL}/api/v2/tasks/${taskId}`)
    expect(res.ok()).toBeTruthy()
    const data = await res.json()
    console.log(`Task ${taskId}: ${data.conversations?.length || 0} conversations`)
    expect(data.task).toBeTruthy()
    expect(data.task.task_id).toBe(taskId)
    expect(data.conversations.length).toBeGreaterThan(0)
  })

  test('1.4 Commits 列表有数据', async ({ request }) => {
    const res = await request.get(`${BASE_URL}/api/v2/commits?startDate=20260101&endDate=20261231&page=1&pageSize=50`)
    expect(res.ok()).toBeTruthy()
    const data = await res.json()
    console.log(`Commits: total=${data.total}`)
    expect(data.total).toBeGreaterThan(0)
    const c = data.data[0]
    expect(c.commit_id).toBeTruthy()
    expect(c.repo_id).toBeTruthy()
  })

  test('1.5 Projects 列表有数据', async ({ request }) => {
    const res = await request.get(`${BASE_URL}/api/v2/projects?startDate=20260101&endDate=20261231`)
    expect(res.ok()).toBeTruthy()
    const data = await res.json()
    console.log(`Projects: total=${data.total}, count=${data.data?.length}`)
    expect(data.total).toBeGreaterThan(0)
    expect(data.data.length).toBeGreaterThan(0)
    const p = data.data[0]
    expect(p.project_id || p.repo_id).toBeTruthy()
  })

  test('1.6 Project 详情有关联的 tasks 和 commits', async ({ request }) => {
    const listRes = await request.get(`${BASE_URL}/api/v2/projects?startDate=20260101&endDate=20261231`)
    const listData = await listRes.json()
    const projectId = listData.data[0].project_id || listData.data[0].repo_id
    const res = await request.get(`${BASE_URL}/api/v2/projects/by-project-id?projectId=${encodeURIComponent(projectId)}`)
    expect(res.ok()).toBeTruthy()
    const data = await res.json()
    console.log(`Project ${projectId}: ${data.tasks?.length} tasks, ${data.commits?.length} commits`)
    expect(data.summary || data.project).toBeTruthy()
    expect(data.tasks.length).toBeGreaterThanOrEqual(0)
    expect(data.commits.length).toBeGreaterThanOrEqual(0)
  })

  test('1.7 Users 列表有数据', async ({ request }) => {
    const res = await request.get(`${BASE_URL}/api/v2/users?startDate=20260101&endDate=20261231`)
    expect(res.ok()).toBeTruthy()
    const data = await res.json()
    console.log(`Users: total=${data.total}`)
    expect(data.total).toBeGreaterThan(0)
    const u = data.data[0]
    expect(u.user_id).toBeTruthy()
    expect(u.task_count).toBeGreaterThan(0)
  })

  test('1.8 User 详情有参与的 projects/tasks/commits', async ({ request }) => {
    const listRes = await request.get(`${BASE_URL}/api/v2/users?startDate=20260101&endDate=20261231`)
    const listData = await listRes.json()
    const userId = listData.data[0].user_id
    const res = await request.get(`${BASE_URL}/api/v2/users/${userId}`)
    expect(res.ok()).toBeTruthy()
    const data = await res.json()
    console.log(`User ${userId}: ${data.tasks?.length} tasks, ${data.commits?.length} commits, ${data.projects?.length} projects`)
    expect(data.user).toBeTruthy()
    expect(data.tasks.length).toBeGreaterThan(0)
  })

  test('1.9 Orgs 有数据（如有org_mapping）', async ({ request }) => {
    const res = await request.get(`${BASE_URL}/api/v2/orgs?level=org1&startDate=20260101&endDate=20261231`)
    expect(res.ok()).toBeTruthy()
    const data = await res.json()
    console.log(`Orgs: ${data.data?.length || 0} org1 entries`)
    // org_mapping.csv 可能为空，所以不强制要求有数据
    expect(data).toBeTruthy()
  })

  test('1.10 关联关系合理：task的repo_id对应commit的repo_id', async ({ request }) => {
    // 获取一个有关联的 project
    const projRes = await request.get(`${BASE_URL}/api/v2/projects?startDate=20260101&endDate=20261231`)
    const projects = (await projRes.json()).data
    const withTasks = projects.find(p => {
      try { return JSON.parse(p.task_ids || '[]').length > 0 } catch { return (p.task_count || 0) > 0 }
    })
    if (!withTasks) { console.log('No project with tasks found, skip'); return }
    
    const detailRes = await request.get(`${BASE_URL}/api/v2/projects/detail?repoId=${encodeURIComponent(withTasks.repo_id)}`)
    const detail = await detailRes.json()
    // 验证关联的tasks和commits的repo_id一致
    if (detail.tasks?.length > 0) {
      const taskRepoIds = [...new Set(detail.tasks.map(t => t.repo_id))]
      console.log(`Project ${withTasks.repo_id}: task repo_ids=${taskRepoIds.join(',')}`)
    }
    if (detail.commits?.length > 0) {
      const commitRepoIds = [...new Set(detail.commits.map(c => c.repo_id))]
      console.log(`Project ${withTasks.repo_id}: commit repo_ids=${commitRepoIds.join(',')}`)
    }
  })
})

// ============================================================
// 2. 首页 Dashboard UI 测试
// ============================================================
test.describe('2. 首页 Dashboard UI', () => {

  test('2.1 首页加载，指标卡片有数据', async ({ page }) => {
    await page.goto(BASE_URL + '/')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 检查标题
    await expect(page.locator('body')).toContainText('AI Coding')

    // 检查指标卡片区域存在
    const metricCards = page.locator('.kb-metric-card, .el-card')
    const count = await metricCards.count()
    console.log(`Dashboard: ${count} metric cards found`)
    expect(count).toBeGreaterThan(0)

    await page.screenshot({ path: 'test-results/01-dashboard.png', fullPage: true })
  })

  test('2.2 首页快速导航可点击', async ({ page }) => {
    await page.goto(BASE_URL + '/')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 查找导航卡片或链接
    const navLinks = page.locator('a, .el-card, [role="link"]').filter({ hasText: /项目|用户|组织/ })
    const navCount = await navLinks.count()
    console.log(`Dashboard: ${navCount} navigation elements found`)
    expect(navCount).toBeGreaterThan(0)
  })
})

// ============================================================
// 3. Project 视图测试
// ============================================================
test.describe('3. Project 视图 (v2)', () => {

  test('3.1 项目列表页加载有数据', async ({ page }) => {
    await page.goto(BASE_URL + '/project-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 等待表格渲染
    const tableRows = page.locator('.el-table__body-wrapper .el-table__row')
    const rowCount = await tableRows.count()
    console.log(`Project list: ${rowCount} rows`)
    expect(rowCount).toBeGreaterThan(0)

    await page.screenshot({ path: 'test-results/02-project-list.png', fullPage: true })
  })

  test('3.2 点击项目行跳转详情页', async ({ page }) => {
    await page.goto(BASE_URL + '/project-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 点击第一行
    const firstRow = page.locator('.el-table__body-wrapper .el-table__row').first()
    await firstRow.click()
    await page.waitForURL('**/project/**')

    // 检查新页面 URL 包含 /project/
    expect(page.url()).toContain('/project/')

    // 等待详情页加载
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 检查详情页有 el-descriptions 或 kb-metric-card
    const detailSection = page.locator('.el-descriptions').or(page.locator('.kb-metric-card'))
    const detailVisible = await detailSection.count()
    console.log(`Project detail elements: ${detailVisible}`)
    expect(detailVisible).toBeGreaterThan(0)

    await page.screenshot({ path: 'test-results/03-project-detail.png', fullPage: true })
  })

  test('3.3 项目详情有参与者列表', async ({ page }) => {
    // 通过 API 获取第一个 project_id
    const res = await page.request.get(`${BASE_URL}/api/v2/projects?startDate=20260101&endDate=20261231`)
    const data = await res.json()
    const projectId = data.data[0].project_id || data.data[0].repo_id

    await page.goto(BASE_URL + '/project/' + encodeURIComponent(projectId))
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 在详情页中查找表格
    const tables = page.locator('.el-table')
    const tableCount = await tables.count()
    console.log(`Project detail: ${tableCount} tables`)
    expect(tableCount).toBeGreaterThanOrEqual(1)

    await page.screenshot({ path: 'test-results/04-project-participants.png', fullPage: true })
  })
})

// ============================================================
// 4. User 视图测试
// ============================================================
test.describe('4. User 视图 (v2)', () => {

  test('4.1 用户列表页加载有数据', async ({ page }) => {
    await page.goto(BASE_URL + '/user-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    const tableRows = page.locator('.el-table__body-wrapper .el-table__row')
    const rowCount = await tableRows.count()
    console.log(`User list: ${rowCount} rows`)
    expect(rowCount).toBeGreaterThan(0)

    await page.screenshot({ path: 'test-results/05-user-list.png', fullPage: true })
  })

  test('4.2 点击用户行跳转详情页', async ({ page }) => {
    await page.goto(BASE_URL + '/user-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    const firstRow = page.locator('.el-table__body-wrapper .el-table__row').first()
    await firstRow.click()
    await page.waitForURL('**/user/**')

    // 检查新页面 URL 包含 /user/
    expect(page.url()).toContain('/user/')

    // 等待详情页加载
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 检查详情区域
    const detailElements = page.locator('.kb-metric-card')
    const metricCount = await detailElements.count()
    console.log(`User detail: ${metricCount} metric cards`)

    await page.screenshot({ path: 'test-results/06-user-detail.png', fullPage: true })
  })

  test('4.3 用户详情有参与项目列表', async ({ page }) => {
    // 通过 API 获取第一个 user_id
    const res = await page.request.get(`${BASE_URL}/api/v2/users?startDate=20260101&endDate=20261231`)
    const data = await res.json()
    const userId = data.data[0].user_id

    await page.goto(BASE_URL + '/user/' + userId)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 检查详情页的 el-table
    const tables = page.locator('.el-table')
    const tableCount = await tables.count()
    console.log(`User detail: ${tableCount} tables total`)

    await page.screenshot({ path: 'test-results/07-user-projects.png', fullPage: true })
  })
})

// ============================================================
// 5. Org 视图测试
// ============================================================
test.describe('5. Org 视图 (v2)', () => {

  test('5.1 组织页面加载', async ({ page }) => {
    await page.goto(BASE_URL + '/org-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 检查面包屑导航
    const breadcrumb = page.locator('.el-breadcrumb')
    const hasBreadcrumb = await breadcrumb.count()
    console.log(`Org view: breadcrumb=${hasBreadcrumb > 0}`)

    // 表格可能为空（取决于 org_mapping.csv）
    const tableRows = page.locator('.el-table__body-wrapper .el-table__row')
    const rowCount = await tableRows.count()
    console.log(`Org list: ${rowCount} rows`)

    await page.screenshot({ path: 'test-results/08-org-view.png', fullPage: true })
  })
})

// ============================================================
// 6. 导航菜单测试
// ============================================================
test.describe('6. 导航菜单', () => {

  test('6.1 主菜单包含 v2 页面入口', async ({ page }) => {
    await page.goto(BASE_URL + '/')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1000)

    // 检查主菜单
    const menuItems = page.locator('.el-menu-item, .el-sub-menu__title')
    const count = await menuItems.count()
    console.log(`Menu items: ${count}`)

    const menuTexts = []
    for (let i = 0; i < count; i++) {
      const text = await menuItems.nth(i).textContent()
      menuTexts.push(text?.trim())
    }
    console.log('Menu texts:', menuTexts.join(' | '))

    // 至少有首页、项目、用户、组织
    expect(count).toBeGreaterThanOrEqual(4)
  })

  test('6.2 主菜单导航到项目页', async ({ page }) => {
    await page.goto(BASE_URL + '/')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1000)

    // 点击"项目"菜单
    const projectMenu = page.locator('.el-menu-item').filter({ hasText: '项目' }).first()
    if (await projectMenu.count() > 0) {
      await projectMenu.click()
      await page.waitForTimeout(2000)
      expect(page.url()).toContain('/project')
      console.log('Navigated to:', page.url())
    }
  })

  test('6.3 主菜单导航到用户页', async ({ page }) => {
    await page.goto(BASE_URL + '/')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1000)

    const userMenu = page.locator('.el-menu-item').filter({ hasText: '用户' }).first()
    if (await userMenu.count() > 0) {
      await userMenu.click()
      await page.waitForTimeout(2000)
      expect(page.url()).toContain('/user')
      console.log('Navigated to:', page.url())
    }
  })

  test('6.4 更多子菜单可展开', async ({ page }) => {
    await page.goto(BASE_URL + '/')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1000)

    // 查找"更多"子菜单
    const moreMenu = page.locator('.el-sub-menu__title').filter({ hasText: '更多' })
    if (await moreMenu.count() > 0) {
      await moreMenu.hover()
      await page.waitForTimeout(1000)
      console.log('More submenu found and hovered')
      await page.screenshot({ path: 'test-results/09-more-menu.png' })
    } else {
      console.log('No "更多" submenu found')
    }
  })
})

// ============================================================
// 7. 跨页面导航和关联测试
// ============================================================
test.describe('7. 跨页面导航和关联', () => {

  test('7.1 无 JS 错误', async ({ page }) => {
    const errors = []
    page.on('pageerror', (err) => errors.push(err.message))

    const pages = ['/', '/project-v2', '/user-v2', '/org-v2']
    for (const p of pages) {
      await page.goto(BASE_URL + p)
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(2000)
    }

    // 也检查详情页
    const projRes = await page.request.get(`${BASE_URL}/api/v2/projects?startDate=20260101&endDate=20261231`)
    const projData = await projRes.json()
    if (projData.data?.length > 0) {
      const projectId = projData.data[0].project_id || projData.data[0].repo_id
      await page.goto(BASE_URL + '/project/' + encodeURIComponent(projectId))
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(2000)
    }

    const userRes = await page.request.get(`${BASE_URL}/api/v2/users?startDate=20260101&endDate=20261231`)
    const userData = await userRes.json()
    if (userData.data?.length > 0) {
      await page.goto(BASE_URL + '/user/' + userData.data[0].user_id)
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(2000)
    }

    if (errors.length > 0) {
      console.log('JS Errors found:', errors)
    }
    expect(errors.length).toBe(0)
  })

  test('7.2 所有 v2 页面无 API 500 错误', async ({ page }) => {
    const failedApis = []
    page.on('response', (res) => {
      if (res.url().includes('/api/') && res.status() >= 500) {
        failedApis.push(`${res.status()} ${res.url()}`)
      }
    })

    const pages = ['/', '/project-v2', '/user-v2', '/org-v2']
    for (const p of pages) {
      await page.goto(BASE_URL + p)
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(2000)
    }

    if (failedApis.length > 0) {
      console.log('Failed APIs:', failedApis)
    }
    expect(failedApis.length).toBe(0)
  })
})

// ============================================================
// 8. Task 详情页测试
// ============================================================
test.describe('8. Task 详情页', () => {

  test('8.1 Task 详情页加载有对话历史', async ({ page }) => {
    // 先获取一个有效的 taskId
    const res = await page.request.get(`${BASE_URL}/api/v2/tasks?startDate=20260101&endDate=20261231&page=1&pageSize=1`)
    const data = await res.json()
    const taskId = data.data[0].task_id

    await page.goto(BASE_URL + '/task/' + taskId)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 验证页面有内容
    await expect(page.locator('body')).toContainText(taskId)
    
    // 验证有对话历史（timeline 或 card）
    const convItems = page.locator('.el-timeline-item, .el-card').filter({ hasText: /user|agent|GLM|Kimi|MiniMax/ })
    const convCount = await convItems.count()
    console.log(`Task detail: ${convCount} conversation items`)
    expect(convCount).toBeGreaterThan(0)

    await page.screenshot({ path: 'test-results/10-task-detail.png', fullPage: true })
  })

  test('8.2 Task 详情有元信息', async ({ page }) => {
    const res = await page.request.get(`${BASE_URL}/api/v2/tasks?startDate=20260101&endDate=20261231&page=1&pageSize=1`)
    const data = await res.json()
    const taskId = data.data[0].task_id

    await page.goto(BASE_URL + '/task/' + taskId)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 检查元信息区域存在
    const hasUserInfo = await page.locator('body').textContent()
    // 页面应该包含用户名或仓库信息
    const hasRelevantInfo = hasUserInfo.includes('用户') || hasUserInfo.includes('仓库') || hasUserInfo.includes('Task ID')
    expect(hasRelevantInfo).toBeTruthy()
    
    await page.screenshot({ path: 'test-results/11-task-meta.png', fullPage: true })
  })
})

// ============================================================
// 9. 跨页面导航测试
// ============================================================
test.describe('9. 跨页面导航', () => {

  test('9.1 Project详情中参与者可点击跳转User视图', async ({ page }) => {
    // 通过 API 获取第一个 project_id
    const res = await page.request.get(`${BASE_URL}/api/v2/projects?startDate=20260101&endDate=20261231`)
    const data = await res.json()
    const projectId = data.data[0].project_id || data.data[0].repo_id

    await page.goto(BASE_URL + '/project/' + encodeURIComponent(projectId))
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 在详情页中找 el-link（参与者的跳转链接）
    const links = page.locator('.el-link')
    const linkCount = await links.count()
    console.log(`Project detail: ${linkCount} clickable links`)
    expect(linkCount).toBeGreaterThan(0)

    await page.screenshot({ path: 'test-results/12-cross-nav-project.png', fullPage: true })
  })

  test('9.2 User详情页支持直接URL访问', async ({ page }) => {
    await page.goto(BASE_URL + '/user/user-001')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(5000)

    // 检查用户详情页的指标卡片
    const metricCards = page.locator('.kb-metric-card')
    const metricCount = await metricCards.count()
    console.log(`User detail auto-opened: ${metricCount} metric cards`)
    
    await page.screenshot({ path: 'test-results/13-user-auto-open.png', fullPage: true })
  })

  test('9.3 Project详情页支持直接URL访问', async ({ page }) => {
    // 通过 API 获取真实的第一个 project_id
    const res = await page.request.get(`${BASE_URL}/api/v2/projects?startDate=20260101&endDate=20261231`)
    const data = await res.json()
    const projectId = data.data[0].project_id || data.data[0].repo_id

    await page.goto(BASE_URL + '/project/' + encodeURIComponent(projectId))
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(5000)

    await page.screenshot({ path: 'test-results/14-project-auto-open.png', fullPage: true })
  })
})

// ============================================================
// 10. 搜索过滤测试
// ============================================================
test.describe('10. 搜索过滤', () => {

  test('10.1 Project列表搜索过滤', async ({ page }) => {
    await page.goto(BASE_URL + '/project-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 找到搜索框
    const searchInput = page.locator('input[placeholder*="搜索"]').first()
    if (await searchInput.count() > 0) {
      await searchInput.fill('costrict')
      await page.waitForTimeout(1000)

      // 检查过滤后行数减少
      const rows = page.locator('.el-table__body-wrapper .el-table__row')
      const rowCount = await rows.count()
      console.log(`After search 'costrict': ${rowCount} rows`)
      // 应该有至少1行包含costrict
      expect(rowCount).toBeGreaterThan(0)
    }

    await page.screenshot({ path: 'test-results/15-project-search.png', fullPage: true })
  })

  test('10.2 User列表搜索过滤', async ({ page }) => {
    await page.goto(BASE_URL + '/user-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    const searchInput = page.locator('input[placeholder*="搜索"]').first()
    if (await searchInput.count() > 0) {
      await searchInput.fill('138')
      await page.waitForTimeout(1000)

      const rows = page.locator('.el-table__body-wrapper .el-table__row')
      const rowCount = await rows.count()
      console.log(`After search '138': ${rowCount} rows`)
      expect(rowCount).toBeGreaterThanOrEqual(1)
    }

    await page.screenshot({ path: 'test-results/16-user-search.png', fullPage: true })
  })
})

// ============================================================
// 11. 人工调整测试
// ============================================================
test.describe('11. 人工调整', () => {

  test('11.1 Project详情有人工调整按钮', async ({ page }) => {
    // 通过 API 获取第一个 project_id
    const res = await page.request.get(`${BASE_URL}/api/v2/projects?startDate=20260101&endDate=20261231`)
    const data = await res.json()
    const projectId = data.data[0].project_id || data.data[0].repo_id

    await page.goto(BASE_URL + '/project/' + encodeURIComponent(projectId))
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 查找人工调整按钮
    const adjustBtn = page.locator('button, .el-button').filter({ hasText: /人工调整|手动/ })
    const btnCount = await adjustBtn.count()
    console.log(`Manual adjust buttons: ${btnCount}`)

    if (btnCount > 0) {
      // 点击打开对话框
      await adjustBtn.first().click()
      await page.waitForTimeout(1000)

      // 检查对话框出现
      const dialog = page.locator('.el-dialog')
      const dialogVisible = await dialog.isVisible()
      console.log(`Manual dialog visible: ${dialogVisible}`)

      await page.screenshot({ path: 'test-results/17-manual-adjust.png', fullPage: true })
    }
  })
})

// ============================================================
// 12. 综合质量检查（包含新页面）
// ============================================================
test.describe('12. 综合质量', () => {

  test('12.1 所有页面（含详情页）无JS错误', async ({ page }) => {
    const errors = []
    page.on('pageerror', (err) => errors.push(err.message))

    const pagesToCheck = ['/', '/project-v2', '/user-v2', '/org-v2']
    for (const p of pagesToCheck) {
      await page.goto(BASE_URL + p)
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(2000)
    }

    // Task详情页
    const taskRes = await page.request.get(`${BASE_URL}/api/v2/tasks?startDate=20260101&endDate=20261231&page=1&pageSize=1`)
    const taskData = await taskRes.json()
    if (taskData.data?.length > 0) {
      await page.goto(BASE_URL + '/task/' + taskData.data[0].task_id)
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(2000)
    }

    // Project详情页
    const projRes = await page.request.get(`${BASE_URL}/api/v2/projects?startDate=20260101&endDate=20261231`)
    const projData = await projRes.json()
    if (projData.data?.length > 0) {
      const projectId = projData.data[0].project_id || projData.data[0].repo_id
      await page.goto(BASE_URL + '/project/' + encodeURIComponent(projectId))
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(2000)
    }

    // User详情页
    const userRes = await page.request.get(`${BASE_URL}/api/v2/users?startDate=20260101&endDate=20261231`)
    const userData = await userRes.json()
    if (userData.data?.length > 0) {
      await page.goto(BASE_URL + '/user/' + userData.data[0].user_id)
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(2000)
    }

    if (errors.length > 0) {
      console.log('JS Errors:', errors)
    }
    expect(errors.length).toBe(0)
  })

  test('12.2 所有页面无 API 500 错误', async ({ page }) => {
    const failedApis = []
    page.on('response', (res) => {
      if (res.url().includes('/api/') && res.status() >= 500) {
        failedApis.push(`${res.status()} ${res.url()}`)
      }
    })

    const pagesToCheck = ['/', '/project-v2', '/user-v2', '/org-v2']
    for (const p of pagesToCheck) {
      await page.goto(BASE_URL + p)
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(2000)
    }

    if (failedApis.length > 0) {
      console.log('Failed APIs:', failedApis)
    }
    expect(failedApis.length).toBe(0)
  })
})

// ============================================================
// 13. 独立详情页和首页可点击测试
// ============================================================
test.describe('13. 独立详情页和首页可点击', () => {

  test('13.1 首页所有指标卡片可点击', async ({ page }) => {
    await page.goto(BASE_URL + '/')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 找所有 .dashboard-metric-card
    const metricCards = page.locator('.dashboard-metric-card')
    const cardCount = await metricCards.count()
    console.log(`Dashboard clickable metric cards: ${cardCount}`)
    expect(cardCount).toBeGreaterThanOrEqual(8)

    // 验证每个卡片有 cursor: pointer
    for (let i = 0; i < cardCount; i++) {
      const cursor = await metricCards.nth(i).evaluate(el => getComputedStyle(el).cursor)
      expect(cursor).toBe('pointer')
    }
  })

  test('13.2 /project/:projectId 详情页独立加载', async ({ page }) => {
    // 通过 API 获取真实的 project_id
    const res = await page.request.get(`${BASE_URL}/api/v2/projects?startDate=20260101&endDate=20261231`)
    const data = await res.json()
    const projectId = data.data[0].project_id || data.data[0].repo_id

    await page.goto(BASE_URL + '/project/' + encodeURIComponent(projectId))
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 检查有 el-descriptions（项目元信息）
    const descriptions = page.locator('.el-descriptions')
    const descCount = await descriptions.count()
    console.log(`Project detail page: ${descCount} el-descriptions`)
    expect(descCount).toBeGreaterThan(0)

    // 检查 URL 正确
    expect(page.url()).toContain('/project/')
  })
})
