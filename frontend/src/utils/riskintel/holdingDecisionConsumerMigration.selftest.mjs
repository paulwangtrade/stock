/**
 * Phase9-C.1 consumer choke — under C.2, no-projection falls back to legacy (bit-identical).
 * Run: node --import ./frontend/scripts/loaders/register-js-ext.mjs frontend/src/utils/riskintel/holdingDecisionConsumerMigration.selftest.mjs
 */
import assert from 'node:assert/strict'
import {
  getHoldingDecision,
  resolveAuthorityHoldingDecision,
  getHoldingDecisionPreferredSource,
  HOLDING_DECISION_SOURCE_PROJECTION,
} from './holdingDecisionAdapter.js'
import {
  resetHoldingDecisionAdapterMetrics,
  getHoldingDecisionAdapterMetrics,
} from './holdingDecisionAdapterMetrics.js'
import { assembleWatchlistDecision } from '../quantDecisionAssemble.js'
import { deriveQuantAction } from '../quantActionDerive.js'
import { projectWatchlistCard } from '../quantWatchlistProjection.js'

const legacyAdvice = {
  action: 'add',
  actionLabel: '倾向加仓',
  summaryLine: '量价偏强',
  suggestPct: 0.2,
  suggestPctDisplay: 20,
  score: 0.6,
  factors: [{ impact: 1, label: '量能', detail: '放量' }],
  marketLevel: 4,
}

resetHoldingDecisionAdapterMetrics()
assert.equal(
  getHoldingDecisionPreferredSource(),
  HOLDING_DECISION_SOURCE_PROJECTION,
  'C.2 preferred projection',
)

// Without projection → fallback legacy (consumer-visible decision === holdingAdvice)
const envelope = getHoldingDecision({
  holdingAdvice: legacyAdvice,
  symbol: 'sh600519',
  record: true,
})
assert.equal(envelope.source, 'legacy')
assert.equal(envelope.fallbackUsed, true)
assert.equal(envelope.decision, legacyAdvice)
assert.equal(envelope.legacyDecision, legacyAdvice)

const viaEnvelope = resolveAuthorityHoldingDecision({
  holdingDecision: envelope,
  holdingAdvice: { action: 'reduce', actionLabel: 'should-not-win' },
})
assert.equal(viaEnvelope, legacyAdvice, 'envelope wins')

const before = getHoldingDecisionAdapterMetrics().total
const viaAdviceOnly = resolveAuthorityHoldingDecision({ holdingAdvice: legacyAdvice })
assert.equal(viaAdviceOnly, legacyAdvice, 'advice-only fallback legacy')
assert.equal(getHoldingDecisionAdapterMetrics().total, before, 'consumer resolve does not record')

const entryAdvice = {
  code: 'sh600519',
  name: '茅台',
  tag: null,
  holdingAdvice: legacyAdvice,
  statusText: '持仓',
}
const entryEnvelope = {
  ...entryAdvice,
  holdingDecision: envelope,
}
const d1 = assembleWatchlistDecision({ entry: entryAdvice, asOf: '2026-07-30T00:00:00.000Z' })
const d2 = assembleWatchlistDecision({ entry: entryEnvelope, asOf: '2026-07-30T00:00:00.000Z' })
assert.deepEqual(d1.holdingBias, d2.holdingBias, 'assemble holdingBias identical')
assert.deepEqual(d1.action, d2.action, 'assemble action identical')
assert.equal(d1.holdingBias.action, 'add')
assert.equal(d1.action.label, '倾向加仓')

const derivedA = deriveQuantAction({ holdingAdvice: legacyAdvice })
const derivedB = deriveQuantAction({ holdingDecision: envelope })
assert.deepEqual(derivedA, derivedB, 'derive identical via advice vs envelope')

const card = projectWatchlistCard({
  result: { 股票名称: '茅台', 股票代码: 'sh600519', costPrice: 10, costVolume: 100 },
  signal: { holdingAdvice: legacyAdvice, holdingDecision: envelope, tag: null },
})
assert.ok(card.holdingAdviceLabel.includes('倾向加仓'), 'card label')
assert.equal(card.holdingAdviceType, 'success')

const t0 = getHoldingDecisionAdapterMetrics().total
getHoldingDecision({ holdingAdvice: legacyAdvice, record: false })
assert.equal(getHoldingDecisionAdapterMetrics().total, t0, 'record:false skips metrics')

console.log('holdingDecisionConsumerMigration.selftest: PASS')
