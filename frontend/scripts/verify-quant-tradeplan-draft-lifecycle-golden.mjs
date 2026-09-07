/**
 * Phase4-B TradePlan Draft Lifecycle Golden
 *   node scripts/verify-quant-tradeplan-draft-lifecycle-golden.mjs
 */
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const goldenPath = join(__dir, '../../backend/tradeplan/draft/testdata/draft_lifecycle_golden.json')
const golden = JSON.parse(readFileSync(goldenPath, 'utf8'))

assert.equal(golden.phase, 'Phase4-B')
assert.equal(golden.harness, 'quant-tradeplan-draft-lifecycle')

assert.equal(golden.case1_lifecycle.from, 'CREATED')
assert.equal(golden.case1_lifecycle.to, 'VALIDATED')
assert.equal(golden.case1_lifecycle.ok, true)

assert.equal(golden.case2_snapshot_changed.ok, false)
assert.ok(golden.case2_snapshot_changed.codes.includes('snapshot_changed'))

assert.equal(golden.case3_draft_mutation.ok, false)
assert.ok(golden.case3_draft_mutation.codes.includes('draft_mutation_detected'))

assert.equal(golden.case4_execute_forbidden.ok, false)
assert.ok(golden.case4_execute_forbidden.codes.includes('execute_forbidden'))
assert.equal(golden.case4_execute_forbidden.enableExecute, false)

assert.equal(golden.authority.ok, true)
assert.equal(golden.authority.canReadDecision, true)
assert.equal(golden.authority.canModifyDecision, false)
assert.equal(golden.authority.canAuthorizeExecute, false)
assert.equal(golden.decisionAndRankUntouched, true)

console.log('Phase4-B TradePlan Draft Lifecycle')
console.log('  CREATED → VALIDATED; guards: snapshot/mutation/execute')
console.log('quant-tradeplan-draft-lifecycle Phase4-B: passed')
