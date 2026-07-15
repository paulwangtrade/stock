/** 自选：减/止 在已设成本+股数时按成本验浮盈并换算股数；未设定则走原技术规则 */

import {
  isSellSignalTag,
  SELL_TAG_TAKE_PROFIT,
  pickPrimaryRecentSignal,
  summarizeBuySignal,
} from './icePointSignals'
import { isRushReduceTag } from './rushReduceSignals'
import { formatSellPositionPct } from './sellPositionRatio'
import { hasPositionContext } from './addPositionSignals'

export { hasPositionContext }

export function roundSellVolume(holding, pct, minLot = 100) {
  const h = Math.floor(Number(holding) || 0)
  if (h <= 0 || !Number.isFinite(pct) || pct <= 0) return 0
  let v = Math.floor((h * pct) / minLot) * minLot
  if (v <= 0 && h >= minLot) v = minLot
  if (v > h) v = Math.floor(h / minLot) * minLot
  return v
}

/** 止：相对自选成本须仍有浮盈（默认 ≥0，可配） */
export function passesCostGateForTakeProfit(barClose, costPrice, options = {}) {
  const cost = Number(costPrice)
  const c = Number(barClose)
  if (!Number.isFinite(cost) || cost <= 0 || !Number.isFinite(c)) return false
  const minGain = options.costTakeProfitMinGainPct ?? 0
  return c >= cost * (1 + minGain)
}

function pickAltBuySummary(summary, bars, options) {
  const closes = bars?.closes ?? []
  const last = summary.signalLastIndex ?? closes.length - 1
  if (last < 0) return null
  const alt = pickPrimaryRecentSignal(summary, last, {
    ...options,
    includeSell: false,
    recentBuyDays: options.recentBuyDays ?? 0,
    recentSellDays: 0,
  })
  if (!alt?.tag) return null
  return summarizeBuySignal(bars, {
    ...options,
    signalLastIndex: last,
    includeSell: false,
  })
}

/** 有成本+股数时：止未达成本浮盈则回退买点标签 */
export function downgradeSellIfCostGateFails(summary, bars, positionCtx, options = {}) {
  if (!summary?.ok || !hasPositionContext(positionCtx)) return summary
  const tag = summary.tag
  if (tag !== SELL_TAG_TAKE_PROFIT) return summary

  const costPrice = Number(positionCtx.costPrice)
  const closes = bars?.closes ?? []
  const barIdx = summary.recentSignalBar
  const closeAtSignal =
    barIdx != null && closes[barIdx] != null ? closes[barIdx] : closes[closes.length - 1]

  if (passesCostGateForTakeProfit(closeAtSignal, costPrice, options)) return summary

  const alt = pickAltBuySummary(summary, bars, options)
  return alt?.tag ? alt : { ...summary, tag: '', sellPositionPct: null, statusText: '—' }
}

/** 有成本+股数：校验止的浮盈，并换算卖出股数 */
export function applyCostAwareSellToSummary(summary, bars, positionCtx, options = {}) {
  if (!summary?.ok || !hasPositionContext(positionCtx)) return summary

  let next = downgradeSellIfCostGateFails(summary, bars, positionCtx, options)
  const tag = next.tag
  if (!isSellSignalTag(tag) && !isRushReduceTag(tag)) return next

  const costPrice = Number(positionCtx.costPrice)
  const costVolume = Number(positionCtx.costVolume)
  const pct = next.sellPositionPct ?? next.rushReducePct
  const sellVolume = roundSellVolume(costVolume, pct)

  let statusText = next.statusText || ''
  const pctLabel = pct != null ? formatSellPositionPct(pct) : ''
  if (sellVolume > 0) {
    const action = tag === '止' ? '止盈' : tag === '减' ? '减仓' : '早减'
    statusText = `${statusText} · 成本${costPrice.toFixed(2)} · ${action}${sellVolume}股${pctLabel ? `(${pctLabel})` : ''}`
  }

  return {
    ...next,
    costPrice,
    costVolume,
    sellVolume,
    statusText,
  }
}

/** K 线：有成本+股数时，止逐根按成本验浮盈 */
export function filterSellMarkersByCost(sig, bars, positionCtx, options = {}) {
  if (!hasPositionContext(positionCtx)) {
    return { sellMa20: [...(sig?.sellMa20 || [])], sellRsi: [...(sig?.sellRsi || [])] }
  }
  const costPrice = Number(positionCtx.costPrice)
  const closes = bars?.closes ?? []
  const sellMa20 = [...(sig?.sellMa20 || [])]
  const sellRsi = (sig?.sellRsi || []).filter((i) => {
    const c = closes[i]
    return passesCostGateForTakeProfit(c, costPrice, options)
  })
  return { sellMa20, sellRsi }
}

export function enrichSellHintWithCost(hint, positionCtx, pct) {
  if (!hint || !hasPositionContext(positionCtx)) return hint
  const sellVolume = roundSellVolume(positionCtx.costVolume, pct)
  if (sellVolume <= 0) return hint
  const costLine = `成本 ${Number(positionCtx.costPrice).toFixed(2)} · 卖 ${sellVolume} 股`
  return {
    ...hint,
    lines: [...(hint.lines || []), costLine],
    tooltip: [hint.tooltip, costLine].filter(Boolean).join('\n'),
  }
}
