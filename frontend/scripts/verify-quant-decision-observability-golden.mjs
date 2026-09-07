/**
 * QuantDecision Phase1-C Observability Golden
 * 运行：
 *   node --import ./scripts/loaders/register-js-ext.mjs scripts/verify-quant-decision-observability-golden.mjs
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
} from '../src/utils/watchlistActionAlert.js'
import {
  buildBarSignalHint,
  BAR_ACTION_SOURCE_DECISION,
} from '../src/utils/signalActionHint.js'
import {
  setQuantEntry,
  quantDecisionFor,
  quantChecklistFor,
  quantEntryFor,
  clearQuantAutomationCaches,
} from '../src/utils/quantAutomationStore.js'
import {
  resetQuantDecisionObservability,
  getActionSourceCounts,
  getActionConflicts,
  assertNoActionConflicts,
  getQuantDecisionObservabilitySnapshot,
  recordActionObservation,
  OBS_CONSUMER_CARD,
  OBS_CONSUMER_ALERT,
  OBS_CONSUMER_KLINE,
  OBS_SOURCE_DECISION,
  OBS_SOURCE_LEGACY,
} from '../src/utils/quantDecisionObservability.js'

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

const ASOF = '2026-07-21T00:00:00.000Z'
const CODE = 'sz000001'

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

clearQuantAutomationCaches()
resetQuantDecisionObservability()

const entry = {
  ok: true,
  code: CODE,
  name: '平安银行',
  tag: '强',
  daysAgo: 0,
  statusText: '今日强化买点',
  buyPriceRange: zoneNear,
  sortRank: 1,
}
const plan = {
  ok: true,
  suggestedShares: 1000,
  suggestedAddShares: 1000,
  positionPct: 12.5,
  reason: '建议买入',
  existingVolume: 0,
}
const checklist = {
  ready: true,
  score: 0.875,
  requiredPassed: true,
  items: [{ id: 'buy_signal', label: '买入信号', passed: true }],
}
const result = { '股票代码': CODE, '股票名称': '平安银行', costVolume: 0, costPrice: 0 }

const decision = assembleWatchlistDecision({
  entry,
  checklist,
  plan,
  existingVolume: 0,
  marketModeKey: 'level3',
  purpose: 'watchlist',
  tradeDate: '2026-07-21',
  asOf: ASOF,
})
assert.ok(decision.id, 'DecisionID assigned')
setQuantEntry(CODE, entry, plan, checklist, decision)
const stored = quantDecisionFor(CODE)
assert.equal(stored.id, decision.id, 'store DecisionID')

resetQuantDecisionObservability()

const card = projectWatchlistCard({
  result,
  signal: {
    tag: '强',
    daysAgo: 0,
    recentSignalDaysAgo: 0,
    statusText: entry.statusText,
  },
  buyPriceRange: zoneNear,
  entryTag: '强',
  quantPlan: plan,
  quantChecklist: checklist,
  decision: stored,
})

const alerts = collectWatchlistActionAlerts(
  { [CODE]: entry },
  {
    quantDecisionFor,
    quantChecklistFor,
    quantEntryFor,
    automation: { enabled: false },
  },
)
const alert = alerts[0]

const barHint = buildBarSignalHint({
  sig: {},
  bars: oneBarBars(),
  barIndex: 0,
  tag: '强',
  livePrice: 10.1,
  decision: stored,
  options: { code: CODE, buyPriceRange: zoneNear, checklistReady: true },
})

// 1) 同 DecisionID 可回溯
assert.equal(card.decisionId, decision.id, 'card.decisionId')
assert.equal(alert.decisionId, decision.id, 'alert.decisionId')
assert.equal(barHint.decisionId, decision.id, 'kline.decisionId')
assert.equal(card.actionSource, OBS_SOURCE_DECISION, 'card source')
assert.equal(alert.actionSource, ALERT_ACTION_SOURCE_DECISION, 'alert source')
assert.equal(barHint.actionSource, BAR_ACTION_SOURCE_DECISION, 'kline source')

// 2) Action label 一致且文案未漂
assert.equal(card.actionLabel, '可买')
assert.equal(alert.actionLabel, '可买')
assert.equal(barHint.label, '可买')
assert.equal(card.actionLabel, alert.actionLabel)
assert.equal(alert.actionLabel, barHint.label)

// 3) actionSource 统计
const counts = getActionSourceCounts()
assert.equal(counts[OBS_SOURCE_DECISION], 3, 'decision count=3')
assert.equal(counts.byConsumer[OBS_CONSUMER_CARD][OBS_SOURCE_DECISION], 1)
assert.equal(counts.byConsumer[OBS_CONSUMER_ALERT][OBS_SOURCE_DECISION], 1)
assert.equal(counts.byConsumer[OBS_CONSUMER_KLINE][OBS_SOURCE_DECISION], 1)

// 4) 默认无冲突
assert.equal(assertNoActionConflicts(), true, 'no conflicts by default')
assert.equal(getActionConflicts().length, 0)

// 5) legacy_fallback 可计数
const legacy = resolveWatchlistAlertAction('sz000099', {
  ok: true,
  tag: '强',
  daysAgo: 0,
  buyPriceRange: zoneNear,
}, {
  quantDecisionFor: () => null,
  quantChecklistFor: () => ({ ready: true }),
  quantEntryFor: () => ({ buyPriceRange: zoneNear }),
})
assert.equal(legacy.actionSource, OBS_SOURCE_LEGACY)
assert.ok(getActionSourceCounts().legacyFallback >= 1, 'legacy countable')

// 6) 冲突检测：同 code+asOf 不同 label
resetQuantDecisionObservability()
recordActionObservation({
  consumer: OBS_CONSUMER_CARD,
  code: CODE,
  asOf: ASOF,
  decisionId: decision.id,
  actionLabel: '可买',
  actionCode: 'ENTER',
  actionSource: OBS_SOURCE_DECISION,
})
recordActionObservation({
  consumer: OBS_CONSUMER_ALERT,
  code: CODE,
  asOf: ASOF,
  decisionId: 'other-id',
  actionLabel: '等回踩',
  actionCode: 'WAIT_PULLBACK',
  actionSource: OBS_SOURCE_DECISION,
})
const conflictProbeCount = getActionConflicts().length
assert.ok(conflictProbeCount >= 1, 'conflict detected')

// 7) UI 文案快照不受观测字段影响（与 B0 关键字段一致）
resetQuantDecisionObservability()
const snap = watchlistProjectionUiSnapshot(projectWatchlistCard({
  result,
  signal: { tag: '强', daysAgo: 0, recentSignalDaysAgo: 0, statusText: entry.statusText },
  buyPriceRange: zoneNear,
  entryTag: '强',
  quantPlan: plan,
  quantChecklist: checklist,
  decision: stored,
}))
assert.equal(snap.actionLabel, '可买')
assert.equal(snap.canCreateDraft, true)
assert.equal('decisionId' in snap, false, 'snapshot excludes decisionId')
assert.equal(assertNoActionConflicts(), true, 'happy path no conflicts after reset')

console.log('quant-decision-observability C golden: passed')
console.log(JSON.stringify({
  decisionId: decision.id,
  consumersAligned: true,
  defaultConflicts: 0,
  sampleCountsAfterHappyPath: counts,
  conflictProbeCount,
  legacyFallbackCountable: true,
}, null, 2))
