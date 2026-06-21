import { describe, it, expect } from 'vitest'
import { entityFromPath, isOrgEntityPath, ENTITY_LABEL, DIMENSION_LABEL, isEntity } from './matrix'

// 维度优先 IA 翻转的核心纯逻辑回归网：
// - entityFromPath 支撑 AppShell 顶部维度链接的「切维保留主体+聚焦」(决策2)
// - isOrgEntityPath 支撑 ScrollToTop 组织树内导航不滚顶的特例
// - 派生标签确保 ENTITIES/DIMENSIONS 单一真源不漂移
describe('matrix 路径与标签纯逻辑', () => {
  it('entityFromPath 提取维度页 /:dim/:entity 的主体段', () => {
    expect(entityFromPath('/usage/org')).toBe('org')
    expect(entityFromPath('/efficiency/user')).toBe('user')
    expect(entityFromPath('/cost/project')).toBe('project')
    expect(entityFromPath('/contribution/repo')).toBe('repo')
  })

  it('entityFromPath 对非维度页/裸维度/脏值返回 null（dimensionHref 据此回退默认主体）', () => {
    expect(entityFromPath('/')).toBeNull() // 总览
    expect(entityFromPath('/needs-v2')).toBeNull() // 需求
    expect(entityFromPath('/user/abc-123')).toBeNull() // 详情叶子页(首段非维度，不应误判)
    expect(entityFromPath('/usage/garbage')).toBeNull() // 脏 entity
    expect(entityFromPath('/usage')).toBeNull() // 裸维度(无 entity 段)
  })

  it('isOrgEntityPath 仅对 /:dim/org 为真', () => {
    expect(isOrgEntityPath('/usage/org')).toBe(true)
    expect(isOrgEntityPath('/efficiency/org')).toBe(true)
    expect(isOrgEntityPath('/efficiency/user')).toBe(false)
    expect(isOrgEntityPath('/user/abc')).toBe(false)
  })

  it('派生标签与有序列表一致（单一真源）', () => {
    expect(ENTITY_LABEL.org).toBe('组织')
    expect(ENTITY_LABEL.repo).toBe('仓库')
    expect(DIMENSION_LABEL.usage).toBe('使用')
    expect(DIMENSION_LABEL.contribution).toBe('贡献')
  })

  it('isEntity 守卫脏 param', () => {
    expect(isEntity('org')).toBe(true)
    expect(isEntity('garbage')).toBe(false)
    expect(isEntity(undefined)).toBe(false)
  })
})
