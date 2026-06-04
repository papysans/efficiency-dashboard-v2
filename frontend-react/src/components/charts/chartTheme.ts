// ECharts 亮暗主题色板 —— option 工厂按 theme 取色，对齐玻璃拟态设计语言。
// 主色 Apple Blue #0071e3，亮暗两套坐标轴/网格/文字/tooltip 色。

export type ChartTheme = 'light' | 'dark'

export interface ChartPalette {
  brand: string
  axisColor: string
  splitLineColor: string
  textColor: string
  tooltipBg: string
  tooltipBorder: string
  tooltipText: string
  /** 面积渐变上端色（带透明度，从 echarts.graphic 拼） */
  areaTop: string
  areaBottom: string
}

const BRAND = '#0071e3'

export function getPalette(theme: ChartTheme): ChartPalette {
  if (theme === 'dark') {
    return {
      brand: BRAND,
      axisColor: '#9ca3af',
      splitLineColor: '#374151',
      textColor: '#9ca3af',
      tooltipBg: 'rgba(30,30,40,0.92)',
      tooltipBorder: 'rgba(255,255,255,0.08)',
      tooltipText: '#e5e7eb',
      areaTop: 'rgba(0,113,227,0.35)',
      areaBottom: 'rgba(0,113,227,0)',
    }
  }
  return {
    brand: BRAND,
    axisColor: '#9ca3af',
    splitLineColor: '#e5e7eb',
    textColor: '#374151',
    tooltipBg: 'rgba(255,255,255,0.95)',
    tooltipBorder: 'rgba(0,0,0,0.06)',
    tooltipText: '#374151',
    areaTop: 'rgba(0,113,227,0.20)',
    areaBottom: 'rgba(0,113,227,0)',
  }
}
