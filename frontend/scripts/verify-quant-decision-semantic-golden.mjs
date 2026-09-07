/**
 * QuantDecision Phase2-B0 Semantic Equality Golden
 * 运行：
 *   node --import ./scripts/loaders/register-js-ext.mjs scripts/verify-quant-decision-semantic-golden.mjs
 */
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { assembleWatchlistDecision } from '../src/utils/quantDecisionAssemble.js'
import {
  normalizeDecisionSemantic,
  compareDecisionSemantic,
  assertSemanticEqual,
} from '../src/utils/quantDecisionSemantic.js'
import { mutateDecisionAction, PRODUCER_GO_ENGINE } from '../src/utils/quantDecisionCompare.js'

const __dir = dirname(fileURLToPath(import.meta.url))
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

const jsLegacy = assembleWatchlistDecision({
  entry: {
    code: 'sz000001',
    name: 'PingAnBank',
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

let passed = 0
const fails = []

function ok(id, fn) {
  try {
    fn()
    passed++
  } catch (e) {
    fails.push({ id, error: e.message })
  }
}

ok('S01-normalize-drops-label', () => {
  const n = normalizeDecisionSemantic(jsLegacy)
  assert.equal(n.action.code, 'ENTER')
  assert.equal(Object.prototype.hasOwnProperty.call(n.action, 'label'), false)
  assert.equal(n.action.allowDraft, true)
  assert.equal(n.action.side, 'buy')
})

ok('S02-label-display-only', () => {
  const right = mutateDecisionAction(jsLegacy, {
    code: 'ENTER',
    label: '建议关注买入',
    allowDraft: true,
    side: 'buy',
  }, { producer: PRODUCER_GO_ENGINE })
  const r = compareDecisionSemantic(jsLegacy, right)
  assert.equal(r.semanticEqual, true, r.summary)
  assert.equal(r.actionCodeEqual, true)
  assert.equal(r.displayDiffs.length, 1)
  assert.equal(r.displayDiffs[0].path, 'action.label')
})

ok('S03-code-highest-priority', () => {
  const right = mutateDecisionAction(jsLegacy, {
    code: 'WAIT_PULLBACK',
    label: '等回踩',
    allowDraft: false,
    side: 'none',
  }, { producer: PRODUCER_GO_ENGINE })
  const r = compareDecisionSemantic(jsLegacy, right)
  assert.equal(r.semanticEqual, false)
  assert.equal(r.actionCodeEqual, false)
  const codeDiff = r.semanticDiffs.find((d) => d.path === 'action.code')
  assert.ok(codeDiff, 'missing action.code diff')
  assert.equal(codeDiff.priority, 'highest')
  assert.ok(r.summary.includes('action.code'))
})

ok('S04-assert-allows-label-drift', () => {
  const right = {
    ...jsLegacy,
    action: { ...jsLegacy.action, label: '完全不同的展示文案' },
    meta: { ...jsLegacy.meta, producer: PRODUCER_GO_ENGINE },
  }
  const r = assertSemanticEqual(jsLegacy, right)
  assert.equal(r.semanticEqual, true)
  assert.ok(r.displayDiffs.length >= 1)
})

ok('S05-js-vs-go-shadow-semantic', () => {
  const goEngine = JSON.parse(readFileSync(goJSONPath, 'utf8'))
  const r = compareDecisionSemantic(jsLegacy, goEngine)
  assert.equal(r.actionCodeEqual, true, `codes ${r.leftActionCode} vs ${r.rightActionCode}`)
  assert.equal(r.semanticEqual, true, `${r.summary} ${JSON.stringify(r.semanticDiffs)}`)
})

const total = 5
console.log(`quant-decision-semantic B0 golden: ${passed}/${total} passed`)
if (fails.length) {
  console.error('FAILS:', JSON.stringify(fails, null, 2))
  process.exit(1)
}
console.log('Phase2-B0: semantic layer OK (Action.Code > Label display)')
