/** K 线信号形态提取与相似匹配（策略结果筛选用） */

/**
 * 从 summarizeBuySignal 结果提取形态特征
 * @param {object} summary computeFullSignals + summarize 合并结果
 * @param {number} lastIndex 信号截止 K 线索引
 * @param {number[]} closes 收盘价序列
 */
export function extractSignalPattern(summary, lastIndex, closes, options = {}) {
  const windowDays = options.windowDays ?? 60
  const inWindow = (indices) =>
    (indices || []).filter((i) => {
      const d = lastIndex - i
      return d >= 0 && d <= windowDays
    })

  const trendHits = inWindow(summary.trendBuy)
  const reversalHits = inWindow(summary.reversalBuy)
  const breakoutHits = inWindow(summary.breakoutBuy)
  const takeProfitHits = inWindow(summary.sellRsi)
  const reduceHits = inWindow(summary.sellMa20)

  const ma20 = summary.ma20?.[lastIndex]
  const close = closes?.[lastIndex]
  const belowMa20 = ma20 != null && close != null && ma20 > 0 && close < ma20

  return {
    tag: summary.tag || '',
    daysAgo: summary.recentSignalDaysAgo ?? null,
    hadTrendOrBreakout: trendHits.length > 0 || reversalHits.length > 0 || breakoutHits.length > 0,
    hadTakeProfit: takeProfitHits.length > 0,
    hadReduce: reduceHits.length > 0,
    inIce: !!summary.inIce || summary.latestStatus?.type === 'info',
    belowMa20,
  }
}

/** 九联科技类：主升(趋/突) → 高位止 → 今日破 MA20 减 */
export const PEAK_PULLBACK_REDUCE_PATTERN = {
  tag: '减',
  daysAgo: 0,
  hadTrendOrBreakout: true,
  hadTakeProfit: true,
  belowMa20: true,
}

export function isSimilarSignalPattern(ref, cand, options = {}) {
  const requireToday = options.requireToday !== false
  if (!ref?.tag || !cand?.tag) return false
  if (ref.tag !== cand.tag) return false
  if (requireToday && (ref.daysAgo !== 0 || cand.daysAgo !== 0)) return false

  if (options.matchUptrendBefore !== false && ref.hadTrendOrBreakout && !cand.hadTrendOrBreakout) {
    return false
  }
  if (options.matchHadTakeProfit && ref.hadTakeProfit && !cand.hadTakeProfit) {
    return false
  }
  if (options.matchBelowMa20 && ref.belowMa20 && !cand.belowMa20) {
    return false
  }
  if (options.matchInIce && ref.inIce && !cand.inIce) {
    return false
  }
  return true
}

export function describeSignalPattern(p) {
  if (!p?.tag) return '—'
  const parts = []
  if (p.daysAgo === 0) parts.push(`今日${p.tag}`)
  else if (p.tag) parts.push(p.tag)
  if (p.hadTrendOrBreakout) parts.push('曾趋/转/突')
  if (p.hadTakeProfit) parts.push('曾止')
  if (p.belowMa20) parts.push('MA20下')
  if (p.inIce) parts.push('冰点区')
  return parts.join(' · ') || p.tag
}
