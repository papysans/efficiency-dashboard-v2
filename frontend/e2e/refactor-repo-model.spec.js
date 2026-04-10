// @ts-check
import { test, expect } from '@playwright/test'

const BASE = 'http://localhost:8880'

test.describe('重构验证：repo-model (repo_addr+repo_branch)', () => {

  // ====== 1. Repo 列表页 /repo-v2 ======
  test('1.1 仓库列表页能加载且表格显示', async ({ page }) => {
    await page.goto(BASE + '/repo-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)
    // 表格应存在
    const table = page.locator('.el-table')
    await expect(table.first()).toBeVisible()
    // 表格有数据行
    const rows = page.locator('.el-table__body-wrapper .el-table__row')
    const count = await rows.count()
    console.log(`仓库列表行数: ${count}`)
    expect(count).toBeGreaterThan(0)
    await page.screenshot({ path: 'test-results/refactor-01-repo-list.png', fullPage: true })
  })

  test('1.2 仓库列表表格中不显示 repo_id 列', async ({ page }) => {
    await page.goto(BASE + '/repo-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)
    // 表头中不应含有 "repo_id" 文本
    const headers = page.locator('.el-table__header-wrapper th')
    const headerTexts = []
    const count = await headers.count()
    for (let i = 0; i < count; i++) {
      headerTexts.push(await headers.nth(i).textContent())
    }
    const headerJoined = headerTexts.join(',').toLowerCase()
    expect(headerJoined).not.toContain('repo_id')
    console.log(`仓库列表表头: ${headerTexts.map(h => h?.trim()).filter(Boolean).join(' | ')}`)
  })

  // ====== 2. Repo 详情页 /repo/:repoAddr/:repoBranch ======
  test('2.1 仓库详情页：分支切换下拉存在', async ({ page }) => {
    // 通过 API 获取第一个仓库的 repo_addr 和 repo_branch
    const res = await page.request.get(`${BASE}/api/v2/repos?startDate=20260101&endDate=20261231&page=1&pageSize=1`)
    const body = await res.json()
    expect(body.data.length).toBeGreaterThan(0)
    const repo = body.data[0]
    console.log(`测试仓库: ${repo.repo_addr}#${repo.repo_branch}`)

    await page.goto(BASE + '/repo/' + encodeURIComponent(repo.repo_addr) + '/' + encodeURIComponent(repo.repo_branch))
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 分支切换下拉框
    const branchSelect = page.locator('.el-select')
    await expect(branchSelect.first()).toBeVisible()
    console.log(`分支下拉框数: ${await branchSelect.count()}`)
    await page.screenshot({ path: 'test-results/refactor-02-repo-detail-branch.png', fullPage: true })
  })

  test('2.2 仓库详情页：效率评估卡片存在', async ({ page }) => {
    const res = await page.request.get(`${BASE}/api/v2/repos?startDate=20260101&endDate=20261231&page=1&pageSize=1`)
    const body = await res.json()
    const repo = body.data[0]

    await page.goto(BASE + '/repo/' + encodeURIComponent(repo.repo_addr) + '/' + encodeURIComponent(repo.repo_branch))
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 效率评估卡片 (古法预估、实际耗时、提效比)
    const cards = page.locator('.dashboard-metric-card')
    const cardCount = await cards.count()
    console.log(`效率评估卡片数: ${cardCount}`)
    expect(cardCount).toBeGreaterThanOrEqual(3)

    // 检查有 "古法预估" "实际耗时" "提效比" 文字
    const bodyText = await page.locator('body').textContent()
    expect(bodyText).toContain('古法预估')
    expect(bodyText).toContain('实际耗时')
    expect(bodyText).toContain('提效比')
  })

  test('2.3 仓库详情页：有 Commits 和 Tasks 表格', async ({ page }) => {
    const res = await page.request.get(`${BASE}/api/v2/repos?startDate=20260101&endDate=20261231&page=1&pageSize=1`)
    const body = await res.json()
    const repo = body.data[0]

    await page.goto(BASE + '/repo/' + encodeURIComponent(repo.repo_addr) + '/' + encodeURIComponent(repo.repo_branch))
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 至少有 Commits 表格
    const tables = page.locator('.el-table')
    const tableCount = await tables.count()
    console.log(`仓库详情表格数: ${tableCount}`)
    expect(tableCount).toBeGreaterThanOrEqual(1)

    // 验证有 Commits 标题
    const bodyText = await page.locator('body').textContent()
    expect(bodyText).toContain('Commits')
    await page.screenshot({ path: 'test-results/refactor-03-repo-detail-tables.png', fullPage: true })
  })

  // ====== 3. Commit 列表页 /commit-v2 ======
  test('3.1 Commit列表页能加载且表格显示', async ({ page }) => {
    await page.goto(BASE + '/commit-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)
    // 表格有数据行
    const rows = page.locator('.el-table__body-wrapper .el-table__row')
    const count = await rows.count()
    console.log(`Commit列表行数: ${count}`)
    expect(count).toBeGreaterThan(0)
    await page.screenshot({ path: 'test-results/refactor-04-commit-list.png', fullPage: true })
  })

  test('3.2 Commit列表表格中不显示 repo_id 列', async ({ page }) => {
    await page.goto(BASE + '/commit-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)
    // 表头不含 "repo_id"
    const headers = page.locator('.el-table__header-wrapper th')
    const headerTexts = []
    const count = await headers.count()
    for (let i = 0; i < count; i++) {
      headerTexts.push(await headers.nth(i).textContent())
    }
    const headerJoined = headerTexts.join(',').toLowerCase()
    expect(headerJoined).not.toContain('repo_id')
    console.log(`Commit列表表头: ${headerTexts.map(h => h?.trim()).filter(Boolean).join(' | ')}`)
  })

  test('3.3 Commit列表显示仓库列(repo_addr格式)', async ({ page }) => {
    await page.goto(BASE + '/commit-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)
    // 表头应含有"仓库"列
    const headers = page.locator('.el-table__header-wrapper th')
    const headerTexts = []
    const count = await headers.count()
    for (let i = 0; i < count; i++) {
      headerTexts.push((await headers.nth(i).textContent())?.trim())
    }
    expect(headerTexts.join(',')).toContain('仓库')
    console.log(`Commit列表确认含"仓库"列: true`)
  })

  // ====== 4. Commit 详情页 /commit/:commitId ======
  test('4.1 Commit详情页：无需 repoId 查询参数', async ({ page }) => {
    // 直接通过 commitId 访问（无 repoId 参数）
    await page.goto(BASE + '/commit/commit-001')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 应该正常显示 commit 详情
    const bodyText = await page.locator('body').textContent()
    expect(bodyText).toContain('Commit 详情')
    expect(bodyText).toContain('commit-001')
    // URL 中不应含 repoId 参数
    expect(page.url()).not.toContain('repoId')
    await page.screenshot({ path: 'test-results/refactor-05-commit-detail.png', fullPage: true })
  })

  test('4.2 Commit详情页：显示仓库链接(repo_addr格式)', async ({ page }) => {
    await page.goto(BASE + '/commit/commit-001')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 仓库链接存在，显示 repo_addr 内容
    const repoLink = page.locator('.el-link').filter({ hasText: /github|gitee/ })
    const linkCount = await repoLink.count()
    console.log(`Commit详情仓库链接数: ${linkCount}`)
    expect(linkCount).toBeGreaterThan(0)
  })

  // ====== 5. 导航链接 ======
  test('5.1 Commit详情页仓库链接跳转到 /repo/:repoAddr/:repoBranch', async ({ page }) => {
    await page.goto(BASE + '/commit/commit-001')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 点击仓库链接
    const repoLink = page.locator('.el-link').filter({ hasText: /github|gitee/ })
    if (await repoLink.count() > 0) {
      await repoLink.first().click()
      await page.waitForTimeout(2000)
      // 应跳转到 /repo/ 路径
      const url = page.url()
      console.log(`仓库链接跳转URL: ${url}`)
      expect(url).toContain('/repo/')
      // URL 格式应为 /repo/{encodedAddr}/{encodedBranch}
      // 不应包含 repoId= 查询参数
      expect(url).not.toContain('repoId=')
      await page.screenshot({ path: 'test-results/refactor-06-nav-repo-link.png', fullPage: true })
    }
  })

  test('5.2 仓库列表点击行跳转到 /repo/:repoAddr/:repoBranch', async ({ page }) => {
    await page.goto(BASE + '/repo-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    const rows = page.locator('.el-table__body-wrapper .el-table__row')
    if (await rows.count() > 0) {
      await rows.first().click()
      await page.waitForTimeout(2000)
      const url = page.url()
      console.log(`仓库列表点击跳转URL: ${url}`)
      expect(url).toContain('/repo/')
      // 不应包含旧式 repoId
      expect(url).not.toContain('repoId=')
      await page.screenshot({ path: 'test-results/refactor-07-repo-list-nav.png', fullPage: true })
    }
  })

  // ====== 6. 综合质量 - 无 JS 错误 ======
  test('6.1 核心页面无JS错误无API 500', async ({ page }) => {
    const errors = []
    const apiErrors = []
    page.on('pageerror', (e) => errors.push(e.message))
    page.on('response', (r) => {
      if (r.url().includes('/api/') && r.status() >= 500) apiErrors.push(`${r.status()} ${r.url()}`)
    })

    // 首页
    await page.goto(BASE + '/')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1500)

    // 仓库列表
    await page.goto(BASE + '/repo-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1500)

    // Commit 列表
    await page.goto(BASE + '/commit-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1500)

    // 通过 API 获取仓库详情 URL
    const repoRes = await page.request.get(`${BASE}/api/v2/repos?startDate=20260101&endDate=20261231&page=1&pageSize=1`)
    const repoBody = await repoRes.json()
    if (repoBody.data && repoBody.data.length > 0) {
      const r = repoBody.data[0]
      await page.goto(BASE + '/repo/' + encodeURIComponent(r.repo_addr) + '/' + encodeURIComponent(r.repo_branch))
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(1500)
    }

    // Commit 详情
    await page.goto(BASE + '/commit/commit-001')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1500)

    if (errors.length) console.log('JS Errors:', errors)
    if (apiErrors.length) console.log('API 500 Errors:', apiErrors)
    expect(errors.length).toBe(0)
    expect(apiErrors.length).toBe(0)
  })
})
