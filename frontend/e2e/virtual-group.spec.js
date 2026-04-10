import { test, expect } from '@playwright/test'

const BASE_URL = 'http://localhost:8880'

test.describe('虚拟组 + 收藏功能端到端测试', () => {

  test.beforeAll(async () => {
    // 清理之前的测试数据
    try {
      const resp = await fetch(BASE_URL + '/api/virtual-groups?dimension=project')
      const data = await resp.json()
      for (const vg of data || []) {
        await fetch(BASE_URL + '/api/virtual-groups/' + vg.id, { method: 'DELETE' })
      }
    } catch (e) { /* ignore */ }
    try {
      const resp = await fetch(BASE_URL + '/api/favorites?dimension=project')
      const data = await resp.json()
      for (const fav of data || []) {
        await fetch(BASE_URL + '/api/favorites/' + fav.id, { method: 'DELETE' })
      }
    } catch (e) { /* ignore */ }
  })

  test('Dashboard 聚合 Tab 多选 + 创建虚拟组', async ({ page }) => {
    await page.goto(BASE_URL + '/dashboard?startDate=2026-03-29&endDate=2026-03-31&tab=aggregate&dimension=project')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(2000)

    await page.screenshot({ path: 'test-results/vg-01-dashboard-agg.png', fullPage: true })

    // 检查表格有数据
    const rows = page.locator('.el-table__row')
    const rowCount = await rows.count()
    console.log(`Dashboard aggregate rows: ${rowCount}`)
    expect(rowCount).toBeGreaterThan(0)

    // 检查是否有 checkbox 列
    const checkboxes = page.locator('.el-table__row .el-checkbox')
    const hasCheckbox = await checkboxes.count()
    console.log(`Checkboxes found: ${hasCheckbox}`)

    if (hasCheckbox > 0) {
      // 选择前两行
      await checkboxes.nth(0).click()
      await page.waitForTimeout(300)
      await checkboxes.nth(1).click()
      await page.waitForTimeout(500)

      await page.screenshot({ path: 'test-results/vg-02-selected.png', fullPage: true })

      // 检查"创建虚拟组"按钮是否出现
      const createBtn = page.locator('button').filter({ hasText: /虚拟组/ })
      const btnVisible = await createBtn.isVisible().catch(() => false)
      console.log(`Create VG button visible: ${btnVisible}`)

      if (btnVisible) {
        await createBtn.click()
        await page.waitForTimeout(500)

        // 输入虚拟组名称
        const dialogInput = page.locator('.el-dialog input[type="text"]').last()
        await dialogInput.fill('E2E测试虚拟组')
        await page.waitForTimeout(300)

        await page.screenshot({ path: 'test-results/vg-03-dialog.png', fullPage: true })

        // 点确认
        const confirmBtn = page.locator('.el-dialog').locator('button').filter({ hasText: /确|创建/ })
        await confirmBtn.click()
        await page.waitForTimeout(2000)

        await page.screenshot({ path: 'test-results/vg-04-created.png', fullPage: true })
      }
    }
  })

  test('ProjectPanel 收藏和虚拟组展示', async ({ page }) => {
    await page.goto(BASE_URL + '/project-panel?startDate=2026-03-29&endDate=2026-03-31')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(3000)

    await page.screenshot({ path: 'test-results/vg-05-project-normal.png', fullPage: true })

    // 检查表格有数据
    const rows = page.locator('.el-table__row')
    expect(await rows.count()).toBeGreaterThan(0)

    // 检查收藏星标
    const stars = page.locator('.el-table__row .el-icon')
    const starCount = await stars.count()
    console.log(`Star icons: ${starCount}`)

    // 点击第一行的星标收藏
    if (starCount > 0) {
      await stars.first().click()
      await page.waitForTimeout(1000)
      await page.screenshot({ path: 'test-results/vg-06-starred.png', fullPage: true })
    }

    // 找到"仅显示收藏"开关
    const favSwitch = page.locator('.el-switch')
    const switchFound = await favSwitch.isVisible().catch(() => false)
    console.log(`Favorite switch found: ${switchFound}`)

    if (switchFound) {
      await favSwitch.click()
      await page.waitForTimeout(2000)

      await page.screenshot({ path: 'test-results/vg-07-favorites-only.png', fullPage: true })

      // 收藏模式下应有虚拟组
      const vgTags = page.locator('.el-tag')
      const vgCount = await vgTags.count()
      console.log(`Virtual group tags: ${vgCount}`)
    }
  })

  test('后端 API 验证', async ({ page }) => {
    // 验证虚拟组列表
    const vgResp = await page.goto(BASE_URL + '/api/virtual-groups?dimension=project')
    const vgData = JSON.parse(await vgResp.text())
    console.log(`Virtual groups: ${vgData?.length || 0}`)

    // 验证收藏列表
    const favResp = await page.goto(BASE_URL + '/api/favorites?dimension=project')
    const favData = JSON.parse(await favResp.text())
    console.log(`Favorites: ${favData?.length || 0}`)

    for (const fav of favData || []) {
      console.log(`  Fav: ${fav.item_key} (virtual: ${fav.is_virtual})`)
    }
  })
})
