/** 自选持仓场景：趋势回踩/强化/突破 → 加仓标记（须已有仓且浮盈） */

import { signalSortRank } from './icePointSignals'
import { calcAddPositionPct, addPositionHint } from './addPositionRatio'

export const ADD_POSITION_TAG = '加'

/** 可升级为加仓的源买点 */
export const ADD_SOURCE_TAGS = new Set(['强', '趋', '突'])

export function isAddPositionTag(tag) {
  return tag === ADD_POSITION_TAG
}

export function hasPositionContext(ctx) {
  const costPrice = Number(ctx?.costPrice) || 0
  const costVolume = Number(ctx?.costVolume) || 0
  return costPrice > 0 && costVolume > 0
}

function collectEntryIndices(sig) {
  const set = new Set()
  for (const key of ['buy', 'strictBuy', 'trendBuy', 'reversalBuy', 'breakoutBuy', 'reboundBuy']) {
    for (const i of sig?.[key] || []) set.add(i)
  }
  return [...set].sort((a, b) => a - b)
}

function hadRecentSell(sig, barIndex, lookback) {
  const sellSet = new Set([...(sig?.sellMa20 || []), ...(sig?.sellRsi || [])])
  for (let j = Math.max(0, barIndex - lookback); j < barIndex; j++) {
    if (sellSet.has(j)) return true
  }
  return false
}

function nearestPriorEntryBefore(barIndex, entryIndices, minGap) {
  let best = null
  for (const j of entryIndices) {
    if (j < barIndex && barIndex - j >= minGap) {
      if (best == null || j > best) best = j
    }
  }
  return best
}

function resolveSourceTag(i, sig) {
  if (sig?.strictBuy?.includes(i)) return '强'
  if (sig?.breakoutBuy?.includes(i)) return '突'
  return '趋'
}

function candidateBarIndices(sig) {
  const set = new Set([
    ...(sig?.strictBuy || []),
    ...(sig?.trendBuy || []),
    ...(sig?.breakoutBuy || []),
  ])
  return [...set].sort((a, b) => a - b)
}

/**
 * @returns {Map<number, { sourceTag: string, addPositionPct: number }>}
 */
export function computeAddPositionMap(sig, bars, positionCtx, options = {}) {
  const map = new Map()
  if (!hasPositionContext(positionCtx) || !sig) return map

  const costPrice = Number(positionCtx.costPrice)
  const minProfitPct = options.addMinProfitPct ?? 0.03
  const minGapDays = options.addMinGapDays ?? 5
  const sellBlockDays = options.addSellBlockDays ?? 6
  const maxRsi = options.addMaxRsi ?? 68

  const closes = bars?.closes ?? []
  const ma20 = sig?.ma20 ?? []
  const rsi = sig?.rsi ?? []
  const entryIndices = collectEntryIndices(sig)

  for (const i of candidateBarIndices(sig)) {
    const c = closes[i]
    if (c == null || c <= costPrice * (1 + minProfitPct)) continue
    if (ma20[i] != null && c < ma20[i]) continue
    if (rsi[i] != null && rsi[i] > maxRsi) continue
    if (hadRecentSell(sig, i, sellBlockDays)) continue
    if (nearestPriorEntryBefore(i, entryIndices, minGapDays) == null) continue

    const sourceTag = resolveSourceTag(i, sig)
    map.set(i, {
      sourceTag,
      addPositionPct: calcAddPositionPct(sourceTag, i, bars, sig, options),
    })
  }
  return map
}

/** 扫描摘要：有持仓时将 强/趋/突 升级为 加 */
export function applyAddPositionToSummary(summary, bars, positionCtx, options = {}) {
  if (!summary?.ok || !hasPositionContext(positionCtx)) return summary
  if (!ADD_SOURCE_TAGS.has(summary.tag)) return summary

  const barIndex = summary.recentSignalBar
  if (barIndex == null) return summary

  const addMap = computeAddPositionMap(summary, bars, positionCtx, options)
  const meta = addMap.get(barIndex)
  if (!meta) return summary

  const days = summary.recentSignalDaysAgo ?? 0
  const hint = addPositionHint(meta.addPositionPct)
  return {
    ...summary,
    tag: ADD_POSITION_TAG,
    sourceTag: meta.sourceTag,
    addPositionPct: meta.addPositionPct,
    tagType: 'success',
    sortRank: signalSortRank(ADD_POSITION_TAG),
    statusText: `${days === 0 ? '今日' : `${days}日前`}加仓(${meta.sourceTag}) · RSI ${summary.latestStatus?.rsi?.toFixed(1) ?? '—'}${hint ? ` · ${hint}` : ''}`,
  }
}

/** K 线聚焦：最近一根加仓 bar */
export function findRecentAddPositionBar(addMap, lastIndex, recentDays = 15) {
  if (!addMap?.size || lastIndex < 0) return null
  let best = null
  for (const [idx, meta] of addMap) {
    const daysAgo = lastIndex - idx
    if (daysAgo < 0 || daysAgo > recentDays) continue
    if (!best || idx > best.index) {
      best = { index: idx, tag: ADD_POSITION_TAG, daysAgo, ...meta }
    }
  }
  return best
}
