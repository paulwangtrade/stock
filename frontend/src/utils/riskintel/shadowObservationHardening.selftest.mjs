/**
 * Phase8.5 shadow observation hardening selftest.
 * Run: node frontend/src/utils/riskintel/shadowObservationHardening.selftest.mjs
 */

import { buildScanRiskAdviceShadow } from './scanRiskAdviceShadow.js'
import {
  OBSERVATION_CATEGORY,
  OBSERVATION_REASON,
  classifyLegacyRiskAdviceObservation,
} from './scanRiskAdviceShadowObservation.js'
import {
  compareLegacyRiskAdvice,
  getShadowMigrationReadiness,
  getScanRiskAdviceShadowEvaluation,
  observeLegacyRiskAdvicePair,
  resetScanRiskAdviceShadowObservation,
} from './scanRiskAdviceShadowEvaluation.js'

function assert(cond, msg) {
  if (!cond) throw new Error(msg)
}

function run() {
  resetScanRiskAdviceShadowObservation()

  const legacyExact = { action: 'hold', marketLevel: 3 }
  const riskExact = { level: 3, category: 'hold_observe', marketDriven: true, reasons: [{ code: 'X', message: 'x', marketDriven: true }] }
  const exact = classifyLegacyRiskAdviceObservation(legacyExact, riskExact)
  assert(exact.observationCategory === OBSERVATION_CATEGORY.MATCH, 'exact category')
  assert(exact.observationReason === OBSERVATION_REASON.EXACT_MATCH, 'exact reason')

  const bothAbsent = classifyLegacyRiskAdviceObservation(null, null)
  assert(bothAbsent.match && bothAbsent.observationReason === OBSERVATION_REASON.BOTH_ABSENT, 'both absent')

  const invalidBoth = classifyLegacyRiskAdviceObservation({ marketLevel: 'bad' }, { level: 'x' })
  assert(invalidBoth.observationCategory === OBSERVATION_CATEGORY.MISMATCH, 'invalid both mismatch')
  assert(invalidBoth.observationReason === OBSERVATION_REASON.LEVEL_DIFF, 'invalid both level diff')

  const levelMismatch = classifyLegacyRiskAdviceObservation({ marketLevel: 1, action: 'reduce' }, { level: 3, category: 'hold_observe', marketDriven: true })
  assert(levelMismatch.observationReason === OBSERVATION_REASON.LEVEL_DIFF, 'level diff')

  const actionMismatch = classifyLegacyRiskAdviceObservation(
    { marketLevel: 3, action: 'reduce' },
    { level: 3, category: 'hold_observe', marketDriven: false },
  )
  assert(actionMismatch.observationReason === OBSERVATION_REASON.ACTION_DIFF, 'action diff')

  const ctxFail = buildScanRiskAdviceShadow(null)
  assert(ctxFail.failReason === 'context_missing', 'context missing fail')
  observeLegacyRiskAdvicePair({ marketLevel: 2, action: 'hold' }, null, { shadowFailure: ctxFail.failReason })
  const evalAfterFail = getScanRiskAdviceShadowEvaluation()
  assert(evalAfterFail.failedCount === 1, 'failed count')
  assert((evalAfterFail.observationReasonFrequency.CONTEXT_MISSING || 0) === 1, 'context missing reason')

  const exc = observeLegacyRiskAdvicePair({ marketLevel: 2 }, null, { shadowFailure: 'exception' })
  assert(exc.observationCategory === OBSERVATION_CATEGORY.FAILED, 'exception failed category')
  assert(exc.observationReason === OBSERVATION_REASON.RISK_ADVICE_EXCEPTION, 'exception reason')

  // accumulate enough exact matches for readiness path
  for (let i = 0; i < 12; i++) {
    observeLegacyRiskAdvicePair(legacyExact, riskExact)
  }
  const readiness = getShadowMigrationReadiness()
  assert(readiness.totalComparisons >= 14, 'total comparisons')
  assert(typeof readiness.matchRate === 'number', 'matchRate')
  assert(typeof readiness.mismatchRate === 'number', 'mismatchRate')
  assert(typeof readiness.failedRate === 'number', 'failedRate')
  assert(Array.isArray(readiness.majorMismatchReasons), 'major mismatch reasons')
  assert([
    'NOT_READY',
    'OBSERVE_LONGER',
    'READY_FOR_PROJECTION',
    'READY_FOR_HOLDING_MIGRATION',
  ].includes(readiness.migrationRecommendation), 'recommendation enum')

  // legacy metrics compatibility fields still present
  const evaluation = getScanRiskAdviceShadowEvaluation()
  assert(typeof evaluation.matchCount === 'number', 'legacy matchCount')
  assert(typeof evaluation.mismatchCount === 'number', 'legacy mismatchCount')
  assert(typeof evaluation.disagreementReasonFrequency === 'object', 'legacy disagreement map')

  console.log('shadowObservationHardening.selftest: PASS')
  console.log('readiness:', readiness)
}

run()
