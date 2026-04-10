/**
 * Commit manual 优先取值辅助函数
 */
export function getEffectiveAncient(row) {
  return row.commit_ancient_minutes_manual ?? row.commit_ancient_minutes ?? null
}

export function getEffectiveReal(row) {
  return row.commit_real_minutes_manual ?? row.commit_real_minutes ?? null
}

/**
 * 提效比颜色计算（与 CommitDetailV2 的 efficiencyColor computed 逻辑一致）
 */
export function getEfficiencyColor(ratio) {
  if (ratio == null) return '#909399'
  if (ratio >= 300) return '#67C23A'
  if (ratio >= 150) return '#409EFF'
  return '#909399'
}
