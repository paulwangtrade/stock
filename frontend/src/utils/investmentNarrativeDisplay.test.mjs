/**
 * Unit tests: Investment Narrative display (Phase14-G0.5b).
 * Run: node frontend/src/utils/investmentNarrativeDisplay.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'investmentNarrativeDisplay.js')).href
const {
  NARRATIVE_EMPTY,
  buildNarrativeDisplayModel,
  buildDiscoverySection,
  buildExitReviewSection,
  buildHoldingSection,
  formatOpportunityActionLabel,
} = await import(modUrl)

const fullNarrative = {
  stock_code: 'sh600363',
  discovery: {
    exists: true,
    status: 'full',
    signal_time: '2026-08-18',
    signal_price: 28.5,
    reason: '强',
  },
  plan_origin: {
    exists: true,
    status: 'full',
    plan_id: 12,
    strategy: 'momentum_v1',
    reason: 'breakout',
  },
  holding_basis: {
    exists: true,
    status: 'full',
    quantity: 1000,
    avg_cost: 30,
  },
  exit_review: {
    exists: true,
    status: 'WATCH',
    latest_outcome: { decision: 'WATCH', reason: '观察' },
  },
  price_story: {
    status: 'full',
    signal_price: 28,
    current_price: 33,
    vs_signal_pct: 17.86,
  },
}

{
  const model = buildNarrativeDisplayModel(fullNarrative, { action: 'WATCH' })
  assert.equal(model.discovery.signalTime, '2026-08-18')
  assert.equal(model.discovery.signalTag, '强')
  assert.equal(model.opportunity.watchState, '已跟踪')
  assert.equal(model.planOrigin.planId, '#12')
  assert.equal(model.holding.quantity, '1,000 股')
  assert.equal(model.exitReview.status, 'WATCH')
  assert.equal(model.exitReview.outcome, '加入观察')
  assert.notEqual(model.priceStory.signalPrice, NARRATIVE_EMPTY)
  assert.notEqual(model.priceStory.vsSignalPct, NARRATIVE_EMPTY)
}

{
  const disc = buildDiscoverySection({ discovery: { exists: false, status: 'missing' } })
  assert.equal(disc.signalTime, NARRATIVE_EMPTY)
  assert.equal(disc.hasData, false)
}

{
  const hold = buildHoldingSection({ holding_basis: { exists: false, status: 'missing' } })
  assert.equal(hold.quantity, NARRATIVE_EMPTY)
}

{
  const exit = buildExitReviewSection({
    exit_review: { exists: false, status: 'not_applicable' },
  })
  assert.equal(exit.status, NARRATIVE_EMPTY)
  assert.equal(exit.notApplicable, true)
}

{
  const exit = buildExitReviewSection({
    exit_review: { exists: true, status: 'REVIEW_REQUIRED' },
  })
  assert.equal(exit.outcome, NARRATIVE_EMPTY)
  assert.equal(exit.status, 'REVIEW_REQUIRED')
}

assert.equal(formatOpportunityActionLabel('WATCH'), '已跟踪')
assert.equal(formatOpportunityActionLabel(''), NARRATIVE_EMPTY)

console.log('investmentNarrativeDisplay.test.mjs: all passed')
