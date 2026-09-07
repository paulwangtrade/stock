/**
 * QuantDecision Phase3-D Authority Boundary Golden
 *   node scripts/verify-quant-decision-authority-golden.mjs
 */
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const goldenPath = join(__dir, '../../backend/decision/authority/testdata/authority_golden.json')
const golden = JSON.parse(readFileSync(goldenPath, 'utf8'))

assert.equal(golden.phase, 'Phase3-D')
assert.equal(golden.harness, 'quant-decision-authority-boundary')
assert.equal(golden.rules.createTradeAlwaysDenied, true)
assert.equal(golden.rules.decisionIsFactNotAuthorization, true)

assert.equal(golden.matrix.UI_CARD.readDecision, true)
assert.equal(golden.matrix.UI_CARD.writeDecision, false)
assert.equal(golden.matrix.ALERT.readDecision, true)
assert.equal(golden.matrix.EXECUTION_FORBIDDEN.readDecision, false)
assert.equal(golden.matrix.TRADEPLAN_DRAFT_FUTURE.readDecision, true)
assert.equal(golden.matrix.TRADEPLAN_DRAFT_FUTURE.createTrade, false)
assert.equal(golden.matrix.TRADEPLAN_CANDIDATE_SHADOW.readDecision, true)
assert.equal(golden.matrix.TRADEPLAN_CANDIDATE_SHADOW.createCandidate, true)
assert.equal(golden.matrix.TRADEPLAN_CANDIDATE_SHADOW.createTrade, false)

const byId = Object.fromEntries(golden.cases.map((c) => [c.id, c]))
assert.equal(byId.A_UI_CARD_read.expect, 'allowed')
assert.equal(byId.A_UI_CARD_read.allowed, true)
assert.equal(byId.B_ALERT_read.allowed, true)
assert.equal(byId.C_EXECUTION_access.expect, 'denied')
assert.equal(byId.C_EXECUTION_access.allowed, false)
assert.equal(byId.D_TradePlan_future_read_only.expect, 'read_only')
assert.equal(byId.D_TradePlan_future_read_only.allowed, true)
assert.equal(byId.E_Decision_mutation.expect, 'blocked')
assert.equal(byId.E_Decision_mutation.mutationDetected, true)

console.log('Phase3-D Decision Authority Boundary')
console.log('  cases:', golden.cases.map((c) => c.id).join(', '))
console.log('quant-decision-authority Phase3-D: passed')
