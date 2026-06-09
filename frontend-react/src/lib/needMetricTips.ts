// Need 指标口径悬浮文案。面向业务用户展示，算法说明使用业务可读名称。

export const ACTUAL_WORK_TIP =
  '实际人力：AI 辅助下，这个需求实际投入的有效工作时间。算法：会话活跃工作时间 + 未覆盖工作估算。'

export const FUSED_BASELINE_WORK_TIP =
  '传统人力预估：如果不用 AI，这个需求大概需要投入多少人工时间。算法：代码量估算 + 相似历史任务 + 模型估算，按置信度综合。'

export const ACTUAL_CALENDAR_TIP =
  '实际周期：这个需求从开始开发到合并，真实经过了多久。算法：结束时间 - 开始时间。'

export const BASELINE_CALENDAR_TIP =
  '传统周期预估：如果不用 AI，按团队常规工作节奏估算这个需求大概会持续多久。算法：传统人力预估 ÷ 团队工作密度。'

export const WORK_RATIO_TIP =
  '人力提效：传统人力预估比实际人力多出的比例，表示 AI 节省了多少人工投入。算法：(传统人力预估 - 实际人力) / 实际人力。'

export const CALENDAR_RATIO_TIP =
  '日历提效：传统周期预估比实际周期多出的比例，表示 AI 帮需求交付缩短了多少周期。算法：(传统周期预估 - 实际周期) / 实际周期。'
