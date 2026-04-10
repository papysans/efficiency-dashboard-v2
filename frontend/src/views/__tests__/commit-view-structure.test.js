import { describe, it, expect } from 'vitest'
import { readFileSync, existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)

const viewsDir = resolve(__dirname, '..')
const srcDir = resolve(__dirname, '..', '..')

const commitViewContent = readFileSync(resolve(viewsDir, 'CommitViewV2.vue'), 'utf-8')
const commitDetailContent = readFileSync(resolve(viewsDir, 'CommitDetailV2.vue'), 'utf-8')
const taskViewContent = readFileSync(resolve(viewsDir, 'TaskViewV2.vue'), 'utf-8')

// ============================================================
// 测试点 4: CommitViewV2 列定义结构验证
// ============================================================
describe('CommitViewV2 列定义结构验证', () => {
  // 提取 script 中列定义的 prop 值
  const propMatches = commitViewContent.match(/prop:\s*'(\w+)'/g) || []
  const props = propMatches.map(m => m.match(/prop:\s*'(\w+)'/)[1])

  // 4.1 恰好 10 列（统一列结构后更新）
  it('列定义包含恰好 10 列', () => {
    expect(props.length).toBe(10)
    expect(props).toEqual([
      'commit_id', 'commit_time', 'user_name', 'comment', 'diff_lines',
      'commit_real_minutes', 'commit_ancient_minutes', 'efficiency_ratio', 'cost', '_tokens'
    ])
  })

  // 4.2 每列都有 prop、label 属性
  it('每列都有 prop、label 属性', () => {
    // 使用正则匹配每个列对象块
    const columnBlocks = commitViewContent.match(/\{\s*\n?\s*prop:\s*'[^']+'/g) || []
    expect(columnBlocks.length).toBe(10)
    for (const prop of props) {
      const propRegex = new RegExp(`prop:\\s*'${prop}'`)
      expect(commitViewContent).toMatch(propRegex)
    }
    // 检查每个 prop 对应的列有 label
    expect((commitViewContent.match(/label:\s*'/g) || []).length).toBeGreaterThanOrEqual(10)
  })

  // 4.3 过滤器类型序列（统一列结构后更新：9 个带 filter 的列）
  it('过滤器类型序列正确', () => {
    const filterTypeMatches = commitViewContent.match(/filter:\s*\{\s*type:\s*'([^']+)'/g) || []
    const filterTypes = filterTypeMatches.map(m => m.match(/type:\s*'([^']+)'/)[1])
    expect(filterTypes).toEqual([
      'date', 'multi-select', 'text', 'multi-select', 'number', 'number', 'number', 'number', 'number', 'number'
    ])
  })

  // 4.4 user_name 列使用 multi-select 过滤器
  it('user_name 列使用 multi-select 过滤器', () => {
    expect(commitViewContent).toMatch(/prop:\s*'user_name'[\s\S]*?filter:\s*\{\s*type:\s*'multi-select'\s*\}/)
  })

  // 4.5 comment 列使用 text 过滤器
  it('comment 列使用 text 过滤器', () => {
    expect(commitViewContent).toMatch(/prop:\s*'comment'[\s\S]*?filter:\s*\{\s*type:\s*'text'\s*\}/)
  })

  // 4.6 commit_time 列使用 date 过滤器且有 serverSide: true
  it('commit_time 列使用 date 过滤器且有 serverSide', () => {
    expect(commitViewContent).toMatch(/prop:\s*'commit_time'[\s\S]*?filter:\s*\{\s*type:\s*'date',\s*serverSide:\s*true\s*\}/)
  })

  // 4.7 efficiency_ratio 列有 slotName
  it('efficiency_ratio 列有 slotName', () => {
    expect(commitViewContent).toMatch(/prop:\s*'efficiency_ratio'[\s\S]*?slotName:\s*'efficiency_ratio'/)
  })

  // 4.8 commit_ancient_minutes 列 filter 有 valueGetter: getEffectiveAncient
  it('commit_ancient_minutes 列 filter 有 valueGetter: getEffectiveAncient', () => {
    expect(commitViewContent).toMatch(/prop:\s*'commit_ancient_minutes'[\s\S]*?valueGetter:\s*getEffectiveAncient/)
  })

  // 4.9 commit_real_minutes 列 filter 有 valueGetter: getEffectiveReal
  it('commit_real_minutes 列 filter 有 valueGetter: getEffectiveReal', () => {
    expect(commitViewContent).toMatch(/prop:\s*'commit_real_minutes'[\s\S]*?valueGetter:\s*getEffectiveReal/)
  })
})

// ============================================================
// 测试点 6: CommitViewV2 组件使用一致性
// ============================================================
describe('CommitViewV2 组件使用一致性', () => {
  // 6.1 使用 KbFilterTable
  it('导入 KbFilterTable 组件', () => {
    expect(commitViewContent).toContain("import KbFilterTable from '@/components/KbFilterTable.vue'")
  })

  // 6.2 不导入 FilterBar
  it('不导入 FilterBar', () => {
    expect(commitViewContent).not.toContain('FilterBar')
  })

  // 6.3 不导入 useChart
  it('不导入 useChart', () => {
    expect(commitViewContent).not.toContain('useChart')
  })

  // 6.4 不导入 useUrlSync
  it('不导入 useUrlSync', () => {
    expect(commitViewContent).not.toContain('useUrlSync')
  })

  // 6.5 导入 getDefaultDateRangeWide
  it('导入 getDefaultDateRangeWide', () => {
    expect(commitViewContent).toContain("import { getDefaultDateRangeWide } from '@/utils/date'")
  })

  // 6.6 handleFilterChange 使用 commit_time，不使用 start_time
  it('handleFilterChange 读取 commit_time 而非 start_time', () => {
    expect(commitViewContent).toContain('allFilters.commit_time')
    expect(commitViewContent).not.toContain('allFilters.start_time')
  })
})

// ============================================================
// 测试点 8: CommitViewV2 导入依赖完整性
// ============================================================
describe('CommitViewV2 导入依赖完整性', () => {
  // 8.1 KbFilterTable.vue 存在
  it('@/components/KbFilterTable.vue 文件存在', () => {
    expect(existsSync(resolve(srcDir, 'components', 'KbFilterTable.vue'))).toBe(true)
  })

  // 8.2 @/api/es 存在
  it('@/api/es 文件存在', () => {
    expect(existsSync(resolve(srcDir, 'api', 'es.js'))).toBe(true)
  })

  // 8.3 @/utils/formatters 存在
  it('@/utils/formatters 文件存在', () => {
    expect(existsSync(resolve(srcDir, 'utils', 'formatters.js'))).toBe(true)
  })

  // 8.4 @/utils/date 存在
  it('@/utils/date 文件存在', () => {
    expect(existsSync(resolve(srcDir, 'utils', 'date.js'))).toBe(true)
  })

  // 额外验证：导入的具体函数在对应文件中有 export 定义
  it('getCommitsV2 在 api/es.js 中有 export 定义', () => {
    const esContent = readFileSync(resolve(srcDir, 'api', 'es.js'), 'utf-8')
    expect(esContent).toMatch(/export\s+(function|const|async\s+function)\s+getCommitsV2/)
  })

  it('formatDuration 在 utils/formatters.js 中有 export 定义', () => {
    const formattersContent = readFileSync(resolve(srcDir, 'utils', 'formatters.js'), 'utf-8')
    expect(formattersContent).toMatch(/export\s+function\s+formatDuration/)
  })

  it('getDefaultDateRangeWide 在 utils/date.js 中有 export 定义', () => {
    const dateContent = readFileSync(resolve(srcDir, 'utils', 'date.js'), 'utf-8')
    expect(dateContent).toMatch(/export\s+function\s+getDefaultDateRangeWide/)
  })
})

// ============================================================
// 测试点 9: 效率标签颜色阈值一致性
// ============================================================
describe('CommitViewV2 效率标签颜色阈值一致性', () => {
  // 9.1 列表页 el-tag 类型判断包含正确阈值
  it('列表页 el-tag 包含 300 和 150 阈值判断', () => {
    expect(commitViewContent).toMatch(/row\.efficiency_ratio\s*>=\s*300\s*\?\s*'success'/)
    expect(commitViewContent).toMatch(/row\.efficiency_ratio\s*>=\s*150\s*\?\s*'primary'/)
  })

  // 9.2 列表页与详情页阈值一致
  it('列表页与详情页使用相同的 300/150 阈值', () => {
    // 从 CommitViewV2 提取阈值
    const viewThresholds = commitViewContent.match(/efficiency_ratio\s*>=\s*(\d+)/g) || []
    const viewValues = viewThresholds.map(m => parseInt(m.match(/(\d+)/g).pop()))

    // 从 commit-helpers.js 读取（因为详情页现在使用 getEfficiencyColor）
    const helpersContent = readFileSync(resolve(srcDir, 'utils', 'commit-helpers.js'), 'utf-8')
    const helperThresholds = helpersContent.match(/ratio\s*>=\s*(\d+)/g) || []
    const helperValues = helperThresholds.map(m => parseInt(m.match(/(\d+)/g).pop()))

    // 两组阈值应该一致
    expect(viewValues).toContain(300)
    expect(viewValues).toContain(150)
    expect(helperValues).toContain(300)
    expect(helperValues).toContain(150)
  })
})

// ============================================================
// 测试点 10: CommitViewV2 与 TaskViewV2 模式对齐验证
// ============================================================
describe('CommitViewV2 与 TaskViewV2 模式对齐验证', () => {
  // 10.1 KbFilterTable 绑定事件
  it('KbFilterTable 绑定了所有必要事件', () => {
    expect(commitViewContent).toContain('@row-click')
    expect(commitViewContent).toContain('@size-change')
    expect(commitViewContent).toContain('@page-change')
    expect(commitViewContent).toContain('@filter-change')
  })

  // 10.2 KbFilterTable v-model 属性
  it('KbFilterTable 使用 v-model:page 和 v-model:pageSize', () => {
    expect(commitViewContent).toContain('v-model:page')
    expect(commitViewContent).toContain('v-model:pageSize')
  })

  // 10.3 efficiency_ratio 插槽模式与 TaskViewV2 结构相同
  it('efficiency_ratio 插槽模式与 TaskViewV2 一致', () => {
    // 两者都使用 #cell-efficiency_ratio 插槽
    expect(commitViewContent).toContain('#cell-efficiency_ratio')
    expect(taskViewContent).toContain('#cell-efficiency_ratio')

    // 两者都使用 toFixed(1)% 格式
    expect(commitViewContent).toContain('.toFixed(1)')
    expect(taskViewContent).toContain('.toFixed(1)')

    // 两者都使用 el-tag 组件
    expect(commitViewContent).toContain('<el-tag')
    expect(taskViewContent).toContain('<el-tag')
  })
})
