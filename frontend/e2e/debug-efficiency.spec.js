import { test, expect } from '@playwright/test'

const BASE_URL = 'http://localhost:8880'

test('调试提效分析页面查询流程', async ({ page }) => {
  // 监听所有网络请求
  const apiCalls = []
  page.on('request', req => {
    if (req.url().includes('/api/')) {
      apiCalls.push({ method: req.method(), url: req.url(), body: req.postData() })
    }
  })
  page.on('response', async resp => {
    if (resp.url().includes('/api/')) {
      const status = resp.status()
      let body = ''
      try { body = await resp.text() } catch(e) {}
      console.log(`[API] ${resp.request().method()} ${resp.url()} → ${status} (${body.length} chars)`)
      if (body.length < 2000) console.log(`  Response: ${body}`)
    }
  })

  // 监听 console.error
  page.on('console', msg => {
    if (msg.type() === 'error') console.log(`[Console Error] ${msg.text()}`)
  })

  await page.goto(BASE_URL + '/efficiency')
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(2000)

  // 截图初始状态
  await page.screenshot({ path: 'test-results/debug-01-initial.png', fullPage: true })

  // 1. 先确认日期范围有值
  const dateInputs = page.locator('.el-range-input')
  const startVal = await dateInputs.first().inputValue()
  const endVal = await dateInputs.last().inputValue()
  console.log(`[Debug] Date range: ${startVal} ~ ${endVal}`)

  // 2. 点开下拉选择
  const select = page.locator('.el-select').first()
  await select.click()
  await page.waitForTimeout(1500)

  // 3. 获取所有选项
  const options = page.locator('.el-select-dropdown__item')
  const count = await options.count()
  console.log(`[Debug] Options count: ${count}`)
  
  if (count > 0) {
    // 打印前 5 个选项文本
    for (let i = 0; i < Math.min(5, count); i++) {
      const text = await options.nth(i).textContent()
      console.log(`[Debug] Option ${i}: "${text}"`)
    }

    // 4. 选择第一个选项
    const firstOptionText = await options.first().textContent()
    console.log(`[Debug] Selecting: "${firstOptionText}"`)
    await options.first().click()
    await page.waitForTimeout(1000)

    // 5. 检查选中的值
    const selectedInput = select.locator('input').first()
    const selectedVal = await selectedInput.inputValue()
    console.log(`[Debug] Selected value: "${selectedVal}"`)

    // 6. 截图选中后状态
    await page.screenshot({ path: 'test-results/debug-02-selected.png', fullPage: true })

    // 7. 点击查询按钮
    const queryBtn = page.locator('button').filter({ hasText: '查询' })
    if (await queryBtn.isVisible()) {
      console.log('[Debug] Clicking query button...')
      await queryBtn.click()
      await page.waitForTimeout(3000)
    } else {
      console.log('[Debug] No query button found, waiting for auto-query...')
      await page.waitForTimeout(3000)
    }

    // 8. 截图查询结果
    await page.screenshot({ path: 'test-results/debug-03-result.png', fullPage: true })

    // 9. 检查页面上是否有指标卡片出现
    const metricCards = page.locator('.kb-metric-card')
    const cardCount = await metricCards.count()
    console.log(`[Debug] Metric cards count: ${cardCount}`)

    // 10. 检查是否有错误提示
    const emptyState = page.locator('.el-empty')
    const hasEmpty = await emptyState.isVisible().catch(() => false)
    console.log(`[Debug] Empty state visible: ${hasEmpty}`)
    if (hasEmpty) {
      const emptyText = await emptyState.textContent()
      console.log(`[Debug] Empty text: ${emptyText}`)
    }
  }

  // 打印所有 API 调用
  console.log('\n[Debug] All API calls:')
  for (const call of apiCalls) {
    console.log(`  ${call.method} ${call.url}`)
  }
})
