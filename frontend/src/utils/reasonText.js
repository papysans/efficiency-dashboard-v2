/*
 * 把 v2 管道生成的机器可读 reason 码（如 "fusion:missing_actual_calendar; outlier:actual_to_baseline=0.017"）
 * 翻译成中文标签 + 原因说明，供 Need 列表/详情展示。
 * 码的来源见 kbcli/efficiency_v2_fusion.go / efficiency_v2_need_aggregate.go / baseline_{knn,llm}.go。
 */

function pct(x) {
  const n = Number(x)
  if (!Number.isFinite(n)) return x
  return `${(n * 100).toFixed(1)}%`
}

const RULES = [
  {
    re: /^outlier:actual_to_baseline=(.+)$/,
    tone: 'error',
    label: m => `工作量比异常 (${m[1]})`,
    hint: m => `实际工作量 / 基线工作量 = ${m[1]}，偏离正常区间（过高或过低），该需求被判为异常样本、不计入汇总。`,
  },
  {
    re: /^outlier:efficiency_ratio=(.+)$/,
    tone: 'error',
    label: m => `提效比极端 (${pct(m[1])})`,
    hint: m => `日历提效 = ${pct(m[1])}，落在极端区间（>90% 或 <-50%），已标记为异常样本。`,
  },
  {
    re: /^fusion:missing_actual_calendar$/,
    tone: 'warning',
    label: () => '缺少实际日历',
    hint: () => '该需求没有可测的开发时间跨度（如仅单次提交、无会话活动），dev_start 与 dev_end 相同，无法计算日历提效。',
  },
  {
    re: /^fusion:no_baselines$/,
    tone: 'warning',
    label: () => '无可用基线',
    hint: () => '算法 / 相似锚点 / LLM 三路工作量估算都缺失，无法给出基线，置信为未知。',
  },
  {
    re: /^fusion:single_baseline$/,
    tone: 'warning',
    label: () => '仅单一基线',
    hint: () => '只有一路估算可用，缺乏交叉校验，置信偏低。',
  },
  {
    re: /^band:spread_exceeds_fused$/,
    tone: 'warning',
    label: () => '基线离散过大',
    hint: () => '各路估算差异过大，导致提效区间下界为负、无业务意义。',
  },
  {
    re: /^need span exceeds max_need_span_days=(\d+)$/,
    tone: 'warning',
    label: m => `需求跨度超限 (${m[1]}天)`,
    hint: m => `开发周期超过 ${m[1]} 天上限，可能是多个需求被并入了同一边界。`,
  },
  {
    re: /^covered_rule=temporal_only$/,
    tone: 'info',
    label: () => '按时间关联',
    hint: () => '仅按时间相邻规则关联 commit（缺少显式的需求↔提交关联证据）。',
  },
  {
    re: /^no_commits$/,
    tone: 'info',
    label: () => '无关联提交',
    hint: () => '该需求没有关联到任何 commit。',
  },
  {
    re: /^zero_loc_commits$/,
    tone: 'info',
    label: () => '提交无有效改动',
    hint: () => '关联的 commit 没有有效代码行改动。',
  },
  {
    re: /^ai_code_ratio=(.+?)<(.+)$/,
    tone: 'info',
    label: m => `AI 占比偏低 (${pct(m[1])})`,
    hint: m => `AI 代码占比 ${pct(m[1])} 低于阈值 ${pct(m[2])}，AI 贡献证据不足。`,
  },
  {
    re: /^uncovered_work_ratio=(.+?)>(.+)$/,
    tone: 'info',
    label: m => `未覆盖工作偏高 (${pct(m[1])})`,
    hint: m => `无法归因的人工工作占比 ${pct(m[1])} 高于阈值 ${pct(m[2])}。`,
  },
  {
    re: /^silica=(.+?)<(.+)$/,
    tone: 'info',
    label: m => `硅含量偏低 (${m[1]})`,
    hint: m => `硅含量 ${m[1]} 低于阈值 ${m[2]}。`,
  },
  {
    re: /^knn:k=(\d+)$/,
    tone: 'info',
    label: m => `相似锚点 k=${m[1]}`,
    hint: m => `基于最近 ${m[1]} 个相似历史锚点估算工作量。`,
  },
  { re: /^knn:no_anchors$/, tone: 'info', label: () => '锚点缺失', hint: () => '没有可用的相似历史锚点。' },
  { re: /^knn:zero_weight$/, tone: 'info', label: () => '锚点权重为 0', hint: () => '相似锚点权重为 0，未参与估算。' },
  { re: /^llm:disabled$/, tone: 'info', label: () => 'LLM 已禁用', hint: () => 'LLM 估算在当前运行中被禁用（离线模式）。' },
  { re: /^llm:(no_json|invalid_json.*)$/, tone: 'info', label: () => 'LLM 解析失败', hint: () => 'LLM 返回内容无法解析为有效结果。' },
  { re: /^llm:call_failed.*$/, tone: 'info', label: () => 'LLM 调用失败', hint: () => '调用 LLM 服务失败。' },
  { re: /^llm:build_prompt_failed.*$/, tone: 'info', label: () => 'LLM 构建提示失败', hint: () => '构建 LLM 提示词失败。' },
  { re: /^llm:(negative_minutes|total_too_large)$/, tone: 'info', label: () => 'LLM 结果不合理', hint: () => 'LLM 给出的估算为负值或过大，已丢弃。' },
  { re: /^llm:retry_ok/, tone: 'info', label: () => 'LLM 重试成功', hint: () => 'LLM 首次失败、重试后成功。' },
  { re: /^llm:retry_failed/, tone: 'info', label: () => 'LLM 重试失败', hint: () => 'LLM 多次重试仍失败。' },
]

/**
 * 解析 reason 字符串为结构化条目数组。
 * @returns {Array<{label:string, hint:string, tone:string, raw:string}>}
 */
export function parseReason(reason) {
  if (!reason) return []
  return String(reason)
    .split(';')
    .map(s => s.trim())
    .filter(Boolean)
    .map(part => {
      for (const rule of RULES) {
        const m = part.match(rule.re)
        if (m) return { label: rule.label(m), hint: rule.hint(m), tone: rule.tone, raw: part }
      }
      // 未识别的码：原样展示，避免信息丢失
      return { label: part, hint: part, tone: 'neutral', raw: part }
    })
}

/** 中文短标签，用分隔符连接（列表单元格用） */
export function reasonSummary(reason, sep = '；') {
  const items = parseReason(reason)
  if (!items.length) return '-'
  return items.map(i => i.label).join(sep)
}

/** 完整原因说明（tooltip 用） */
export function reasonHints(reason) {
  const items = parseReason(reason)
  if (!items.length) return ''
  return items.map(i => `• ${i.hint}`).join('\n')
}
