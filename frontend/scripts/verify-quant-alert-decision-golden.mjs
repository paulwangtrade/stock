/**
 * QuantDecision Phase1-B1 Golden：
 * Decision.action.label ≡ Card projection.actionLabel ≡ Alert actionLabel
 * 运行：
 *   node --import ./scripts/loaders/register-js-ext.mjs scripts/verify-quant-alert-decision-golden.mjs
 */
import assert from 'node:assert/strict'
import { assembleWatchlistDecision } from '../src/utils/quantDecisionAssemble.js'
import {
  projectWatchlistCard,
  watchlistProjectionUiSnapshot,
} from '../src/utils/quantWatchlistProjection.js'
import {
  collectWatchlistActionAlerts,
  resolveWatchlistAlertAction,
  ALERT_ACTION_SOURCE_DECISION,
  ALERT_ACTION_SOURCE_LEGACY,
} from '../src/utils/watchlistActionAlert.js'
import {
  setQuantEntry,
  quantDecisionFor,
  quantChecklistFor,
  quantEntryFor,
  clearQuantAutomationCaches,
} from '../src/utils/quantAutomationStore.js'

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

const cases = [
  {
    id: 'A01-buy-ready',
    entry: {
      ok: true,
      code: 'sz000001',
      name: '平安银行',
      tag: '强',
      daysAgo: 0,
      statusText: '今日强化买点',
      buyPriceRange: zoneNear,
      sortRank: 1,
    },
    plan: {
      ok: true,
      suggestedShares: 1000,
      suggestedAddShares: 1000,
      suggestedAmount: 102000,
      positionPct: 12.5,
      stopPrice: 9.85,
      reason: '建议买入 1000 股',
      existingVolume: 0,
    },
    checklist: {
      ready: true,
      score: 0.875,
      requiredPassed: true,
      items: [{ id: 'buy_signal', label: '买入信号', passed: true }],
    },
    result: { '股票代码': 'sz000001', '股票名称': '平安银行', costVolume: 0, costPrice: 0 },
    expectLabel: '可买',
  },
  {
    id: 'A02-wait',
    entry: {
      ok: true,
      code: 'sz000002',
      name: '万科A',
      tag: '趋',
      daysAgo: 0,
      statusText: '趋势买点',
      buyPriceRange: zoneAbove,
      sortRank: 2,
    },
    plan: {
      ok: false,
      reason: '风险/敞口约束下建议仓位不足一手',
      suggestedShares: 0,
      suggestedAddShares: 0,
    },
    checklist: {
      ready: false,
      score: 0.5,
      requiredPassed: false,
      items: [{ id: 'zone', label: '价区', passed: false }],
    },
    result: { '股票代码': 'sz000002', '股票名称': '万科A', costVolume: 0, costPrice: 0 },
    expectLabel: '等回踩',
  },
  {
    id: 'A03-sell',
    entry: {
      ok: true,
      code: 'sz000003',
      name: '国农科技',
      tag: '减',
      daysAgo: 0,
      sellPositionPct: 0.3,
      statusText: '减仓信号',
      sortRank: 3,
    },
    plan: null,
    checklist: null,
    result: {
      '股票代码': 'sz000003',
      '股票名称': '国农科技',
      costVolume: 1000,
      costPrice: 9.5,
    },
    expectLabel: '先风控',
  },
]

let passed = 0
const diffs = []

clearQuantAutomationCaches()

for (const row of cases) {
  const decision = assembleWatchlistDecision({
    entry: row.entry,
    checklist: row.checklist,
    plan: row.plan,
    existingVolume: Number(row.result.costVolume) || 0,
    marketModeKey: 'level3',
    purpose: 'watchlist',
    tradeDate: '2026-07-21',
    asOf: '2026-07-21T00:00:00.000Z',
  })
  setQuantEntry(row.entry.code, row.entry, row.plan, row.checklist, decision)

  const stored = quantDecisionFor(row.entry.code)
  const cardSnap = watchlistProjectionUiSnapshot(
    projectWatchlistCard({
      result: row.result,
      signal: {
        tag: row.entry.tag,
        daysAgo: 0,
        recentSignalDaysAgo: 0,
        statusText: row.entry.statusText,
        sellPositionPct: row.entry.sellPositionPct,
      },
      buyPriceRange: row.entry.buyPriceRange || null,
      entryTag: row.entry.tag,
      quantPlan: row.plan,
      quantChecklist: row.checklist,
      decision: stored,
    }),
  )

  const byCode = { [row.entry.code]: row.entry }
  const alerts = collectWatchlistActionAlerts(byCode, {
    quantDecisionFor,
    quantChecklistFor,
    quantEntryFor,
    automation: { enabled: false },
  })
  const alert = alerts.find((a) => a.code === row.entry.code)

  try {
    assert.ok(stored, `${row.id}: store decision`)
    assert.equal(stored.action.label, row.expectLabel, `${row.id}: decision.label`)
    assert.equal(cardSnap.actionLabel, row.expectLabel, `${row.id}: card.actionLabel`)
    assert.ok(alert, `${row.id}: alert missing`)
    assert.equal(alert.actionLabel, row.expectLabel, `${row.id}: alert.actionLabel`)
    assert.equal(alert.actionSource, ALERT_ACTION_SOURCE_DECISION, `${row.id}: actionSource`)
    assert.equal(
      stored.action.label,
      cardSnap.actionLabel,
      `${row.id}: decision ≡ card`,
    )
    assert.equal(
      cardSnap.actionLabel,
      alert.actionLabel,
      `${row.id}: card ≡ alert`,
    )
    assert.equal(
      stored.action.label,
      alert.actionLabel,
      `${row.id}: decision ≡ alert`,
    )
    passed++
  } catch (e) {
    diffs.push({
      id: row.id,
      error: e.message,
      decisionLabel: stored?.action?.label,
      cardLabel: cardSnap?.actionLabel,
      alertLabel: alert?.actionLabel,
      actionSource: alert?.actionSource,
    })
  }
}

// legacy_fallback：无 Decision 时标记路径，文案仍经 adapter（不改可见 label）
clearQuantAutomationCaches()
const legacyEntry = {
  ok: true,
  code: 'sz000099',
  name: '遗留回退',
  tag: '强',
  daysAgo: 0,
  buyPriceRange: zoneNear,
  statusText: '今日强化买点',
}
setQuantEntry(legacyEntry.code, legacyEntry, null, { ready: true, score: 0.9, items: [] }, undefined)
// 故意不写 decision
const legacyResolved = resolveWatchlistAlertAction(legacyEntry.code, legacyEntry, {
  quantDecisionFor: () => null,
  quantChecklistFor: () => ({ ready: true, score: 0.9, items: [] }),
  quantEntryFor: () => legacyEntry,
})
try {
  assert.equal(legacyResolved.actionSource, ALERT_ACTION_SOURCE_LEGACY, 'legacy source')
  assert.equal(legacyResolved.actionHint?.label, '可买', 'legacy label')
  passed++
} catch (e) {
  diffs.push({ id: 'LEGACY-fallback', error: e.message, resolved: legacyResolved })
}

const total = cases.length + 1
console.log(`quant-alert-decision B1 golden: ${passed}/${total} passed`)
if (diffs.length) {
  console.error('DIFFS:')
  for (const d of diffs) {
    console.error(JSON.stringify(d, null, 2))
  }
  process.exit(1)
}

console.log('F1 closed: Alert consumes Decision.action (legacy_fallback marked)')
console.log('Samples:', cases.map((c) => `${c.id}:${c.expectLabel}`).join(' · '), '· LEGACY:可买')
