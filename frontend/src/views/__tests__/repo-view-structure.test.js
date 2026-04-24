import { describe, it, expect } from 'vitest'
import { readFileSync, existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)

const viewsDir = resolve(__dirname, '..')
const srcDir = resolve(__dirname, '..', '..')

const repoViewContent = readFileSync(resolve(viewsDir, 'RepoViewV2.vue'), 'utf-8')
const repoDetailContent = readFileSync(resolve(viewsDir, 'RepoDetailV2.vue'), 'utf-8')
const taskViewContent = readFileSync(resolve(viewsDir, 'TaskViewV2.vue'), 'utf-8')
const routerContent = readFileSync(resolve(srcDir, 'router', 'index.js'), 'utf-8')

// ============================================================
// 测试点 6: RepoViewV2 列定义：恰好 8 列且 props 正确
// ============================================================
describe('RepoViewV2 列定义结构验证', () => {
  const propMatches = repoViewContent.match(/prop:\s*'(\w+)'/g) || []
  const props = propMatches.map(m => m.match(/prop:\s*'(\w+)'/)[1])

  // 6.1 恰好 8 列
  it('列定义包含恰好 8 列', () => {
    expect(props.length).toBe(8)
    expect(props).toEqual([
      'repo_addr', 'repo_branch', 'commit_count', 'task_count',
      'sum_ancient_minutes', 'sum_real_minutes', 'efficiency_ratio', 'start_time'
    ])
  })

  // 6.2 每列都有 prop、label、filter 属性
  it('每列都有 prop、label、filter 属性', () => {
    expect((repoViewContent.match(/label:\s*'/g) || []).length).toBeGreaterThanOrEqual(8)
    expect((repoViewContent.match(/filter:\s*\{/g) || []).length).toBeGreaterThanOrEqual(8)
  })
})

// ============================================================
// 测试点 7: RepoViewV2 过滤器类型序列正确
// ============================================================
describe('RepoViewV2 过滤器类型序列', () => {
  it('过滤器类型序列正确', () => {
    const filterTypeMatches = repoViewContent.match(/filter:\s*\{\s*type:\s*'([^']+)'/g) || []
    const filterTypes = filterTypeMatches.map(m => m.match(/type:\s*'([^']+)'/)[1])
    expect(filterTypes).toEqual([
      'text', 'multi-select', 'number', 'number', 'number', 'number', 'number', 'date'
    ])
  })
})

// ============================================================
// 测试点 8: RepoViewV2 KbFilterTable 组件绑定完整性
// ============================================================
describe('RepoViewV2 KbFilterTable 组件绑定完整性', () => {
  // 8.1 导入 KbFilterTable
  it('导入 KbFilterTable 组件', () => {
    expect(repoViewContent).toContain("import KbFilterTable from '@/components/KbFilterTable.vue'")
  })

  // 8.2 ref 绑定
  it('KbFilterTable 有 ref 绑定', () => {
    expect(repoViewContent).toContain('ref="filterTableRef"')
  })

  // 8.3 核心属性绑定
  it('KbFilterTable 绑定 :columns, :data, :loading', () => {
    expect(repoViewContent).toContain(':columns="columns"')
    expect(repoViewContent).toContain(':data="tableData"')
    expect(repoViewContent).toContain(':loading="loading"')
  })

  // 8.4 v-model 双向绑定
  it('KbFilterTable 使用 v-model:page 和 v-model:pageSize', () => {
    expect(repoViewContent).toContain('v-model:page="page"')
    expect(repoViewContent).toContain('v-model:pageSize="pageSize"')
  })

  // 8.5 事件绑定
  it('KbFilterTable 绑定了所有必要事件', () => {
    expect(repoViewContent).toContain('@row-click')
    expect(repoViewContent).toContain('@size-change')
    expect(repoViewContent).toContain('@page-change')
    expect(repoViewContent).toContain('@filter-change')
  })
})

// ============================================================
// 测试点 9: RepoViewV2 efficiency_ratio 插槽颜色逻辑
// ============================================================
describe('RepoViewV2 efficiency_ratio 插槽颜色逻辑', () => {
  // 9.1 有 #cell-efficiency_ratio 插槽
  it('包含 #cell-efficiency_ratio 插槽', () => {
    expect(repoViewContent).toContain('#cell-efficiency_ratio')
  })

  // 9.2 阈值 300 → success
  it('300 阈值判断 success', () => {
    expect(repoViewContent).toMatch(/row\.efficiency_ratio\s*>=\s*300\s*\?\s*'success'/)
  })

  // 9.3 阈值 150 → primary
  it('150 阈值判断 primary', () => {
    expect(repoViewContent).toMatch(/row\.efficiency_ratio\s*>=\s*150\s*\?\s*'primary'/)
  })

  // 9.4 使用 toFixed(1)% 格式
  it('使用 toFixed(1) 格式', () => {
    expect(repoViewContent).toContain('.toFixed(1)')
  })
})

// ============================================================
// 测试点 10: RepoDetailV2 reason 条件渲染
// ============================================================
describe('RepoDetailV2 reason 条件渲染', () => {
  // 10.1 ancient reason 有 v-if 条件
  it('ancient reason 有 v-if 条件渲染', () => {
    expect(repoDetailContent).toContain('v-if="efficiency.repo_ancient_minutes_reason"')
  })

  // 10.2 real reason 有 v-if 条件
  it('real reason 有 v-if 条件渲染', () => {
    expect(repoDetailContent).toContain('v-if="efficiency.repo_real_minutes_reason"')
  })

  // 10.3 ancient reason 使用 el-tooltip
  it('ancient reason 使用 el-tooltip 包裹', () => {
    // el-tooltip 的 :content 引用 ancient reason
    expect(repoDetailContent).toMatch(/el-tooltip[\s\S]*?efficiency\.repo_ancient_minutes_reason/)
  })

  // 10.4 real reason 使用 el-tooltip
  it('real reason 使用 el-tooltip 包裹', () => {
    expect(repoDetailContent).toMatch(/el-tooltip[\s\S]*?efficiency\.repo_real_minutes_reason/)
  })
})

// ============================================================
// 测试点 11: RepoDetailV2 metric-reason CSS 类
// ============================================================
describe('RepoDetailV2 metric-reason CSS 类', () => {
  // 11.1 模板中使用 metric-reason class
  it('模板中使用 metric-reason class', () => {
    expect(repoDetailContent).toContain('class="metric-reason"')
  })

  // 11.2 style 中定义 .metric-reason
  it('style 中定义 .metric-reason', () => {
    expect(repoDetailContent).toMatch(/\.metric-reason\s*\{/)
  })
})

// ============================================================
// 测试点 12: Router 导入路径指向正确的重命名文件
// ============================================================
describe('Router 导入路径正确性', () => {
  // 12.1 包含 RepoViewV2.vue 导入
  it('路由包含 RepoViewV2.vue 导入', () => {
    expect(routerContent).toContain('RepoViewV2.vue')
  })

  // 12.2 包含 RepoDetailV2.vue 导入
  it('路由包含 RepoDetailV2.vue 导入', () => {
    expect(routerContent).toContain('RepoDetailV2.vue')
  })

  // 12.3 不包含旧名称 ProjectViewV2
  it('路由不包含旧名称 ProjectViewV2', () => {
    expect(routerContent).not.toContain('ProjectViewV2')
  })

  // 12.4 不包含旧名称 ProjectDetailV2
  it('路由不包含旧名称 ProjectDetailV2', () => {
    expect(routerContent).not.toContain('ProjectDetailV2')
  })
})

// ============================================================
// 测试点 13: RepoViewV2 和 RepoDetailV2 导入依赖完整性
// ============================================================
describe('Repo 页面导入依赖完整性', () => {
  // 13.1 KbFilterTable.vue 存在
  it('@/components/KbFilterTable.vue 文件存在', () => {
    expect(existsSync(resolve(srcDir, 'components', 'KbFilterTable.vue'))).toBe(true)
  })

  // 13.2 FilterBar.vue 存在（RepoDetailV2 使用）
  it('@/components/FilterBar.vue 文件存在', () => {
    expect(existsSync(resolve(srcDir, 'components', 'FilterBar.vue'))).toBe(true)
  })

  // 13.3 api/es.js 存在
  it('@/api/es.js 文件存在', () => {
    expect(existsSync(resolve(srcDir, 'api', 'es.js'))).toBe(true)
  })

  // 13.4 utils/formatters.js 存在
  it('@/utils/formatters.js 文件存在', () => {
    expect(existsSync(resolve(srcDir, 'utils', 'formatters.js'))).toBe(true)
  })

  // 13.5 utils/date.js 存在
  it('@/utils/date.js 文件存在', () => {
    expect(existsSync(resolve(srcDir, 'utils', 'date.js'))).toBe(true)
  })

  // 13.6 getReposV2 在 api/es.js 中有 export 定义
  it('getReposV2 在 api/es.js 中有 export 定义', () => {
    const esContent = readFileSync(resolve(srcDir, 'api', 'es.js'), 'utf-8')
    expect(esContent).toMatch(/export\s+(function|const|async\s+function)\s+getReposV2/)
  })

  // 13.7 getRepoDetailV2New 在 api/es.js 中有 export 定义
  it('getRepoDetailV2New 在 api/es.js 中有 export 定义', () => {
    const esContent = readFileSync(resolve(srcDir, 'api', 'es.js'), 'utf-8')
    expect(esContent).toMatch(/export\s+(function|const|async\s+function)\s+getRepoDetailV2New/)
  })
})

// ============================================================
// 测试点 14: RepoViewV2 与 TaskViewV2 模式对齐
// ============================================================
describe('RepoViewV2 与 TaskViewV2 模式对齐验证', () => {
  // 14.1 两者都使用 #cell-efficiency_ratio 插槽
  it('两者都使用 #cell-efficiency_ratio 插槽', () => {
    expect(repoViewContent).toContain('#cell-efficiency_ratio')
    expect(taskViewContent).toContain('#cell-efficiency_ratio')
  })

  // 14.2 两者都使用 toFixed(1)% 格式
  it('两者都使用 toFixed(1) 格式', () => {
    expect(repoViewContent).toContain('.toFixed(1)')
    expect(taskViewContent).toContain('.toFixed(1)')
  })

  // 14.3 两者都使用 el-tag 组件
  it('两者都使用 el-tag 组件', () => {
    expect(repoViewContent).toContain('<el-tag')
    expect(taskViewContent).toContain('<el-tag')
  })

  // 14.4 两者都导入 KbFilterTable
  it('两者都导入 KbFilterTable', () => {
    expect(repoViewContent).toContain("import KbFilterTable from '@/components/KbFilterTable.vue'")
    expect(taskViewContent).toContain("import KbFilterTable from '@/components/KbFilterTable.vue'")
  })
})
