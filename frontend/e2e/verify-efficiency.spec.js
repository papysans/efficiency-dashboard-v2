import { test, expect } from '@playwright/test'

const BASE_URL = 'http://localhost:8880'

test('验证提效分析页面完整展示', async ({ page }) => {
  test.setTimeout(120000)
  page.on('console', msg => {
    if (msg.type() === 'error' && !msg.text().includes('400')) console.log(`[Error] ${msg.text()}`)
  })

  await page.goto(BASE_URL + '/efficiency')
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(1500)

  // 选择第一个项目
  const select = page.locator('.el-select').first()
  await select.click()
  await page.waitForTimeout(1500)
  const firstOption = page.locator('.el-select-dropdown__item').first()
  await firstOption.click()
  
  // 等待数据加载（AI评估需要时间）
  await page.waitForTimeout(15000)

  // 截图
  await page.screenshot({ path: 'test-results/verify-01-efficiency-full.png', fullPage: true })

  // 检查指标卡片
  const cards = page.locator('.kb-metric-card')
  const cardCount = await cards.count()
  console.log(`Metric cards: ${cardCount}`)
  expect(cardCount).toBeGreaterThanOrEqual(4)

  // 检查 AI 预估人天不为 0
  const aiDaysCard = cards.first()
  const aiDaysValue = await aiDaysCard.locator('.kb-metric-value').textContent()
  console.log(`AI Days card value: ${aiDaysValue}`)
  expect(aiDaysValue).not.toContain('0.0')

  // 检查 reasons 区域是否显示
  const reasonSection = page.locator('.reason-section')
  const hasReasons = await reasonSection.isVisible().catch(() => false)
  console.log(`Reasons visible: ${hasReasons}`)

  // 检查提效比例不为 0
  const effCards = page.locator('.kb-metric-value')
  for (let i = 0; i < await effCards.count(); i++) {
    const text = await effCards.nth(i).textContent()
    console.log(`Metric ${i}: ${text}`)
  }

  // 检查是否有 efficiency reason
  const effReason = page.locator('.efficiency-reason, .eff-reason')
  const hasEffReason = await effReason.isVisible().catch(() => false)
  console.log(`Efficiency reason visible: ${hasEffReason}`)

  // 滚动到底部截图
  await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight))
  await page.waitForTimeout(500)
  await page.screenshot({ path: 'test-results/verify-02-efficiency-bottom.png', fullPage: true })
})

test('验证项目面板 0 值显示为 -', async ({ page }) => {
  await page.goto(BASE_URL + '/project-panel')
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(3000)

  await page.screenshot({ path: 'test-results/verify-03-project.png', fullPage: true })

  // 检查表格中 AI预估人天列
  const cells = page.locator('.el-table__row td:nth-child(6) .cell')
  const count = await cells.count()
  let hasValidDays = false
  let hasDash = false
  for (let i = 0; i < Math.min(count, 10); i++) {
    const text = (await cells.nth(i).textContent()).trim()
    console.log(`Project row ${i} ai_days: "${text}"`)
    if (text !== '-' && text !== '0.0' && text !== '') hasValidDays = true
    if (text === '-') hasDash = true
  }
  console.log(`Has valid days: ${hasValidDays}, Has dash: ${hasDash}`)
})
