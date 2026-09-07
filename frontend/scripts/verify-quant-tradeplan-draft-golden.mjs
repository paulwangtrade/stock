/**
 * Phase4-A Decision → TradePlanDraft Golden
 *   node scripts/verify-quant-tradeplan-draft-golden.mjs
 */
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const goldenPath = join(__dir, '../../backend/tradeplan/draft/testdata/decision_to_draft_golden.json')
const golden = JSON.parse(readFileSync(goldenPath, 'utf8'))

assert.equal(golden.phase, 'Phase4-A')
assert.equal(golden.harness, 'quant-decision-to-tradeplan-draft')
assert.equal(golden.draft.enableExecute, false)
assert.equal(golden.draft.status, 'VALIDATED')
assert.equal(golden.draft.actionCode, 'ENTER')
assert.equal(golden.draft.targetShares, 1000)
assert.equal(golden.draft.consumerRole, 'TRADEPLAN_DRAFT_FUTURE')
assert.equal(golden.draft.sourceDecisionId, golden.source.decisionId)
assert.equal(golden.draft.snapshotHash, golden.source.snapshotHash)
assert.ok(golden.draft.draftHash)
assert.equal(golden.validateSourceOK, true)
assert.equal(golden.immutableOK, true)

console.log('Phase4-A TradePlan Draft Projection')
console.log('  sourceDecisionId:', golden.draft.sourceDecisionId)
console.log('  snapshotHash:', String(golden.draft.snapshotHash).slice(0, 12) + '…')
console.log('quant-tradeplan-draft Phase4-A: passed')
