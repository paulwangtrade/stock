/**
 * QuantDecision Phase2-C Shadow Stability Report Golden
 * 运行：
 *   node --import ./scripts/loaders/register-js-ext.mjs scripts/verify-quant-shadow-stability-golden.mjs
 */
import assert from 'node:assert/strict'
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { assembleWatchlistDecision } from '../src/utils/quantDecisionAssemble.js'
import { mutateDecisionAction, PRODUCER_GO_ENGINE } from '../src/utils/quantDecisionCompare.js'
import {
  buildShadowStabilityReport,
  topSliceMismatchPaths,
} from '../src/utils/quantDecisionShadowStability.js'

const __dir = dirname(fileURLToPath(import.meta.url))
const outDir = join(__dir, '../../backend/decision/semantic/testdata')
const goJSONPath = join(__dir, '../../backend/decision/shadow/testdata/go_engine_buy_ready.json')

const zoneNear = {
  text: '10.00~10.20',
  instantText: '10.10',
  instantPrice: 10.1,
  mode: 'near',
  deferMode: 'same',
  daysAgo: 0,
  note: 'near',
  low: 10,
  high: 10.2,
}

function assembleBuy(code, name = 'T') {
  return assembleWatchlistDecision({
    entry: {
      code,
      name,
      tag: '强',
      daysAgo: 0,
      buyPriceRange: zoneNear,
      statusText: 'today-strong',
    },
    checklist: {
      ready: true,
      score: 0.875,
      requiredPassed: true,
      readyThreshold: 0.85,
      items: [{ id: 'buy_signal', label: 'buy_signal', passed: true }],
    },
    plan: {
      ok: true,
      suggestedShares: 1000,
      suggestedAddShares: 1000,
      positionPct: 12.5,
      stopPrice: 9.85,
      reason: 'buy-1000',
    },
    existingVolume: 0,
    marketModeKey: 'level3',
    purpose: 'watchlist',
    tradeDate: '2026-07-21',
    asOf: '2026-07-21T00:00:00.000Z',
  })
}

const js1 = assembleBuy('sz000001', 'A')
const js2 = assembleBuy('sz000002', 'B')
const js3 = assembleBuy('sz000003', 'C')

let goShadow = null
try {
  goShadow = JSON.parse(readFileSync(goJSONPath, 'utf8'))
} catch {
  goShadow = mutateDecisionAction(js1, {
    code: 'ENTER',
    label: '可买',
    allowDraft: true,
    side: 'buy',
  }, { producer: PRODUCER_GO_ENGINE })
}

const pairs = [
  // stable + optional display drift
  {
    id: 'stable-1',
    code: 'sz000001',
    baseline: js1,
    candidate: goShadow,
  },
  {
    id: 'stable-display',
    code: 'sz000002',
    baseline: js2,
    candidate: mutateDecisionAction(js2, {
      code: 'ENTER',
      label: '建议关注买入',
      allowDraft: true,
      side: 'buy',
    }, { producer: PRODUCER_GO_ENGINE }),
  },
  // action.code conflict + zone mismatch
  {
    id: 'conflict-code',
    code: 'sz000003',
    baseline: js3,
    candidate: mutateDecisionAction({
      ...js3,
      entryZone: {
        ...(js3.entryZone || zoneNear),
        mode: 'above',
        deferMode: 'wait',
      },
    }, {
      code: 'WAIT_PULLBACK',
      label: '等回踩',
      allowDraft: false,
      side: 'none',
    }, { producer: PRODUCER_GO_ENGINE }),
  },
  // second same conflict pattern for stats count
  {
    id: 'conflict-code-2',
    code: 'sz000004',
    baseline: assembleBuy('sz000004', 'D'),
    candidate: mutateDecisionAction(assembleBuy('sz000004', 'D'), {
      code: 'WAIT_PULLBACK',
      label: '等回踩',
      allowDraft: false,
      side: 'none',
    }, { producer: PRODUCER_GO_ENGINE }),
  },
]

const report = buildShadowStabilityReport(pairs)

assert.equal(report.phase, 'Phase2-C')
assert.equal(report.totalPairs, 4)
assert.equal(report.semanticEqualCount, 2)
assert.equal(report.semanticMismatchCount, 2)
assert.equal(report.displayOnlyDiffCount, 1)
assert.equal(report.actionCode.equalCount, 2)
assert.equal(report.actionCode.conflictCount, 2)

const topConflict = report.actionCode.conflictStats[0]
assert.ok(topConflict)
assert.equal(topConflict.leftCode, 'ENTER')
assert.equal(topConflict.rightCode, 'WAIT_PULLBACK')
assert.equal(topConflict.count, 2)

assert.ok(report.mismatchBySlice.action.mismatchPairCount >= 2)
assert.ok(report.mismatchBySlice.entryZone.mismatchPairCount >= 1)

const actionPaths = topSliceMismatchPaths(report, 'action', 5)
assert.ok(actionPaths.some((p) => p.path.includes('action.code') || p.path === 'action.code'))

assert.ok(report.summary.includes('UNSTABLE') || report.actionCode.conflictCount > 0)

mkdirSync(outDir, { recursive: true })
const outPath = join(outDir, 'shadow_stability_report.json')
writeFileSync(outPath, JSON.stringify(report, null, 2), 'utf8')

console.log('Phase2-C Shadow Stability Report')
console.log(' ', report.summary)
console.log('  actionCode conflicts:', report.actionCode.conflictCount, report.actionCode.conflictStats)
console.log('  mismatchBySlice:', Object.fromEntries(
  Object.entries(report.mismatchBySlice).map(([k, v]) => [k, v.mismatchPairCount]),
))
console.log('  wrote:', outPath)
console.log('quant-shadow-stability Phase2-C: passed')
