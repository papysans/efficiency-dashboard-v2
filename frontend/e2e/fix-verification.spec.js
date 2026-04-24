// @ts-check
import { test, expect } from '@playwright/test'

const BASE = 'http://localhost:8880'
const TASK_ID = '019d41ba-5781-7436-8e80-e698d0d344c3'
const TASK_URL = `${BASE}/task/${TASK_ID}`

test.describe('Task详情页修复验证', () => {

  test('1. 仓库字段显示-，Project字段有值', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    const desc = page.locator('.el-descriptions')
    const text = await desc.textContent()
    console.log('元信息:', text?.substring(0, 600))

    // 仓库显示 - （repo_id == project_id 时）
    expect(text).toMatch(/仓库\s*-/)
    // Project 有值
    expect(text).toContain('797e102c29')

    await page.screenshot({ path: 'test-results/fix-01-repo-project.png', fullPage: true })
  })

  test('2. 古法预估和实际耗时旁有问号图标(tooltip)', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 在整个 el-descriptions 区域中查找 QuestionFilled 问号图标
    // Element Plus 的 el-icon 渲染为 <i class="el-icon"> 或 <span class="el-icon">
    const descArea = page.locator('.el-descriptions')
    const icons = descArea.locator('.el-icon')
    const iconCount = await icons.count()
    console.log('描述区域问号图标数:', iconCount)
    // 至少有2个（古法预估的reason和实际耗时的reason都有值）
    expect(iconCount).toBeGreaterThanOrEqual(2)

    // hover 第一个图标触发 tooltip
    await icons.first().hover()
    await page.waitForTimeout(800)

    // 检查 tooltip 弹出
    const tooltip = page.locator('.el-popper:visible, [role="tooltip"]:visible')
    const tooltipCount = await tooltip.count()
    console.log('可见tooltip数:', tooltipCount)
    if (tooltipCount > 0) {
      const text = await tooltip.first().textContent()
      console.log('tooltip片段:', text?.substring(0, 100))
      expect(text?.length).toBeGreaterThan(5)
    }
  })

  test('3. 提效统计卡片中也有问号图标', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 3 个提效卡片
    const metricCards = page.locator('.kb-metric-card')
    const cardCount = await metricCards.count()
    console.log('提效卡片数:', cardCount)
    expect(cardCount).toBeGreaterThanOrEqual(3)

    // 卡片区域中也应有问号图标
    const cardIcons = page.locator('.kb-metric-card .el-icon, .kb-metric-label .el-icon')
    const cardIconCount = await cardIcons.count()
    console.log('卡片区域问号图标数:', cardIconCount)
    // 至少有1个（实际耗时的reason有值）
    expect(cardIconCount).toBeGreaterThanOrEqual(1)
  })

  test('4. 页面无JS报错', async ({ page }) => {
    const errors = []
    page.on('pageerror', err => errors.push(err.message))

    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    const realErrors = errors.filter(e => !e.includes('ResizeObserver'))
    expect(realErrors.length).toBe(0)
  })

  test('5. Commit详情页正常加载', async ({ page }) => {
    await page.goto(`${BASE}/commit/commit-001?repoId=${encodeURIComponent('repo-costrict-main')}`)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    const title = page.locator('text=Commit 详情')
    await expect(title).toBeVisible()

    // 元信息应有数据
    const desc = page.locator('.el-descriptions')
    const text = await desc.textContent()
    console.log('Commit元信息:', text?.substring(0, 300))
    expect(text).toContain('commit-001')

    await page.screenshot({ path: 'test-results/fix-05-commit-detail.png', fullPage: true })
  })
})
