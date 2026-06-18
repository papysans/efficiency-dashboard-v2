import { describe, it, expect } from 'vitest'
import { rollupCostTree, flattenDeptIds, type DeptDirectAgg } from './costTreeRollup'
import type { DeptTreeNode } from '@/api/types'

// 构造 mock 部门节点（只填 rollup 关心的字段，其余给默认值）。
function dept(deptId: string, deptName: string, children: DeptTreeNode[] = []): DeptTreeNode {
  return {
    dept_id: deptId,
    dept_name: deptName,
    parent_dept_id: '',
    dept_path: '',
    dept_level: 0,
    order_num: 0,
    child_dept_count: children.length,
    status: 1,
    children,
  }
}

function agg(directCost: number, directMembers: number, directActive: number): DeptDirectAgg {
  return { directCost, directMembers, directActive }
}

describe('rollupCostTree — 部门成本自底向上递归求和', () => {
  // 公司
  //   ├─ 研发部 (直属 ¥100, 2人, 2活)
  //   │    ├─ 前端组 (直属 ¥30, 3人, 1活)
  //   │    └─ 后端组 (直属 ¥50, 1人, 1活)
  //   └─ 产品部 (直属 ¥20, 4人, 0活)
  const tree: DeptTreeNode[] = [
    dept('root', '公司', [
      dept('rd', '研发部', [dept('fe', '前端组'), dept('be', '后端组')]),
      dept('pm', '产品部'),
    ]),
  ]

  const directAgg = new Map<string, DeptDirectAgg>([
    ['root', agg(0, 0, 0)],
    ['rd', agg(100, 2, 2)],
    ['fe', agg(30, 3, 1)],
    ['be', agg(50, 1, 1)],
    ['pm', agg(20, 4, 0)],
  ])

  const nodes = rollupCostTree(tree, directAgg)
  const root = nodes[0]
  const rd = root.children[0]
  const fe = rd.children[0]
  const be = rd.children[1]
  const pm = root.children[1]

  it('叶子节点子树 = 自身直属', () => {
    expect(fe.subtreeCost).toBe(30)
    expect(fe.subtreeMembers).toBe(3)
    expect(fe.subtreeActive).toBe(1)
    expect(be.subtreeCost).toBe(50)
    expect(pm.subtreeCost).toBe(20)
    expect(pm.subtreeMembers).toBe(4)
    expect(pm.subtreeActive).toBe(0)
  })

  it('中间节点子树 = 自身直属 + Σ 子节点子树', () => {
    // 研发部：100 + 30 + 50 = 180
    expect(rd.subtreeCost).toBe(180)
    // 成员：2 + 3 + 1 = 6
    expect(rd.subtreeMembers).toBe(6)
    // 活跃：2 + 1 + 1 = 4
    expect(rd.subtreeActive).toBe(4)
  })

  it('根节点子树 = 全树总和（公司层 rollup）', () => {
    // 公司：0(直属) + 研发部180 + 产品部20 = 200
    expect(root.subtreeCost).toBe(200)
    // 成员：0 + 6 + 4 = 10
    expect(root.subtreeMembers).toBe(10)
    // 活跃：0 + 4 + 0 = 4
    expect(root.subtreeActive).toBe(4)
  })

  it('保留直属值（不被子树覆盖）+ depth/层级正确', () => {
    expect(rd.directCost).toBe(100)
    expect(rd.subtreeCost).not.toBe(rd.directCost)
    expect(root.depth).toBe(0)
    expect(rd.depth).toBe(1)
    expect(fe.depth).toBe(2)
  })

  it('hasChildren 体现是否有子部门', () => {
    expect(root.hasChildren).toBe(true)
    expect(rd.hasChildren).toBe(true)
    expect(fe.hasChildren).toBe(false)
    expect(pm.hasChildren).toBe(false)
  })

  it('缺失直属聚合的部门视为 0（不丢节点）', () => {
    const sparse = new Map<string, DeptDirectAgg>([['fe', agg(30, 3, 1)]])
    const out = rollupCostTree(tree, sparse)
    const r = out[0]
    // 只有前端组有数据：root 子树成本 = 30，研发部 = 30，产品部 = 0
    expect(r.subtreeCost).toBe(30)
    expect(r.children[0].subtreeCost).toBe(30)
    expect(r.children[1].subtreeCost).toBe(0)
  })

  it('空树 / 空映射安全返回', () => {
    expect(rollupCostTree([], new Map())).toEqual([])
    const z = rollupCostTree(tree, new Map())[0]
    expect(z.subtreeCost).toBe(0)
    expect(z.subtreeMembers).toBe(0)
  })
})

describe('flattenDeptIds — 扁平化全树 dept_id（去空去重保序）', () => {
  it('深度优先取所有 dept_id', () => {
    const tree: DeptTreeNode[] = [
      dept('root', '公司', [dept('rd', '研发部', [dept('fe', '前端组')]), dept('pm', '产品部')]),
    ]
    expect(flattenDeptIds(tree)).toEqual(['root', 'rd', 'fe', 'pm'])
  })

  it('去重 + 跳过空 dept_id', () => {
    const tree: DeptTreeNode[] = [
      dept('a', 'A', [dept('', '空', []), dept('a', '重复A', [])]),
    ]
    expect(flattenDeptIds(tree)).toEqual(['a'])
  })

  it('空树返回空数组', () => {
    expect(flattenDeptIds([])).toEqual([])
  })
})
