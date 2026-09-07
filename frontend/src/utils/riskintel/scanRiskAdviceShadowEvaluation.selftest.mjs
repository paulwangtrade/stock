import {
  compareLegacyRiskAdvice,
  getShadowEvaluationSummary,
  getShadowMigrationReadiness,
  observeLegacyRiskAdvicePair,
  resetScanRiskAdviceShadowObservation,
  OBSERVATION_REASON,
} from './scanRiskAdviceShadowEvaluation.js'
import { buildScanRiskAdviceShadow } from './scanRiskAdviceShadow.js'

function assert(cond, msg) { if (!cond) throw new Error(msg) }

function run() {
  resetScanRiskAdviceShadowObservation()
  const legacy = { action: 'hold', marketLevel: 3 }
  const risk = buildScanRiskAdviceShadow({ stockCode: 'x', marketModeKey: 'level3', effectiveMarketMode: { key: 'level3', level: 3 }, asOf: '2026-07-29T00:00:00.000Z' }).riskAdvice
  observeLegacyRiskAdvicePair(legacy, risk)
  const c = compareLegacyRiskAdvice(legacy, risk)
  assert(c.observationReason === OBSERVATION_REASON.EXACT_MATCH, 'exact')
  const summary = getShadowEvaluationSummary()
  assert(summary.totalComparisons >= 1, 'total')
  assert(getShadowMigrationReadiness().migrationRecommendation, 'readiness')
  console.log('scanRiskAdviceShadowEvaluation.selftest: PASS')
}
run()
