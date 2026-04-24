/**
 * 统一配色常量
 */
export const CHART_COLORS = {
  blue: '#409EFF',
  orange: '#E6A23C',
  green: '#67C23A',
  red: '#F56C6C',
  gray: '#909399',
}

/**
 * 截断名称，超长加 '...'
 */
export function truncateName(name, maxLen = 30) {
  if (!name) return ''
  return name.length > maxLen ? name.substring(0, maxLen) + '...' : name
}

/**
 * 创建横向柱状图 option
 * @param {string} title - 图表标题
 * @param {Array<{name: string, value: number}>} data - 数据
 * @param {string} color - 柱条颜色
 * @param {Function} [valueFormatter] - 值格式化函数
 * @returns {Object} ECharts option
 */
export function createBarOption(title, data, color, valueFormatter) {
  data.sort((a, b) => a.value - b.value)

  return {
    title: {
      text: title,
      left: 'center',
      textStyle: { fontSize: 14, fontWeight: 'bold' }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter: (params) => {
        const p = params[0]
        const fullName = data.find(d => truncateName(d.name) === p.name)?.name || p.name
        const val = valueFormatter ? valueFormatter(p.value) : p.value
        return `${fullName}<br/>${val}`
      }
    },
    grid: { left: '35%', right: '8%', top: '15%', bottom: '5%' },
    xAxis: {
      type: 'value',
      axisLabel: {
        formatter: (val) => valueFormatter ? valueFormatter(val) : val
      }
    },
    yAxis: {
      type: 'category',
      data: data.map(d => truncateName(d.name)),
      axisLabel: { fontSize: 11, width: 180, overflow: 'truncate' }
    },
    series: [{
      type: 'bar',
      data: data.map(d => d.value),
      itemStyle: { color },
      barMaxWidth: 30
    }]
  }
}

/**
 * 创建双柱对比图 option
 * 双柱对比图（如 Token 对比图），通用提取
 */
export function createDualBarOption(title, data1, data2, label1, label2, color1, color2) {
  const allNames = [...new Set([...data1.map(d => d.name), ...data2.map(d => d.name)])]
  const map1 = Object.fromEntries(data1.map(d => [d.name, d.value]))
  const map2 = Object.fromEntries(data2.map(d => [d.name, d.value]))
  const sortedNames = allNames.sort((a, b) => (map1[a] || 0) - (map1[b] || 0))

  return {
    title: {
      text: title,
      left: 'center',
      textStyle: { fontSize: 14, fontWeight: 'bold' }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' }
    },
    legend: {
      data: [label1, label2],
      top: '8%'
    },
    grid: { left: '35%', right: '8%', top: '18%', bottom: '5%' },
    xAxis: { type: 'value' },
    yAxis: {
      type: 'category',
      data: sortedNames.map(n => truncateName(n)),
      axisLabel: { fontSize: 11, width: 180, overflow: 'truncate' }
    },
    series: [
      {
        name: label1,
        type: 'bar',
        data: sortedNames.map(n => map1[n] || 0),
        itemStyle: { color: color1 },
        barMaxWidth: 20
      },
      {
        name: label2,
        type: 'bar',
        data: sortedNames.map(n => map2[n] || 0),
        itemStyle: { color: color2 },
        barMaxWidth: 20
      }
    ]
  }
}
