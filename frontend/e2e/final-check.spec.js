import { test } from '@playwright/test'

const BASE_URL = 'http://localhost:8880'

test('截图所有页面验证数据', async ({ page }) => {
  test.setTimeout(120000)
  const pages = [
    { url: '/', name: 'home' },
    { url: '/dashboard', name: 'dashboard' },
    { url: '/dashboard?startDate=2026-03-29&endDate=2026-03-31&tab=request', name: 'dashboard-request' },
    { url: '/dashboard?startDate=2026-03-29&endDate=2026-03-31&tab=aggregate&dimension=project', name: 'dashboard-aggregate' },
    { url: '/efficiency', name: 'efficiency' },
    { url: '/project-panel', name: 'project-panel' },
    { url: '/repo-panel', name: 'repo-panel' },
    { url: '/user-panel', name: 'user-panel' },
    { url: '/org-panel', name: 'org-panel' },
  ]

  for (const p of pages) {
    await page.goto(BASE_URL + p.url)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 检查表格行数
    const rows = await page.locator('.el-table__row').count()
    console.log(`${p.name}: ${rows} rows`)

    await page.screenshot({ path: `test-results/final-${p.name}.png`, fullPage: true })
  }
})
