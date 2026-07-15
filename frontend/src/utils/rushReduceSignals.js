/** 自选持仓：大阳/新高后首阴 → 早减（早于「减」，保利润） */

import { signalSortRank } from './icePointSignals'
import { hasPositionContext } from './addPositionSignals'
import { calcRushReducePct, rushReduceHint } from './rushReduceRatio'

export const RUSH_REDUCE_TAG = '冲'

export function isRushReduceTag(tag) {
  return tag === RUSH_REDUCE_TAG
}

function nearRecentHigh(i, closes, highs, lookback) {
  let peak = -Infinity
  const start = Math.max(0, i - lookback)
  for (let j = start; j < i; j++) {
    const h = highs[j] ?? closes[j]
    if (h != null) peak = Math.max(peak, h)
  }
  const cPrev = closes[i - 1]
  if (!Number.isFinite(peak) || peak <= 0 || cPrev == null) return false
  return cPrev >= peak * 0.97
}

function hadRecentRushOrSell(sig, barIndex, addMap, lookback) {
  for (let j = Math.max(0, barIndex - lookback); j < barIndex; j++) {
    if (addMap?.has(j)) continue
    if ((sig?.sellMa20 || []).includes(j) || (sig?.sellRsi || []).includes(j)) return true
  }
  return false
}

/**
 * @returns {Map<number, { rushReducePct: number }>}
 */
export function computeRushReduceMap(sig, bars, positionCtx, options = {}, existingRush = new Map()) {
  const map = new Map(existingRush)
  if (!hasPositionContext(positionCtx) || !sig) return map

  const costPrice = Number(positionCtx.costPrice)
  const minProfitPct = options.rushReduceMinProfitPct ?? 0.03
  const minPrevBodyPct = options.rushReduceMinPrevBodyPct ?? 0.015
  const minDropPct = options.rushReduceMinDropPct ?? 0.012
  const highLookback = options.rushReduceHighLookback ?? 8
  const minRsiPrev = options.rushReduceMinRsiPrev ?? 55
  const minGap = options.rushReduceMinGap ?? 5
  const blockDays = options.rushReduceSellBlockDays ?? 4

  const { closes = [], opens = [], highs = [] } = bars || {}
  const rsi = sig?.rsi ?? []
  const ma20 = sig?.ma20 ?? []
  const len = closes.length

  for (let i = 1; i < len; i++) {
    if (map.has(i)) continue

    const o = opens[i]
    const c = closes[i]
    const oPrev = opens[i - 1]
    const cPrev = closes[i - 1]
    if (o == null || c == null || oPrev == null || cPrev == null) continue

    if (c < costPrice * (1 + minProfitPct)) continue
    if (!(cPrev > oPrev && c < o)) continue

    const bodyPrev = oPrev > 0 ? (cPrev - oPrev) / oPrev : 0
    if (bodyPrev < minPrevBodyPct) continue

    const drop = cPrev > 0 ? (cPrev - c) / cPrev : 0
    if (drop < minDropPct) continue

    const rPrev = rsi[i - 1]
    if (rPrev != null && rPrev < minRsiPrev) continue

    if (!nearRecentHigh(i, closes, highs, highLookback)) continue

    const m = ma20[i]
    if (m != null && c < m * 0.96) continue

    if (hadRecentRushOrSell(sig, i, map, blockDays)) continue

    let tooSoon = false
    for (const j of map.keys()) {
      if (i - j < minGap) {
        tooSoon = true
        break
      }
    }
    if (tooSoon) continue

    map.set(i, {
      rushReducePct: calcRushReducePct(i, bars, sig, options),
    })
  }
  return map
}

/** 有持仓时：今日若满足早减，优先于「减」展示 */
export function applyRushReduceToSummary(summary, bars, positionCtx, options = {}) {
  if (!summary?.ok || !hasPositionContext(positionCtx)) return summary

  const last = summary.signalLastIndex ?? (bars?.closes?.length ?? 1) - 1
  const addMap = computeRushReduceMap(summary, bars, positionCtx, options)
  const hit = addMap.get(last)
  if (!hit) return summary

  const hint = rushReduceHint(hit.rushReducePct)
  return {
    ...summary,
    tag: RUSH_REDUCE_TAG,
    tagType: 'warning',
    sellPositionPct: hit.rushReducePct,
    rushReducePct: hit.rushReducePct,
    sortRank: signalSortRank(RUSH_REDUCE_TAG),
    statusText: `今日早减 · 阳后阴回落 · RSI ${summary.latestStatus?.rsi?.toFixed(1) ?? '—'}${hint ? ` · ${hint}` : ''}`,
    recentSignalBar: last,
    recentSignalDaysAgo: 0,
  }
}

export function findRecentRushReduceBar(rushMap, lastIndex, recentDays = 15) {
  if (!rushMap?.size || lastIndex < 0) return null
  let best = null
  for (const [idx, meta] of rushMap) {
    const daysAgo = lastIndex - idx
    if (daysAgo < 0 || daysAgo > recentDays) continue
    if (!best || idx > best.index) {
      best = { index: idx, tag: RUSH_REDUCE_TAG, daysAgo, ...meta }
    }
  }
  return best
}
