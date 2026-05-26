/*
 * ECharts 选项工厂，移植自 costrict-web/opencode app-ai-native 的 kanban/lib/chart-options.ts，
 * 保持图表观感与那边一致。
 */
export function kanbanChart(title, labels, list, opts = {}) {
  const interval = labels.length > 18 ? Math.ceil(labels.length / 18) - 1 : 0
  const type = opts.type ?? 'bar'
  const isRatio = title.includes('提效') || title.includes('Efficiency') || title.includes('Ratio')

  return {
    title: {
      text: title,
      top: 10,
      left: 'center',
      textStyle: { fontSize: opts.titleSize ?? 13, fontWeight: 'bold' },
    },
    tooltip: opts.format
      ? {
          trigger: 'axis',
          formatter(items) {
            const rows = Array.isArray(items) ? items : [items]
            return rows.reduce(
              (txt, item, index) =>
                `${txt}${index === 0 ? `${item.axisValue}<br/>` : ''}${item.marker}${item.seriesName}: ${opts.format(Number(item.value ?? 0))}<br/>`,
              '',
            )
          },
        }
      : { trigger: 'axis' },
    legend: { data: list.map(item => item.name), top: 40, left: 20, right: 20, type: 'scroll' },
    grid: { left: '5%', right: '5%', top: list.length > 1 ? 92 : 56, bottom: 44, containLabel: true },
    xAxis: {
      type: 'category',
      data: labels,
      axisLabel: { rotate: 0, fontSize: 11, margin: 12, hideOverlap: true, interval },
    },
    yAxis: isRatio ? { type: 'value', axisLabel: { formatter: '{value}%' } } : { type: 'value' },
    series: list.map(item => {
      const next = item.type ?? type
      return { name: item.name, type: next, smooth: next === 'line', data: item.data }
    }),
  }
}
