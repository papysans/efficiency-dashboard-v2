import { describe, it, expect } from 'vitest'
import { getEffectiveAncient, getEffectiveReal, getEfficiencyColor } from './commit-helpers.js'

// ============================================================
// 测试点 1: getEffectiveAncient — manual 优先取值
// ============================================================
describe('getEffectiveAncient — manual 优先取值', () => {
  // 1.1 manual 值存在，返回 manual
  it('manual 值存在时返回 manual', () => {
    expect(getEffectiveAncient({ commit_ancient_minutes_manual: 120, commit_ancient_minutes: 60 })).toBe(120)
  })

  // 1.2 manual 为 null，回退到 original
  it('manual 为 null 时回退到 original', () => {
    expect(getEffectiveAncient({ commit_ancient_minutes_manual: null, commit_ancient_minutes: 60 })).toBe(60)
  })

  // 1.3 都为 null，返回 null
  it('都为 null 时返回 null', () => {
    expect(getEffectiveAncient({ commit_ancient_minutes_manual: null, commit_ancient_minutes: null })).toBeNull()
  })

  // 1.4 manual 为 0（falsy 但 ?? 不触发），返回 0
  it('manual 为 0 时返回 0（?? 不触发）', () => {
    expect(getEffectiveAncient({ commit_ancient_minutes_manual: 0, commit_ancient_minutes: 60 })).toBe(0)
  })

  // 1.5 manual 为 undefined，?? 触发回退到 original
  it('manual 为 undefined 时回退到 original', () => {
    expect(getEffectiveAncient({ commit_ancient_minutes: 45 })).toBe(45)
  })
})

// ============================================================
// 测试点 2: getEffectiveReal — manual 优先取值
// ============================================================
describe('getEffectiveReal — manual 优先取值', () => {
  // 2.1 manual 值存在，返回 manual
  it('manual 值存在时返回 manual', () => {
    expect(getEffectiveReal({ commit_real_minutes_manual: 90, commit_real_minutes: 30 })).toBe(90)
  })

  // 2.2 manual 为 null，回退
  it('manual 为 null 时回退到 original', () => {
    expect(getEffectiveReal({ commit_real_minutes_manual: null, commit_real_minutes: 30 })).toBe(30)
  })

  // 2.3 都为 null
  it('都为 null 时返回 null', () => {
    expect(getEffectiveReal({ commit_real_minutes_manual: null, commit_real_minutes: null })).toBeNull()
  })

  // 2.4 manual 为 0，?? 不回退
  it('manual 为 0 时返回 0（?? 不触发）', () => {
    expect(getEffectiveReal({ commit_real_minutes_manual: 0, commit_real_minutes: 30 })).toBe(0)
  })

  // 2.5 manual undefined → 回退
  it('manual 为 undefined 时回退到 original', () => {
    expect(getEffectiveReal({ commit_real_minutes: 15 })).toBe(15)
  })
})

// ============================================================
// 测试点 3: getEfficiencyColor — 提效比颜色逻辑
// ============================================================
describe('getEfficiencyColor — 提效比颜色逻辑', () => {
  // 3.1 null → 灰色
  it('null 返回灰色 #909399', () => {
    expect(getEfficiencyColor(null)).toBe('#909399')
  })

  // 3.2 undefined → 灰色（== null 成立）
  it('undefined 返回灰色 #909399', () => {
    expect(getEfficiencyColor(undefined)).toBe('#909399')
  })

  // 3.3 300 → 绿色（>= 300 边界）
  it('300 返回绿色 #67C23A（>= 300 边界值）', () => {
    expect(getEfficiencyColor(300)).toBe('#67C23A')
  })

  // 3.4 500 → 绿色（远超阈值）
  it('500 返回绿色 #67C23A', () => {
    expect(getEfficiencyColor(500)).toBe('#67C23A')
  })

  // 3.5 150 → 蓝色（>= 150 边界）
  it('150 返回蓝色 #409EFF（>= 150 边界值）', () => {
    expect(getEfficiencyColor(150)).toBe('#409EFF')
  })

  // 3.6 299 → 蓝色（< 300 但 >= 150）
  it('299 返回蓝色 #409EFF', () => {
    expect(getEfficiencyColor(299)).toBe('#409EFF')
  })

  // 3.7 100 → 灰色（< 150）
  it('100 返回灰色 #909399', () => {
    expect(getEfficiencyColor(100)).toBe('#909399')
  })

  // 3.8 0 → 灰色（零值）
  it('0 返回灰色 #909399', () => {
    expect(getEfficiencyColor(0)).toBe('#909399')
  })
})
