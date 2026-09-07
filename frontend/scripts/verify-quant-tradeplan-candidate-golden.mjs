/**
 * Phase5-B TradePlanCandidate Shadow Golden
 *   node scripts/verify-quant-tradeplan-candidate-golden.mjs
 */
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const goldenPath = join(__dir, '../../backend/tradeplan/candidate/testdata/tradeplan_candidate_golden.json')
const golden = JSON.parse(readFileSync(goldenPath, 'utf8'))

assert.equal(golden.phase, 'Phase5-B')
assert.equal(golden.harness, 'quant-tradeplan-candidate-shadow')

assert.equal(golden.case1_ok.ok, true)
assert.equal(golden.case1_ok.executable, false)
assert.equal(golden.case1_ok.intentKind, 'ENTER_HINT')
assert.equal(golden.case1_ok.targetShares, 1000)
assert.ok(golden.case1_ok.sourceDecisionId)
assert.ok(golden.case1_ok.sourceSnapshotHash)

assert.equal(golden.case2_draft_not_validated.ok, false)
assert.ok(String(golden.case2_draft_not_validated.err).includes('draft_not_validated'))

assert.equal(golden.case3_execute_forbidden.ok, false)
assert.equal(golden.case3_execute_forbidden.executable, false)
assert.ok(String(golden.case3_execute_forbidden.err).includes('candidate_execute_forbidden'))

assert.equal(golden.case4_mutation.ok, false)
assert.ok(golden.case4_mutation.codes.includes('candidate_mutation_detected'))

assert.equal(golden.authority.ok, true)
assert.equal(golden.authority.canCreateCandidate, true)
assert.equal(golden.authority.canAuthorizeExecute, false)

console.log('Phase5-B TradePlanCandidate Shadow')
console.log('  IntentKind:', golden.case1_ok.intentKind, 'Executable=false')
console.log('quant-tradeplan-candidate Phase5-B: passed')
