/**
 * Phase8-0 Step3-C / Phase8.5 observation evaluation (read-only).
 * Does not change trading behavior, UI contracts, or projection/Card.
 */

import {
  getScanRiskAdviceShadowMetrics,
  resetScanRiskAdviceShadowMetrics,
  setScanRiskAdviceShadowMetricsEnabled,
} from './scanRiskAdviceShadowMetrics.js'
import {
  getRiskAdviceProjectionMetrics,
  resetRiskAdviceProjectionMetrics,
  setRiskAdviceProjectionMetricsEnabled,
} from './riskAdviceProjectionMetrics.js'
import {
  getHoldingDecisionAdapterMetrics,
  resetHoldingDecisionAdapterMetrics,
  setHoldingDecisionAdapterMetricsEnabled,
} from './holdingDecisionAdapterMetrics.js'
import { getHoldingMigrationReadiness, getHoldingMigrationStability, getControlledSwitchStatus } from './holdingDecisionAdapter.js'
import {
  getHoldingMigrationGateSnapshot,
  resetHoldingMigrationGateHistory,
} from './holdingMigrationGate.js'
import {
  classifyLegacyRiskAdviceObservation,
  normalizeAdviceLevel,
  OBSERVATION_CATEGORY,
  OBSERVATION_REASON,
} from './scanRiskAdviceShadowObservation.js'

function createEmptyEvalState() {
  return {
    enabled: true,
    matchCount: 0,
    mismatchCount: 0,
    failedCount: 0,
    levelDeltaDistribution: {},
    disagreementReasonFrequency: {},
    observationCategoryFrequency: {
      [OBSERVATION_CATEGORY.MATCH]: 0,
      [OBSERVATION_CATEGORY.MISMATCH]: 0,
      [OBSERVATION_CATEGORY.FAILED]: 0,
    },
    observationReasonFrequency: {},
  }
}

let evalState = createEmptyEvalState()

function bump(map, key) {
  const k = String(key)
  map[k] = (map[k] || 0) + 1
}

export { normalizeAdviceLevel, OBSERVATION_CATEGORY, OBSERVATION_REASON }

export function compareLegacyRiskAdvice(holdingAdvice, riskAdvice, meta = {}) {
  return classifyLegacyRiskAdviceObservation(holdingAdvice, riskAdvice, meta)
}

function levelDeltaKey(delta) {
  if (delta == null || !Number.isFinite(delta)) return 'unknown'
  if (delta === 0) return '0'
  return delta > 0 ? `+${delta}` : String(delta)
}

export function observeLegacyRiskAdvicePair(holdingAdvice, riskAdvice, meta = {}) {
  const result = compareLegacyRiskAdvice(holdingAdvice, riskAdvice, meta)
  if (!evalState.enabled) return result

  if (result.observationCategory === OBSERVATION_CATEGORY.MATCH) {
    evalState.matchCount += 1
    evalState.observationCategoryFrequency[OBSERVATION_CATEGORY.MATCH] += 1
    if (result.levelDelta != null) bump(evalState.levelDeltaDistribution, levelDeltaKey(result.levelDelta))
  } else if (result.observationCategory === OBSERVATION_CATEGORY.FAILED) {
    evalState.failedCount += 1
    evalState.observationCategoryFrequency[OBSERVATION_CATEGORY.FAILED] += 1
    bump(evalState.levelDeltaDistribution, 'unknown')
  } else {
    evalState.mismatchCount += 1
    evalState.observationCategoryFrequency[OBSERVATION_CATEGORY.MISMATCH] += 1
    bump(evalState.levelDeltaDistribution, levelDeltaKey(result.levelDelta))
  }

  bump(evalState.observationReasonFrequency, result.observationReason)
  if (!result.match) bump(evalState.disagreementReasonFrequency, result.reason)
  else if (result.observationReason === OBSERVATION_REASON.BOTH_ABSENT) {
    bump(evalState.disagreementReasonFrequency, result.reason)
  }

  return result
}

export function resetScanRiskAdviceShadowEvaluation() {
  evalState = createEmptyEvalState()
}

export function setScanRiskAdviceShadowEvaluationEnabled(on) {
  evalState.enabled = !!on
}

export function getTopDisagreementReasons(limit = 10) {
  return Object.entries(evalState.disagreementReasonFrequency)
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
    .slice(0, limit)
    .map(([reason, count]) => ({ reason, count }))
}

export function getTopObservationReasons(limit = 10) {
  return Object.entries(evalState.observationReasonFrequency)
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
    .slice(0, limit)
    .map(([reason, count]) => ({ reason, count }))
}

export function getScanRiskAdviceShadowEvaluation() {
  return {
    matchCount: evalState.matchCount,
    mismatchCount: evalState.mismatchCount,
    failedCount: evalState.failedCount,
    levelDeltaDistribution: { ...evalState.levelDeltaDistribution },
    disagreementReasonFrequency: { ...evalState.disagreementReasonFrequency },
    observationCategoryFrequency: { ...evalState.observationCategoryFrequency },
    observationReasonFrequency: { ...evalState.observationReasonFrequency },
    topDisagreementReasons: getTopDisagreementReasons(10),
    topObservationReasons: getTopObservationReasons(10),
    enabled: evalState.enabled,
  }
}

export function getShadowEvaluationSummary() {
  const evaluation = getScanRiskAdviceShadowEvaluation()
  const matched = evaluation.matchCount
  const mismatched = evaluation.mismatchCount
  const failed = evaluation.failedCount
  const totalComparisons = matched + mismatched + failed
  const mismatchRate = totalComparisons > 0 ? mismatched / totalComparisons : 0
  const matchRate = totalComparisons > 0 ? matched / totalComparisons : 0
  const failedRate = totalComparisons > 0 ? failed / totalComparisons : 0
  return {
    totalComparisons,
    matched,
    mismatched,
    failed,
    matchRate: Number(matchRate.toFixed(4)),
    mismatchRate: Number(mismatchRate.toFixed(4)),
    failedRate: Number(failedRate.toFixed(4)),
    topMismatchReasons: getTopObservationReasons(10).filter((r) =>
      ![
        OBSERVATION_REASON.EXACT_MATCH,
        OBSERVATION_REASON.BOTH_ABSENT,
      ].includes(r.reason),
    ),
    levelDeltaDistribution: evaluation.levelDeltaDistribution,
    observationCategoryFrequency: evaluation.observationCategoryFrequency,
    observationReasonFrequency: evaluation.observationReasonFrequency,
    enabled: evaluation.enabled,
  }
}

export function getShadowMigrationReadiness() {
  const summary = getShadowEvaluationSummary()
  const metrics = getScanRiskAdviceShadowMetrics()
  const obs = summary.observationReasonFrequency
  const exactMatch = obs[OBSERVATION_REASON.EXACT_MATCH] || 0
  const bothAbsent = obs[OBSERVATION_REASON.BOTH_ABSENT] || 0
  const effectiveDenominator = Math.max(0, summary.totalComparisons - bothAbsent)
  const effectiveMatchRate = effectiveDenominator > 0 ? exactMatch / effectiveDenominator : 0
  const buildAttempts = metrics.generated + metrics.failed
  const buildFailedRate = buildAttempts > 0 ? metrics.failed / buildAttempts : 0

  const majorMismatchReasons = getTopObservationReasons(5).filter((row) =>
    [
      OBSERVATION_REASON.LEVEL_DIFF,
      OBSERVATION_REASON.ACTION_DIFF,
      OBSERVATION_REASON.PRIORITY_DIFF,
      OBSERVATION_REASON.TARGET_DIFF,
      OBSERVATION_REASON.LEGACY_ONLY,
      OBSERVATION_REASON.RISK_ADVICE_ONLY,
    ].includes(row.reason),
  )

  let migrationRecommendation = 'NOT_READY'
  if (effectiveDenominator < 10 || buildFailedRate > 0.05 || summary.failedRate > 0.05) {
    migrationRecommendation = 'NOT_READY'
  } else if (effectiveMatchRate >= 0.85 && summary.mismatchRate <= 0.15 && buildFailedRate < 0.01) {
    migrationRecommendation = 'READY_FOR_HOLDING_MIGRATION'
  } else if (effectiveMatchRate >= 0.7 && summary.mismatchRate <= 0.25 && buildFailedRate < 0.02) {
    migrationRecommendation = 'READY_FOR_PROJECTION'
  } else {
    migrationRecommendation = 'OBSERVE_LONGER'
  }

  return {
    totalComparisons: summary.totalComparisons,
    matchRate: Number(summary.matchRate.toFixed(4)),
    effectiveMatchRate: Number(effectiveMatchRate.toFixed(4)),
    mismatchRate: Number(summary.mismatchRate.toFixed(4)),
    failedRate: Number(Math.max(summary.failedRate, buildFailedRate).toFixed(4)),
    majorMismatchReasons,
    buildAttempts,
    shadowGenerated: metrics.generated,
    shadowFailed: metrics.failed,
    migrationRecommendation,
  }
}

export function getScanRiskAdviceShadowObservation() {
  return {
    metrics: getScanRiskAdviceShadowMetrics(),
    evaluation: getScanRiskAdviceShadowEvaluation(),
    summary: getShadowEvaluationSummary(),
    readiness: getShadowMigrationReadiness(),
    projection: getRiskAdviceProjectionMetrics(),
    holdingDecisionAdapter: getHoldingDecisionAdapterMetrics(),
    holdingMigrationReadiness: getHoldingMigrationReadiness(),
    holdingMigrationStability: getHoldingMigrationStability(),
    holdingMigrationGate: getHoldingMigrationGateSnapshot({ record: false }),
    controlledSwitchStatus: getControlledSwitchStatus(),
  }
}

export function exportScanRiskAdviceShadowObservationJSON() {
  return JSON.stringify(getScanRiskAdviceShadowObservation(), null, 2)
}

export function resetScanRiskAdviceShadowObservation() {
  resetScanRiskAdviceShadowMetrics()
  resetScanRiskAdviceShadowEvaluation()
  resetRiskAdviceProjectionMetrics()
  resetHoldingDecisionAdapterMetrics()
  resetHoldingMigrationGateHistory()
}

export function setScanRiskAdviceShadowObservationEnabled(on) {
  setScanRiskAdviceShadowMetricsEnabled(on)
  setScanRiskAdviceShadowEvaluationEnabled(on)
  setRiskAdviceProjectionMetricsEnabled(on)
  setHoldingDecisionAdapterMetricsEnabled(on)
}

export function formatShadowEvaluationReport() {
  const metrics = getScanRiskAdviceShadowMetrics()
  const s = getShadowEvaluationSummary()
  const readiness = getShadowMigrationReadiness()
  const lines = [
    '=== Shadow Evaluation (observation only) ===',
    `total comparisons: ${s.totalComparisons}`,
    `matched: ${s.matched}`,
    `mismatched: ${s.mismatched}`,
    `failed: ${s.failed}`,
    `match rate: ${(s.matchRate * 100).toFixed(2)}%`,
    `mismatch rate: ${(s.mismatchRate * 100).toFixed(2)}%`,
    `failed rate: ${(s.failedRate * 100).toFixed(2)}%`,
    `migration recommendation: ${readiness.migrationRecommendation}`,
    `shadow generated: ${metrics.generated}`,
    `shadow failed: ${metrics.failed}`,
    'top observation reasons:',
  ]
  const top = getTopObservationReasons(10)
  if (!top.length) lines.push('  (none)')
  else for (const r of top) lines.push(`  - ${r.reason}: ${r.count}`)
  lines.push('major mismatch reasons:')
  if (!readiness.majorMismatchReasons.length) lines.push('  (none)')
  else for (const r of readiness.majorMismatchReasons) lines.push(`  - ${r.reason}: ${r.count}`)
  return lines.join('\n')
}
