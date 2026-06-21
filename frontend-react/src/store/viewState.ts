// 全局视图状态（zustand）。
// 第 1 段：全局时间范围，供顶部统一 DateRangePicker 绑定（替代旧的「每页各自 DateRangePicker」）。
// 第 2 段：聚焦对象（org/user/project/repo 四下钻选定的具体对象）。聚焦对象**不放 store 持久化**，
//   而是以 URL query（?object=<id>）为单一数据源，刷新/深链保持、切维度 Tab 不丢（详见
//   DimensionEntityLayout 的 useFocusObject）。store 只持久化时间范围。
// 时间范围 localStorage 持久化，刷新保留。默认复用 lib/date 的 30 天默认（getDefaultDateRangeWide(30)）。
import { create } from 'zustand'
import { getDefaultDateRangeWide } from '@/lib/date'

const TIME_RANGE_KEY = 'viewState.timeRange'

type DateRange = [string, string]

function loadTimeRange(): DateRange {
  try {
    const raw = localStorage.getItem(TIME_RANGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (
        Array.isArray(parsed) &&
        parsed.length === 2 &&
        typeof parsed[0] === 'string' &&
        typeof parsed[1] === 'string'
      ) {
        return [parsed[0], parsed[1]]
      }
    }
  } catch {
    // 解析失败回退默认
  }
  return getDefaultDateRangeWide(30)
}

interface ViewState {
  /** 全局时间范围 [start, end]，格式 YYYY-MM-DD。 */
  timeRange: DateRange
  setTimeRange: (range: DateRange) => void
}

export const useViewState = create<ViewState>((set) => ({
  timeRange: loadTimeRange(),
  setTimeRange: (range) => {
    try {
      localStorage.setItem(TIME_RANGE_KEY, JSON.stringify(range))
    } catch {
      // 写入失败忽略（隐私模式等），仍更新内存态
    }
    set({ timeRange: range })
  },
}))
