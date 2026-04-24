// @ts-check
import { test, expect } from '@playwright/test'

const BASE = 'http://localhost:8880'
const TASK_URL = `${BASE}/task/019d4295-0dad-7031-8621-92f051a9b632`

test.describe('古法预估数据恢复验证', () => {

  test('1. 古法预估有系统值，不再显示"AI未出值"', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    const desc = page.locator('.el-descriptions')
    const descText = await desc.textContent()
    console.log('描述文本:', descText?.substring(0, 600))

    // 不应再有"AI未出值"
    expect(descText).not.toContain('AI未出值')

    // 古法预估应显示有值（manual=20分钟 + 系统值=2640分钟=5.5人天）
    expect(descText).toContain('20分钟')
    // 系统值5.5人天 = 2640分钟，formatDuration 显示为 "5.5人天"
    expect(descText).toMatch(/5\.5人天/)

    await page.screenshot({ path: 'test-results/recover-01.png', fullPage: true })
  })

  test('2. 提效卡片也显示系统值', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    const cards = page.locator('.kb-metric-card')
    const allText = await cards.allTextContents()
    console.log('卡片文本:', allText)

    const ancientCard = allText.find(t => t.includes('古法预估'))
    expect(ancientCard).not.toContain('AI未出值')
    expect(ancientCard).toContain('20分钟')
    expect(ancientCard).toMatch(/5\.5人天/)
  })

  test('3. 另一个task(019d41ba)古法预估也有值了', async ({ page }) => {
    await page.goto(`${BASE}/task/019d41ba-5781-7436-8e80-e698d0d344c3`)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    const desc = page.locator('.el-descriptions')
    const descText = await desc.textContent()
    console.log('019d41ba描述:', descText?.substring(0, 400))

    // 这个task的古法预估应该有值（之前上次从reason中提取到了4.5人天=2160分钟）
    expect(descText).not.toContain('AI未出值')
    expect(descText).toMatch(/人天|小时|分钟/)
  })

  test('4. 页面无报错', async ({ page }) => {
    const errors = []
    page.on('pageerror', err => errors.push(err.message))
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)
    expect(errors.filter(e => !e.includes('ResizeObserver')).length).toBe(0)
  })
})
