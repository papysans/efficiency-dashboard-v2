import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)

const viewsDir = resolve(__dirname, '..')
const componentsDir = resolve(__dirname, '..', '..', 'components')

const taskViewContent = readFileSync(resolve(viewsDir, 'TaskViewV2.vue'), 'utf-8')
const taskUserReportContent = readFileSync(resolve(viewsDir, 'TaskUserReport.vue'), 'utf-8')
const kbFilterTableContent = readFileSync(resolve(componentsDir, 'KbFilterTable.vue'), 'utf-8')

// ============================================================
// TP-04: TaskViewV2 包含 org_display 列且使用 cascade-org 筛选
// ============================================================
describe('TP-04: TaskViewV2 org 列配置', () => {
  it('包含 org_display 列定义', () => {
    expect(taskViewContent).toMatch(/prop:\s*'org_display'/)
  })

  it('org_display 列使用 cascade-org 筛选类型', () => {
    expect(taskViewContent).toMatch(/filter:\s*\{\s*type:\s*'cascade-org'\s*\}/)
  })

  it('包含 #cell-org_display 自定义插槽', () => {
    expect(taskViewContent).toContain('#cell-org_display')
  })

  it('org_display 插槽拼接 org1~org4', () => {
    // 验证模板中包含 [row.org1, row.org2, row.org3, row.org4].filter(Boolean).join('/')
    expect(taskViewContent).toContain("row.org1, row.org2, row.org3, row.org4")
    expect(taskViewContent).toContain(".filter(Boolean).join('/')")
  })
})

// ============================================================
// TP-05: KbFilterTable 支持 cascade-org 筛选类型
// ============================================================
describe('TP-05: KbFilterTable cascade-org 筛选支持', () => {
  it('模板包含 cascade-org 类型判断', () => {
    expect(kbFilterTableContent).toContain("cascade-org")
  })

  it('cascade-org 筛选渲染 org1-org4 四个下拉框', () => {
    // 检查 cascade-org 区域有 4 个级联 select
    expect(kbFilterTableContent).toContain('cascadeOrg.org1.value')
    expect(kbFilterTableContent).toContain('cascadeOrg.org2.value')
    expect(kbFilterTableContent).toContain('cascadeOrg.org3.value')
    expect(kbFilterTableContent).toContain('cascadeOrg.org4.value')
  })

  it('cascade-org 筛选逻辑按 org1-org4 逐级匹配', () => {
    // filteredData 计算属性中包含 cascade-org 的过滤逻辑
    expect(kbFilterTableContent).toContain("f.type === 'cascade-org'")
    expect(kbFilterTableContent).toContain('orgFilter.org1 && row.org1 !== orgFilter.org1')
    expect(kbFilterTableContent).toContain('orgFilter.org2 && row.org2 !== orgFilter.org2')
    expect(kbFilterTableContent).toContain('orgFilter.org3 && row.org3 !== orgFilter.org3')
    expect(kbFilterTableContent).toContain('orgFilter.org4 && row.org4 !== orgFilter.org4')
  })

  it('cascade-org 筛选标签显示 org 路径', () => {
    // activeFilterTags 中对 cascade-org 的 display 是 parts.join('/')
    expect(kbFilterTableContent).toMatch(/cascade-org.*parts\.join\('\/'\)/s)
  })
})

// ============================================================
// TP-06: TaskUserReport 页面结构完整性
// ============================================================
describe('TP-06: TaskUserReport 页面结构', () => {
  it('包含 DateRangePicker 组件', () => {
    expect(taskUserReportContent).toContain('DateRangePicker')
  })

  it('包含 4 级组织筛选下拉框', () => {
    expect(taskUserReportContent).toContain('filterOrg1')
    expect(taskUserReportContent).toContain('filterOrg2')
    expect(taskUserReportContent).toContain('filterOrg3')
    expect(taskUserReportContent).toContain('filterOrg4')
  })

  it('包含 6 个汇总指标卡', () => {
    expect(taskUserReportContent).toContain('总Task数')
    expect(taskUserReportContent).toContain('总代码行数')
    expect(taskUserReportContent).toContain('总传统耗时')
    expect(taskUserReportContent).toContain('总实际耗时')
    expect(taskUserReportContent).toContain('总费用')
    expect(taskUserReportContent).toContain('平均提效比')
  })

  it('包含 6 个图表 ref', () => {
    expect(taskUserReportContent).toContain('chart1Ref')
    expect(taskUserReportContent).toContain('chart2Ref')
    expect(taskUserReportContent).toContain('chart3Ref')
    expect(taskUserReportContent).toContain('chart4Ref')
    expect(taskUserReportContent).toContain('chart5Ref')
    expect(taskUserReportContent).toContain('chart6Ref')
  })

  it('调用 getUsersV2 获取数据', () => {
    expect(taskUserReportContent).toContain('getUsersV2')
  })

  it('调用 getOrgV2 加载组织选项', () => {
    expect(taskUserReportContent).toContain('getOrgV2')
  })

  it('avgEfficiencyRatio 计算逻辑正确（ancient/real*100）', () => {
    expect(taskUserReportContent).toMatch(/ancient\s*\/\s*real\)\s*\*\s*100/)
  })
})

// ============================================================
// TP-07: 路由配置验证
// ============================================================
describe('TP-07: task-v2 路由配置', () => {
  const routerDir = resolve(__dirname, '..', '..', 'router')
  const routerContent = readFileSync(resolve(routerDir, 'index.js'), 'utf-8')

  it('包含 /task-v2/report/user 路由', () => {
    expect(routerContent).toContain('/task-v2/report/user')
  })

  it('路由指向 TaskUserReport 组件', () => {
    expect(routerContent).toContain('TaskUserReport')
  })
})
