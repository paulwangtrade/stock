/** 买入仓位：固定比例风险 + 信号置信度 + 持仓/敞口约束（参考值，非投资建议） */

import { BUY_ENTRY_TAGS } from './buyPriceRange.js'
import { calculateModelPositionCap } from './tradingLevelRules.js'

function clamp(n, min, max) {
  return Math.max(min, Math.min(max, n))
}

function roundLot(shares, lot) {
  const l = Math.max(1, lot || 100)
  return Math.floor(shares / l) * l
}

/** 止损价：信号日低点下方 1.5%，或 MA20 下方 1% */
export function resolveStopPrice(summary, buyPriceRange, entryPrice) {
  const sigLow = buyPriceRange?.signalLow
  const ma20 = summary?.latestStatus?.ma20
  if (Number.isFinite(sigLow) && sigLow > 0) return sigLow * 0.985
  if (Number.isFinite(ma20) && ma20 > 0) return ma20 * 0.99
  if (Number.isFinite(entryPrice) && entryPrice > 0) return entryPrice * 0.95
  return null
}

/**
 * @returns {{
 *   ok: boolean,
 *   tag: string,
 *   entryPrice: number,
 *   stopPrice: number,
 *   riskPerShare: number,
 *   confidence: number,
 *   suggestedShares: number,
 *   suggestedAddShares: number,
 *   suggestedAmount: number,
 *   positionPct: number,
 *   riskAmount: number,
 *   reason: string,
 * }}
 * 通过 ctx.marketModeKey 显式接入等级仓位上限，避免依赖 positionPolicy 的设置状态。
 */
export function calcBuyPositionPlan(ctx) {
  const {
    summary,
    buyPriceRange,
    livePrice,
    automation,
    existingVolume = 0,
    existingCostPrice = 0,
    totalExposureValue = 0,
    marketModeKey,
  } = ctx

  const tag = summary?.tag
  if (!BUY_ENTRY_TAGS.has(tag)) {
    return { ok: false, reason: '非买点信号' }
  }

  const cfg = automation || {}
  const equity = Number(cfg.accountEquity) || 500000
  const riskPct = Number(cfg.riskPerTradePct) || 0.01
  const maxPosPct = Number(cfg.maxPositionPct) || 0.15
  const configuredMaxExpPct = Number(cfg.maxTotalExposurePct) || 0.85
  const maxExpPct = marketModeKey
    ? calculateModelPositionCap(marketModeKey, configuredMaxExpPct).pct
    : configuredMaxExpPct
  const lot = Number(cfg.minLotSize) || 100
  const confMap = cfg.tagConfidence || {}
  const confidence = confMap[tag] ?? 0.5

  const entryPrice =
    Number(livePrice) ||
    Number(buyPriceRange?.instantPrice) ||
    Number(buyPriceRange?.high) ||
    0
  if (!entryPrice || entryPrice <= 0) return { ok: false, reason: '无有效入场价' }

  const stopPrice = resolveStopPrice(summary, buyPriceRange, entryPrice)
  if (!stopPrice || stopPrice <= 0 || stopPrice >= entryPrice) {
    return { ok: false, reason: '止损价无效' }
  }

  const riskPerShare = entryPrice - stopPrice
  const riskBudget = equity * riskPct * confidence
  let shares = roundLot(riskBudget / riskPerShare, lot)

  const maxSharesByPosition = roundLot((equity * maxPosPct) / entryPrice, lot)
  shares = Math.min(shares, maxSharesByPosition)

  const roomExposure = Math.max(0, equity * maxExpPct - totalExposureValue)
  const maxSharesByExposure = roundLot(roomExposure / entryPrice, lot)
  shares = Math.min(shares, maxSharesByExposure)

  if (shares < lot) {
    return {
      ok: false,
      reason: '风险/敞口约束下建议仓位不足一手',
      entryPrice,
      stopPrice,
      confidence,
    }
  }

  const existingVal = existingVolume * (existingCostPrice || entryPrice)
  const targetVal = shares * entryPrice
  const addShares = Math.max(0, shares - existingVolume)
  const addAmount = addShares * entryPrice
  const positionPct = (targetVal / equity) * 100

  return {
    ok: true,
    tag,
    entryPrice,
    stopPrice,
    riskPerShare,
    confidence,
    suggestedShares: shares,
    suggestedAddShares: addShares,
    suggestedAmount: targetVal,
    suggestedAddAmount: addAmount,
    positionPct: Math.round(positionPct * 10) / 10,
    riskAmount: Math.round(shares * riskPerShare * 100) / 100,
    existingVolume,
    reason: existingVolume > 0
      ? addShares > 0
        ? `已有 ${existingVolume} 股，建议加仓 ${addShares} 股至目标 ${shares} 股`
        : `已有 ${existingVolume} 股，已达/超过目标仓位 ${shares} 股`
      : `建议买入 ${shares} 股，约占权益 ${positionPct.toFixed(1)}%`,
  }
}

export function formatPositionPlanText(plan) {
  if (!plan?.ok) return plan?.reason || '—'
  if (plan.existingVolume > 0 && plan.suggestedAddShares <= 0) {
    return `持仓 ${plan.existingVolume} · 已满仓`
  }
  if (plan.existingVolume > 0) {
    return `加 ${plan.suggestedAddShares} 股 → 共 ${plan.suggestedShares} 股`
  }
  return `${plan.suggestedShares} 股 · ${plan.positionPct}%`
}
