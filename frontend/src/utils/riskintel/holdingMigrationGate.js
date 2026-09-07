/**
 * Phase9-B.6: Holding Migration Gate (observation only).
 * Memory snapshots + trend; never switches source or affects trading.
 */

import {
  getHoldingMigrationReadiness,
  getHoldingMigrationStability,
  getHoldingDecisionPreferredSource,
} from './holdingDecisionAdapter.js'

export const HOLDING_MIGRATION_GATE_HISTORY_SIZE = 20

/** @type {object[]} */
let gateHistory = []

export function resetHoldingMigrationGateHistory() {
  gateHistory = []
}

/**
 * Capture a point-in-time gate snapshot (memory only; no DB).
 * Optionally push into rolling history when `options.record !== false`.
 */
export function getHoldingMigrationGateSnapshot(options = {}) {
  const record = options.record !== false
  const readiness = getHoldingMigrationReadiness()
  const stability = getHoldingMigrationStability()

  const snapshot = {
    timestamp: new Date().toISOString(),
    sampleCount: readiness.total,
    readiness: {
      recommendation: readiness.recommendation,
      exactMatchRate: readiness.exactMatchRate,
      alignedMatchRate: readiness.alignedMatchRate,
      diffRate: readiness.diffRate,
      projectionUnavailableRate: readiness.projectionUnavailableRate,
      criteria: readiness.criteria,
    },
    stability: {
      rollingMatchRate: stability.rollingMatchRate,
      rollingDiffRate: stability.rollingDiffRate,
      rollingUnavailableRate: stability.rollingUnavailableRate ?? 0,
      recentTrend: stability.recentTrend,
      stable: stability.stable,
      windowFill: stability.windowFill,
    },
    topDiffReasons: Array.isArray(readiness.majorDiffReasons)
      ? readiness.majorDiffReasons.slice(0, 5)
      : [],
    topDiffSymbols: Array.isArray(readiness.highestFrequencyDiffSymbols)
      ? readiness.highestFrequencyDiffSymbols.slice(0, 5)
      : [],
    projectionUnavailableRate: readiness.projectionUnavailableRate,
    recommendation: readiness.recommendation,
    sourceLocked: getHoldingDecisionPreferredSource(),
    authoritySource: getHoldingDecisionPreferredSource(),
    controlledSwitchEnabled: readiness.controlledSwitchEnabled === true,
  }

  if (record) {
    gateHistory.push(snapshot)
    if (gateHistory.length > HOLDING_MIGRATION_GATE_HISTORY_SIZE) {
      gateHistory.shift()
    }
  }

  return snapshot
}

function scoreSnapshot(snap) {
  if (!snap) return 0
  const aligned = Number(snap.readiness?.alignedMatchRate) || 0
  const diff = Number(snap.readiness?.diffRate) || 0
  const unavail = Number(snap.projectionUnavailableRate) || 0
  const rolling = Number(snap.stability?.rollingMatchRate) || 0
  // Higher is better
  return aligned * 0.45 + rolling * 0.35 - diff * 0.15 - unavail * 0.2
}

/**
 * Compare previous vs current gate snapshots (observation only).
 */
export function getHoldingMigrationTrend() {
  const current = gateHistory.length
    ? gateHistory[gateHistory.length - 1]
    : getHoldingMigrationGateSnapshot({ record: false })
  const previous = gateHistory.length >= 2
    ? gateHistory[gateHistory.length - 2]
    : null

  if (!previous) {
    return {
      previous: null,
      current,
      improving: false,
      degrading: false,
      historySize: gateHistory.length,
    }
  }

  const prevScore = scoreSnapshot(previous)
  const curScore = scoreSnapshot(current)
  const delta = curScore - prevScore
  const improving = delta >= 0.03
  const degrading = delta <= -0.03

  return {
    previous,
    current,
    improving,
    degrading,
    scoreDelta: Number(delta.toFixed(4)),
    historySize: gateHistory.length,
  }
}

export function getHoldingMigrationGateHistory() {
  return gateHistory.slice()
}

/**
 * Human-readable gate report (opt-in; does not switch source).
 */
export function formatHoldingMigrationGateReport() {
  const snap = getHoldingMigrationGateSnapshot({ record: false })
  const trend = getHoldingMigrationTrend()
  let trendLabel = 'insufficient_history'
  if (trend.previous) {
    if (trend.improving) trendLabel = 'improving'
    else if (trend.degrading) trendLabel = 'degrading'
    else trendLabel = 'flat'
  }

  const lines = [
    '=== Holding Migration Gate (observation only) ===',
    `Sample: ${snap.sampleCount}`,
    `Match Rate: ${(snap.readiness.alignedMatchRate * 100).toFixed(2)}% (aligned) / ${(snap.readiness.exactMatchRate * 100).toFixed(2)}% (exact)`,
    `Diff Rate: ${(snap.readiness.diffRate * 100).toFixed(2)}%`,
    `Unavailable: ${(snap.projectionUnavailableRate * 100).toFixed(2)}%`,
    `Trend: ${trendLabel}`,
    `Stability: rollingMatch=${(snap.stability.rollingMatchRate * 100).toFixed(2)}% stable=${snap.stability.stable}`,
    `Recommendation: ${snap.recommendation}`,
    `Authority Source: ${snap.authoritySource || snap.sourceLocked}`,
  ]
  if (snap.topDiffReasons.length) {
    lines.push('Top Diff Reasons:')
    for (const r of snap.topDiffReasons) lines.push(`  - ${r.reason}: ${r.count}`)
  }
  if (snap.topDiffSymbols.length) {
    lines.push('Top Diff Symbols:')
    for (const s of snap.topDiffSymbols) lines.push(`  - ${s.key}: ${s.count}`)
  }
  return lines.join('\n')
}
