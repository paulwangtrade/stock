/**
 * buildRiskContextBundle — Phase8-0 Step1
 * Pure Context assembly. Not PlanFilter / Advice / Execution.
 *
 * Does not import holdingPositionAdjust (keeps builder isolated from legacy Advice module).
 */

import { POLICY_VERSION, PRODUCER_WATCHLIST_SCAN } from './constants.js'
import { resolveTradingLevel } from '../tradingLevelRules.js'

/**
 * Keys that must never appear on the Bundle output tree.
 * Includes Object Contract forbidden names + scan action fields.
 */
export const BUNDLE_OUTPUT_FORBIDDEN_KEYS = Object.freeze([
  'mustSell',
  'forceLiquidate',
  'forceClear',
  'sellRatio',
  'buyRatio',
  'sellPositionPct',
  'addPositionPct',
  'rushReducePct',
  'holdingAdvice',
  'order',
  'orders',
  'orderSide',
  'orderQty',
  'execution',
  'executionId',
  'executionPayload',
  'submitOrder',
  'broker',
  'brokerRequest',
  'brokerOrderId',
  'fillId',
  'fillQty',
  'action',
  'actionLabel',
  'suggestPct',
  'suggestPctDisplay',
  'mustBuy',
])

/**
 * @param {unknown} level
 * @returns {number|null}
 */
function normalizeLevel(level) {
  if (level == null || level === '') return null
  const n = Number(level)
  if (!Number.isFinite(n) || n < 1 || n > 5) return null
  return n
}

/**
 * @param {unknown} v
 * @returns {string}
 */
function trimStr(v) {
  return v == null ? '' : String(v).trim()
}

/**
 * Local volume/turnover summary (Context only; mirrors analyzeVolumeTurnover semantics).
 * @param {object} bars
 * @param {number} lastIdx
 * @param {number} [lookback]
 */
function summarizeVolumeTurnover(bars, lastIdx, lookback = 5) {
  const volumes = bars?.volumes ?? []
  const turnoverRates = bars?.turnoverRates ?? []
  const closes = bars?.closes ?? []
  const opens = bars?.opens ?? []
  const i = lastIdx ?? closes.length - 1
  if (i < 1 || !closes[i]) {
    return null
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
  const turnover =
    turnoverRaw != null && Number.isFinite(Number(turnoverRaw)) ? Number(turnoverRaw) : null
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
 * @param {object} input
 */
function resolveEffectiveLevelAndKey(input) {
  const effective = input?.effectiveMarketMode
  if (effective && (effective.level != null || effective.key)) {
    const key = trimStr(effective.key) || trimStr(input.marketModeKey) || 'unknown'
    const level =
      normalizeLevel(effective.level) ??
      normalizeLevel(resolveTradingLevel(key).level)
    const levelName =
      trimStr(effective.name) ||
      trimStr(effective.label) ||
      resolveTradingLevel(key).name ||
      undefined
    return { modeKey: key || 'unknown', level, levelName: levelName || undefined }
  }

  const marketModeKey = trimStr(input?.marketModeKey) || 'unknown'
  const rule = resolveTradingLevel(marketModeKey)
  return {
    modeKey: marketModeKey,
    level: normalizeLevel(rule.level),
    levelName: rule.name || undefined,
  }
}

/**
 * @param {object} input
 * @returns {{ volumeRatio?: number, turnover?: number, turnoverRatio?: number, priceUp?: boolean }|null}
 */
function resolveVolumeSummary(input) {
  const vs = input?.volumeSummary
  if (vs && typeof vs === 'object') {
    const out = {}
    if (vs.volumeRatio != null && Number.isFinite(Number(vs.volumeRatio))) {
      out.volumeRatio = Number(vs.volumeRatio)
    }
    if (vs.turnover != null && Number.isFinite(Number(vs.turnover))) {
      out.turnover = Number(vs.turnover)
    }
    if (vs.turnoverRatio != null && Number.isFinite(Number(vs.turnoverRatio))) {
      out.turnoverRatio = Number(vs.turnoverRatio)
    }
    if (typeof vs.priceUp === 'boolean') out.priceUp = vs.priceUp
    return Object.keys(out).length ? out : null
  }

  const bars = input?.bars
  if (!bars || typeof bars !== 'object') return null
  const lastIdx =
    input.lastIdx != null && Number.isFinite(Number(input.lastIdx))
      ? Number(input.lastIdx)
      : (bars.closes?.length ?? 1) - 1
  const analyzed = summarizeVolumeTurnover(bars, lastIdx)
  if (!analyzed) return null
  const out = {}
  if (analyzed.volumeRatio != null) out.volumeRatio = analyzed.volumeRatio
  if (analyzed.turnover != null) out.turnover = analyzed.turnover
  if (analyzed.turnoverRatio != null) out.turnoverRatio = analyzed.turnoverRatio
  if (typeof analyzed.priceUp === 'boolean') out.priceUp = analyzed.priceUp
  return Object.keys(out).length ? out : null
}

/**
 * @param {object} parts
 * @returns {number}
 */
function computeConfidence(parts) {
  let c = 0.3
  if (parts.level != null) c += 0.25
  if (parts.globalLevel != null) c += 0.12
  if (parts.segmentLevel != null || parts.segmentKey) c += 0.12
  if (parts.hasSector) c += 0.1
  if (parts.hasVolume) c += 0.11
  if (parts.stockCode) c += 0.05
  return Math.max(0, Math.min(1, Math.round(c * 100) / 100))
}

/**
 * Recursively collect forbidden keys present on value.
 * @param {unknown} value
 * @param {string[]} [acc]
 * @returns {string[]}
 */
export function scanForbiddenBundleKeys(value, acc = []) {
  if (value == null || typeof value !== 'object') return acc
  if (Array.isArray(value)) {
    for (const item of value) scanForbiddenBundleKeys(item, acc)
    return acc
  }
  for (const [k, v] of Object.entries(value)) {
    if (BUNDLE_OUTPUT_FORBIDDEN_KEYS.includes(k) && !acc.includes(k)) acc.push(k)
    scanForbiddenBundleKeys(v, acc)
  }
  return acc
}

/**
 * Build a RiskContextBundle from approved Context inputs only.
 *
 * @param {object} [input]
 * @param {string} [input.stockCode]
 * @param {string} [input.marketModeKey]
 * @param {object} [input.effectiveMarketMode]
 * @param {object} [input.globalMarketMode]
 * @param {object} [input.segmentMarketMode]
 * @param {object} [input.marketSegment]
 * @param {object} [input.sectorFlow]
 * @param {object} [input.volumeSummary]
 * @param {object} [input.bars]
 * @param {number} [input.lastIdx]
 * @param {string} [input.asOf]
 * @returns {object} RiskContextBundle
 */
export function buildRiskContextBundle(input = {}) {
  const stockCode = trimStr(input.stockCode)
  const { modeKey, level, levelName } = resolveEffectiveLevelAndKey(input)

  const globalLevel = normalizeLevel(input.globalMarketMode?.level)
  const segmentLevel = normalizeLevel(input.segmentMarketMode?.level)
  const segmentKey = trimStr(input.marketSegment?.key) || undefined
  const marketCode =
    trimStr(input.marketSegment?.indexCode) ||
    trimStr(input.marketSegment?.key) ||
    undefined

  const marketState = {
    level,
    modeKey,
  }
  if (levelName) marketState.levelName = levelName
  if (marketCode) marketState.marketCode = marketCode
  if (globalLevel != null) marketState.globalLevel = globalLevel
  if (segmentLevel != null) marketState.segmentLevel = segmentLevel
  if (segmentKey) marketState.segmentKey = segmentKey

  const inputsUsed = ['market_mode']
  if (segmentKey) inputsUsed.push('market_segment')
  if (globalLevel != null) inputsUsed.push('global_market_mode')
  if (segmentLevel != null) inputsUsed.push('segment_market_mode')

  const source = {
    producer: PRODUCER_WATCHLIST_SCAN,
    inputs: inputsUsed,
  }
  if (stockCode) source.stockCode = stockCode

  const sectorFlow = input.sectorFlow
  let sector
  let hasSector = false
  if (sectorFlow && typeof sectorFlow === 'object') {
    hasSector = true
    inputsUsed.push('sector_flow')
    sector = {
      name: trimStr(sectorFlow.name) || undefined,
      inflow: typeof sectorFlow.inflow === 'boolean' ? sectorFlow.inflow : undefined,
      rank: Number.isFinite(Number(sectorFlow.rank)) ? Number(sectorFlow.rank) : undefined,
      netamount: Number.isFinite(Number(sectorFlow.netamount))
        ? Number(sectorFlow.netamount)
        : undefined,
      ratioamount: Number.isFinite(Number(sectorFlow.ratioamount))
        ? Number(sectorFlow.ratioamount)
        : undefined,
    }
  }

  const volumeSummary = resolveVolumeSummary(input)
  let stock
  let hasVolume = false
  if (volumeSummary) {
    hasVolume = true
    inputsUsed.push('volume_summary')
    stock = {
      ...(stockCode ? { stockCode } : {}),
      ...volumeSummary,
    }
  } else if (stockCode) {
    stock = { stockCode }
  }

  const asOf = trimStr(input.asOf) || new Date().toISOString()

  const confidence = computeConfidence({
    level,
    globalLevel,
    segmentLevel,
    segmentKey,
    hasSector,
    hasVolume,
    stockCode,
  })

  const bundle = {
    marketState,
    source,
    asOf,
    policyVersion: POLICY_VERSION,
    confidence,
  }

  if (sector) bundle.sector = sector
  if (stock) bundle.stock = stock
  if (stockCode) bundle.subjectScope = 'stock'

  return bundle
}
