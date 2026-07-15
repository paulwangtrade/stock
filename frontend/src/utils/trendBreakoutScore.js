/** 股票信息筛选 · 趋势/突破场景 4 维综合分 */

const TREND_PATTERN_KEYS = [
  'MACD_GOLDEN_FORK',
  'KDJ_GOLDEN_FORK',
  'BREAK_THROUGH',
  'BREAKUP_MA_5DAYS',
  'LONG_AVG_ARRAY',
  'ONE_DAYANG_LINE',
  'TWO_DAYANG_LINES',
  'RISE_SUN',
  'UPPER_8DAYS',
  'UPPER_4DAYS',
  'UPSIDE_VOLUME',
  'MORNING_STAR',
  'FIRST_DAWN',
  'POWER_FULGUN',
]

const TREND_NUMERIC_KEYS = ['UPNDAY', 'UPP_DAYS', 'CONCERN_RANK_7DAYS']

export const TREND_SCORE_WEIGHTS = {
  signal: 0.45,
  momentum: 0.2,
  liquidity: 0.2,
  pattern: 0.15,
}

function clamp(n, min = 0, max = 100) {
  return Math.max(min, Math.min(max, Math.round(n)))
}

function toNumber(value, defaultValue = 0) {
  if (value == null || value === '') return defaultValue
  const n = Number(String(value).replace(/,/g, ''))
  return Number.isFinite(n) ? n : defaultValue
}

/** 趋势场景信号分：强/趋/突优先，冰点/卖点降权 */
export function calcTrendSignalScore(summary) {
  if (!summary?.ok) return 32
  if (!summary.tag) return 35
  const baseByTag = { 强: 96, 趋: 88, 转: 86, 突: 82, 弹: 76, 买: 52, 冰: 38, 减: 10, 止: 18, 卖: 12 }
  let score = baseByTag[summary.tag] ?? 35
  const days = summary.recentSignalDaysAgo
  if (days != null && days > 0) {
    score -= Math.min(18, days * 4)
  }
  if (days === 0 && (summary.tag === '强' || summary.tag === '趋' || summary.tag === '转' || summary.tag === '突' || summary.tag === '弹')) {
    score += 6
  }
  return clamp(score)
}

/** 价格动能：温和上涨优先，暴涨/深跌降分 */
export function calcMomentumScore(row) {
  const rate = toNumber(row?.CHANGE_RATE, 0)
  if (rate >= 0.5 && rate <= 3) return 92
  if (rate > 3 && rate <= 5) return 82
  if (rate > 5 && rate <= 7) return 68
  if (rate > 7) return 48
  if (rate >= 0 && rate < 0.5) return 72
  if (rate >= -2 && rate < 0) return 58
  if (rate >= -5 && rate < -2) return 42
  return 28
}

/** 流动性：成交额分档 */
export function calcLiquidityScore(row) {
  const amount = toNumber(row?.DEAL_AMOUNT, 0)
  if (amount >= 500000000) return 96
  if (amount >= 100000000) return 88
  if (amount >= 50000000) return 78
  if (amount >= 10000000) return 62
  if (amount >= 5000000) return 48
  return 30
}

/** 形态共振：量比、换手、收盘强度 + 用户勾选的趋势筛选加成 */
export function calcPatternResonanceScore(row, activeIndicators) {
  let score = 50
  const volRatio = toNumber(row?.VOLUME_RATIO, 1)
  const turnover = toNumber(row?.TURNOVERRATE, 0)
  const high = toNumber(row?.HIGH_PRICE, 0)
  const low = toNumber(row?.LOW_PRICE, 0)
  const close = toNumber(row?.NEW_PRICE, 0)

  if (volRatio >= 2) score += 16
  else if (volRatio >= 1.5) score += 11
  else if (volRatio >= 1.2) score += 6
  else if (volRatio < 0.8) score -= 12

  if (turnover >= 2 && turnover <= 12) score += 10
  else if (turnover > 18) score -= 6

  if (high > low && close > 0) {
    const pos = (close - low) / (high - low)
    if (pos >= 0.85) score += 12
    else if (pos >= 0.7) score += 6
    else if (pos <= 0.35) score -= 8
  }

  if (activeIndicators && typeof activeIndicators === 'object') {
    let trendHits = 0
    for (const k of TREND_PATTERN_KEYS) {
      if (activeIndicators[k]) trendHits++
    }
    for (const k of TREND_NUMERIC_KEYS) {
      if (Number(activeIndicators[k]) > 0) trendHits++
    }
    if (trendHits > 0) {
      score += Math.min(18, trendHits * 4)
    }
  }

  return clamp(score)
}

/** 4 维加权综合分 */
export function calcTrendCompositeScore(row, signalSummary, activeIndicators) {
  const signal = calcTrendSignalScore(signalSummary)
  const momentum = calcMomentumScore(row)
  const liquidity = calcLiquidityScore(row)
  const pattern = calcPatternResonanceScore(row, activeIndicators)
  const w = TREND_SCORE_WEIGHTS
  const total = clamp(
    signal * w.signal + momentum * w.momentum + liquidity * w.liquidity + pattern * w.pattern,
  )
  return {
    total,
    signal,
    momentum,
    liquidity,
    pattern,
    tooltip: `信号 ${signal} · 动能 ${momentum} · 流动 ${liquidity} · 形态 ${pattern}`,
  }
}

export function trendScoreTextType(total) {
  if (total >= 80) return 'success'
  if (total >= 65) return 'warning'
  if (total >= 45) return 'info'
  return 'default'
}

/** 趋势分文字色（与 naive type 相比对比更强） */
export function getTrendScoreStyle(total) {
  const n = Number(total)
  if (!Number.isFinite(n)) return { color: '#94a3b8' }
  if (n >= 85) return { color: '#16a34a', fontWeight: '600' }
  if (n >= 70) return { color: '#ea580c', fontWeight: '600' }
  if (n >= 55) return { color: '#2563eb' }
  if (n >= 45) return { color: '#64748b' }
  return { color: '#94a3b8' }
}
