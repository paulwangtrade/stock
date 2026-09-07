/**
 * QuantDecision Phase2-D Historical Shadow Replay Golden
 * 运行：
 *   node --import ./scripts/loaders/register-js-ext.mjs scripts/verify-quant-historical-shadow-replay-golden.mjs
 */
import assert from 'node:assert/strict'
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import {
  buildActionTransitionMatrix,
  runHistoricalShadowReplay,
} from '../src/utils/quantDecisionHistoricalReplay.js'

const __dir = dirname(fileURLToPath(import.meta.url))
const fixturePath = join(__dir, '../../backend/decision/semantic/testdata/historical_replay_fixture.json')
const outDir = join(__dir, '../../backend/decision/semantic/testdata')
const outPath = join(outDir, 'historical_shadow_replay_report.json')

const series = JSON.parse(readFileSync(fixturePath, 'utf8'))
const report = runHistoricalShadowReplay(series)

assert.equal(report.phase, 'Phase2-D')
assert.equal(report.harness, 'quant-decision-historical-shadow-replay')
assert.equal(report.totalDays, 4)
assert.equal(report.daily.length, 4)

// daily shadow report wraps Phase2-C stability
for (const day of report.daily) {
  assert.ok(day.tradeDate)
  assert.equal(day.stability.phase, 'Phase2-C')
  assert.equal(day.stability.totalPairs, 1)
}

// day2 has intentional candidate actionPatch → Action.Code conflict
const day2 = report.daily.find((d) => d.tradeDate === '2026-07-15')
assert.ok(day2)
assert.equal(day2.stability.actionCode.conflictCount, 1)
assert.notEqual(day2.baselineActionCode, day2.candidateActionCode)

// rollup aggregates all days
assert.equal(report.rollup.totalPairs, 4)
assert.equal(report.rollup.actionCode.conflictCount, 1)
assert.ok(report.rollup.semanticMismatchCount >= 1)

// Action transition matrix (baseline consecutive days)
const bt = report.actionTransitions.baseline
assert.equal(bt.transitionCount, 3)
assert.ok(bt.uniqueEdges >= 1)
assert.ok(Array.isArray(bt.transitions))

const ct = report.actionTransitions.candidate
assert.equal(ct.transitionCount, 3)

const cross = report.actionTransitions.crossProducerSameDay
assert.equal(cross.dayCount, 4)
assert.ok(cross.transitions.some((e) => e.fromCode !== e.toCode))

// unit: transition matrix helper
const m = buildActionTransitionMatrix(['ENTER', 'WAIT_PULLBACK', 'WATCH'])
assert.equal(m.transitionCount, 2)
assert.equal(m.uniqueEdges, 2)
assert.deepEqual(m.transitions.map((e) => `${e.fromCode}→${e.toCode}`).sort(), [
  'ENTER→WAIT_PULLBACK',
  'WAIT_PULLBACK→WATCH',
].sort())

// baseline actions derived from fixed Signal/Gate/Zone/Size (js_legacy assemble, producer unchanged)
assert.ok(report.daily.every((d) => typeof d.baselineActionCode === 'string' && d.baselineActionCode.length > 0))

mkdirSync(outDir, { recursive: true })
writeFileSync(outPath, JSON.stringify(report, null, 2), 'utf8')

console.log('Phase2-D Historical Shadow Replay')
console.log(' ', report.summary)
console.log('  daily:', report.daily.map((d) => ({
  date: d.tradeDate,
  base: d.baselineActionCode,
  cand: d.candidateActionCode,
  conflict: d.stability.actionCode.conflictCount,
})))
console.log('  baseline transitions:', bt.transitions)
console.log('  crossProducerSameDay:', cross.transitions)
console.log('  wrote:', outPath)
console.log('quant-historical-shadow-replay Phase2-D: passed')
