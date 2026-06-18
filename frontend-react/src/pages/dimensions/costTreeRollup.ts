// 组织「成本树」纯递归 rollup —— 把部门树 + 各部门「直属成本/直属人数」自底向上求和，
// 得到每个节点的「子树成本/子树人数」（个人→团队→组织→公司层层累加）。
//
// ⚠️ 口径：成本 = 平台 AI 调用花费（estimated_total_cost，¥），与看板人天不同源（成本双源陷阱）。
// ⚠️ 截断：直属成本由 ranking Top N 命中求和而来（见 platformDeptCostTree），排行外用户漏算，
//    会让所有节点偏小 —— 本模块只做纯求和，截断的告知交由 UI（TruncationNote）。
//
// 纯函数（无 React/无 IO），便于单测（见 costTreeRollup.test.ts）。
import type { DeptTreeNode } from '@/api/types'

/** 一个部门节点的「直属」聚合（仅该部门直属成员，非递归）。 */
export interface DeptDirectAgg {
  /** 直属成本（直属成员命中平台 ranking 的 estimated_total_cost 求和，¥）。 */
  directCost: number
  /** 直属成员人数（dept-sync 名册）。 */
  directMembers: number
  /** 其中在区间内有平台调用记录的直属成员数。 */
  directActive: number
}

/** rollup 后的成本树节点：自带子树递归汇总 + 层级/子节点。 */
export interface CostTreeNode {
  deptId: string
  deptName: string
  /** 树深度（根为 0），用于缩进渲染。 */
  depth: number
  /** 直属成本（不含子部门）。 */
  directCost: number
  /** 直属成员人数（不含子部门）。 */
  directMembers: number
  /** 子树成本 = 自身直属 + Σ 子节点子树成本（递归 rollup 结果）。 */
  subtreeCost: number
  /** 子树成员人数 = 自身直属 + Σ 子节点子树人数。 */
  subtreeMembers: number
  /** 子树活跃（有平台记录）成员人数。 */
  subtreeActive: number
  /** 是否有子部门（child_dept_count>0 或 children 非空）。 */
  hasChildren: boolean
  children: CostTreeNode[]
}

/**
 * 自底向上 rollup 整棵部门树：
 *   subtreeCost(node) = directCost(node) + Σ subtreeCost(child)
 *   subtreeMembers / subtreeActive 同理累加。
 * directAggByDept: dept_id → 该部门直属聚合（缺失视为 0，部门仍出现在树里）。
 * 返回与输入同构的树（每节点带 subtree* + depth），保持原 children 顺序。
 */
export function rollupCostTree(
  tree: DeptTreeNode[],
  directAggByDept: Map<string, DeptDirectAgg>,
): CostTreeNode[] {
  const visit = (node: DeptTreeNode, depth: number): CostTreeNode => {
    const direct = directAggByDept.get(node.dept_id)
    const directCost = direct?.directCost ?? 0
    const directMembers = direct?.directMembers ?? 0
    const directActive = direct?.directActive ?? 0

    const children = (node.children ?? []).map((ch) => visit(ch, depth + 1))

    let subtreeCost = directCost
    let subtreeMembers = directMembers
    let subtreeActive = directActive
    for (const ch of children) {
      subtreeCost += ch.subtreeCost
      subtreeMembers += ch.subtreeMembers
      subtreeActive += ch.subtreeActive
    }

    return {
      deptId: node.dept_id,
      deptName: node.dept_name,
      depth,
      directCost,
      directMembers,
      subtreeCost,
      subtreeMembers,
      subtreeActive,
      hasChildren: (node.children?.length ?? 0) > 0 || (node.child_dept_count ?? 0) > 0,
      children,
    }
  }
  return (tree ?? []).map((n) => visit(n, 0))
}

/** 扁平化部门树取所有 dept_id（建直属成员拉取计划用；去空、去重保序）。 */
export function flattenDeptIds(tree: DeptTreeNode[]): string[] {
  const out: string[] = []
  const seen = new Set<string>()
  const walk = (nodes: DeptTreeNode[]) => {
    for (const n of nodes) {
      if (n.dept_id && !seen.has(n.dept_id)) {
        seen.add(n.dept_id)
        out.push(n.dept_id)
      }
      if (n.children?.length) walk(n.children)
    }
  }
  walk(tree ?? [])
  return out
}
