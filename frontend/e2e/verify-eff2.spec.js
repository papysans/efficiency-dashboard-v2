import { test, expect } from '@playwright/test'

const BASE_URL = 'http://localhost:8880'

test('验证提效分析页面', async ({ page }) => {
  test.setTimeout(120000)
  page.on('response', async resp => {
    if (resp.url().includes('/api/analysis/efficiency') && !resp.url().includes('keys')) {
      console.log(`[API] ${resp.url()} → ${resp.status()}`)
    }
  })

  await page.goto(BASE_URL + '/efficiency')
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(2000)

  // 选择第一个项目
  const select = page.locator('.el-select').first()
  await select.click()
  await page.waitForTimeout(2000)

  const options = page.locator('.el-select-dropdown__item')
  const count = await options.count()
  console.log(`Options: ${count}`)
  if (count === 0) {
    console.log('No options - skipping')
    return
  }

  await options.first().click()
  await page.waitForTimeout(1000)

  // 检查 select 的值
  const inputVal = await page.locator('.el-select input').first().inputValue()
  console.log(`Selected input value: "${inputVal}"`)

  // 点查询按钮
  const queryBtn = page.locator('button').filter({ hasText: '查询' })
  await queryBtn.click()

  // 等待较长时间（AI评估）
  console.log('Waiting for AI assessment...')
  await page.waitForTimeout(30000)

  await page.screenshot({ path: 'test-results/verify-eff-final.png', fullPage: true })

  const cards = page.locator('.kb-metric-card')
  const cardCount = await cards.count()
  console.log(`Metric cards: ${cardCount}`)

  // 打印所有 metric 值
  const metricValues = page.locator('.kb-metric-value')
  for (let i = 0; i < await metricValues.count(); i++) {
    console.log(`Metric ${i}: ${await metricValues.nth(i).textContent()}`)
  }
})
