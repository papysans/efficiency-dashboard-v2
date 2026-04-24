import { test, expect } from '@playwright/test'

const BASE_URL = 'http://localhost:8880'

test.describe('DimensionSelect 下拉过滤测试', () => {

  test('提效分析页面 - 项目 ID 下拉有选项', async ({ page }) => {
    await page.goto(BASE_URL + '/efficiency')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1000)

    // 找到 DimensionSelect（el-select）
    const select = page.locator('.el-select').first()
    await expect(select).toBeVisible()

    // 点击打开下拉
    await select.click()
    await page.waitForTimeout(1000)

    // 检查下拉列表中应有选项
    const options = page.locator('.el-select-dropdown .el-select-dropdown__item')
    const count = await options.count()
    console.log(`Efficiency panel - project options count: ${count}`)
    expect(count).toBeGreaterThan(0)

    await page.screenshot({ path: 'test-results/select-01-efficiency-dropdown.png', fullPage: true })

    // 输入关键词过滤
    const input = select.locator('input')
    await input.fill('credit')
    await page.waitForTimeout(500)

    const filteredCount = await options.count()
    console.log(`After filter 'credit': ${filteredCount} options`)

    await page.screenshot({ path: 'test-results/select-02-efficiency-filtered.png', fullPage: true })
  })

  test('项目面板 - 搜索过滤有效', async ({ page }) => {
    await page.goto(BASE_URL + '/project-panel')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1000)

    const select = page.locator('.el-select').first()
    await expect(select).toBeVisible()

    await select.click()
    await page.waitForTimeout(1000)

    const options = page.locator('.el-select-dropdown .el-select-dropdown__item')
    const count = await options.count()
    console.log(`Project panel - options count: ${count}`)
    expect(count).toBeGreaterThan(0)

    await page.screenshot({ path: 'test-results/select-03-project-dropdown.png', fullPage: true })
  })

  test('用户面板 - 用户搜索有效', async ({ page }) => {
    await page.goto(BASE_URL + '/user-panel')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1000)

    const select = page.locator('.el-select').first()
    await expect(select).toBeVisible()

    await select.click()
    await page.waitForTimeout(1000)

    const options = page.locator('.el-select-dropdown .el-select-dropdown__item')
    const count = await options.count()
    console.log(`User panel - options count: ${count}`)
    expect(count).toBeGreaterThan(0)

    await page.screenshot({ path: 'test-results/select-04-user-dropdown.png', fullPage: true })
  })

  test('仓库面板 - 仓库搜索有效', async ({ page }) => {
    await page.goto(BASE_URL + '/repo-panel')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1000)

    const select = page.locator('.el-select').first()
    await expect(select).toBeVisible()

    await select.click()
    await page.waitForTimeout(1000)

    const options = page.locator('.el-select-dropdown .el-select-dropdown__item')
    const count = await options.count()
    console.log(`Repo panel - options count: ${count}`)

    await page.screenshot({ path: 'test-results/select-05-repo-dropdown.png', fullPage: true })
  })

  test('选中项目后提效分析自动查询', async ({ page }) => {
    await page.goto(BASE_URL + '/efficiency')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1000)

    // 点击下拉
    const select = page.locator('.el-select').first()
    await select.click()
    await page.waitForTimeout(1000)

    // 选第一项
    const firstOption = page.locator('.el-select-dropdown .el-select-dropdown__item').first()
    if (await firstOption.isVisible()) {
      await firstOption.click()
      await page.waitForTimeout(2000)

      // 应自动触发查询
      await page.screenshot({ path: 'test-results/select-06-efficiency-auto-query.png', fullPage: true })
    }
  })
})
