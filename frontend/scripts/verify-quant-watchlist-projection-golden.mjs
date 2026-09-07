/**
 * QuantDecision Phase1-B0 Projection Golden：
 * 有完整 Decision / decision-only / legacy fallback 三者 UI 快照一致。
 * 运行：
 *   node --import ./scripts/loaders/register-js-ext.mjs scripts/verify-quant-watchlist-projection-golden.mjs
 */
import assert from 'node:assert/strict'
import { assembleWatchlistDecision } from '../src/utils/quantDecisionAssemble.js'
import { resolveSignalActionHint } from '../src/utils/signalActionHint.js'
import {
  projectWatchlistCard,
  watchlistProjectionUiSnapshot,
} from '../src/utils/quantWatchlistProjection.js'

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

function assertSnapshotEqual(caseId, label, a, b) {
  assert.deepEqual(a, b, `${caseId}: ${label}`)
}

/** @type {{ id: string, result: object, signal: object|null, buyPriceRange: object|null, entryTag?: string, plan: object|null, checklist: object|null, expect?: Partial<object> }[]} */
const cases = [
  {
    id: 'UI01-buy-ready',
    result: { '股票代码': 'sz000001', '股票名称': '平安银行', costVolume: 0, costPrice: 0 },
    signal: {
      tag: '强',
      daysAgo: 0,
      recentSignalDaysAgo: 0,
      statusText: '今日强化买点',
    },
    buyPriceRange: zoneNear,
    entryTag: '强',
    plan: {
      ok: true,
      suggestedShares: 1000,
      suggestedAddShares: 1000,
      suggestedAmount: 102000,
      positionPct: 12.5,
      stopPrice: 9.85,
      reason: '建议买入 1000 股，约占权益 12.5%',
      existingVolume: 0,
    },
    checklist: {
      ready: true,
      score: 0.875,
      requiredPassed: true,
      items: [
        { id: 'buy_signal', label: '买入信号', passed: true },
        { id: 'zone', label: '价区', passed: true },
      ],
    },
    expect: {
      signalLabel: '强',
      actionLabel: '可买',
      actionType: 'success',
      buyRangeLabel: '今/明日参考',
      buyPriceDisplay: '10.00~10.20',
      checklistText: '就绪 88%',
      planText: '1000 股 · 12.5%',
      draftButtonLabel: '生成买入草稿',
      canCreateDraft: true,
      showQuantStat: true,
      showBuyRange: true,
    },
  },
  {
    id: 'UI02-wait-not-ready',
    result: { '股票代码': 'sz000002', '股票名称': '万科A', costVolume: 0, costPrice: 0 },
    signal: { tag: '趋', daysAgo: 0, recentSignalDaysAgo: 0, statusText: '趋势买点' },
    buyPriceRange: zoneAbove,
    entryTag: '趋',
    plan: {
      ok: false,
      reason: '风险/敞口约束下建议仓位不足一手',
      suggestedShares: 0,
      suggestedAddShares: 0,
      existingVolume: 0,
    },
    checklist: {
      ready: false,
      score: 0.5,
      requiredPassed: false,
      items: [{ id: 'zone', label: '价区', passed: false }],
    },
    expect: {
      signalLabel: '趋',
      actionLabel: '等回踩',
      actionType: 'warning',
      buyRangeLabel: '明日参考',
      buyPriceDisplay: '10.00~10.20',
      checklistText: '50%',
      planText: '风险/敞口约束下建议仓位不足一手',
      draftButtonLabel: '生成交易草稿',
      canCreateDraft: false,
      showQuantStat: true,
      showBuyRange: true,
    },
  },
  {
    id: 'UI03-sell-reduce',
    result: {
      '股票代码': 'sz000003',
      '股票名称': '国农科技',
      costVolume: 1000,
      costPrice: 9.5,
    },
    signal: {
      tag: '减',
      daysAgo: 0,
      recentSignalDaysAgo: 0,
      sellPositionPct: 0.3,
      statusText: '减仓信号',
    },
    buyPriceRange: null,
    entryTag: '减',
    plan: null,
    checklist: null,
    expect: {
      signalLabel: '减30%',
      actionLabel: '先风控',
      actionType: 'error',
      draftButtonLabel: '生成卖出草稿',
      canCreateDraft: true,
      showQuantStat: false,
      showBuyRange: false,
    },
  },
  {
    id: 'UI04-holding-advice',
    result: {
      '股票代码': 'sz000004',
      '股票名称': '国华网安',
      costVolume: 500,
      costPrice: 8.2,
    },
    signal: {
      tag: '',
      daysAgo: 0,
      holdingAdvice: {
        action: 'add',
        actionLabel: '倾向加仓',
        suggestPctDisplay: 20,
        factors: [{ impact: 1, label: '量价', detail: '偏强' }],
      },
    },
    buyPriceRange: null,
    entryTag: '',
    plan: null,
    checklist: null,
    expect: {
      signalLabel: '',
      actionLabel: '倾向加仓',
      holdingAdviceLabel: '倾向加仓 20%',
      showHoldingAdvice: true,
      canCreateDraft: false,
      draftButtonLabel: '生成交易草稿',
      showQuantStat: false,
    },
  },
]

let passed = 0
const diffs = []

for (const row of cases) {
  const baseInput = {
    result: row.result,
    signal: row.signal,
    buyPriceRange: row.buyPriceRange,
    entryTag: row.entryTag || '',
    quantPlan: row.plan,
    quantChecklist: row.checklist,
  }

  const entry = {
    code: row.result['股票代码'],
    name: row.result['股票名称'],
    tag: row.signal?.tag || '',
    daysAgo: row.signal?.daysAgo ?? 0,
    buyPriceRange: row.buyPriceRange,
    statusText: row.signal?.statusText || '',
    sellPositionPct: row.signal?.sellPositionPct,
    addPositionPct: row.signal?.addPositionPct,
    rushReducePct: row.signal?.rushReducePct,
    holdingAdvice: row.signal?.holdingAdvice,
  }
  const existingVolume = Number(row.result.costVolume) || 0
  const decision = assembleWatchlistDecision({
    entry,
    checklist: row.checklist,
    plan: row.plan,
    existingVolume,
    marketModeKey: 'level3',
    purpose: 'watchlist',
    tradeDate: '2026-07-21',
    asOf: '2026-07-21T00:00:00.000Z',
  })

  // ① 有完整 Decision（散装 props + decision）
  const fullSnap = watchlistProjectionUiSnapshot(
    projectWatchlistCard({ ...baseInput, decision }),
  )
  // ② decision-only（仅 result + decision，消费 entryZone/gate/size/action）
  const decisionOnlySnap = watchlistProjectionUiSnapshot(
    projectWatchlistCard({ result: row.result, decision }),
  )
  // ③ legacy fallback（无 decision，散装 props）
  const legacySnap = watchlistProjectionUiSnapshot(
    projectWatchlistCard(baseInput),
  )

  const hintCtx = {
    tag: row.signal?.tag,
    buyPriceRange: row.buyPriceRange,
    daysAgo: row.signal?.daysAgo ?? 0,
    isHistorical: (row.signal?.daysAgo ?? 0) > 0,
    checklistReady: row.checklist?.ready === true,
    sellPositionPct: row.signal?.sellPositionPct,
    addPositionPct: row.signal?.addPositionPct,
    rushReducePct: row.signal?.rushReducePct,
    holdingAdvice: row.signal?.holdingAdvice,
  }
  const adapterHint = resolveSignalActionHint(hintCtx)

  try {
    assertSnapshotEqual(row.id, 'full === decision-only', fullSnap, decisionOnlySnap)
    assertSnapshotEqual(row.id, 'full === legacy', fullSnap, legacySnap)
    assertSnapshotEqual(row.id, 'decision-only === legacy', decisionOnlySnap, legacySnap)

    if (adapterHint) {
      assert.equal(fullSnap.actionLabel, adapterHint.label, `${row.id}: action vs adapter`)
      assert.equal(fullSnap.actionType, adapterHint.type, `${row.id}: type vs adapter`)
    } else {
      assert.equal(fullSnap.actionLabel, '', `${row.id}: empty action`)
    }
    if (row.expect) {
      for (const [k, v] of Object.entries(row.expect)) {
        assert.equal(fullSnap[k], v, `${row.id}: expect.${k}`)
      }
    }
    // F6：有 Decision 时 canCreateDraft 必须等于 action.allowDraft
    assert.equal(
      fullSnap.canCreateDraft,
      !!decision.action?.allowDraft,
      `${row.id}: F6 allowDraft`,
    )
    // F3：decision-only 在有 entryZone 时应展示买区
    if (decision.entryZone?.text) {
      assert.equal(decisionOnlySnap.showBuyRange, true, `${row.id}: F3 showBuyRange`)
      assert.equal(decisionOnlySnap.buyPriceDisplay, row.buyPriceRange.text, `${row.id}: F3 display`)
    }
    passed++
  } catch (e) {
    diffs.push({
      id: row.id,
      error: e.message,
      full: fullSnap,
      decisionOnly: decisionOnlySnap,
      legacy: legacySnap,
      decisionAction: decision.action && {
        code: decision.action.code,
        label: decision.action.label,
        allowDraft: decision.action.allowDraft,
      },
      entryZone: decision.entryZone,
    })
  }
}

console.log(`quant-watchlist-projection B0 golden: ${passed}/${cases.length} passed (full≡decision-only≡legacy)`)
if (diffs.length) {
  console.error('DIFFS:')
  for (const d of diffs) {
    console.error(JSON.stringify(d, null, 2))
  }
  process.exit(1)
}

console.log('Phase1-B0: F3 entryZone + F6 allowDraft hardened; no visible copy drift')
console.log('Samples:', cases.map((c) => {
  const decision = assembleWatchlistDecision({
    entry: {
      code: c.result['股票代码'],
      name: c.result['股票名称'],
      tag: c.signal?.tag || '',
      daysAgo: c.signal?.daysAgo ?? 0,
      buyPriceRange: c.buyPriceRange,
      statusText: c.signal?.statusText || '',
      sellPositionPct: c.signal?.sellPositionPct,
      holdingAdvice: c.signal?.holdingAdvice,
    },
    checklist: c.checklist,
    plan: c.plan,
    existingVolume: Number(c.result.costVolume) || 0,
    asOf: '2026-07-21T00:00:00.000Z',
  })
  const snap = watchlistProjectionUiSnapshot(projectWatchlistCard({ result: c.result, decision }))
  return `${c.id}:${snap.actionLabel || '-'}|zone=${snap.showBuyRange}|draft=${snap.canCreateDraft}`
}).join(' · '))
