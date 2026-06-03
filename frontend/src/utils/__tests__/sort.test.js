import { describe, it, expect } from 'vitest'
import { parseOrder, toOrder, compareValue, sortRows } from '../sort.js'

// ============================================================
// parseOrder
// ============================================================
describe('parseOrder', () => {
  it("'-foo' → 降序", () => {
    expect(parseOrder('-foo')).toEqual({ field: 'foo', desc: true })
  })
  it("'foo' → 升序", () => {
    expect(parseOrder('foo')).toEqual({ field: 'foo', desc: false })
  })
  it('空串 → null', () => {
    expect(parseOrder('')).toBeNull()
  })
  it('null → null', () => {
    expect(parseOrder(null)).toBeNull()
  })
  it('undefined → null', () => {
    expect(parseOrder(undefined)).toBeNull()
  })
  it('两侧空格被 trim', () => {
    expect(parseOrder('  -foo  ')).toEqual({ field: 'foo', desc: true })
  })
})

// ============================================================
// toOrder
// ============================================================
describe('toOrder', () => {
  it('field 为空 → undefined', () => {
    expect(toOrder('', true)).toBeUndefined()
    expect(toOrder(null, false)).toBeUndefined()
    expect(toOrder(undefined, true)).toBeUndefined()
  })
  it('desc → 前缀 -', () => {
    expect(toOrder('foo', true)).toBe('-foo')
  })
  it('asc → 原字段名', () => {
    expect(toOrder('foo', false)).toBe('foo')
  })
  it('parseOrder/toOrder 往返一致', () => {
    expect(toOrder('a', true)).toBe('-a')
    expect(parseOrder('-a')).toEqual({ field: 'a', desc: true })
    expect(toOrder('a', false)).toBe('a')
    expect(parseOrder('a')).toEqual({ field: 'a', desc: false })
  })
})

// ============================================================
// compareValue — 三类型
// ============================================================
describe('compareValue', () => {
  // number
  it('number: 数值比较', () => {
    expect(compareValue(1, 2)).toBeLessThan(0)
    expect(compareValue(2, 1)).toBeGreaterThan(0)
    expect(compareValue(5, 5)).toBe(0)
  })
  it('number: 负数', () => {
    expect(compareValue(-3, 1)).toBeLessThan(0)
  })

  // boolean
  it('boolean: false < true', () => {
    expect(compareValue(false, true)).toBeLessThan(0)
    expect(compareValue(true, false)).toBeGreaterThan(0)
    expect(compareValue(true, true)).toBe(0)
    expect(compareValue(false, false)).toBe(0)
  })

  // string / 其它
  it('string: localeCompare', () => {
    expect(compareValue('a', 'b')).toBeLessThan(0)
    expect(compareValue('b', 'a')).toBeGreaterThan(0)
    expect(compareValue('a', 'a')).toBe(0)
  })
  it('其它类型: 转字符串比较', () => {
    // 混合类型走 String() 分支
    expect(compareValue('10', '9')).toBeLessThan(0) // 字符串 '1' < '9'
  })
})

// ============================================================
// sortRows — null 沉底、稳定、升降
// ============================================================
describe('sortRows', () => {
  const get = r => r.v

  it('升序：非空升序，null 沉底', () => {
    const rows = [{ v: 3 }, { v: null }, { v: 1 }, { v: 2 }]
    expect(sortRows(rows, get, false).map(get)).toEqual([1, 2, 3, null])
  })

  it('降序：非空降序，null 仍沉底（不翻转）', () => {
    const rows = [{ v: 3 }, { v: null }, { v: 1 }, { v: 2 }]
    expect(sortRows(rows, get, true).map(get)).toEqual([3, 2, 1, null])
  })

  it('undefined 同样沉底', () => {
    const rows = [{ v: 2 }, { v: undefined }, { v: 1 }]
    expect(sortRows(rows, get, false).map(get)).toEqual([1, 2, undefined])
    expect(sortRows(rows, get, true).map(get)).toEqual([2, 1, undefined])
  })

  it('稳定：同值保持原相对顺序', () => {
    const rows = [
      { v: 1, id: 'a' },
      { v: 1, id: 'b' },
      { v: 1, id: 'c' },
    ]
    expect(sortRows(rows, get, false).map(r => r.id)).toEqual(['a', 'b', 'c'])
    expect(sortRows(rows, get, true).map(r => r.id)).toEqual(['a', 'b', 'c'])
  })

  it('稳定：多个 null 之间保持原相对顺序', () => {
    const rows = [
      { v: null, id: 'a' },
      { v: 1, id: 'b' },
      { v: null, id: 'c' },
    ]
    // 升序：1 在前，两个 null 按原序 a,c 在后
    expect(sortRows(rows, get, false).map(r => r.id)).toEqual(['b', 'a', 'c'])
  })

  it('空数组', () => {
    expect(sortRows([], get, false)).toEqual([])
    expect(sortRows([], get, true)).toEqual([])
  })

  it('全 null：保持原顺序（升降一致）', () => {
    const rows = [{ v: null, id: 'a' }, { v: null, id: 'b' }, { v: null, id: 'c' }]
    expect(sortRows(rows, get, false).map(r => r.id)).toEqual(['a', 'b', 'c'])
    expect(sortRows(rows, get, true).map(r => r.id)).toEqual(['a', 'b', 'c'])
  })

  it('字符串排序 + null 沉底', () => {
    const rows = [{ v: 'banana' }, { v: null }, { v: 'apple' }, { v: 'cherry' }]
    expect(sortRows(rows, get, false).map(get)).toEqual(['apple', 'banana', 'cherry', null])
  })

  it('不修改入参数组', () => {
    const rows = [{ v: 2 }, { v: 1 }]
    const snapshot = rows.slice()
    sortRows(rows, get, false)
    expect(rows).toEqual(snapshot)
  })
})
