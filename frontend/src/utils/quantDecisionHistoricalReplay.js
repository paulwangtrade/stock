/**
 * QuantDecision Phase2-D：Historical Shadow Replay
 *
 * Design（harness-only，不接 UI / TradePlan / Execution，不改 js_legacy producer）：
 * 1. 输入：按日固定的 Signal / Gate / Zone / Size fixture
 * 2. 每日：组装 baseline Decision，与 candidate 做 Phase2-B0 semantic compare
 * 3. 聚合：复用 Phase2-C `buildShadowStabilityReport` 生成 daily + rollup
 * 4. Action transition matrix：相邻交易日 Action.Code 转移计数（baseline / candidate 各一份）
 */

import { assembleWatchlistDecision } from './quantDecisionAssemble.js'
import { mutateDecisionAction, PRODUCER_GO_ENGINE } from './quantDecisionCompare.js'
import { buildShadowStabilityReport } from './quantDecisionShadowStability.js'

/**
 * @typedef {object} ReplayDayFixture
 * @property {string} tradeDate
 * @property {string} [asOf]
 * @property {string} [code]
 * @property {string} [name]
 * @property {object} signal   固定 Signal 输入（至少 tag）
 * @property {object|null} [entryZone] 固定 Zone
 * @property {object|null} [gate] 固定 Gate / checklist 形
 * @property {object|null} [size] 固定 Size / plan 形
 * @property {number} [existingVolume]
 * @property {string} [marketModeKey]
 * @property {object} [candidate] candidate 覆盖：{ signal?, entryZone?, gate?, size?, actionPatch? }
 */

/**
 * 由固定 Signal/Gate/Zone/Size fixture 组装 js_legacy Decision（只读调用 assemble，不修改 producer）。
 * @param {ReplayDayFixture} day
 * @param {object} [seriesDefaults]
 */
export function decisionFromFixedSlices(day = {}, seriesDefaults = {}) {
  const code = day.code || seriesDefaults.code || ''
  const name = day.name || seriesDefaults.name || code || 'T'
  const signal = day.signal || {}
  const zone = day.entryZone ?? null
  const gate = day.gate ?? null
  const size = day.size ?? null

  const checklist = gate
    ? {
        ready: !!gate.ready,
        score: gate.score ?? 0,
        requiredPassed: gate.requiredPassed ?? false,
        readyThreshold: gate.readyThreshold ?? 0.85,
        items: Array.isArray(gate.items) ? gate.items : [],
      }
    : null

  const plan = size
    ? {
        ok: !!size.ok,
        suggestedShares: size.targetShares ?? size.suggestedShares ?? 0,
        suggestedAddShares: size.addShares ?? size.suggestedAddShares ?? 0,
        suggestedAmount: size.targetAmount ?? size.suggestedAmount,
        positionPct: size.positionPct,
        stopPrice: size.stopPrice,
        entryPrice: size.entryPrice,
        riskPerShare: size.riskPerShare,
        confidence: size.confidence,
        reason: size.reason || '',
        bindingConstraint: size.bindingConstraint || '',
      }
    : null

  return assembleWatchlistDecision({
    entry: {
      code,
      name,
      tag: signal.tag || '',
      daysAgo: signal.daysAgo ?? 0,
      buyPriceRange: zone,
      statusText: signal.summary || signal.statusText || '',
      signalScore: signal.score ?? 0,
      sourceTag: signal.sourceTag || '',
      sellPositionPct: signal.sellPositionPct,
      addPositionPct: signal.addPositionPct,
      rushReducePct: signal.rushReducePct,
      signalBar: signal.barIndex ?? signal.signalBar,
      effectiveSignalDayKey: signal.dayKey || '',
    },
    checklist,
    plan,
    existingVolume: day.existingVolume ?? seriesDefaults.existingVolume ?? 0,
    marketModeKey: day.marketModeKey || seriesDefaults.marketModeKey || 'level3',
    purpose: seriesDefaults.purpose || 'watchlist',
    tradeDate: day.tradeDate || '',
    asOf: day.asOf || (day.tradeDate ? `${day.tradeDate}T00:00:00.000Z` : null),
  })
}

/**
 * 构建 candidate：默认同切片 + producer=go_engine；可用 candidate 覆盖切片或 actionPatch。
 * @param {object} baseline
 * @param {ReplayDayFixture} day
 * @param {object} [seriesDefaults]
 */
export function candidateFromDay(baseline, day = {}, seriesDefaults = {}) {
  const override = day.candidate || {}
  let candidate
  if (override.signal || override.entryZone || override.gate || override.size) {
    candidate = decisionFromFixedSlices(
      {
        ...day,
        signal: override.signal || day.signal,
        entryZone: override.entryZone !== undefined ? override.entryZone : day.entryZone,
        gate: override.gate !== undefined ? override.gate : day.gate,
        size: override.size !== undefined ? override.size : day.size,
      },
      seriesDefaults,
    )
    candidate = {
      ...candidate,
      meta: { ...(candidate.meta || {}), producer: PRODUCER_GO_ENGINE },
    }
  } else {
    candidate = {
      ...baseline,
      meta: { ...(baseline.meta || {}), producer: PRODUCER_GO_ENGINE },
    }
  }
  if (override.actionPatch) {
    candidate = mutateDecisionAction(candidate, override.actionPatch, { producer: PRODUCER_GO_ENGINE })
  }
  return candidate
}

/**
 * Action.Code 转移矩阵（相邻日 from→to 计数）。
 * @param {string[]} codes 按时间序排列的 Action.Code
 */
export function buildActionTransitionMatrix(codes = []) {
  const cells = new Map()
  const fromTotals = new Map()
  const toTotals = new Map()
  let transitionCount = 0

  for (let i = 0; i < codes.length - 1; i++) {
    const from = codes[i] || '∅'
    const to = codes[i + 1] || '∅'
    const key = `${from}→${to}`
    cells.set(key, (cells.get(key) || 0) + 1)
    fromTotals.set(from, (fromTotals.get(from) || 0) + 1)
    toTotals.set(to, (toTotals.get(to) || 0) + 1)
    transitionCount++
  }

  const transitions = [...cells.entries()]
    .map(([pair, count]) => {
      const [fromCode, toCode] = pair.split('→')
      return { fromCode, toCode, count }
    })
    .sort((a, b) => b.count - a.count || a.fromCode.localeCompare(b.fromCode) || a.toCode.localeCompare(b.toCode))

  return {
    transitionCount,
    uniqueEdges: transitions.length,
    transitions,
    fromTotals: Object.fromEntries(fromTotals),
    toTotals: Object.fromEntries(toTotals),
  }
}

/**
 * 历史 Shadow Replay：逐日 semantic compare → daily Phase2-C report → rollup + transition matrix。
 *
 * @param {object} series
 * @param {string} [series.seriesId]
 * @param {string} [series.code]
 * @param {string} [series.name]
 * @param {ReplayDayFixture[]} series.days
 * @param {object} [options]
 */
export function runHistoricalShadowReplay(series = {}, options = {}) {
  const days = Array.isArray(series.days) ? series.days : []
  const seriesDefaults = {
    code: series.code || '',
    name: series.name || '',
    marketModeKey: series.marketModeKey || 'level3',
    existingVolume: series.existingVolume ?? 0,
    purpose: series.purpose || 'watchlist',
  }

  const daily = []
  const allPairs = []
  const baselineCodes = []
  const candidateCodes = []

  for (let i = 0; i < days.length; i++) {
    const day = days[i] || {}
    const tradeDate = day.tradeDate || `day-${i + 1}`
    const baseline = decisionFromFixedSlices(day, seriesDefaults)
    const candidate = candidateFromDay(baseline, day, seriesDefaults)

    const pair = {
      id: `${series.seriesId || 'hist'}:${tradeDate}`,
      code: baseline.instrument?.stockCode || seriesDefaults.code,
      tradeDate,
      baseline,
      candidate,
    }
    allPairs.push(pair)

    const dayStability = buildShadowStabilityReport([pair], options)
    daily.push({
      tradeDate,
      asOf: baseline.asOf,
      baselineActionCode: baseline.action?.code || '',
      candidateActionCode: candidate.action?.code || '',
      stability: dayStability,
    })

    baselineCodes.push(baseline.action?.code || '')
    candidateCodes.push(candidate.action?.code || '')
  }

  const rollup = buildShadowStabilityReport(allPairs, options)
  const baselineTransitions = buildActionTransitionMatrix(baselineCodes)
  const candidateTransitions = buildActionTransitionMatrix(candidateCodes)

  return {
    phase: 'Phase2-D',
    harness: 'quant-decision-historical-shadow-replay',
    generatedAt: new Date().toISOString(),
    seriesId: series.seriesId || 'unnamed',
    code: seriesDefaults.code,
    totalDays: days.length,
    daily,
    rollup,
    actionTransitions: {
      baseline: baselineTransitions,
      candidate: candidateTransitions,
      /** baseline→candidate 同日 Action.Code 对照边（非时间转移，便于看 shadow 分歧） */
      crossProducerSameDay: buildCrossProducerSameDayMatrix(baselineCodes, candidateCodes),
    },
    summary: summarizeReplay({
      totalDays: days.length,
      rollup,
      baselineTransitions,
    }),
  }
}

function buildCrossProducerSameDayMatrix(leftCodes, rightCodes) {
  const n = Math.min(leftCodes.length, rightCodes.length)
  const codesL = leftCodes.slice(0, n)
  const codesR = rightCodes.slice(0, n)
  // 复用转移矩阵形：把同日 L→R 当作边
  const cells = new Map()
  for (let i = 0; i < n; i++) {
    const from = codesL[i] || '∅'
    const to = codesR[i] || '∅'
    const key = `${from}→${to}`
    cells.set(key, (cells.get(key) || 0) + 1)
  }
  const transitions = [...cells.entries()]
    .map(([pair, count]) => {
      const [fromCode, toCode] = pair.split('→')
      return { fromCode, toCode, count }
    })
    .sort((a, b) => b.count - a.count || a.fromCode.localeCompare(b.fromCode))
  return {
    dayCount: n,
    uniqueEdges: transitions.length,
    transitions,
  }
}

function summarizeReplay({ totalDays, rollup, baselineTransitions }) {
  if (totalDays === 0) return 'EMPTY: no replay days'
  const fail = rollup?.semanticMismatchCount || 0
  const conflicts = rollup?.actionCode?.conflictCount || 0
  const edges = baselineTransitions?.uniqueEdges || 0
  if (fail === 0 && conflicts === 0) {
    return `STABLE_REPLAY: ${totalDays}d; actionTransitions=${edges}; 0 action.code conflicts`
  }
  return `UNSTABLE_REPLAY: ${totalDays}d; semanticFail=${fail}; actionCodeConflicts=${conflicts}; baselineTransitions=${edges}`
}
