/**
 * Phase9-A/B/B.5/C.2: Holding Decision Adapter + controlled switch.
 * C.2: preferred authority may be riskAdvice_projection (fallback legacy).
 * Legacy holdingAdvice generation and shadow comparison are retained.
 */

import { compareLegacyHoldingToProjection } from './riskAdviceProjection.js'
import {
  HOLDING_DECISION_COMPARISON,
  ROLLING_WINDOW_SIZE,
  getHoldingDecisionAdapterMetrics,
  getHoldingDecisionRecentOutcomes,
  recordHoldingDecisionAdapterObservation,
  recordHoldingDecisionSimulatedSwitch,
} from './holdingDecisionAdapterMetrics.js'

export const HOLDING_DECISION_SOURCE_LEGACY = 'legacy'
export const HOLDING_DECISION_SOURCE_PROJECTION = 'riskAdvice_projection'
export { HOLDING_DECISION_COMPARISON, ROLLING_WINDOW_SIZE }

/** Minimum samples before READY_FOR_CONTROLLED_SWITCH is allowed. */
export const HOLDING_MIGRATION_MIN_SAMPLES = 30
/** Rolling window considered "stable" when match rate stays high. */
export const HOLDING_MIGRATION_STABLE_MATCH_FLOOR = 0.8

const SWITCH_OBSERVATION_HISTORY_SIZE = 50

/** @type {string} Phase9-C.2: controlled switch ON by default */
let preferredAuthoritySource = HOLDING_DECISION_SOURCE_PROJECTION
/** @type {object[]} */
let controlledSwitchObservations = []
let initialSwitchObservationRecorded = false
/** ISO timestamp when current projection-preferred session started */
let controlledSwitchActiveSince = new Date().toISOString()

/** Status thresholds for getControlledSwitchStatus (observation only). */
export const CONTROLLED_SWITCH_FALLBACK_DEGRADED = 0.1
export const CONTROLLED_SWITCH_FALLBACK_ROLLBACK = 0.25
export const CONTROLLED_SWITCH_DIFF_DEGRADED = 0.2
export const CONTROLLED_SWITCH_DIFF_ROLLBACK = 0.4
export const CONTROLLED_SWITCH_UNAVAILABLE_DEGRADED = 0.15

function normalizeLevel(level) {
  if (level == null || level === '') return null
  const n = Number(level)
  return Number.isFinite(n) ? n : null
}

function hasTargetHint(holdingAdvice) {
  const pct = holdingAdvice?.suggestPct
  return pct != null && Number.isFinite(Number(pct)) && Number(pct) > 0
}

function normalizePreferredSource(source) {
  return source === HOLDING_DECISION_SOURCE_PROJECTION
    ? HOLDING_DECISION_SOURCE_PROJECTION
    : HOLDING_DECISION_SOURCE_LEGACY
}

/**
 * Record switch observation snapshot (memory only; no DB).
 * Shape: { source, timestamp, sampleCount, diffRate, unavailableRate, ... }
 */
export function recordControlledSwitchObservation(extra = {}) {
  const m = getHoldingDecisionAdapterMetrics()
  const total = m.total
  const freq = m.shadowMigrationMetrics.comparisonFrequency
  const exact = freq.EXACT_MATCH || 0
  const targetAligned = freq.TARGET_DIFF || 0
  const unavailable = freq.LEGACY_ONLY || 0
  const aligned = exact + targetAligned
  const diffCount = Math.max(0, total - aligned - unavailable)
  const diffRate = total > 0 ? diffCount / total : 0
  const unavailableRate = total > 0 ? unavailable / total : 0

  const row = {
    source: preferredAuthoritySource,
    timestamp: new Date().toISOString(),
    sampleCount: total,
    diffRate: Number(diffRate.toFixed(4)),
    unavailableRate: Number(unavailableRate.toFixed(4)),
    previousSource: extra.previousSource ?? null,
    reason: extra.reason || 'switch',
  }
  controlledSwitchObservations.push(row)
  if (controlledSwitchObservations.length > SWITCH_OBSERVATION_HISTORY_SIZE) {
    controlledSwitchObservations.shift()
  }
  return row
}

function ensureInitialControlledSwitchObservation() {
  if (initialSwitchObservationRecorded) return
  initialSwitchObservationRecorded = true
  recordControlledSwitchObservation({
    previousSource: HOLDING_DECISION_SOURCE_LEGACY,
    reason: 'c2_controlled_switch_active',
  })
}

export function getHoldingDecisionPreferredSource() {
  return preferredAuthoritySource
}

export function isHoldingDecisionControlledSwitchEnabled() {
  return preferredAuthoritySource === HOLDING_DECISION_SOURCE_PROJECTION
}

/**
 * Set authority preferred source. Consumers unchanged — only adapter decision selection.
 * Use rollbackHoldingDecisionToLegacy() to restore legacy authority.
 */
export function setHoldingDecisionAuthoritySource(source, options = {}) {
  const previousSource = preferredAuthoritySource
  preferredAuthoritySource = normalizePreferredSource(source)
  if (
    preferredAuthoritySource === HOLDING_DECISION_SOURCE_PROJECTION
    && previousSource !== HOLDING_DECISION_SOURCE_PROJECTION
  ) {
    controlledSwitchActiveSince = new Date().toISOString()
  }
  if (options.record !== false) {
    recordControlledSwitchObservation({
      previousSource,
      reason: options.reason || 'manual_set',
    })
  }
  return preferredAuthoritySource
}

/** Enable controlled switch (authority preferred = riskAdvice_projection). */
export function enableHoldingDecisionControlledSwitch(options = {}) {
  return setHoldingDecisionAuthoritySource(HOLDING_DECISION_SOURCE_PROJECTION, {
    reason: 'enable_controlled_switch',
    ...options,
  })
}

/** Rollback: restore authority preferred = legacy (consumers need no code change). */
export function rollbackHoldingDecisionToLegacy(options = {}) {
  return setHoldingDecisionAuthoritySource(HOLDING_DECISION_SOURCE_LEGACY, {
    reason: 'rollback_legacy',
    ...options,
  })
}

export function getControlledSwitchObservations() {
  return controlledSwitchObservations.slice()
}

export function resetControlledSwitchObservations() {
  controlledSwitchObservations = []
  initialSwitchObservationRecorded = false
}

/**
 * Phase9-C.3: Switch Metrics snapshot (projection vs fallback vs diff).
 */
export function getControlledSwitchMetrics() {
  const m = getHoldingDecisionAdapterMetrics()
  const sw = m.switchMetrics || {
    projectionSelectedCount: 0,
    legacyFallbackCount: 0,
    preferredLegacyCount: 0,
    fallbackRate: 0,
    projectionUsageRate: 0,
    decisionDiffRate: 0,
    projectionUnavailableReasons: {},
  }
  return {
    projectionSelectedCount: sw.projectionSelectedCount,
    legacyFallbackCount: sw.legacyFallbackCount,
    preferredLegacyCount: sw.preferredLegacyCount,
    fallbackRate: sw.fallbackRate,
    projectionUsageRate: sw.projectionUsageRate,
    decisionDiffRate: sw.decisionDiffRate,
    projectionUnavailableReasons: { ...sw.projectionUnavailableReasons },
    totalDecisions: m.total,
    preferredSource: getHoldingDecisionPreferredSource(),
    activeSince: controlledSwitchActiveSince,
  }
}

/**
 * Phase9-C.3: Controlled switch stability summary.
 * Does not auto-rollback — status is advisory only.
 *
 * @returns {{
 *   source: string,
 *   activeSince: string,
 *   totalDecisions: number,
 *   projectionUsageRate: number,
 *   fallbackRate: number,
 *   diffRate: number,
 *   status: 'HEALTHY'|'DEGRADED'|'ROLLBACK_RECOMMENDED',
 * }}
 */
export function getControlledSwitchStatus() {
  const metrics = getControlledSwitchMetrics()
  const source = getHoldingDecisionPreferredSource()
  const totalDecisions = metrics.totalDecisions
  const projectionUsageRate = metrics.projectionUsageRate
  const fallbackRate = metrics.fallbackRate
  const diffRate = metrics.decisionDiffRate
  const unavailableRate = totalDecisions > 0
    ? (getHoldingDecisionAdapterMetrics().projectionUnavailable || 0) / totalDecisions
    : 0

  let status = 'HEALTHY'
  if (source === HOLDING_DECISION_SOURCE_LEGACY) {
    status = 'ROLLBACK_RECOMMENDED'
  } else if (
    fallbackRate > CONTROLLED_SWITCH_FALLBACK_ROLLBACK
    || diffRate > CONTROLLED_SWITCH_DIFF_ROLLBACK
  ) {
    status = 'ROLLBACK_RECOMMENDED'
  } else if (
    fallbackRate > CONTROLLED_SWITCH_FALLBACK_DEGRADED
    || diffRate > CONTROLLED_SWITCH_DIFF_DEGRADED
    || unavailableRate > CONTROLLED_SWITCH_UNAVAILABLE_DEGRADED
  ) {
    status = 'DEGRADED'
  }

  return {
    source,
    activeSince: controlledSwitchActiveSince,
    totalDecisions,
    projectionUsageRate: Number(projectionUsageRate.toFixed(4)),
    fallbackRate: Number(fallbackRate.toFixed(4)),
    diffRate: Number(diffRate.toFixed(4)),
    status,
    unavailableRate: Number(unavailableRate.toFixed(4)),
    projectionSelectedCount: metrics.projectionSelectedCount,
    legacyFallbackCount: metrics.legacyFallbackCount,
    rolledBack: source === HOLDING_DECISION_SOURCE_LEGACY,
  }
}

export function classifyHoldingDecisionComparison(holdingAdvice, shadowProjection) {
  const hasLegacy = !!(holdingAdvice && typeof holdingAdvice === 'object')
  const hasProjection = !!(shadowProjection && typeof shadowProjection === 'object')

  if (hasLegacy && !hasProjection) return HOLDING_DECISION_COMPARISON.LEGACY_ONLY
  if (!hasLegacy && hasProjection) return HOLDING_DECISION_COMPARISON.PROJECTION_ONLY
  if (!hasLegacy && !hasProjection) return HOLDING_DECISION_COMPARISON.LEGACY_ONLY

  const cmp = compareLegacyHoldingToProjection(holdingAdvice, shadowProjection)
  const levelDiff = !cmp.levelMatch
  const actionDiff = !cmp.actionMatch

  if (!levelDiff && !actionDiff) {
    if (hasTargetHint(holdingAdvice)) return HOLDING_DECISION_COMPARISON.TARGET_DIFF
    return HOLDING_DECISION_COMPARISON.EXACT_MATCH
  }
  if (levelDiff && actionDiff) return HOLDING_DECISION_COMPARISON.MULTI_FIELD_DIFF
  if (levelDiff) return HOLDING_DECISION_COMPARISON.LEVEL_DIFF
  return HOLDING_DECISION_COMPARISON.ACTION_DIFF
}

export function resolveHoldingDecisionDiff(holdingAdvice, shadowProjection) {
  const code = classifyHoldingDecisionComparison(holdingAdvice, shadowProjection)
  if (code === HOLDING_DECISION_COMPARISON.LEGACY_ONLY) return 'projectionUnavailable'
  if (code === HOLDING_DECISION_COMPARISON.EXACT_MATCH) return 'same'
  return 'different'
}

export function mapProjectionToSimulatedDecision(shadowProjection) {
  if (!shadowProjection || typeof shadowProjection !== 'object') return null
  return {
    action: shadowProjection.projectedAction || 'none',
    actionLabel: `projected:${shadowProjection.projectedAction || 'none'}`,
    marketLevel: normalizeLevel(shadowProjection.projectedLevel),
    suggestPct: null,
    suggestPctDisplay: null,
    summaryLine: Array.isArray(shadowProjection.riskReasons)
      ? shadowProjection.riskReasons.map((r) => r.code).filter(Boolean).slice(0, 3).join('·')
      : '',
    factors: [],
    source: HOLDING_DECISION_SOURCE_PROJECTION,
    category: shadowProjection.category || null,
    marketDriven: !!shadowProjection.marketDriven,
    confidence: shadowProjection.confidence ?? null,
  }
}

export function simulateRiskAdviceSource(input = {}) {
  const holdingAdvice = input.holdingAdvice && typeof input.holdingAdvice === 'object'
    ? input.holdingAdvice
    : (input.legacyDecision && typeof input.legacyDecision === 'object' ? input.legacyDecision : null)
  const shadowProjection = input.riskAdviceProjection && typeof input.riskAdviceProjection === 'object'
    ? input.riskAdviceProjection
    : (input.shadowProjection && typeof input.shadowProjection === 'object'
      ? input.shadowProjection
      : null)

  const comparisonCode = classifyHoldingDecisionComparison(holdingAdvice, shadowProjection)
  const simulatedDecision = mapProjectionToSimulatedDecision(shadowProjection)
  recordHoldingDecisionSimulatedSwitch()

  return {
    simulatedSource: HOLDING_DECISION_SOURCE_PROJECTION,
    simulatedDecision,
    legacyDecision: holdingAdvice,
    diff: comparisonCode,
    decisionDiff: resolveHoldingDecisionDiff(holdingAdvice, shadowProjection),
  }
}

function resolveUnavailableReason(holdingAdvice, shadowProjection, meta = {}) {
  if (shadowProjection) return null
  if (meta.unavailableReason) return String(meta.unavailableReason)
  if (meta.shadowFailure) return `shadow_${meta.shadowFailure}`
  if (!holdingAdvice) return 'both_absent'
  return 'missing_projection'
}

/**
 * Resolve effective authority decision for current preferred source.
 * Projection preferred + missing projection → fallback legacy (never bare null if legacy exists).
 */
function resolveAuthorityDecisionPair(holdingAdvice, shadowProjection, preferred) {
  if (preferred === HOLDING_DECISION_SOURCE_PROJECTION) {
    if (shadowProjection) {
      return {
        source: HOLDING_DECISION_SOURCE_PROJECTION,
        decision: mapProjectionToSimulatedDecision(shadowProjection),
        fallbackUsed: false,
      }
    }
    return {
      source: HOLDING_DECISION_SOURCE_LEGACY,
      decision: holdingAdvice,
      fallbackUsed: true,
    }
  }
  return {
    source: HOLDING_DECISION_SOURCE_LEGACY,
    decision: holdingAdvice,
    fallbackUsed: false,
  }
}

export function getHoldingDecision(input = {}) {
  ensureInitialControlledSwitchObservation()

  const holdingAdvice = input.holdingAdvice && typeof input.holdingAdvice === 'object'
    ? input.holdingAdvice
    : null
  const shadowProjection = input.riskAdviceProjection && typeof input.riskAdviceProjection === 'object'
    ? input.riskAdviceProjection
    : (input.shadowProjection && typeof input.shadowProjection === 'object'
      ? input.shadowProjection
      : null)
  const meta = input.meta && typeof input.meta === 'object' ? input.meta : {}
  const symbol = String(input.symbol || meta.stockCode || shadowProjection?.symbol || '').trim() || null
  const record = input.record !== false
  const preferred = getHoldingDecisionPreferredSource()

  const resolved = resolveAuthorityDecisionPair(holdingAdvice, shadowProjection, preferred)
  const source = resolved.source
  const decision = resolved.decision
  const fallbackUsed = resolved.fallbackUsed

  const comparisonCode = classifyHoldingDecisionComparison(holdingAdvice, shadowProjection)
  const decisionDiff = resolveHoldingDecisionDiff(holdingAdvice, shadowProjection)
  const unavailableReason = resolveUnavailableReason(holdingAdvice, shadowProjection, {
    ...meta,
    shadowFailure: input.shadowFailure || meta.shadowFailure,
  })

  const result = {
    source,
    decision,
    preferredSource: preferred,
    fallbackUsed,
    shadowProjection,
    decisionDiff,
    comparisonCode,
    legacyDecision: holdingAdvice,
    projectionDecision: shadowProjection,
  }

  if (record) {
    recordHoldingDecisionAdapterObservation({
      source,
      hasLegacy: !!holdingAdvice,
      hasProjection: !!shadowProjection,
      decisionDiff,
      comparisonCode,
      symbol,
      action: decision?.action || holdingAdvice?.action || shadowProjection?.projectedAction || null,
      level: decision?.marketLevel ?? holdingAdvice?.marketLevel ?? shadowProjection?.projectedLevel ?? null,
      unavailableReason,
      fallbackUsed,
    })
  }

  return result
}

/**
 * Phase9-C.1/C.2 consumer choke: authority decision object only (no source).
 * Prefer scan-precomputed `holdingDecision.decision`; else resolve via adapter.
 */
export function resolveAuthorityHoldingDecision(input = {}) {
  if (input == null || typeof input !== 'object') return null
  const bundled = input.holdingDecision
  if (bundled && typeof bundled === 'object' && Object.prototype.hasOwnProperty.call(bundled, 'decision')) {
    const d = bundled.decision
    return d && typeof d === 'object' ? d : null
  }
  const resolved = getHoldingDecision({
    holdingAdvice: input.holdingAdvice,
    riskAdviceProjection: input.riskAdviceProjection,
    shadowProjection: input.shadowProjection,
    symbol: input.symbol || input.code,
    shadowFailure: input.shadowFailure,
    meta: input.meta,
    record: false,
  })
  return resolved.decision
}

/**
 * Rolling stability snapshot (observation only — does not auto-switch).
 */
export function getHoldingMigrationStability() {
  const recent = getHoldingDecisionRecentOutcomes()
  const totalSamples = getHoldingDecisionAdapterMetrics().total
  const n = recent.length
  if (!n) {
    return {
      totalSamples,
      rollingMatchRate: 0,
      rollingDiffRate: 0,
      recentTrend: 'insufficient_data',
      stable: false,
      windowSize: ROLLING_WINDOW_SIZE,
      windowFill: 0,
    }
  }

  let match = 0
  let unavailable = 0
  for (const o of recent) {
    if (o.match) match += 1
    if (o.unavailable) unavailable += 1
  }
  const diff = n - match - unavailable
  const rollingMatchRate = match / n
  const rollingDiffRate = Math.max(0, diff) / n
  const rollingUnavailableRate = unavailable / n

  const half = Math.floor(n / 2)
  let recentTrend = 'flat'
  if (half >= 5) {
    const older = recent.slice(0, half)
    const newer = recent.slice(half)
    const olderMatch = older.filter((o) => o.match).length / older.length
    const newerMatch = newer.filter((o) => o.match).length / newer.length
    const delta = newerMatch - olderMatch
    if (delta >= 0.05) recentTrend = 'improving'
    else if (delta <= -0.05) recentTrend = 'worsening'
    else recentTrend = 'flat'
  } else {
    recentTrend = 'insufficient_data'
  }

  const stable = n >= Math.min(20, ROLLING_WINDOW_SIZE)
    && rollingMatchRate >= HOLDING_MIGRATION_STABLE_MATCH_FLOOR
    && rollingUnavailableRate <= 0.1
    && recentTrend !== 'worsening'

  return {
    totalSamples,
    rollingMatchRate: Number(rollingMatchRate.toFixed(4)),
    rollingDiffRate: Number(rollingDiffRate.toFixed(4)),
    rollingUnavailableRate: Number(rollingUnavailableRate.toFixed(4)),
    recentTrend,
    stable,
    windowSize: ROLLING_WINDOW_SIZE,
    windowFill: n,
  }
}

/**
 * Concentration of non-exact comparison types (0–1). Higher = more concentrated.
 */
function diffTypeConcentration(freq) {
  const entries = Object.entries(freq || {})
    .filter(([k, n]) => n > 0 && k !== 'EXACT_MATCH' && k !== 'TARGET_DIFF')
  const sum = entries.reduce((a, [, n]) => a + n, 0)
  if (sum <= 0) return 1
  const top = Math.max(...entries.map(([, n]) => n))
  return top / sum
}

/**
 * Migration readiness (observation only; does not auto-switch preferred source).
 */
export function getHoldingMigrationReadiness() {
  const m = getHoldingDecisionAdapterMetrics()
  const stability = getHoldingMigrationStability()
  const freq = m.shadowMigrationMetrics.comparisonFrequency
  const total = m.total
  const exact = freq.EXACT_MATCH || 0
  const targetAligned = freq.TARGET_DIFF || 0
  const unavailable = (freq.LEGACY_ONLY || 0)
  const aligned = exact + targetAligned
  const diffCount = Math.max(0, total - aligned - unavailable)

  const exactMatchRate = total > 0 ? exact / total : 0
  const alignedMatchRate = total > 0 ? aligned / total : 0
  const diffRate = total > 0 ? diffCount / total : 0
  const projectionUnavailableRate = total > 0 ? unavailable / total : 0
  const concentration = diffTypeConcentration(freq)

  const majorDiffReasons = Object.entries(freq)
    .filter(([k, n]) => n > 0 && k !== 'EXACT_MATCH')
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
    .slice(0, 5)
    .map(([reason, count]) => ({ reason, count }))

  const criteria = {
    minSamplesMet: total >= HOLDING_MIGRATION_MIN_SAMPLES,
    alignedMatchOk: alignedMatchRate >= 0.85,
    unavailableOk: projectionUnavailableRate <= 0.05,
    diffOk: diffRate <= 0.15,
    diffConcentrated: concentration >= 0.6 || diffCount === 0,
    unavailableStable: stability.rollingUnavailableRate <= 0.1,
    rollingStable: stability.stable === true,
  }

  let recommendation = 'NOT_READY'
  if (!criteria.minSamplesMet || projectionUnavailableRate > 0.2 || alignedMatchRate < 0.5) {
    recommendation = 'NOT_READY'
  } else if (
    criteria.minSamplesMet
    && criteria.alignedMatchOk
    && criteria.unavailableOk
    && criteria.diffOk
    && criteria.unavailableStable
    && (criteria.diffConcentrated || criteria.rollingStable)
  ) {
    recommendation = 'READY_FOR_CONTROLLED_SWITCH'
  } else {
    recommendation = 'NEED_MORE_OBSERVATION'
  }

  const preferred = getHoldingDecisionPreferredSource()
  return {
    total,
    exactMatchRate: Number(exactMatchRate.toFixed(4)),
    alignedMatchRate: Number(alignedMatchRate.toFixed(4)),
    diffRate: Number(diffRate.toFixed(4)),
    projectionUnavailableRate: Number(projectionUnavailableRate.toFixed(4)),
    diffTypeConcentration: Number(concentration.toFixed(4)),
    majorDiffReasons,
    highestFrequencyDiffSymbols: m.shadowMigrationMetrics.highestFrequencyDiffSymbols,
    highestFrequencyDiffTypes: m.shadowMigrationMetrics.highestFrequencyDiffTypes,
    projectionUnavailableReasons: m.shadowMigrationMetrics.projectionUnavailableReasons,
    stability,
    criteria,
    recommendation,
    /** Preferred authority source (C.2); not auto-mutated by Gate. */
    sourceLocked: preferred,
    authoritySource: preferred,
    controlledSwitchEnabled: preferred === HOLDING_DECISION_SOURCE_PROJECTION,
    simulatedSwitchCount: m.shadowMigrationMetrics.simulatedSwitchCount,
  }
}
