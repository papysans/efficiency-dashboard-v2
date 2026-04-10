// @ts-check
import { test, expect } from '@playwright/test'

const BASE = 'http://localhost:8880'
const TASK_URL = `${BASE}/task/019d4295-0dad-7031-8621-92f051a9b632`

test.describe('用户链接+总费用修复', () => {

  test('1. 用户名是可点击链接，跳转到用户详情', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 用户字段应该是 el-link（可点击链接），不是纯文本
    const desc = page.locator('.el-descriptions')
    const userLink = desc.locator('.el-link').filter({ hasText: '13162290627' })
    const linkCount = await userLink.count()
    console.log('用户链接数:', linkCount)
    expect(linkCount).toBeGreaterThan(0)

    // 点击用户链接应跳转到用户详情页
    await userLink.first().click()
    await page.waitForTimeout(1000)
    console.log('跳转后URL:', page.url())
    expect(page.url()).toContain('/user/')
  })

  test('2. 总费用有值（不是-）', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    const desc = page.locator('.el-descriptions')
    const descText = await desc.textContent()

    // 找到"总费用"标签后面的值
    const cells = await page.locator('.el-descriptions__cell').allTextContents()
    const feeIdx = cells.findIndex(c => c.includes('总费用'))
    const feeValue = feeIdx >= 0 ? cells[feeIdx + 1] || cells[feeIdx] : 'not found'
    console.log('总费用:', feeValue)

    // task.cost = 0.03，不应显示 -
    expect(feeValue).not.toBe('-')
    expect(feeValue).toContain('0.03')
  })

  test('3. 页面无JS报错', async ({ page }) => {
    const errors = []
    page.on('pageerror', err => errors.push(err.message))
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)
    const realErrors = errors.filter(e => !e.includes('ResizeObserver'))
    expect(realErrors.length).toBe(0)
  })
})
