// @ts-check
import { test, expect } from '@playwright/test'

const BASE = 'http://localhost:8880'
const TASK_URL = `${BASE}/task/019d4295-0dad-7031-8621-92f051a9b632`

test.describe('Project链接跳转修复', () => {

  test('1. 从Task详情点击Project链接，跳转到仓库详情页且有数据', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 找到 Project 链接
    const desc = page.locator('.el-descriptions')
    const projectLink = desc.locator('.el-link').filter({ hasText: '5af735238e' })
    const linkCount = await projectLink.count()
    console.log('Project链接数:', linkCount)
    expect(linkCount).toBeGreaterThan(0)

    // 点击 Project 链接
    await projectLink.first().click()
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 应跳转到 /repo/ 路径（不是 /project/）
    console.log('跳转后URL:', page.url())
    expect(page.url()).toContain('/repo/')
    expect(page.url()).not.toContain('/project/')

    // 页面应有"仓库详情"标题
    const title = page.locator('text=仓库详情')
    await expect(title).toBeVisible()

    // 页面应有数据（不是空白）
    const descDetail = page.locator('.el-descriptions')
    const detailText = await descDetail.textContent()
    console.log('仓库详情内容:', detailText?.substring(0, 300))

    // 应包含关联Task数等信息
    expect(detailText).toContain('关联Task数')

    await page.screenshot({ path: 'test-results/project-link-01.png', fullPage: true })
  })

  test('2. 无JS报错', async ({ page }) => {
    const errors = []
    page.on('pageerror', err => errors.push(err.message))

    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 点击 Project 链接跳转
    const projectLink = page.locator('.el-descriptions .el-link').filter({ hasText: '5af735238e' })
    if (await projectLink.count() > 0) {
      await projectLink.first().click()
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(2000)
    }

    const realErrors = errors.filter(e => !e.includes('ResizeObserver'))
    expect(realErrors.length).toBe(0)
  })
})
