/**
 * QuantDecision Phase1-D Dual Run Comparator Golden
 * 运行：
 *   node --import ./scripts/loaders/register-js-ext.mjs scripts/verify-quant-decision-compare-golden.mjs
 */
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { assembleWatchlistDecision } from '../src/utils/quantDecisionAssemble.js'
import {
  compareQuantDecisions,
  runDualDecisionCompare,
  mutateDecisionAction,
  PRODUCER_JS_LEGACY,
  PRODUCER_GO_ENGINE,
  DEFAULT_IGNORE_PATHS,
} from '../src/utils/quantDecisionCompare.js'

const __dir = dirname(fileURLToPath(import.meta.url))
const fixturePath = join(__dir, 'fixtures/quant-decision-dual-run-baseline.json')
const fixture = JSON.parse(readFileSync(fixturePath, 'utf8'))
const baseline = fixture.decision

let passed = 0
const diffsOut = []

function ok(id, fn) {
  try {
    fn()
    passed++
  } catch (e) {
    diffsOut.push({ id, error: e.message })
  }
}

// D01：同 fixture 自比 → 无 diff
ok('D01-same-fixture', () => {
  const r = compareQuantDecisions(baseline, structuredClone(baseline))
  assert.equal(r.equal, true, r.summary)
  assert.equal(r.diffs.length, 0)
})

// D02：忽略 id / asOf / producer 差异
ok('D02-ignore-id-asof-producer', () => {
  const right = structuredClone(baseline)
  right.id = 'totally-different-id'
  right.asOf = '2099-01-01T00:00:00.000Z'
  right.tradeDate = '2099-01-01'
  right.meta = { ...right.meta, producer: PRODUCER_GO_ENGINE }
  const r = runDualDecisionCompare({
    baseline,
    candidate: right,
    baselineProducer: PRODUCER_JS_LEGACY,
    candidateProducer: PRODUCER_GO_ENGINE,
  })
  assert.equal(r.equal, true, r.summary)
  assert.ok(DEFAULT_IGNORE_PATHS.includes('id'))
  assert.ok(DEFAULT_IGNORE_PATHS.includes('asOf'))
  assert.ok(DEFAULT_IGNORE_PATHS.includes('meta.producer'))
})

// D03：人工制造 Action 差异可检测
ok('D03-action-diff-detectable', () => {
  const candidate = mutateDecisionAction(baseline, {
    code: 'WAIT_PULLBACK',
    label: '等回踩',
    allowDraft: false,
    side: 'none',
    type: 'warning',
  })
  candidate.meta = { ...candidate.meta, producer: PRODUCER_GO_ENGINE }
  const r = runDualDecisionCompare({ baseline, candidate })
  assert.equal(r.equal, false, 'should differ')
  assert.ok(r.diffs.length >= 1)
  assert.ok(r.bySlice.action && r.bySlice.action.equal === false)
  const paths = r.diffs.map((d) => d.path)
  assert.ok(paths.some((p) => p.startsWith('action.')), `paths=${paths.join(',')}`)
  assert.ok(
    r.diffs.some((d) => d.path === 'action.label' && d.left === '可买' && d.right === '等回踩'),
    'action.label diff',
  )
  assert.ok(
    r.diffs.some((d) => d.path === 'action.code' && d.left === 'ENTER' && d.right === 'WAIT_PULLBACK'),
    'action.code diff',
  )
})

// D04：EntryZone 字段级 diff
ok('D04-entryZone-diff', () => {
  const candidate = structuredClone(baseline)
  candidate.meta.producer = PRODUCER_GO_ENGINE
  candidate.entryZone = { ...candidate.entryZone, mode: 'above', deferMode: 'wait' }
  const r = compareQuantDecisions(baseline, candidate)
  assert.equal(r.equal, false)
  assert.equal(r.bySlice.entryZone.equal, false)
  assert.ok(r.diffs.some((d) => d.path === 'entryZone.mode'))
  assert.equal(r.bySlice.action.equal, true, 'action untouched')
})

// D05：assemble 产物 vs 自身（经 harness）无 diff
ok('D05-assemble-self', () => {
  const live = assembleWatchlistDecision({
    entry: {
      code: 'sz000001',
      name: '平安银行',
      tag: '强',
      daysAgo: 0,
      buyPriceRange: {
        text: '10.00~10.20',
        instantText: '10.10',
        instantPrice: 10.1,
        mode: 'near',
        deferMode: 'same',
        daysAgo: 0,
        note: '贴近区间',
        low: 10,
        high: 10.2,
      },
      statusText: '今日强化买点',
    },
    checklist: {
      ready: true,
      score: 0.875,
      requiredPassed: true,
      readyThreshold: 0.85,
      items: [{ id: 'buy_signal', label: '买入信号', passed: true }],
    },
    plan: {
      ok: true,
      suggestedShares: 1000,
      suggestedAddShares: 1000,
      positionPct: 12.5,
      stopPrice: 9.85,
      reason: '建议买入 1000 股',
    },
    existingVolume: 0,
    marketModeKey: 'level3',
    tradeDate: '2026-07-21',
    asOf: '2026-07-21T00:00:00.000Z',
  })
  const twin = structuredClone(live)
  twin.id = 'other'
  twin.asOf = '2099-01-01T00:00:00.000Z'
  twin.meta = { ...twin.meta, producer: PRODUCER_GO_ENGINE }
  const r = runDualDecisionCompare({ baseline: live, candidate: twin })
  assert.equal(r.equal, true, r.summary)
})

// D06：Gate 差异可检
ok('D06-gate-diff', () => {
  const candidate = structuredClone(baseline)
  candidate.gate = { ...candidate.gate, ready: false, score: 0.4 }
  candidate.meta.producer = PRODUCER_GO_ENGINE
  const r = compareQuantDecisions(baseline, candidate)
  assert.equal(r.bySlice.gate.equal, false)
  assert.ok(r.diffs.some((d) => d.path === 'gate.ready'))
})

const total = 6
console.log(`quant-decision-compare D golden: ${passed}/${total} passed`)
if (diffsOut.length) {
  console.error('DIFFS:')
  for (const d of diffsOut) console.error(JSON.stringify(d, null, 2))
  process.exit(1)
}

const demo = runDualDecisionCompare({
  baseline,
  candidate: mutateDecisionAction(baseline, { code: 'WAIT_PULLBACK', label: '等回踩' }),
})
console.log('Phase1-D Dual Run Harness ready')
console.log('Sample Action diff report:', demo.summary)
console.log('Field diffs:', demo.diffs.map((d) => d.path).join(', '))
