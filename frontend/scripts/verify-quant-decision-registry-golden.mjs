/**
 * QuantDecision Phase3-B Registry Golden（校验已提交的 Go golden JSON 结构）
 * 运行：
 *   node scripts/verify-quant-decision-registry-golden.mjs
 * Go 权威 golden：
 *   go test ./backend/decision/registry/ -run TestRegistryGolden
 */
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const goldenPath = join(__dir, '../../backend/decision/registry/testdata/registry_trace_golden.json')

const golden = JSON.parse(readFileSync(goldenPath, 'utf8'))

assert.equal(golden.phase, 'Phase3-B')
assert.equal(golden.harness, 'quant-decision-registry')
assert.equal(golden.count, 2)
assert.ok(Array.isArray(golden.ids) && golden.ids.length === 2)
assert.ok(Array.isArray(golden.traces) && golden.traces.length === 4)

assert.equal(golden.summary.ok, 2)
assert.equal(golden.summary.stale, 1)
assert.equal(golden.summary.unbound, 1)

const byCode = Object.fromEntries(golden.traces.map((t) => [t.code, t]))
assert.equal(byCode.sz000001.rank, 1)
assert.equal(byCode.sz000001.score, 0.91)
assert.equal(byCode.sz000001.trace.found, true)
assert.equal(byCode.sz000001.trace.stale, false)
assert.equal(byCode.sz000001.trace.decision.action.code, 'ENTER')

assert.equal(byCode.sz000003.trace.stale, true)
assert.equal(byCode.sz000003.trace.staleReason, 'missing_from_registry')
assert.equal(byCode.sz000003.rank, 3)
assert.equal(byCode.sz000003.score, 0.6)

assert.equal(byCode.sz000004.trace.bound, false)
assert.equal(byCode.sz000004.rank, 4)

// Score/Rank must remain the binding-invariant values from fixture
for (const row of golden.traces) {
  assert.equal(typeof row.rank, 'number')
  assert.equal(typeof row.score, 'number')
  assert.equal(row.trace.itemRank, row.rank)
  assert.equal(row.trace.itemScore, row.score)
}

console.log('Phase3-B Decision Registry Golden')
console.log('  ids:', golden.ids.length, 'ok/stale/unbound:', golden.summary)
console.log('quant-decision-registry Phase3-B: passed')
