import { toEastMoneyCode } from './stockCode.js'
import { toDisplayTradingLevel } from './tradingLevelRules.js'

export const STOCK_MARKET_SEGMENTS = {
  shMain: { key: 'shMain', short: '沪', name: '沪主板', indexCode: '000001.SH', indexName: '上证指数' },
  szMain: { key: 'szMain', short: '深', name: '深主板', indexCode: '399001.SZ', indexName: '深证成指' },
  star: { key: 'star', short: '科', name: '科创板', indexCode: '000688.SH', indexName: '科创50' },
  chinext: { key: 'chinext', short: '创', name: '创业板', indexCode: '399006.SZ', indexName: '创业板指' },
  beijing: { key: 'beijing', short: '京', name: '北交所', indexCode: '899050.BJ', indexName: '北证50' },
  unknown: { key: 'unknown', short: '—', name: '未知市场', indexCode: '', indexName: '' },
}

export const STOCK_MARKET_SEGMENT_ORDER = ['shMain', 'szMain', 'star', 'chinext', 'beijing']

/** A 股代码 -> 沪主板 / 深主板 / 科创板 / 创业板 / 北交所。 */
export function resolveStockMarketSegment(code) {
  const normalized = toEastMoneyCode(code).toUpperCase()
  const number = normalized.match(/^(\d{6})\.(SH|SZ|BJ)$/)?.[1] || ''
  if (!number) return STOCK_MARKET_SEGMENTS.unknown

  if (/^(688|689)/.test(number)) return STOCK_MARKET_SEGMENTS.star
  if (/^(300|301)/.test(number)) return STOCK_MARKET_SEGMENTS.chinext
  if (/^(4|8|920)/.test(number) || normalized.endsWith('.BJ')) return STOCK_MARKET_SEGMENTS.beijing
  if (/^(600|601|603|605)/.test(number) || normalized.endsWith('.SH')) return STOCK_MARKET_SEGMENTS.shMain
  if (/^(000|001|002|003)/.test(number) || normalized.endsWith('.SZ')) return STOCK_MARKET_SEGMENTS.szMain
  return STOCK_MARKET_SEGMENTS.unknown
}

function modeLevel(mode, fallback = 3) {
  const level = Number(mode?.level)
  return Number.isFinite(level) ? level : fallback
}

/** 总市场与所属市场取更谨慎的级别，所属市场不能反向放宽总仓位纪律。 */
export function resolveEffectiveStockMarketMode(globalMode, segmentMode, segment) {
  if (!globalMode?.level && !segmentMode?.level) {
    return {
      key: 'unknown', level: null, name: '数据不足', label: '数据不足',
      reason: '总市场及所属市场指数数据不足',
    }
  }

  const globalLevel = modeLevel(globalMode)
  const segmentLevel = modeLevel(segmentMode)
  const effective = segmentLevel <= globalLevel ? segmentMode : globalMode
  const level = Math.min(globalLevel, segmentLevel)
  const source = segmentLevel <= globalLevel ? segment?.name || '所属市场' : '总市场'
  return {
    ...effective,
    key: `level${level}`,
    level,
    label: `${toDisplayTradingLevel(level)}级 ${effective?.name || ''}`.trim(),
    reason: `${source}约束：总市场${toDisplayTradingLevel(globalLevel)}级，${segment?.name || '所属市场'}${toDisplayTradingLevel(segmentLevel)}级`,
  }
}
