// @ts-check
import { test, expect } from '@playwright/test'

const BASE = 'http://localhost:8880'
const TASK_URL = `${BASE}/task/019d4295-0dad-7031-8621-92f051a9b632`

test.describe('Task详情页UI修复', () => {

  test('1. 辅助信息在el-descriptions表格中，不是独立div', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 不应存在独立的辅助信息 div（style 包含 padding-left: 4px 的灰色文字行）
    const standaloneDiv = page.locator('div[style*="padding-left: 4px"]').filter({ hasText: '总请求数' })
    const divCount = await standaloneDiv.count()
    console.log('独立辅助信息div数:', divCount)
    expect(divCount).toBe(0)

    // 总请求数、总Tokens、总费用应该在 el-descriptions 表格内
    const desc = page.locator('.el-descriptions')
    const descText = await desc.textContent()
    console.log('descriptions内容片段:', descText?.substring(0, 800))

    expect(descText).toContain('总请求数')
    expect(descText).toContain('总Tokens')
    expect(descText).toContain('总费用')

    await page.screenshot({ path: 'test-results/ui-fix-01-desc-table.png', fullPage: true })
  })

  test('2. 对话历史header无冗余"实际耗时"', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 对话历史 card 的 header 应该只有"对话历史"文字，不应有"实际耗时：XX分钟"
    const historyCard = page.locator('.el-card').filter({ hasText: '对话历史' })
    const header = historyCard.locator('.el-card__header')
    const headerText = await header.textContent()
    console.log('对话历史header:', headerText?.trim())

    expect(headerText?.trim()).toBe('对话历史')
    // 不应包含"实际耗时"
    expect(headerText).not.toContain('实际耗时')
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
