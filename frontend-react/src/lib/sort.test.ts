import { describe, it, expect } from 'vitest'
import { parseOrder, toOrder, compareValue, sortRows } from './sort'

describe('parseOrder', () => {
  it('升序', () => expect(parseOrder('foo')).toEqual({ field: 'foo', desc: false }))
  it('降序前缀 -', () => expect(parseOrder('-foo')).toEqual({ field: 'foo', desc: true }))
  it('空/null/空白 → null', () => {
    expect(parseOrder('')).toBeNull()
    expect(parseOrder(null)).toBeNull()
    expect(parseOrder(undefined)).toBeNull()
    expect(parseOrder('   ')).toBeNull()
  })
})

describe('toOrder（parseOrder 的逆）', () => {
  it('field 空 → undefined（清除排序）', () => {
    expect(toOrder('')).toBeUndefined()
    expect(toOrder(null)).toBeUndefined()
  })
  it('升降构造', () => {
    expect(toOrder('foo', false)).toBe('foo')
    expect(toOrder('foo', true)).toBe('-foo')
  })
  it('与 parseOrder 往返一致', () => {
    expect(parseOrder(toOrder('x', true))).toEqual({ field: 'x', desc: true })
    expect(parseOrder(toOrder('x', false))).toEqual({ field: 'x', desc: false })
  })
})

describe('compareValue', () => {
  it('数值比较', () => expect(Math.sign(compareValue(1, 2))).toBe(-1))
  it('布尔 false<true', () => {
    expect(compareValue(false, true)).toBe(-1)
    expect(compareValue(true, false)).toBe(1)
    expect(compareValue(true, true)).toBe(0)
  })
  it('字符串 localeCompare', () => expect(Math.sign(compareValue('a', 'b'))).toBe(-1))
})

describe('sortRows（稳定 + null 恒沉底）', () => {
  const rows = [
    { id: 'a', v: 3 },
    { id: 'b', v: null as number | null },
    { id: 'c', v: 1 },
    { id: 'd', v: 3 },
    { id: 'e', v: null as number | null },
  ]
  const get = (r: { v: number | null }) => r.v

  it('升序：非空升序，null 末尾', () => {
    const out = sortRows(rows, get, false).map((r) => r.id)
    // 1(c) < 3(a) < 3(d, 稳定保 a 在前) ... null(b,e) 沉底且按原序
    expect(out).toEqual(['c', 'a', 'd', 'b', 'e'])
  })

  it('降序：非空降序，null 仍在末尾（不随方向翻转）', () => {
    const out = sortRows(rows, get, true).map((r) => r.id)
    expect(out).toEqual(['a', 'd', 'c', 'b', 'e'])
  })

  it('同值按原始 index 稳定保序', () => {
    const eq = [{ id: 'x', v: 5 }, { id: 'y', v: 5 }, { id: 'z', v: 5 }]
    expect(sortRows(eq, (r) => r.v, true).map((r) => r.id)).toEqual(['x', 'y', 'z'])
  })

  it('不修改原数组', () => {
    const orig = [...rows]
    sortRows(rows, get, false)
    expect(rows).toEqual(orig)
  })
})
