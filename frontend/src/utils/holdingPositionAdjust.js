/** 持仓增减仓辅助：量能、换手率、板块资金流 + 大盘模式（参考，非投资建议） */

import { hasPositionContext, isAddPositionTag } from './addPositionSignals'
import { isRushReduceTag } from './rushReduceSignals'
import { isSellSignalTag } from './icePointSignals'
import { resolveTradingLevel } from './positionPolicy'
import { MARKET_LEVEL_IMPACT, toDisplayTradingLevel } from './tradingLevelRules'
import { BUY_ENTRY_TAGS } from './buyPriceRange'

function clamp(n, min, max) {
  return Math.max(min, Math.min(max, n))
}

function roundPct01(p) {
  return Math.round(p * 20) / 20
}

function normText(s) {
  return String(s || '')
    .trim()
    .replace(/板块|概念|行业/g, '')
    .replace(/\s+/g, '')
}

/** 行业/板块名 ↔ 新浪板块资金榜 */
export function matchSectorFlow(industry, bkName, rankLists = {}) {
  const targets = [industry, bkName].map(normText).filter(Boolean)
  if (!targets.length) return null

  const pools = [rankLists.industry || [], rankLists.concept || []]
  for (const pool of pools) {
    for (let i = 0; i < pool.length; i++) {
      const row = pool[i]
      const name = normText(row?.name)
      if (!name) continue
      for (const t of targets) {
        if (name.includes(t) || t.includes(name)) {
          const net = Number(row.netamount)
          const ratio = Number(row.ratioamount)
          return {
            name: row.name,
            netamount: Number.isFinite(net) ? net : 0,
            ratioamount: Number.isFinite(ratio) ? ratio : 0,
            rank: i + 1,
            inflow: net > 0,
          }
        }
      }
    }
  }
  return null
}

/** 近 N 日量均 vs 当日量；当日换手 */
export function analyzeVolumeTurnover(bars, lastIdx, lookback = 5) {
  const volumes = bars?.volumes ?? []
  const turnoverRates = bars?.turnoverRates ?? []
  const closes = bars?.closes ?? []
  const opens = bars?.opens ?? []
  const i = lastIdx ?? closes.length - 1
  if (i < 1 || !closes[i]) {
    return { volumeRatio: null, turnover: null, turnoverRatio: null, priceUp: null }
  }

  const volToday = volumes[i] || 0
  let sum = 0
  let cnt = 0
  for (let j = Math.max(0, i - lookback); j < i; j++) {
    if (volumes[j] > 0) {
      sum += volumes[j]
      cnt++
    }
  }
  const avgVol = cnt > 0 ? sum / cnt : null
  const volumeRatio = avgVol && avgVol > 0 && volToday > 0 ? volToday / avgVol : null

  const turnoverRaw = turnoverRates[i]
  const turnover = turnoverRaw != null && Number.isFinite(Number(turnoverRaw)) ? Number(turnoverRaw) : null
  let turnoverSum = 0
  let turnoverCount = 0
  for (let j = Math.max(0, i - lookback); j < i; j++) {
    const value = Number(turnoverRates[j])
    if (Number.isFinite(value) && value > 0) {
      turnoverSum += value
      turnoverCount++
    }
  }
  const avgTurnover = turnoverCount ? turnoverSum / turnoverCount : null
  const turnoverRatio = turnover != null && avgTurnover > 0 ? turnover / avgTurnover : null

  const c = closes[i]
  const o = opens[i] ?? c
  const prev = closes[i - 1]
  const priceUp = prev != null ? c >= prev && c >= o : c >= o

  return { volumeRatio, turnover, turnoverRatio, priceUp }
}

/**
 * @returns {{ action: string, actionLabel: string, score: number, suggestPct: number|null, factors: object[], summaryLine: string }}
 */
export function evaluateHoldingPositionAdjust(ctx = {}) {
  const {
    bars,
    lastIdx,
    positionCtx,
    marketModeKey = 'unknown',
    globalMarketMode = null,
    segmentMarketMode = null,
    marketSegment = null,
    sectorFlow = null,
    profitPct = null,
  } = ctx

  const factors = []
  let score = 0

  const marketRule = resolveTradingLevel(marketModeKey)
  const marketLevel = marketRule.level ?? 3
  const marketImpact = MARKET_LEVEL_IMPACT[marketLevel] ?? MARKET_LEVEL_IMPACT[3]
  score += marketImpact
  factors.push({
    key: 'market',
    label: `${marketSegment?.short || '总'}${toDisplayTradingLevel(marketLevel)}级${marketRule.name}`,
    impact: marketImpact,
    detail: globalMarketMode?.level && segmentMarketMode?.level
      ? `总市场${toDisplayTradingLevel(globalMarketMode.level)}级、${marketSegment?.name || '所属市场'}${toDisplayTradingLevel(segmentMarketMode.level)}级，按${toDisplayTradingLevel(marketLevel)}级执行；仓位 ${marketRule.rangeLabel}`
      : `模型总仓位 ${marketRule.rangeLabel}；${marketRule.action}`,
  })

  const { volumeRatio, turnover, turnoverRatio, priceUp } = analyzeVolumeTurnover(bars, lastIdx)
  if (volumeRatio != null) {
    if (priceUp && volumeRatio >= 1.25) {
      score += 10
      factors.push({
        key: 'volume',
        label: '放量上涨',
        impact: 10,
        detail: `量比约 ${volumeRatio.toFixed(2)}，价涨量增`,
      })
    } else if (priceUp && volumeRatio < 0.75) {
      score -= 6
      factors.push({
        key: 'volume',
        label: '缩量上涨',
        impact: -6,
        detail: `量比约 ${volumeRatio.toFixed(2)}，上攻动能偏弱`,
      })
    } else if (!priceUp && volumeRatio >= 1.25) {
      score -= 14
      factors.push({
        key: 'volume',
        label: '放量下跌',
        impact: -14,
        detail: `量比约 ${volumeRatio.toFixed(2)}，疑似派发`,
      })
    } else if (!priceUp && volumeRatio < 0.8) {
      score += 4
      factors.push({
        key: 'volume',
        label: '缩量回调',
        impact: 4,
        detail: `量比约 ${volumeRatio.toFixed(2)}，回调未放量`,
      })
    }
  }

  if (turnover != null) {
    if (turnover > 18 || (turnoverRatio != null && turnoverRatio >= 2.2)) {
      score -= 9
      factors.push({
        key: 'turnover',
        label: '换手过热',
        impact: -9,
        detail: `换手 ${turnover.toFixed(2)}%${turnoverRatio ? `，约近5日均值 ${turnoverRatio.toFixed(2)} 倍` : ''}`,
      })
    } else if (turnover >= 1 && turnover <= 12 && (turnoverRatio == null || turnoverRatio >= 0.8)) {
      score += 5
      factors.push({
        key: 'turnover',
        label: '换手健康',
        impact: 5,
        detail: `换手 ${turnover.toFixed(2)}%${turnoverRatio ? `，约近5日均值 ${turnoverRatio.toFixed(2)} 倍` : ''}`,
      })
    } else if (turnover < 0.8 || (turnoverRatio != null && turnoverRatio < 0.6)) {
      score -= 4
      factors.push({ key: 'turnover', label: '换手偏低', impact: -4, detail: `换手 ${turnover.toFixed(2)}%，流动性一般` })
    }
  }

  if (sectorFlow) {
    const netWan = sectorFlow.netamount / 10000
    const flowPct = sectorFlow.ratioamount * 100
    if (sectorFlow.inflow && sectorFlow.rank <= 8) {
      score += 12
      factors.push({
        key: 'sector',
        label: '板块流入',
        impact: 12,
        detail: `${sectorFlow.name} 净流入约 ${netWan.toFixed(0)} 万、净流入率 ${flowPct.toFixed(2)}%（榜 ${sectorFlow.rank}）`,
      })
    } else if (!sectorFlow.inflow && flowPct <= -1) {
      score -= 10
      factors.push({
        key: 'sector',
        label: '板块流出',
        impact: -10,
        detail: `${sectorFlow.name} 净流出约 ${Math.abs(netWan).toFixed(0)} 万、净流入率 ${flowPct.toFixed(2)}%`,
      })
    } else if (sectorFlow.inflow) {
      score += 5
      factors.push({
        key: 'sector',
        label: '板块偏强',
        impact: 5,
        detail: `${sectorFlow.name} 资金净流入`,
      })
    } else {
      score -= 5
      factors.push({
        key: 'sector',
        label: '板块偏弱',
        impact: -5,
        detail: `${sectorFlow.name} 资金净流出`,
      })
    }
  }

  if (profitPct != null && Number.isFinite(profitPct)) {
    if (profitPct >= 15) {
      score -= 4
      factors.push({ key: 'profit', label: '浮盈较大', impact: -4, detail: `浮盈 ${profitPct.toFixed(1)}%，宜保护利润` })
    } else if (profitPct <= -8) {
      score -= 8
      factors.push({ key: 'profit', label: '浮亏较深', impact: -8, detail: `浮亏 ${profitPct.toFixed(1)}%，勿盲目加仓` })
    }
  }

  let action = 'hold'
  let actionLabel = '持有观望'
  let suggestPct = null

  if (marketLevel === 1) {
    action = 'reduce'
    actionLabel = '清仓防守'
    suggestPct = 1
  } else if (marketLevel === 2) {
    action = 'reduce'
    actionLabel = '逢高减仓'
    suggestPct = clamp(0.3 + Math.max(0, -score) / 120, 0.3, 0.7)
  } else if (score >= 22 && marketLevel >= 4) {
    action = 'add'
    actionLabel = '倾向加仓'
    suggestPct = clamp(0.15 + score / 200, 0.12, 0.3)
  } else if (score <= -22) {
    action = 'reduce'
    actionLabel = '倾向减仓'
    suggestPct = clamp(0.18 + Math.abs(score) / 250, 0.15, 0.35)
  }

  const topFactors = factors
    .slice()
    .sort((a, b) => Math.abs(b.impact) - Math.abs(a.impact))
    .slice(0, 3)
    .map((f) => f.label)
  const summaryLine = topFactors.length ? topFactors.join('·') : '量价与板块中性'

  return {
    action,
    actionLabel,
    score,
    suggestPct,
    suggestPctDisplay: suggestPct == null ? null : Math.round(suggestPct * 100),
    marketLevel,
    marketLevelName: marketRule.name,
    marketPositionRange: marketRule.rangeLabel,
    globalMarketLevel: globalMarketMode?.level ?? null,
    segmentMarketLevel: segmentMarketMode?.level ?? null,
    marketSegmentKey: marketSegment?.key || 'unknown',
    marketSegmentName: marketSegment?.name || '所属市场',
    marketSegmentShort: marketSegment?.short || '—',
    factors,
    summaryLine,
  }
}

function adjustPctByScore(basePct, score, direction) {
  if (basePct == null || !Number.isFinite(basePct)) return basePct
  const delta = direction === 'add' ? score / 400 : -score / 400
  return roundPct01(clamp(basePct + delta, 0.1, 0.4))
}

/** 仅持仓：叠加量能/换手/板块流，微调加减仓比例并给出 holdingAdvice */
export function applyHoldingPositionAdjustToSummary(summary, bars, positionCtx, adjustCtx = {}) {
  if (!summary?.ok || !hasPositionContext(positionCtx)) return summary

  const lastIdx = summary.recentSignalBar ?? (bars?.closes?.length ?? 1) - 1
  const holdingAdvice = evaluateHoldingPositionAdjust({
    bars,
    lastIdx,
    positionCtx,
    marketModeKey: adjustCtx.marketModeKey,
    globalMarketMode: adjustCtx.globalMarketMode,
    segmentMarketMode: adjustCtx.segmentMarketMode,
    marketSegment: adjustCtx.marketSegment,
    sectorFlow: adjustCtx.sectorFlow,
    profitPct: adjustCtx.profitPct,
  })

  let next = { ...summary, holdingAdvice }

  const tag = next.tag
  const score = holdingAdvice.score

  if ((isAddPositionTag(tag) || BUY_ENTRY_TAGS.has(tag)) && holdingAdvice.marketLevel <= 3) {
    next = {
      ...next,
      tag: null,
      tagType: 'default',
      addPositionPct: null,
      statusText: `持仓 · ${holdingAdvice.actionLabel} · 总${toDisplayTradingLevel(holdingAdvice.globalMarketLevel) ?? '—'}级｜${holdingAdvice.marketSegmentShort}${toDisplayTradingLevel(holdingAdvice.segmentMarketLevel) ?? '—'}级｜执行${toDisplayTradingLevel(holdingAdvice.marketLevel) ?? '—'}级`,
    }
  }

  if (isAddPositionTag(next.tag) && next.addPositionPct != null) {
    next.addPositionPct = adjustPctByScore(next.addPositionPct, score, 'add')
  }
  if ((isSellSignalTag(next.tag) || isRushReduceTag(next.tag)) && (next.sellPositionPct ?? next.rushReducePct) != null) {
    const key = isRushReduceTag(next.tag) ? 'rushReducePct' : 'sellPositionPct'
    next[key] = adjustPctByScore(next[key], score, 'reduce')
    if (holdingAdvice.marketLevel === 1) next[key] = 1
    else if (holdingAdvice.marketLevel === 2) next[key] = Math.max(next[key], 0.5)
  }

  const suffix = ` · ${holdingAdvice.actionLabel}(${holdingAdvice.summaryLine})`
  if (next.statusText && !next.statusText.includes(holdingAdvice.actionLabel)) {
    next.statusText = `${next.statusText}${suffix}`
  } else if (!next.statusText || next.statusText === '—') {
    next.statusText = `持仓 · ${holdingAdvice.actionLabel} · ${holdingAdvice.summaryLine}`
  }

  // 无技术信号但综合分极端：保留 advice 供卡片展示
  if (!next.tag && holdingAdvice.suggestPct != null) {
    next.holdingAdvice = {
      ...holdingAdvice,
      suggestPctDisplay: Math.round(holdingAdvice.suggestPct * 100),
    }
  }

  return next
}
