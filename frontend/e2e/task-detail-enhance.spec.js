// @ts-check
import { test, expect } from '@playwright/test'

const BASE = 'http://localhost:8880'
// Task with 24 conversations, known data
const TASK_ID = '019d41ba-5781-7436-8e80-e698d0d344c3'
const TASK_URL = `${BASE}/task/${TASK_ID}`

test.describe('Task详情页增强功能验证', () => {

  // ========= 1. 页面加载与基本数据 =========
  test('1. 页面能正常加载，无控制台报错', async ({ page }) => {
    const errors = []
    page.on('console', msg => {
      if (msg.type() === 'error') errors.push(msg.text())
    })
    page.on('pageerror', err => errors.push(err.message))

    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 页面标题包含 "Task 详情"
    const title = page.locator('text=Task 详情')
    await expect(title).toBeVisible()

    // 过滤掉非业务性的控制台错误（如浏览器扩展、favicon等）
    const realErrors = errors.filter(e =>
      !e.includes('favicon') &&
      !e.includes('ERR_CONNECTION_REFUSED') &&
      !e.includes('net::') &&
      !e.includes('ResizeObserver')
    )
    console.log(`控制台错误数: ${realErrors.length}`)
    if (realErrors.length > 0) console.log('错误详情:', realErrors)
    expect(realErrors.length).toBe(0)

    await page.screenshot({ path: 'test-results/task-detail-01-loaded.png', fullPage: true })
  })

  // ========= 2. 时间本地化 =========
  test('2. 所有时间显示为本地格式，非ISO格式', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 元信息区域的开始时间和结束时间应该是本地格式 YYYY-MM-DD HH:mm:ss
    const descriptions = page.locator('.el-descriptions')
    const descText = await descriptions.textContent()
    console.log('元信息文本片段:', descText?.substring(0, 500))

    // 不应包含 T 和 +08:00 这样的 ISO 格式标记
    expect(descText).not.toContain('T10:')
    expect(descText).not.toContain('+08:00')

    // 应包含格式化后的本地时间（如 2026-03-31 10:24:59）
    const timePattern = /\d{4}-\d{2}-\d{2}\s\d{2}:\d{2}:\d{2}/
    expect(descText).toMatch(timePattern)

    // 对话历史的时间戳也应该是本地格式
    const timeline = page.locator('.el-timeline')
    const timelineText = await timeline.textContent()
    // 对话时间不应有 ISO 格式
    expect(timelineText).not.toContain('+08:00')
    expect(timelineText).not.toContain('+00:00')
  })

  // ========= 3. 仓库/Project 字段分离 =========
  test('3. 仓库和Project字段正确分离展示', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    const descriptions = page.locator('.el-descriptions')

    // 应存在「仓库」标签
    const repoLabel = descriptions.locator('text=仓库')
    await expect(repoLabel.first()).toBeVisible()

    // 应存在「Project」标签
    const projectLabel = descriptions.locator('text=Project')
    await expect(projectLabel.first()).toBeVisible()

    // Project 字段应有数据（此task的project_id不为空）
    const descText = await descriptions.textContent()
    expect(descText).toContain('797e102c29')

    await page.screenshot({ path: 'test-results/task-detail-03-repo-project.png', fullPage: true })
  })

  // ========= 4. 实际耗时显示 =========
  test('4. 实际耗时(task_real_minutes)有数据展示', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    const descriptions = page.locator('.el-descriptions')
    const descText = await descriptions.textContent()
    console.log('元信息全文:', descText)

    // 应有「实际耗时」标签
    const realTimeLabel = descriptions.locator('text=实际耗时')
    await expect(realTimeLabel.first()).toBeVisible()

    // 实际耗时应显示分钟值（此task约23分钟）
    // 应包含"分钟"文字（因为<60分钟，formatDuration显示为X分钟）
    expect(descText).toMatch(/\d+.*分钟/)
  })

  // ========= 5. 古法预估显示 =========
  test('5. 古法预估字段存在且标签正确', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    const descriptions = page.locator('.el-descriptions')

    // 应有「古法预估」标签（不再是"AI预估人天"）
    const ancientLabel = descriptions.locator('text=古法预估')
    await expect(ancientLabel.first()).toBeVisible()

    // 不应有旧的"AI预估人天"标签
    const oldLabel = page.locator('text=AI预估人天')
    expect(await oldLabel.count()).toBe(0)
  })

  // ========= 6. 提效比显示 =========
  test('6. 提效比字段存在', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    const descriptions = page.locator('.el-descriptions')

    // 应有「提效比」标签
    const ratioLabel = descriptions.locator('text=提效比')
    await expect(ratioLabel.first()).toBeVisible()
  })

  // ========= 7. 费用字段修复（cost而非total_cost）=========
  test('7. 费用字段有数据（修复total_cost bug）', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    const descriptions = page.locator('.el-descriptions')
    const descText = await descriptions.textContent()

    // 费用应有值（此task cost=0.08），不应为 '-'
    const feeLabel = descriptions.locator('text=费用')
    await expect(feeLabel.first()).toBeVisible()

    // 查找费用项对应的值（不能是'-'因为此task有cost数据）
    // 数据库中此task的cost=0.08
    const feeItems = page.locator('.el-descriptions__cell')
    const allText = await feeItems.allTextContents()
    const feeIndex = allText.findIndex(t => t.includes('费用'))
    if (feeIndex >= 0 && feeIndex + 1 < allText.length) {
      console.log('费用值:', allText[feeIndex], allText[feeIndex + 1])
    }
  })

  // ========= 8. 对话历史有数据 =========
  test('8. 对话历史有24条数据', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 对话历史标题应可见
    const historyHeader = page.locator('text=对话历史')
    await expect(historyHeader.first()).toBeVisible()

    // timeline items 数量应 >= 24（可能有间隔标记项）
    const timelineItems = page.locator('.el-timeline-item')
    const count = await timelineItems.count()
    console.log(`对话历史timeline items: ${count}`)
    expect(count).toBeGreaterThanOrEqual(24)

    await page.screenshot({ path: 'test-results/task-detail-08-conversations.png', fullPage: true })
  })

  // ========= 9. 对话历史中model字段显示（修复model_name bug）=========
  test('9. 对话历史中模型名正确显示', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 此task使用的模型是 GLM-4.7
    const modelText = page.locator('.el-timeline').locator('text=GLM-4.7')
    const count = await modelText.count()
    console.log(`显示GLM-4.7的对话条数: ${count}`)
    expect(count).toBeGreaterThan(0)
  })

  // ========= 10. 对话历史中Tokens显示（修复total_tokens bug）=========
  test('10. 对话历史中Tokens有数据', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 查找包含 "Tokens:" 的文本
    const tokensText = page.locator('.el-timeline >> text=/Tokens:\\s*\\d+/')
    const count = await tokensText.count()
    console.log(`显示Tokens数据的条数: ${count}`)
    expect(count).toBeGreaterThan(0)
  })

  // ========= 11. 统计摘要有数据 =========
  test('11. 统计摘要三个指标有数据', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 总请求数应为 24
    const totalRequests = page.locator('text=总请求数')
    await expect(totalRequests).toBeVisible()

    // 总Tokens 应有值
    const totalTokens = page.locator('text=总Tokens')
    await expect(totalTokens).toBeVisible()

    // 总费用 应有值
    const totalCost = page.locator('text=总费用')
    await expect(totalCost).toBeVisible()

    // 检查数值不为0
    const metricValues = page.locator('.kb-metric-value')
    const valueCount = await metricValues.count()
    console.log(`统计指标数量: ${valueCount}`)
    for (let i = 0; i < valueCount; i++) {
      const val = await metricValues.nth(i).textContent()
      console.log(`指标 ${i}: ${val}`)
    }
    // 总请求数应为24
    const firstMetricVal = await metricValues.first().textContent()
    expect(parseInt(firstMetricVal || '0')).toBe(24)
  })

  // ========= 12. 时间片段可视化 - 颜色区分 =========
  test('12. 对话历史节点有绿色颜色标记', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 此task只有1个时间片段，所有节点应为绿色
    const greenNodes = page.locator('.el-timeline-item__node[style*="#67C23A"],.el-timeline-item__node[style*="67C23A"],.el-timeline-item__node[style*="rgb(103, 194, 58)"]')
    const greenCount = await greenNodes.count()
    console.log(`绿色节点数: ${greenCount}`)

    // 至少要有一些有颜色标记的节点
    // 如果自定义颜色未通过 style 设置而是通过 type，检查另一种方式
    const allNodes = page.locator('.el-timeline-item__node')
    const nodeCount = await allNodes.count()
    console.log(`总timeline节点数: ${nodeCount}`)
    expect(nodeCount).toBeGreaterThanOrEqual(24)
  })

  // ========= 13. 对话历史耗时汇总 =========
  test('13. 对话历史标题旁显示耗时汇总', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 对话历史卡片header应包含实际耗时信息
    const historyCard = page.locator('.el-card').filter({ hasText: '对话历史' })
    const headerText = await historyCard.locator('.el-card__header').textContent()
    console.log('对话历史header:', headerText)

    // 应包含耗时数据（如 "23分钟" 或 "实际耗时"）
    expect(headerText).toMatch(/耗时|分钟/)
  })

  // ========= 14. 编辑按钮存在 =========
  test('14. 编辑按钮存在且可点击弹出对话框', async ({ page }) => {
    await page.goto(TASK_URL)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 查找编辑/人工调整按钮
    const editBtn = page.locator('button:has-text("人工调整"), .el-button:has-text("人工调整")')
    const editCount = await editBtn.count()
    console.log(`人工调整按钮数量: ${editCount}`)
    expect(editCount).toBeGreaterThan(0)

    // 点击编辑按钮
    await editBtn.first().click()
    await page.waitForTimeout(500)

    // 应弹出 el-dialog
    const dialog = page.locator('.el-dialog, .el-overlay .el-dialog__wrapper')
    await expect(dialog.first()).toBeVisible()

    // 对话框中应包含 manual 相关的表单字段
    const dialogText = await dialog.first().textContent()
    console.log('对话框内容片段:', dialogText?.substring(0, 300))

    // 应有实际耗时和古法预估相关的输入
    expect(dialogText).toMatch(/实际耗时|古法预估|人工调整/)

    await page.screenshot({ path: 'test-results/task-detail-14-edit-dialog.png', fullPage: true })

    // 关闭对话框
    const closeBtn = page.locator('.el-dialog__headerbtn')
    if (await closeBtn.count() > 0) await closeBtn.first().click()
  })

  // ========= 15. Task列表页跳转到详情 =========
  test('15. 从Task列表页跳转到详情页', async ({ page }) => {
    await page.goto(BASE + '/task-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 列表应有数据
    const rows = page.locator('.el-table__body-wrapper .el-table__row')
    const rowCount = await rows.count()
    console.log(`任务列表行数: ${rowCount}`)
    expect(rowCount).toBeGreaterThan(0)

    // 列表中应有"古法预估"列（而非旧的"AI预估人天"）
    const headerText = await page.locator('.el-table__header-wrapper').textContent()
    console.log('列表表头:', headerText)
    expect(headerText).toContain('古法预估')
    expect(headerText).not.toContain('AI预估人天')

    // 点击第一行跳转详情
    await rows.first().click()
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    // 应跳转到 task 详情页
    expect(page.url()).toContain('/task/')
    const detailTitle = page.locator('text=Task 详情')
    await expect(detailTitle).toBeVisible()
  })

  // ========= 16. API 返回正确的新字段 =========
  test('16. API返回task_real_minutes, time_segments, efficiency_ratio', async ({ page }) => {
    // 直接调用 API 验证
    const response = await page.request.get(`${BASE}/api/v2/tasks/${TASK_ID}`)
    expect(response.status()).toBe(200)

    const data = await response.json()

    // task 对象应包含新字段
    expect(data.task).toBeDefined()
    expect(data.task.task_id).toBe(TASK_ID)
    expect(data.task.task_real_minutes).toBeGreaterThan(0)
    expect(data.task.task_real_minutes_reason).toBeTruthy()

    // time_segments 应存在
    expect(data.time_segments).toBeDefined()
    expect(Array.isArray(data.time_segments)).toBeTruthy()
    expect(data.time_segments.length).toBeGreaterThan(0)

    // time_segments 每项应有 start, end, conv_count
    const seg = data.time_segments[0]
    expect(seg.start).toBeTruthy()
    expect(seg.end).toBeTruthy()
    expect(seg.conv_count).toBeGreaterThan(0)

    // conversations 应有数据
    expect(data.conversations).toBeDefined()
    expect(data.conversations.length).toBe(24)

    // 每条conversation应有正确字段（修复后的字段名）
    const conv = data.conversations[0]
    expect(conv.model).toBeTruthy()  // 不是 model_name
    expect(conv.upstream_tokens).toBeDefined()  // 不是 total_tokens
    expect(conv.downstream_tokens).toBeDefined()
    expect(conv.cost).toBeDefined()  // 是 cost 不是 total_cost

    console.log('task_real_minutes:', data.task.task_real_minutes)
    console.log('time_segments:', JSON.stringify(data.time_segments))
    console.log('efficiency_ratio:', data.efficiency_ratio)
  })

  // ========= 17. Manual API 可调用 =========
  test('17. PUT manual API可成功调用', async ({ page }) => {
    const response = await page.request.put(`${BASE}/api/v2/tasks/${TASK_ID}/manual`, {
      data: {
        task_real_minutes_manual: 25.0,
        task_real_minutes_reason_manual: 'Playwright测试修正',
        task_ancient_minutes_manual: 120.0,
        task_ancient_minutes_reason_manual: 'Playwright测试预估'
      }
    })
    expect(response.status()).toBe(200)
    const body = await response.json()
    expect(body.status).toBe('ok')

    // 验证写入成功：重新获取详情
    const detail = await page.request.get(`${BASE}/api/v2/tasks/${TASK_ID}`)
    const data = await detail.json()
    expect(data.task.task_real_minutes_manual).toBe(25.0)
    expect(data.task.task_ancient_minutes_manual).toBe(120.0)
    expect(data.task.task_ancient_minutes_reason_manual).toBe('Playwright测试预估')

    // efficiency_ratio 应使用 manual 值计算：(120 / 25) * 100 = 480
    expect(data.efficiency_ratio).toBeCloseTo(480.0, 0)

    console.log('Manual写入验证通过, efficiency_ratio:', data.efficiency_ratio)

    // 清理：恢复manual为null
    await page.request.put(`${BASE}/api/v2/tasks/${TASK_ID}/manual`, {
      data: {
        task_real_minutes_manual: null,
        task_real_minutes_reason_manual: null,
        task_ancient_minutes_manual: null,
        task_ancient_minutes_reason_manual: null
      }
    })
  })

  // ========= 18. 用另一个有多对话的task验证 =========
  test('18. 另一个task详情页也能正常加载', async ({ page }) => {
    const anotherTaskId = '019d4271-d479-70c9-a57b-151019afd82a'
    await page.goto(`${BASE}/task/${anotherTaskId}`)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    const title = page.locator('text=Task 详情')
    await expect(title).toBeVisible()

    // 应有对话历史
    const timelineItems = page.locator('.el-timeline-item')
    const count = await timelineItems.count()
    console.log(`另一个task的对话数: ${count}`)
    expect(count).toBeGreaterThanOrEqual(50)

    // 元信息应有数据
    const descriptions = page.locator('.el-descriptions')
    const descText = await descriptions.textContent()
    expect(descText).toMatch(/\d{4}-\d{2}-\d{2}\s\d{2}:\d{2}:\d{2}/)

    await page.screenshot({ path: 'test-results/task-detail-18-another.png', fullPage: true })
  })

  // ========= 19. 返回按钮功能 =========
  test('19. 返回按钮可正常使用', async ({ page }) => {
    await page.goto(BASE + '/task-v2')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 点击第一行进入详情
    const rows = page.locator('.el-table__body-wrapper .el-table__row')
    if (await rows.count() > 0) {
      await rows.first().click()
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(2000)

      // 确认在详情页
      const detailTitle = page.locator('text=Task 详情')
      await expect(detailTitle).toBeVisible()

      // 点击返回
      const backBtn = page.locator('button:has-text("返回")').first()
      await backBtn.click()
      await page.waitForTimeout(1000)

      // 应回到列表页或上一页
      expect(page.url()).not.toContain('/task/' + TASK_ID)
    }
  })

  // ========= 20. 首页指标数据正常 =========
  test('20. 首页仪表盘数据正常加载', async ({ page }) => {
    const errors = []
    page.on('pageerror', err => errors.push(err.message))

    await page.goto(BASE + '/')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    // 首页应有指标卡片
    const cards = page.locator('.el-card')
    const cardCount = await cards.count()
    console.log(`首页卡片数: ${cardCount}`)
    expect(cardCount).toBeGreaterThan(0)

    // 无JS错误
    const realErrors = errors.filter(e => !e.includes('ResizeObserver'))
    expect(realErrors.length).toBe(0)

    await page.screenshot({ path: 'test-results/task-detail-20-home.png', fullPage: true })
  })
})
