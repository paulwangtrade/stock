/**
 * QuantDecision Phase1-A Golden：deriveQuantAction vs resolveSignalActionHint 输出一致。
 * 运行：
 *   node --import ./scripts/loaders/register-js-ext.mjs scripts/verify-quant-action-derive-golden.mjs
 */
import assert from 'node:assert/strict'
import { resolveSignalActionHint } from '../src/utils/signalActionHint.js'
import { deriveQuantAction, toLegacyActionHint } from '../src/utils/quantActionDerive.js'
import { assembleWatchlistDecision } from '../src/utils/quantDecisionAssemble.js'

function assertHintEqual(caseId, legacy, derivedHint) {
  assert.equal(legacy == null, derivedHint == null, `${caseId}: null mismatch`)
  if (legacy == null) return
  assert.equal(derivedHint.label, legacy.label, `${caseId}: label`)
  assert.equal(derivedHint.type, legacy.type, `${caseId}: type`)
  assert.deepEqual(derivedHint.lines, legacy.lines, `${caseId}: lines`)
  assert.equal(derivedHint.tooltip, legacy.tooltip, `${caseId}: tooltip`)
  if (legacy.tag !== undefined) {
    assert.equal(derivedHint.tag, legacy.tag, `${caseId}: tag`)
  } else {
    assert.equal(derivedHint.tag, undefined, `${caseId}: tag should be absent`)
  }
  if (Object.prototype.hasOwnProperty.call(legacy, 'buyPriceRange')) {
    assert.equal(derivedHint.buyPriceRange, legacy.buyPriceRange, `${caseId}: buyPriceRange ref`)
  }
}

const zoneNear = { text: '10.00~10.20', instantText: '10.10', instantPrice: 10.1, mode: 'near', deferMode: 'same', daysAgo: 0 }
const zoneAbove = { text: '10.00~10.20', instantText: '10.10 ↑', instantPrice: 10.1, mode: 'above', deferMode: 'wait', daysAgo: 0 }
const zoneBelow = { text: '10.00~10.20', instantText: '10.10 ↓', instantPrice: 10.1, mode: 'below', deferMode: 'defer', daysAgo: 0 }
const zoneIn = { text: '10.00~10.20', instantText: '10.10', instantPrice: 10.1, mode: 'inZone', deferMode: 'todayOrTomorrow', daysAgo: 0 }

/** @type {{ id: string, ctx: object }[]} */
const cases = [
  { id: 'G01', ctx: { tag: '减', daysAgo: 0, sellPositionPct: 0.3 } },
  { id: 'G02', ctx: { tag: '止', daysAgo: 0, sellPositionPct: 0.4 } },
  { id: 'G03', ctx: { tag: '减', daysAgo: 2, isHistorical: true, sellPositionPct: 0.3 } },
  { id: 'G04', ctx: { tag: '冰', daysAgo: 0 } },
  { id: 'G05', ctx: { tag: '冲', daysAgo: 0, rushReducePct: 0.35 } },
  { id: 'G06', ctx: { tag: '加', daysAgo: 0, addPositionPct: 0.2, sourceTag: '强' } },
  { id: 'G07', ctx: { tag: '强', daysAgo: 1, isHistorical: true, buyPriceRange: zoneNear } },
  { id: 'G08', ctx: { tag: '强', daysAgo: 0, checklistReady: true, buyPriceRange: zoneAbove } },
  { id: 'G09', ctx: { tag: '趋', daysAgo: 0, checklistReady: false, buyPriceRange: zoneAbove } },
  { id: 'G10', ctx: { tag: '趋', daysAgo: 0, checklistReady: false, buyPriceRange: { ...zoneNear, mode: 'near', deferMode: 'wait' } } },
  { id: 'G11', ctx: { tag: '买', daysAgo: 0, checklistReady: false, buyPriceRange: zoneIn } },
  { id: 'G12', ctx: { tag: '强', daysAgo: 0, checklistReady: false, buyPriceRange: zoneNear } },
  { id: 'G13', ctx: { tag: '突', daysAgo: 0, checklistReady: false, buyPriceRange: zoneBelow } },
  { id: 'G14', ctx: { tag: '买', daysAgo: 2, isHistorical: false, checklistReady: false, buyPriceRange: { text: '—', mode: 'unknown', daysAgo: 2 } } },
  { id: 'G15', ctx: { tag: '转', daysAgo: 0, checklistReady: false } },
  { id: 'G16', ctx: { tag: '强', daysAgo: 0, checklistReady: false } },
  {
    id: 'G17',
    ctx: {
      tag: '',
      holdingAdvice: { action: 'add', actionLabel: '倾向加仓', summaryLine: '量价偏强', suggestPctDisplay: 20 },
    },
  },
  {
    id: 'G18',
    ctx: {
      tag: '',
      holdingAdvice: { action: 'hold', actionLabel: '持有观望', summaryLine: '中性' },
    },
  },
  { id: 'G19', ctx: { tag: '强', daysAgo: 0, checklistReady: true, buyPriceRange: zoneAbove } },
  {
    id: 'G20',
    ctx: {
      tag: '减',
      daysAgo: 0,
      sellPositionPct: 0.25,
      sellVolume: 500,
      costPrice: 9.5,
    },
  },
  { id: 'G21', ctx: { tag: '弹', daysAgo: 0, checklistReady: false } },
  { id: 'G22', ctx: { tag: '买', daysAgo: 0, checklistReady: false } },
]

let passed = 0
const diffs = []

for (const { id, ctx } of cases) {
  const legacy = resolveSignalActionHint(ctx)
  const derived = deriveQuantAction(ctx)
  const projected = toLegacyActionHint(derived)
  try {
    assertHintEqual(id, legacy, projected)
    passed++
  } catch (e) {
    diffs.push({
      id,
      error: e.message,
      legacy: legacy && { label: legacy.label, type: legacy.type, lines: legacy.lines },
      derived: projected && { label: projected.label, type: projected.type, lines: projected.lines, code: derived?.code },
    })
  }
}

// assemble：_legacyHint 与 resolveSignalActionHint 一致
const assembleCases = [
  {
    id: 'A01',
    entry: { code: 'sz000001', name: '平安银行', tag: '强', daysAgo: 0, buyPriceRange: zoneNear, statusText: '今日强化买点' },
    checklist: { ready: true, score: 0.9, requiredPassed: true, items: [] },
    plan: { ok: true, suggestedShares: 1000, suggestedAddShares: 1000, suggestedAmount: 10000, positionPct: 10, reason: '建议买入' },
  },
  {
    id: 'A02',
    entry: { code: 'sz000001', tag: '趋', daysAgo: 0, buyPriceRange: zoneAbove },
    checklist: { ready: false, score: 0.5, requiredPassed: false, items: [] },
    plan: { ok: false, reason: '风险/敞口约束下建议仓位不足一手' },
  },
]

for (const row of assembleCases) {
  const decision = assembleWatchlistDecision(row)
  const ctx = {
    tag: row.entry.tag,
    buyPriceRange: row.entry.buyPriceRange,
    daysAgo: row.entry.daysAgo,
    isHistorical: row.entry.daysAgo > 0,
    checklistReady: row.checklist?.ready === true,
  }
  const legacy = resolveSignalActionHint(ctx)
  try {
    assertHintEqual(row.id, legacy, decision._legacyHint)
    assert.equal(decision.action.label, legacy?.label ?? '', `${row.id}: action.label`)
    assert.equal(decision.meta.producer, 'js_legacy')
    assert.equal(decision.meta.schemaVersion, 1)
    passed++
  } catch (e) {
    diffs.push({
      id: row.id,
      error: e.message,
      legacy: legacy && { label: legacy.label, type: legacy.type },
      assembled: decision._legacyHint && { label: decision._legacyHint.label, type: decision._legacyHint.type },
      actionCode: decision.action?.code,
    })
  }
}

console.log(`quant-action-derive golden: ${passed}/${cases.length + assembleCases.length} passed`)
if (diffs.length) {
  console.error('DIFFS:')
  for (const d of diffs) {
    console.error(JSON.stringify(d, null, 2))
  }
  process.exit(1)
}

console.log('Action diff: none (legacy resolveSignalActionHint === deriveQuantAction projection)')
console.log('Sample codes:', cases.slice(0, 8).map(({ id, ctx }) => {
  const d = deriveQuantAction(ctx)
  return `${id}:${d?.label}->${d?.code || 'null'}`
}).join(' | '))

// Step2：store 写入 Decision（不接 Card / 不改 hint）
const { setQuantEntry, quantDecisionFor, quantPlanFor, quantChecklistFor, clearQuantAutomationCaches } =
  await import('../src/utils/quantAutomationStore.js')

clearQuantAutomationCaches()
const step2Entry = {
  code: 'sz000001',
  name: '平安银行',
  tag: '强',
  daysAgo: 0,
  statusText: '今日强化买点 · RSI 45.0',
  buyPriceRange: zoneNear,
  ok: true,
}
const step2Checklist = { ready: true, score: 0.875, requiredPassed: true, items: [{ id: 'buy_signal', passed: true }] }
const step2Plan = {
  ok: true,
  suggestedShares: 1000,
  suggestedAddShares: 1000,
  suggestedAmount: 102000,
  positionPct: 12.5,
  stopPrice: 9.85,
  reason: '建议买入 1000 股，约占权益 12.5%',
}
const step2Decision = assembleWatchlistDecision({
  entry: step2Entry,
  checklist: step2Checklist,
  plan: step2Plan,
  marketModeKey: 'level3',
  existingVolume: 0,
  purpose: 'watchlist',
  tradeDate: '2026-07-21',
  asOf: '2026-07-21T00:00:00.000Z',
})
setQuantEntry(step2Entry.code, step2Entry, step2Plan, step2Checklist, step2Decision)

const stored = quantDecisionFor('sz000001')
assert.ok(stored, 'store decision missing')
assert.equal(stored.meta.producer, 'js_legacy')
assert.equal(stored.meta.schemaVersion, 1)
assert.equal(stored.action.code, 'ENTER')
assert.equal(stored.action.label, '可买')
assert.equal(quantPlanFor('sz000001')?.ok, true)
assert.equal(quantChecklistFor('sz000001')?.ready, true)
assert.equal(stored.action.label, resolveSignalActionHint({
  tag: '强',
  buyPriceRange: zoneNear,
  daysAgo: 0,
  checklistReady: true,
})?.label)

console.log('Step2 store write: ok')
console.log('Decision sample JSON:')
console.log(JSON.stringify({
  purpose: stored.purpose,
  instrument: stored.instrument,
  regime: stored.regime,
  signal: stored.signal,
  entryZone: stored.entryZone,
  gate: { score: stored.gate.score, ready: stored.gate.ready },
  risk: stored.risk,
  size: {
    ok: stored.size.ok,
    targetShares: stored.size.targetShares,
    positionPct: stored.size.positionPct,
  },
  action: {
    code: stored.action.code,
    label: stored.action.label,
    allowDraft: stored.action.allowDraft,
    side: stored.action.side,
  },
  meta: stored.meta,
}, null, 2))
