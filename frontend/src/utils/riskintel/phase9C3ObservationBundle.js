/**
 * Phase9-C.3 Observation Enablement — read-only aggregation bundle.
 * Does not change authority, decision logic, execution, DB, or config.
 */

import {
  getControlledSwitchStatus,
  getControlledSwitchMetrics,
  HOLDING_DECISION_SOURCE_PROJECTION,
  HOLDING_MIGRATION_MIN_SAMPLES,
  CONTROLLED_SWITCH_FALLBACK_DEGRADED,
  CONTROLLED_SWITCH_DIFF_DEGRADED,
  CONTROLLED_SWITCH_UNAVAILABLE_DEGRADED,
} from './holdingDecisionAdapter.js'
import { getHoldingMigrationGateSnapshot, getHoldingMigrationTrend } from './holdingMigrationGate.js'
import { getScanRiskAdviceShadowObservation } from './scanRiskAdviceShadowEvaluation.js'

export const PHASE9_C3_OBSERVATION_CHECKPOINT_EVENT = 'phase9_c3_observation_checkpoint'

/** @type {object|null} */
let lastCheckpoint = null

/**
 * Pure eligibility label for Runbook §4 (does not mutate adapter status).
 *
 * @param {{ status?: object, metrics?: object, meta?: object }} input
 * @returns {{
 *   label: 'HEALTHY'|'DEGRADED'|'INSUFFICIENT SAMPLE'|'INVALID',
 *   countsTowardHealthyWindow: boolean,
 *   reasons: string[],
 *   checks: object,
 * }}
 */
export function evaluateHealthyWindowEligibility(input = {}) {
  const status = input.status || {}
  const metrics = input.metrics || {}
  const meta = input.meta || {}
  const reasons = []

  const totalDecisions = Number(status.totalDecisions ?? metrics.totalDecisions ?? 0) || 0
  const source = String(status.source || metrics.preferredSource || '')
  const apiStatus = String(status.status || '')
  const fallbackRate = Number(status.fallbackRate ?? metrics.fallbackRate ?? 0) || 0
  const diffRate = Number(status.diffRate ?? metrics.decisionDiffRate ?? 0) || 0
  const unavailableRate = Number(status.unavailableRate ?? 0) || 0

  const checks = {
    sameProcessHint: meta.trigger === 'scan_success' || meta.sameProcessScan === true || totalDecisions > 0,
    sourceIsProjection: source === HOLDING_DECISION_SOURCE_PROJECTION,
    apiStatusHealthy: apiStatus === 'HEALTHY',
    totalDecisionsMet: totalDecisions >= HOLDING_MIGRATION_MIN_SAMPLES,
    fallbackOk: fallbackRate <= CONTROLLED_SWITCH_FALLBACK_DEGRADED,
    diffOk: diffRate <= CONTROLLED_SWITCH_DIFF_DEGRADED,
    unavailableOk: unavailableRate <= CONTROLLED_SWITCH_UNAVAILABLE_DEGRADED,
    notVacuumHealthy: !(totalDecisions === 0 && apiStatus === 'HEALTHY'),
  }

  if (totalDecisions <= 0) {
    reasons.push('totalDecisions=0 (insufficient / vacuum)')
    return {
      label: 'INSUFFICIENT SAMPLE',
      countsTowardHealthyWindow: false,
      reasons,
      checks,
    }
  }

  if (!checks.totalDecisionsMet) {
    reasons.push(`totalDecisions=${totalDecisions} < ${HOLDING_MIGRATION_MIN_SAMPLES}`)
    return {
      label: 'INSUFFICIENT SAMPLE',
      countsTowardHealthyWindow: false,
      reasons,
      checks,
    }
  }

  if (!checks.sourceIsProjection) {
    reasons.push(`source=${source || '(empty)'} (expected ${HOLDING_DECISION_SOURCE_PROJECTION})`)
  }
  if (!checks.apiStatusHealthy) {
    reasons.push(`status.status=${apiStatus || '(empty)'}`)
  }
  if (!checks.fallbackOk) {
    reasons.push(`fallbackRate=${fallbackRate} > ${CONTROLLED_SWITCH_FALLBACK_DEGRADED}`)
  }
  if (!checks.diffOk) {
    reasons.push(`diffRate=${diffRate} > ${CONTROLLED_SWITCH_DIFF_DEGRADED}`)
  }
  if (!checks.unavailableOk) {
    reasons.push(`unavailableRate=${unavailableRate} > ${CONTROLLED_SWITCH_UNAVAILABLE_DEGRADED}`)
  }

  if (
    checks.sourceIsProjection
    && checks.apiStatusHealthy
    && checks.fallbackOk
    && checks.diffOk
    && checks.unavailableOk
  ) {
    return {
      label: 'HEALTHY',
      countsTowardHealthyWindow: true,
      reasons: ['all Runbook §4 quantitative checks passed'],
      checks,
    }
  }

  if (apiStatus === 'ROLLBACK_RECOMMENDED' || source !== HOLDING_DECISION_SOURCE_PROJECTION) {
    return {
      label: apiStatus === 'ROLLBACK_RECOMMENDED' ? 'DEGRADED' : 'DEGRADED',
      countsTowardHealthyWindow: false,
      reasons: reasons.length ? reasons : ['not eligible for HEALTHY window'],
      checks,
    }
  }

  return {
    label: 'DEGRADED',
    countsTowardHealthyWindow: false,
    reasons: reasons.length ? reasons : ['not eligible for HEALTHY window'],
    checks,
  }
}

/**
 * Assemble observation-only bundle for UI / checkpoint / ENTRY copy.
 *
 * @param {{ recordGate?: boolean, meta?: object }} [options]
 */
export function getPhase9C3ObservationBundle(options = {}) {
  const recordGate = options.recordGate === true
  const meta = options.meta && typeof options.meta === 'object' ? { ...options.meta } : {}

  const status = getControlledSwitchStatus()
  const metrics = getControlledSwitchMetrics()
  const gate = getHoldingMigrationGateSnapshot({ record: recordGate })
  const trend = getHoldingMigrationTrend()
  const shadowObservation = getScanRiskAdviceShadowObservation()
  const healthyWindowHint = evaluateHealthyWindowEligibility({ status, metrics, meta })

  return {
    status,
    metrics,
    gate,
    trend,
    shadowObservation,
    healthyWindowHint,
    capturedAt: new Date().toISOString(),
    meta: {
      observationOnly: true,
      recordGate,
      doesNotChangeGateState: true,
      doesNotTriggerMigration: true,
      doesNotAffectExecution: true,
      ...meta,
    },
  }
}

export function exportPhase9C3ObservationBundleJSON(options = {}) {
  return JSON.stringify(getPhase9C3ObservationBundle(options), null, 2)
}

export function getLastPhase9C3ObservationCheckpoint() {
  return lastCheckpoint
}

export function rememberPhase9C3ObservationCheckpoint(bundle) {
  lastCheckpoint = bundle && typeof bundle === 'object' ? bundle : null
  return lastCheckpoint
}

/**
 * Build + remember checkpoint payload (observation metrics only).
 * Caller may EventsEmit; this function never throws into decision/execution paths
 * when wrapped by the service try/catch.
 *
 * @param {{ recordGate?: boolean, meta?: object }} [options]
 */
export function buildPhase9C3ObservationCheckpoint(options = {}) {
  const bundle = getPhase9C3ObservationBundle({
    recordGate: options.recordGate !== false,
    meta: {
      trigger: 'scan_success',
      sameProcessScan: true,
      ...(options.meta || {}),
    },
  })
  rememberPhase9C3ObservationCheckpoint(bundle)
  return bundle
}
