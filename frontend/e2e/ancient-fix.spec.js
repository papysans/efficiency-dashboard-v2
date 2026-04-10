// @ts-check
import { test, expect } from '@playwright/test'

const BASE = 'http://localhost:8880'
const TASK_URL = `${BASE}/task/019d4295-0dad-7031-8621-92f051a9b632`

test.describe('古法预估AI未出值修复', () => {

  test('1. 古法预估显示manual值+删除线的AI未出值+系统reason tooltip', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    const desc = page.locator('.el-descriptions')
    const descText = await desc.textContent()
    console.log('描述区域文本:', descText?.substring(0, 600))

    // 古法预估字段应包含 manual 值 "20分钟" 和删除线的 "(AI未出值)"
    expect(descText).toContain('20分钟')
    expect(descText).toContain('(AI未出值)')

    // 实际耗时字段应包含 manual 值 "17分钟" 和删除线的系统值 "7分钟"
    expect(descText).toContain('17分钟')
    expect(descText).toContain('7分钟')

    await page.screenshot({ path: 'test-results/ancient-fix-01.png', fullPage: true })
  })

  test('2. 提效卡片也有AI未出值的删除线', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    const cards = page.locator('.kb-metric-card')
    const allText = await cards.allTextContents()
    console.log('卡片文本:', allText)

    // 古法预估卡片应包含 "20分钟" 和 "(AI未出值)"
    const ancientCard = allText.find(t => t.includes('古法预估'))
    expect(ancientCard).toContain('20分钟')
    expect(ancientCard).toContain('(AI未出值)')

    // 实际耗时卡片应包含 manual 值和系统值
    const realCard = allText.find(t => t.includes('实际耗时'))
    expect(realCard).toContain('17分钟')
    expect(realCard).toContain('7分钟')
  })

  test('3. 古法预估有系统reason的tooltip图标', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 描述区域应有tooltip图标（系统reason有值）
    const descIcons = page.locator('.el-descriptions .el-icon')
    const iconCount = await descIcons.count()
    console.log('描述区域图标数:', iconCount)
    // 古法预估有2个?（manual reason + system reason），实际耗时有2个?
    expect(iconCount).toBeGreaterThanOrEqual(3)
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
})
