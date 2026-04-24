import { test, expect } from '@playwright/test'

const BASE_URL = 'http://localhost:8880'

test('验证前端数据显示', async ({ page }) => {
  // 1. 首页
  await page.goto(BASE_URL + '/')
  await page.waitForLoadState('networkidle')
  await page.screenshot({ path: 'test-results/data-01-home.png', fullPage: true })

  // 2. Dashboard - 默认日期范围应包含3/29-3/31
  await page.goto(BASE_URL + '/dashboard')
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(2000) // 等待数据加载
  await page.screenshot({ path: 'test-results/data-02-dashboard.png', fullPage: true })

  // 检查是否有数据行
  const rows = page.locator('.el-table__row')
  const rowCount = await rows.count()
  console.log(`Dashboard rows: ${rowCount}`)

  // 3. Dashboard 带明确日期参数
  await page.goto(BASE_URL + '/dashboard?startDate=2026-03-29&endDate=2026-03-31&tab=request')
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(2000)
  await page.screenshot({ path: 'test-results/data-03-dashboard-dated.png', fullPage: true })
  const rows2 = page.locator('.el-table__row')
  const rowCount2 = await rows2.count()
  console.log(`Dashboard with dates rows: ${rowCount2}`)

  // 4. 聚合 Tab
  await page.goto(BASE_URL + '/dashboard?startDate=2026-03-29&endDate=2026-03-31&tab=aggregate')
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(2000)
  await page.screenshot({ path: 'test-results/data-04-dashboard-aggregate.png', fullPage: true })

  // 5. 项目面板
  await page.goto(BASE_URL + '/project-panel')
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(2000)
  await page.screenshot({ path: 'test-results/data-05-project.png', fullPage: true })

  // 6. 用户面板
  await page.goto(BASE_URL + '/user-panel')
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(2000)
  await page.screenshot({ path: 'test-results/data-06-user.png', fullPage: true })

  // 7. 检查 API 直接调用
  const apiResp = await page.goto(BASE_URL + '/api/requests?startDate=20260329&endDate=20260331&page=1&pageSize=5')
  const body = await apiResp.text()
  console.log(`API response length: ${body.length}`)
  console.log(`API response snippet: ${body.substring(0, 200)}`)
})
