/**
 * QuantDecision Phase1-B2 Golden：
 * buildBarSignalHint Decision path ≡ legacy path 文案；actionSource 可观测。
 * 运行：
 *   node --import ./scripts/loaders/register-js-ext.mjs scripts/verify-quant-bar-signal-hint-golden.mjs
 */
import assert from 'node:assert/strict'
import { assembleWatchlistDecision } from '../src/utils/quantDecisionAssemble.js'
import {
  buildBarSignalHint,
  BAR_ACTION_SOURCE_DECISION,
  BAR_ACTION_SOURCE_LEGACY,
} from '../src/utils/signalActionHint.js'

const zoneNear = {
  text: '10.00~10.20',
  instantText: '10.10',
  instantPrice: 10.1,
  mode: 'near',
  deferMode: 'same',
  daysAgo: 0,
  note: '贴近区间',
  extended: false,
}
const zoneAbove = {
  text: '10.00~10.20',
  instantText: '10.10 ↑',
  instantPrice: 10.1,
  mode: 'above',
  deferMode: 'wait',
  daysAgo: 0,
  note: '高于区间',
  extended: false,
}

/** 单根 K：barIndex=0 → daysAgo=0 */
function oneBarBars(close = 10.1) {
  return {
    closes: [close],
    opens: [close],
    highs: [close],
    lows: [close],
    volumes: [1000],
    dayKeys: ['20260721'],
  }
}

const cases = [
  {
    id: 'B01-buy-ready',
    tag: '强',
    buyPriceRange: zoneNear,
    checklistReady: true,
    checklist: { ready: true, score: 0.9, requiredPassed: true, items: [] },
    plan: {
      ok: true,
      suggestedShares: 1000,
      suggestedAddShares: 1000,
      positionPct: 10,
      reason: '建议买入',
    },
    entryExtra: {},
    expectLabel: '可买',
  },
  {
    id: 'B02-wait',
    tag: '趋',
    buyPriceRange: zoneAbove,
    checklistReady: false,
    checklist: { ready: false, score: 0.4, requiredPassed: false, items: [] },
    plan: { ok: false, reason: '不足一手', suggestedShares: 0, suggestedAddShares: 0 },
    entryExtra: {},
    expectLabel: '等回踩',
  },
  {
    id: 'B03-sell',
    tag: '减',
    buyPriceRange: null,
    checklistReady: false,
    checklist: null,
    plan: null,
    entryExtra: { sellPositionPct: 0.3 },
    expectLabel: '先风控',
  },
  {
    id: 'B04-ice',
    tag: '冰',
    buyPriceRange: null,
    checklistReady: false,
    checklist: null,
    plan: null,
    entryExtra: {},
    expectLabel: '仅观察',
  },
]

let passed = 0
const diffs = []

for (const row of cases) {
  const bars = oneBarBars()
  const entry = {
    code: 'sz000001',
    name: '测试',
    tag: row.tag,
    daysAgo: 0,
    buyPriceRange: row.buyPriceRange,
    statusText: `${row.tag}信号`,
    ...row.entryExtra,
  }
  const decision = assembleWatchlistDecision({
    entry,
    checklist: row.checklist,
    plan: row.plan,
    existingVolume: row.tag === '减' ? 1000 : 0,
    marketModeKey: 'level3',
    asOf: '2026-07-21T00:00:00.000Z',
  })

  const common = {
    sig: {},
    bars,
    barIndex: 0,
    tag: row.tag,
    livePrice: 10.1,
    options: {
      buyPriceRange: row.buyPriceRange || undefined,
      checklistReady: row.checklistReady,
      sellPositionPct: row.entryExtra.sellPositionPct,
    },
  }

  const hintDecision = buildBarSignalHint({ ...common, decision })
  const hintLegacy = buildBarSignalHint({ ...common, decision: null })

  try {
    assert.ok(hintDecision, `${row.id}: decision hint`)
    assert.ok(hintLegacy, `${row.id}: legacy hint`)
    assert.equal(hintDecision.actionSource, BAR_ACTION_SOURCE_DECISION, `${row.id}: source decision`)
    assert.equal(hintLegacy.actionSource, BAR_ACTION_SOURCE_LEGACY, `${row.id}: source legacy`)
    assert.equal(hintDecision.label, row.expectLabel, `${row.id}: decision label`)
    assert.equal(hintLegacy.label, row.expectLabel, `${row.id}: legacy label`)
    assert.equal(hintDecision.label, hintLegacy.label, `${row.id}: decision ≡ legacy label`)
    assert.equal(hintDecision.type, hintLegacy.type, `${row.id}: type`)
    assert.deepEqual(hintDecision.lines, hintLegacy.lines, `${row.id}: lines`)
    assert.equal(hintDecision.label, decision.action.label, `${row.id}: ≡ assemble`)
    passed++
  } catch (e) {
    diffs.push({
      id: row.id,
      error: e.message,
      decisionHint: hintDecision && {
        label: hintDecision.label,
        type: hintDecision.type,
        actionSource: hintDecision.actionSource,
        lines: hintDecision.lines,
      },
      legacyHint: hintLegacy && {
        label: hintLegacy.label,
        type: hintLegacy.type,
        actionSource: hintLegacy.actionSource,
        lines: hintLegacy.lines,
      },
      assembleLabel: decision.action?.label,
    })
  }
}

// 错配 tag：有 Decision 但不匹配 → 必须走 legacy_fallback
{
  const decision = assembleWatchlistDecision({
    entry: { code: 'x', tag: '强', daysAgo: 0, buyPriceRange: zoneNear },
    checklist: { ready: true, score: 0.9, items: [] },
    plan: { ok: true, suggestedShares: 100, suggestedAddShares: 100 },
    asOf: '2026-07-21T00:00:00.000Z',
  })
  const hint = buildBarSignalHint({
    sig: {},
    bars: oneBarBars(),
    barIndex: 0,
    tag: '冰',
    decision,
    options: {},
  })
  try {
    assert.equal(hint?.actionSource, BAR_ACTION_SOURCE_LEGACY, 'mismatch → legacy')
    assert.equal(hint?.label, '仅观察', 'mismatch ice label')
    passed++
  } catch (e) {
    diffs.push({ id: 'MISMATCH-tag', error: e.message, hint })
  }
}

const total = cases.length + 1
console.log(`quant-bar-signal-hint B2 golden: ${passed}/${total} passed`)
if (diffs.length) {
  console.error('DIFFS:')
  for (const d of diffs) {
    console.error(JSON.stringify(d, null, 2))
  }
  process.exit(1)
}

console.log('F2 closed: K-line Action via Decision or marked legacy_fallback; no independent labels')
console.log('Samples:', cases.map((c) => `${c.id}:${c.expectLabel}`).join(' · '), '· MISMATCH→legacy')
