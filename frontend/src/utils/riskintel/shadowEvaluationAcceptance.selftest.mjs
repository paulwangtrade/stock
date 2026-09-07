/**
 * Updated acceptance selftest for Phase8.5 taxonomy.
 */
import { attachScanRiskAdviceShadow, buildScanRiskAdviceShadow } from './scanRiskAdviceShadow.js'
import {
  OBSERVATION_REASON,
  compareLegacyRiskAdvice,
  formatShadowEvaluationReport,
  getScanRiskAdviceShadowObservation,
  getShadowEvaluationSummary,
  getShadowMigrationReadiness,
  observeLegacyRiskAdvicePair,
  resetScanRiskAdviceShadowObservation,
} from './scanRiskAdviceShadowEvaluation.js'
import { scanForbiddenAdviceKeys } from './riskAdvice.js'

function assert(cond, msg) { if (!cond) throw new Error(msg) }

function legacyRow(holdingAdvice, extra = {}) {
  return { code: 'sh600519', ok: true, tag: null, holdingAdvice, sellPositionPct: null, statusText: 'legacy', ...extra }
}

function run() {
  resetScanRiskAdviceShadowObservation()

  const bundleL1 = { stockCode: 'sh600519', marketModeKey: 'level1', effectiveMarketMode: { key: 'level1', level: 1, name: 'level1' }, asOf: '2026-07-29T00:00:00.000Z' }
  const bundleL3 = { stockCode: 'sz000001', marketModeKey: 'level3', effectiveMarketMode: { key: 'level3', level: 3, name: 'level3' }, asOf: '2026-07-29T00:00:00.000Z' }

  const legacyL1 = { action: 'reduce', marketLevel: 1, suggestPct: 1, suggestPctDisplay: 100 }
  const legacyL3 = { action: 'hold', marketLevel: 3 }

  const shadowOk = buildScanRiskAdviceShadow(bundleL1)
  assert(shadowOk.riskAdvice, 'riskAdvice generated')
  assert(shadowOk.failReason == null, 'no fail')

  const rowBefore = legacyRow(legacyL3)
  const snap = JSON.stringify({ holdingAdvice: rowBefore.holdingAdvice, tag: rowBefore.tag })
  const rowAfter = attachScanRiskAdviceShadow(rowBefore, bundleL3)
  assert(JSON.stringify({ holdingAdvice: rowAfter.holdingAdvice, tag: rowAfter.tag }) === snap, 'legacy unchanged')

  observeLegacyRiskAdvicePair(legacyL3, buildScanRiskAdviceShadow(bundleL3).riskAdvice)
  observeLegacyRiskAdvicePair(legacyL1, buildScanRiskAdviceShadow(bundleL3).riskAdvice)

  const bothAbsent = compareLegacyRiskAdvice(null, null)
  assert(bothAbsent.observationReason === OBSERVATION_REASON.BOTH_ABSENT, 'both absent')
  observeLegacyRiskAdvicePair(null, null)

  const invalidBoth = compareLegacyRiskAdvice({ marketLevel: 'bad' }, { level: 'x' })
  assert(invalidBoth.observationReason === OBSERVATION_REASON.LEVEL_DIFF, 'invalid both')

  const ctx = buildScanRiskAdviceShadow(null)
  observeLegacyRiskAdvicePair(legacyL1, null, { shadowFailure: ctx.failReason })

  const summary = getShadowEvaluationSummary()
  assert(summary.totalComparisons === summary.matched + summary.mismatched + summary.failed, 'total sum')
  assert(summary.failed >= 1, 'failed tracked')

  const obs = getScanRiskAdviceShadowObservation()
  assert(obs.readiness.migrationRecommendation, 'readiness in observation')
  assert(obs.summary.observationReasonFrequency, 'taxonomy frequencies')

  const report = formatShadowEvaluationReport()
  assert(report.includes('migration recommendation:'), 'report readiness')

  console.log('shadowEvaluationAcceptance.selftest: PASS')
}

run()
