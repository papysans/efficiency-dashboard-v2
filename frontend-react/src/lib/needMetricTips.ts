// Need 指标口径悬浮文案 —— 从 Vue frontend/src/utils/needMetricTips.js 原样搬运。

// density=0.35 脚注：人工分钟换算墙钟，老办法 1 墙钟分钟只有 0.35 分钟真在干活 → 基线日历 ≈ 人工 ×2.86
const DENSITY_NOTE =
  'density=0.35：人工分钟换算墙钟，老办法 1 墙钟分钟只有 0.35 分钟真在干活 → 基线日历 ≈ 人工 ×2.86'

export const ACTUAL_WORK_TIP =
  '实际工作量：这活真正花了多少人工分钟（从会话里测的活跃敲键时间 + 没覆盖到的估算）｜ 算法：测量值'

export const FUSED_BASELINE_WORK_TIP =
  '融合基线工作量：「老办法（没 AI）干这活要多少人工分钟」——古法公式估时 + kNN 人工锚点 + LLM 三个估计按权重融合 ｜ 算法：algo/kNN/LLM 加权'

export const ACTUAL_CALENDAR_TIP =
  '实际日历：这需求从开始到合并的墙钟跨度（真实过了多少时间）｜ 算法：dev_end − dev_start'

export const BASELINE_CALENDAR_TIP =
  '基线日历：「老办法干这活会拖多少墙钟时间」= 融合基线工作量 ÷ density(0.35)｜ 算法：fused ÷ 0.35。' + DENSITY_NOTE

export const WORK_RATIO_TIP =
  '工作量提效：老办法的人工 比 实际人工 多省百分之几 ｜ 算法：(fused − 实际工作量) / 实际工作量'

export const CALENDAR_RATIO_TIP =
  '日历提效：老办法的墙钟 比 实际墙钟 多省百分之几 ｜ 算法：(基线日历 − 实际日历) / 实际日历'
