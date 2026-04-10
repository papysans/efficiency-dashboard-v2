import { describe, it, expect } from 'vitest'
import { readFileSync, existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)

const viewsDir = resolve(__dirname, '..')
const srcDir = resolve(__dirname, '..', '..')

const commitDetailContent = readFileSync(resolve(viewsDir, 'CommitDetailV2.vue'), 'utf-8')

// ============================================================
// 测试点 5: CommitDetailV2 模板结构一致性
// ============================================================
describe('CommitDetailV2 模板结构一致性', () => {
  // 5.1 基础信息卡片
  it('包含基础信息卡片', () => {
    expect(commitDetailContent).toContain('header="基础信息"')
  })

  // 5.2 度量信息卡片
  it('包含度量信息卡片', () => {
    expect(commitDetailContent).toContain('header="度量信息"')
  })

  // 5.3 el-descriptions 属性
  it('el-descriptions 使用 :column="3" border', () => {
    expect(commitDetailContent).toContain(':column="3" border')
  })

  // 5.4 user_id 导航链接
  it('包含 user_id 导航链接', () => {
    expect(commitDetailContent).toContain('commit.user_id')
    expect(commitDetailContent).toMatch(/router\.push\(['"]\/user\//)
  })
})

// ============================================================
// 测试点 7: CommitDetailV2 导入依赖完整性
// ============================================================
describe('CommitDetailV2 导入依赖完整性', () => {
  // 7.1 @/api/es 存在
  it('@/api/es 文件存在', () => {
    expect(existsSync(resolve(srcDir, 'api', 'es.js'))).toBe(true)
  })

  // 7.2 @/utils/formatters 存在
  it('@/utils/formatters 文件存在', () => {
    expect(existsSync(resolve(srcDir, 'utils', 'formatters.js'))).toBe(true)
  })

  // 额外验证：导入的具体函数在对应文件中有 export 定义
  it('getCommitDetailV2 在 api/es.js 中有 export 定义', () => {
    const esContent = readFileSync(resolve(srcDir, 'api', 'es.js'), 'utf-8')
    expect(esContent).toMatch(/export\s+(function|const|async\s+function)\s+getCommitDetailV2/)
  })

  it('updateCommitManualV2 在 api/es.js 中有 export 定义', () => {
    const esContent = readFileSync(resolve(srcDir, 'api', 'es.js'), 'utf-8')
    expect(esContent).toMatch(/export\s+(function|const|async\s+function)\s+updateCommitManualV2/)
  })
})
