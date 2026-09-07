/**
 * QuantDecision Phase3-C Snapshot Integrity Golden
 * 运行：
 *   node scripts/verify-quant-decision-integrity-golden.mjs
 * Go：
 *   go test ./backend/decision/registry/ -run 'Integrity|Immutable'
 */
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const goldenPath = join(__dir, '../../backend/decision/registry/testdata/snapshot_integrity_golden.json')

const golden = JSON.parse(readFileSync(goldenPath, 'utf8'))

assert.equal(golden.phase, 'Phase3-C')
assert.equal(golden.harness, 'quant-decision-snapshot-integrity')
assert.equal(golden.sealed.intact, true)
assert.ok(golden.sealed.hash)
assert.equal(golden.overwriteAttempt.conflictKind, 'id_content_conflict')
assert.equal(golden.overwriteAttempt.keptHash, golden.sealed.hash)
assert.equal(golden.actionConflict.conflictKind, 'action_code_conflict')
assert.equal(golden.actionConflict.left, 'ENTER')
assert.equal(golden.actionConflict.right, 'WATCH')
assert.equal(golden.count, 2) // original + action-conflict peer; overwrite refused
assert.ok(golden.conflictCount >= 2)
assert.ok(golden.conflictKinds.includes('id_content_conflict'))
assert.ok(golden.conflictKinds.includes('action_code_conflict'))

console.log('Phase3-C Decision Snapshot Integrity')
console.log('  sealed hash:', golden.sealed.hash.slice(0, 12) + '…')
console.log('  conflicts:', golden.conflictKinds)
console.log('quant-decision-snapshot-integrity Phase3-C: passed')
