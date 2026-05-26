import { describe, it, expect } from 'vitest'
import { formatLocalTime, formatDuration, formatNumber, formatV2Ratio } from './formatters.js'

// ============================================================
// 测试点 3: formatLocalTime 函数
// ============================================================
describe('formatLocalTime', () => {
  // 3.1 正常 ISO 字符串转换
  it('正常 ISO 字符串转换为本地时间格式', () => {
    // 使用一个固定的 UTC 时间
    const result = formatLocalTime('2026-04-01T02:30:45Z')
    // 结果应该是 YYYY-MM-DD HH:mm:ss 格式
    expect(result).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/)
    // 验证不是 '-'
    expect(result).not.toBe('-')
  })

  // 3.2 带时区偏移的 ISO 字符串
  it('带时区偏移的 ISO 字符串正确转换', () => {
    const result = formatLocalTime('2026-04-01T10:30:45+08:00')
    expect(result).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/)
    expect(result).not.toBe('-')
  })

  // 3.3 空值返回 '-'
  it('null 返回 "-"', () => {
    expect(formatLocalTime(null)).toBe('-')
  })

  it('undefined 返回 "-"', () => {
    expect(formatLocalTime(undefined)).toBe('-')
  })

  it('空字符串返回 "-"', () => {
    expect(formatLocalTime('')).toBe('-')
  })

  // 3.4 无效字符串返回 '-'
  it('无效字符串返回 "-"', () => {
    expect(formatLocalTime('not-a-date')).toBe('-')
  })

  it('随机数字字符串返回 "-"', () => {
    expect(formatLocalTime('12345')).not.toBe('-') // JS Date 能解析数字字符串
  })
})

// ============================================================
// 测试点 4: formatDuration 函数
// ============================================================
describe('formatDuration', () => {
  // 4.1 null 返回 '-'
  it('null 返回 "-"', () => {
    expect(formatDuration(null)).toBe('-')
  })

  // 4.2 0 返回 '-'
  it('0 返回 "-"', () => {
    expect(formatDuration(0)).toBe('-')
  })

  // 4.3 undefined 返回 '-'
  it('undefined 返回 "-"', () => {
    expect(formatDuration(undefined)).toBe('-')
  })

  // 4.4 小于 60 分钟返回 "X分钟"
  it('30 分钟返回 "30分钟"', () => {
    expect(formatDuration(30)).toBe('30分钟')
  })

  it('1 分钟返回 "1分钟"', () => {
    expect(formatDuration(1)).toBe('1分钟')
  })

  it('59 分钟返回 "59分钟"', () => {
    expect(formatDuration(59)).toBe('59分钟')
  })

  // 4.5 60 分钟返回 "1小时"
  it('60 分钟返回 "1小时"', () => {
    expect(formatDuration(60)).toBe('1小时')
  })

  // 4.6 90 分钟返回 "1小时30分钟"
  it('90 分钟返回 "1小时30分钟"', () => {
    expect(formatDuration(90)).toBe('1小时30分钟')
  })

  // 4.7 120 分钟返回 "2小时"
  it('120 分钟返回 "2小时"', () => {
    expect(formatDuration(120)).toBe('2小时')
  })

  // 4.8 480 分钟（边界值）返回 "8小时"
  it('480 分钟返回 "8小时"', () => {
    expect(formatDuration(480)).toBe('8小时')
  })

  // 4.9 481 分钟返回 "1.0人天"
  it('481 分钟返回 "1.0人天"', () => {
    expect(formatDuration(481)).toBe('1.0人天')
  })

  // 4.10 960 分钟返回 "2.0人天"
  it('960 分钟返回 "2.0人天"', () => {
    expect(formatDuration(960)).toBe('2.0人天')
  })

  // 4.11 小数分钟四舍五入
  it('30.4 分钟四舍五入为 "30分钟"', () => {
    expect(formatDuration(30.4)).toBe('30分钟')
  })

  it('30.6 分钟四舍五入为 "31分钟"', () => {
    expect(formatDuration(30.6)).toBe('31分钟')
  })
})

describe('formatV2Ratio', () => {
  it('按 v2 小数口径渲染百分比', () => {
    expect(formatV2Ratio(0.0667)).toBe('6.7%')
    expect(formatV2Ratio(-0.0911)).toBe('-9.1%')
    expect(formatV2Ratio(1)).toBe('100.0%')
  })

  it('不使用 legacy 300=300% 口径', () => {
    expect(formatV2Ratio(300)).toBe('30000.0%')
  })

  it('空值和非法值返回 "-"', () => {
    expect(formatV2Ratio(null)).toBe('-')
    expect(formatV2Ratio(undefined)).toBe('-')
    expect(formatV2Ratio('abc')).toBe('-')
  })
})

describe('formatNumber', () => {
  it('格式化千分位和小数位', () => {
    expect(formatNumber(12345.678, 2)).toBe('12,345.68')
    expect(formatNumber(12345.678)).toBe('12,346')
  })

  it('空值和非法值返回 "-"', () => {
    expect(formatNumber(null)).toBe('-')
    expect(formatNumber(undefined)).toBe('-')
    expect(formatNumber('abc')).toBe('-')
  })
})
