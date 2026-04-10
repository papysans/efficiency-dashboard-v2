import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)

const viewsDir = resolve(__dirname, '..')

const commitViewContent = readFileSync(resolve(viewsDir, 'CommitViewV2.vue'), 'utf-8')
const taskViewContent = readFileSync(resolve(viewsDir, 'TaskViewV2.vue'), 'utf-8')
const repoDetailContent = readFileSync(resolve(viewsDir, 'RepoDetailV2.vue'), 'utf-8')

// ============================================================
// TP-09: CommitViewV2 恰好 10 列且顺序正确
// ============================================================
describe('TP-09: CommitViewV2 恰好 10 列且顺序正确', () => {
  const propMatches = commitViewContent.match(/prop:\s*'(\w+)'/g) || []
  const props = propMatches.map(m => m.match(/prop:\s*'(\w+)'/)[1])

  it('列定义包含恰好 10 列', () => {
    expect(props.length).toBe(10)
  })

  it('列 prop 顺序正确', () => {
    expect(props).toEqual([
      'commit_id', 'commit_time', 'user_name', 'comment', 'diff_lines',
      'commit_real_minutes', 'commit_ancient_minutes', 'efficiency_ratio', 'cost', '_tokens'
    ])
  })
})

// ============================================================
// TP-10: CommitViewV2 有 #cell-commit_id 插槽且导入 fmtCost
// ============================================================
describe('TP-10: CommitViewV2 有 #cell-commit_id 插槽且导入 fmtCost', () => {
  it('模板包含 #cell-commit_id 插槽', () => {
    expect(commitViewContent).toContain('#cell-commit_id')
  })

  it('导入 fmtCost', () => {
    expect(commitViewContent).toMatch(/import\s.*fmtCost.*from\s+'@\/utils\/formatters'/)
  })

  it('cost 列使用 formatter: fmtCost', () => {
    // 验证 prop: 'cost' 附近有 formatter: fmtCost
    expect(commitViewContent).toMatch(/prop:\s*'cost'[\s\S]*?formatter:\s*fmtCost/)
  })
})

// ============================================================
// TP-11: CommitViewV2 Tokens列正确聚合 upstream + downstream
// ============================================================
describe('TP-11: CommitViewV2 Tokens列正确聚合 upstream + downstream', () => {
  it('包含 _tokens 列定义', () => {
    expect(commitViewContent).toMatch(/prop:\s*'_tokens'/)
  })

  it('_tokens formatter 包含 upstream_tokens + downstream_tokens 聚合', () => {
    // 检查 formatter 中包含 upstream_tokens 和 downstream_tokens 的加法
    expect(commitViewContent).toMatch(/\(row\.upstream_tokens\s*\|\|\s*0\)\s*\+\s*\(row\.downstream_tokens\s*\|\|\s*0\)/)
  })

  it('_tokens 列 formatter 使用 toLocaleString() 格式化', () => {
    // 找到 _tokens 列区域，检查包含 toLocaleString
    const tokensSection = commitViewContent.match(/prop:\s*'_tokens'[\s\S]*?(?=prop:|$)/)?.[0] || ''
    expect(tokensSection).toContain('toLocaleString()')
  })
})

// ============================================================
// TP-12: TaskViewV2 恰好 10 列且顺序正确
// ============================================================
describe('TP-12: TaskViewV2 恰好 10 列且顺序正确', () => {
  const propMatches = taskViewContent.match(/prop:\s*'(\w+)'/g) || []
  const props = propMatches.map(m => m.match(/prop:\s*'(\w+)'/)[1])

  it('列定义包含恰好 10 列', () => {
    expect(props.length).toBe(10)
  })

  it('列 prop 顺序正确', () => {
    expect(props).toEqual([
      'task_id', 'start_time', 'user_name', 'title', 'diff_lines',
      'task_real_minutes', 'task_ancient_minutes', 'efficiency_ratio', 'cost', '_tokens'
    ])
  })
})

// ============================================================
// TP-13: TaskViewV2 有 #cell-task_id 插槽且无旧列
// ============================================================
describe('TP-13: TaskViewV2 有 #cell-task_id 插槽且无旧列', () => {
  it('模板包含 #cell-task_id 插槽', () => {
    expect(taskViewContent).toContain('#cell-task_id')
  })

  it('不包含 work_dir 作为独立列', () => {
    expect(taskViewContent).not.toMatch(/prop:\s*'work_dir'/)
  })

  it('不包含 mode 作为独立列', () => {
    expect(taskViewContent).not.toMatch(/prop:\s*'mode'/)
  })

  it('不包含 upstream_tokens 作为独立列', () => {
    expect(taskViewContent).not.toMatch(/prop:\s*'upstream_tokens'/)
  })

  it('不包含 downstream_tokens 作为独立列', () => {
    expect(taskViewContent).not.toMatch(/prop:\s*'downstream_tokens'/)
  })
})

// ============================================================
// TP-14: TaskViewV2 cost 列使用 fmtCost、_tokens 列聚合正确
// ============================================================
describe('TP-14: TaskViewV2 cost 列使用 fmtCost、_tokens 列聚合正确', () => {
  it('导入 fmtCost', () => {
    expect(taskViewContent).toMatch(/import\s.*fmtCost.*from\s+'@\/utils\/formatters'/)
  })

  it('cost 列使用 formatter: fmtCost', () => {
    expect(taskViewContent).toMatch(/prop:\s*'cost'[\s\S]*?formatter:\s*fmtCost/)
  })

  it('_tokens 列包含 upstream + downstream 聚合逻辑', () => {
    // 找到 _tokens 列附近区域
    const tokensSection = taskViewContent.match(/prop:\s*'_tokens'[\s\S]*?(?=\n\s*\]|$)/)?.[0] || ''
    expect(tokensSection).toMatch(/upstream_tokens/)
    expect(tokensSection).toMatch(/downstream_tokens/)
  })
})

// ============================================================
// TP-15: RepoDetailV2 Commits 表恰好 10 列
// ============================================================
describe('TP-15: RepoDetailV2 Commits 表恰好 10 列', () => {
  // 提取 Commits 表格区域：从 "Commits (" 到第一个 "</el-table>"
  const commitsSection = repoDetailContent.match(/Commits\s*\([\s\S]*?<\/el-table>/)?.[0] || ''

  it('Commits 表格区域恰好 10 个 el-table-column', () => {
    const columnMatches = commitsSection.match(/<el-table-column/g) || []
    expect(columnMatches.length).toBe(10)
  })

  it('Commits 表格列标签正确', () => {
    const labelMatches = commitsSection.match(/label="([^"]+)"/g) || []
    const labels = labelMatches.map(m => m.match(/label="([^"]+)"/)[1])
    expect(labels).toEqual([
      'Commit ID', '时间', '用户', '说明', '代码行数',
      '实际耗时', '传统开发时长预估', '提效比', '费用', 'Tokens消耗'
    ])
  })
})

// ============================================================
// TP-16: RepoDetailV2 Tasks 表恰好 10 列
// ============================================================
describe('TP-16: RepoDetailV2 Tasks 表恰好 10 列', () => {
  // 提取 Tasks 表格区域：从 "Tasks (" 到对应的 "</el-table>"
  const tasksSection = repoDetailContent.match(/Tasks\s*\([\s\S]*?<\/el-table>/)?.[0] || ''

  it('Tasks 表格区域恰好 10 个 el-table-column', () => {
    const columnMatches = tasksSection.match(/<el-table-column/g) || []
    expect(columnMatches.length).toBe(10)
  })

  it('Tasks 表格列标签正确', () => {
    const labelMatches = tasksSection.match(/label="([^"]+)"/g) || []
    const labels = labelMatches.map(m => m.match(/label="([^"]+)"/)[1])
    expect(labels).toEqual([
      'Task ID', '时间', '用户', '说明', '代码行数',
      '实际耗时', '传统开发时长预估', '提效比', '费用', 'Tokens消耗'
    ])
  })
})

// ============================================================
// TP-17: RepoDetailV2 两表 minWidth 一致
// ============================================================
describe('TP-17: RepoDetailV2 两表 minWidth 一致', () => {
  const expectedMinWidths = [100, 150, 90, 200, 90, 100, 140, 90, 80, 110]

  const commitsSection = repoDetailContent.match(/Commits\s*\([\s\S]*?<\/el-table>/)?.[0] || ''
  const tasksSection = repoDetailContent.match(/Tasks\s*\([\s\S]*?<\/el-table>/)?.[0] || ''

  const commitMinWidths = (commitsSection.match(/min-width="(\d+)"/g) || []).map(m => parseInt(m.match(/min-width="(\d+)"/)[1]))
  const taskMinWidths = (tasksSection.match(/min-width="(\d+)"/g) || []).map(m => parseInt(m.match(/min-width="(\d+)"/)[1]))

  it('Commits 表 minWidth 序列正确', () => {
    expect(commitMinWidths).toEqual(expectedMinWidths)
  })

  it('Tasks 表 minWidth 序列正确', () => {
    expect(taskMinWidths).toEqual(expectedMinWidths)
  })

  it('两表 minWidth 序列完全相等', () => {
    expect(commitMinWidths).toEqual(taskMinWidths)
  })
})

// ============================================================
// TP-18: 三页面列标签一致性验证
// ============================================================

/**
 * 从 KbFilterTable columns 数组中提取列级别的 label
 * 策略：找到 columns = [ 到对应的 ] 区域，然后按 prop: 分割提取每个列的 label
 */
function extractColumnLabels(content) {
  // 提取 columns = [ ... ] 区域
  const columnsMatch = content.match(/const columns\s*=\s*\[([\s\S]*?)\n\]/)
  if (!columnsMatch) return []
  const columnsBlock = columnsMatch[1]
  // 按 "prop:" 分割，每个块代表一个列对象
  const parts = columnsBlock.split(/(?=prop:\s*')/)
  return parts
    .map(part => {
      const labelMatch = part.match(/^\s*prop:[\s\S]*?label:\s*'([^']+)'/)
      return labelMatch ? labelMatch[1] : null
    })
    .filter(Boolean)
}

describe('TP-18: 三页面列标签一致性验证', () => {
  const commitLabels = extractColumnLabels(commitViewContent)
  const taskLabels = extractColumnLabels(taskViewContent)

  // RepoDetailV2 Commits 表使用 HTML label="xxx"
  const commitsSection = repoDetailContent.match(/Commits\s*\([\s\S]*?<\/el-table>/)?.[0] || ''
  const repoCommitLabelMatches = commitsSection.match(/label="([^"]+)"/g) || []
  const repoCommitLabels = repoCommitLabelMatches.map(m => m.match(/label="([^"]+)"/)[1])

  it('CommitViewV2 标签序列正确', () => {
    expect(commitLabels).toEqual([
      'Commit ID', '时间', '用户', '说明', '代码行数',
      '实际耗时', '传统开发时长预估', '提效比', '费用', 'Tokens消耗'
    ])
  })

  it('TaskViewV2 标签序列正确', () => {
    expect(taskLabels).toEqual([
      'Task ID', '时间', '用户', '说明', '代码行数',
      '实际耗时', '传统开发时长预估', '提效比', '费用', 'Tokens消耗'
    ])
  })

  it('三页面第 2-10 列标签完全一致', () => {
    // 提取第 2-10 列标签（索引 1 到 9）
    const commitCommon = commitLabels.slice(1)
    const taskCommon = taskLabels.slice(1)
    const repoCommitCommon = repoCommitLabels.slice(1)

    expect(commitCommon).toEqual(taskCommon)
    expect(commitCommon).toEqual(repoCommitCommon)
  })
})
