/**
 * OutcomeProjection display helpers (Phase16-D4).
 * Formats GET /api/opportunities/outcomes for Outcome Tab UI.
 * Does not compute returns — all metrics come from Outcome API fields.
 */
import {
  CANDIDATE_STATUS_LABEL,
  DECISION_STATUS_LABEL,
  PROJECTION_FIELD_LABEL,
} from './opportunityProjectionDisplay.js'
import { ORIGIN_EMPTY } from './tradePlanOriginDisplay.js'

export const OUTCOME_STATUS_LABEL = {
  OPEN: '持仓中',
  CLOSED: '已平仓',
  NO_TRADE: '无成交',
}

export const OUTCOME_DISCLAIMER =
  '本面板为机会成交结果只读解释（Signal → Opportunity → Decision → Entry → Exit → Performance）。' +
  '收益按账户级 FIFO 配对计算，不代表真实券商成交。'

export const OUTCOME_EMPTY_MESSAGE = '暂无交易结果投影'

export const OUTCOME_LAYER_SOURCE = {
  signal: 'SignalSnapshot',
  opportunity: 'CandidatePool',
  decision: 'TradePlanItem',
  plan: 'TradePlan',
  entry: 'PaperSimFill',
  exit: 'PaperSimFill',
  performance: 'FIFO matcher',
}

function str(v) {
  if (v == null) return ''
  return String(v).trim()
}

function formatMoney(value) {
  const n = Number(value)
  if (!Number.isFinite(n)) return ORIGIN_EMPTY
  return n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function formatQty(value) {
  const n = Math.trunc(Number(value))
  if (!Number.isFinite(n)) return ORIGIN_EMPTY
  return n.toLocaleString('zh-CN')
}

function field(label, value, source, missing = false) {
  return {
    label,
    value: missing ? ORIGIN_EMPTY : value || ORIGIN_EMPTY,
    source,
    missing,
  }
}

function enumLabel(map, key, fallback = ORIGIN_EMPTY) {
  const k = str(key).toUpperCase()
  return map[k] || key || fallback
}

export function outcomeStatusTagType(status) {
  const key = str(status).toUpperCase()
  if (key === 'CLOSED') return 'success'
  if (key === 'OPEN') return 'info'
  if (key === 'NO_TRADE') return 'default'
  return 'default'
}

export function outcomeQualityTagType(quality) {
  return quality === 'complete' ? 'success' : 'warning'
}

export function outcomeQualityLabel(quality) {
  return quality === 'complete' ? '解释完整' : '部分层缺失'
}

/** Format API realized_return_pct only — no client-side calculation. */
export function formatOutcomeReturnPct(pct) {
  const n = Number(pct)
  if (!Number.isFinite(n)) return ORIGIN_EMPTY
  const sign = n > 0 ? '+' : ''
  return `${sign}${n.toFixed(2)}%`
}

export function formatOutcomeHoldingDays(days) {
  const n = Math.trunc(Number(days))
  if (!Number.isFinite(n) || n < 0) return ORIGIN_EMPTY
  return `${n} 天`
}

export function formatOutcomeDate(value) {
  const s = str(value)
  if (!s) return ORIGIN_EMPTY
  return s.includes('T') ? s.replace('T', ' ').slice(0, 19) : s
}

export function buildOutcomeLegSummary(leg) {
  if (!leg || typeof leg !== 'object') return ORIGIN_EMPTY
  const status = str(leg.outcome_status).toUpperCase()
  const label = OUTCOME_STATUS_LABEL[status] || status || ORIGIN_EMPTY
  const entryDate = formatOutcomeDate(leg.entry?.entry_date)
  const pct = leg.performance?.realized_return_pct
  if (status === 'CLOSED' && Number.isFinite(Number(pct))) {
    return `${label} · ${formatOutcomeReturnPct(pct)}`
  }
  if (status === 'OPEN' && Number.isFinite(Number(leg.performance?.holding_days))) {
    return `${label} · ${formatOutcomeHoldingDays(leg.performance.holding_days)}`
  }
  if (entryDate !== ORIGIN_EMPTY) {
    return `${label} · ${entryDate}`
  }
  return label
}

/** Pick default leg index; optionally align with provenance trades (newest fill first). */
export function defaultLegIndex(items, provenanceTrades) {
  const legs = Array.isArray(items) ? items : []
  if (legs.length === 0) return 0
  const trades = Array.isArray(provenanceTrades) ? provenanceTrades : []
  const targetFillId = trades[0]?.fillId
  if (targetFillId) {
    const idx = legs.findIndex((leg) => Number(leg?.entry?.buy_fill_id) === Number(targetFillId))
    if (idx >= 0) return idx
  }
  const openIdx = legs.findIndex((leg) => str(leg?.outcome_status).toUpperCase() === 'OPEN')
  if (openIdx >= 0) return openIdx
  return 0
}

function buildExplainSections(leg) {
  const sig = leg?.signal || {}
  const opp = leg?.opportunity || {}
  const dec = leg?.decision || {}
  const entry = leg?.entry || {}
  const sigPresent = !!sig.present
  const oppPresent = !!opp.present

  return [
    {
      key: 'signal',
      title: '一、发现',
      sourceBanner: `来源：${OUTCOME_LAYER_SOURCE.signal}`,
      fields: [
        field(PROJECTION_FIELD_LABEL.signal_tag, sigPresent ? str(sig.signal_tag) : '', OUTCOME_LAYER_SOURCE.signal, !sigPresent || !str(sig.signal_tag)),
        field(PROJECTION_FIELD_LABEL.signal_price, sigPresent ? formatMoney(sig.signal_price) : '', OUTCOME_LAYER_SOURCE.signal, !sigPresent || !Number.isFinite(Number(sig.signal_price))),
        field(PROJECTION_FIELD_LABEL.signal_time, sigPresent ? formatOutcomeDate(sig.signal_time) : '', OUTCOME_LAYER_SOURCE.signal, !sigPresent || !str(sig.signal_time)),
        field('发现依据', sigPresent ? str(sig.trigger_reason) : '', OUTCOME_LAYER_SOURCE.signal, !sigPresent || !str(sig.trigger_reason)),
      ],
    },
    {
      key: 'opportunity',
      title: '二、关注',
      sourceBanner: `来源：${OUTCOME_LAYER_SOURCE.opportunity}`,
      fields: [
        field(PROJECTION_FIELD_LABEL.rank, oppPresent ? String(opp.rank ?? '') : '', OUTCOME_LAYER_SOURCE.opportunity, !oppPresent || !(Math.trunc(Number(opp.rank)) > 0)),
        field(PROJECTION_FIELD_LABEL.score, oppPresent ? String(opp.score ?? '') : '', OUTCOME_LAYER_SOURCE.opportunity, !oppPresent || !Number.isFinite(Number(opp.score))),
        field('策略', oppPresent ? str(opp.strategy_name) : '', OUTCOME_LAYER_SOURCE.opportunity, !oppPresent || !str(opp.strategy_name)),
      ],
    },
    {
      key: 'decision',
      title: '三、决策',
      sourceBanner: `来源：${OUTCOME_LAYER_SOURCE.decision}`,
      fields: [
        field(PROJECTION_FIELD_LABEL.decision_status, enumLabel(DECISION_STATUS_LABEL, dec.decision_status), OUTCOME_LAYER_SOURCE.decision, !str(dec.decision_status)),
        field(PROJECTION_FIELD_LABEL.candidate_status, enumLabel(CANDIDATE_STATUS_LABEL, dec.candidate_status), OUTCOME_LAYER_SOURCE.decision, !str(dec.candidate_status)),
      ],
    },
    {
      key: 'plan',
      title: '四、计划',
      sourceBanner: `来源：${OUTCOME_LAYER_SOURCE.plan}`,
      fields: [
        field(
          PROJECTION_FIELD_LABEL.plan_id,
          entry.present && entry.buy_plan_id ? String(entry.buy_plan_id) : '',
          OUTCOME_LAYER_SOURCE.plan,
          !entry.present || !(Math.trunc(Number(entry.buy_plan_id)) > 0),
        ),
      ],
      planId: entry.present ? entry.buy_plan_id : undefined,
    },
  ]
}

function buildResultSections(leg) {
  const status = str(leg?.outcome_status).toUpperCase()
  const entry = leg?.entry || {}
  const exit = leg?.exit || {}
  const perf = leg?.performance || {}
  const entryPresent = !!entry.present
  const exitPresent = !!exit.present
  const isOpenLeg = status === 'OPEN'
  const isNoTrade = status === 'NO_TRADE'

  const entryFields = [
    field('买入价格', entryPresent ? formatMoney(entry.entry_price) : '', OUTCOME_LAYER_SOURCE.entry, !entryPresent || !Number.isFinite(Number(entry.entry_price))),
    field('买入数量', entryPresent ? formatQty(entry.entry_qty) : '', OUTCOME_LAYER_SOURCE.entry, !entryPresent || !Number.isFinite(Number(entry.entry_qty))),
    field('买入日期', entryPresent ? formatOutcomeDate(entry.entry_date) : '', OUTCOME_LAYER_SOURCE.entry, !entryPresent || !str(entry.entry_date)),
    field('buy_fill_id', entryPresent && entry.buy_fill_id ? String(entry.buy_fill_id) : '', OUTCOME_LAYER_SOURCE.entry, !entryPresent || !(Math.trunc(Number(entry.buy_fill_id)) > 0)),
  ]

  const exitFields = exitPresent
    ? [
        field('卖出价格', formatMoney(exit.exit_price), OUTCOME_LAYER_SOURCE.exit, !Number.isFinite(Number(exit.exit_price))),
        field('卖出数量', formatQty(exit.exit_qty), OUTCOME_LAYER_SOURCE.exit, !Number.isFinite(Number(exit.exit_qty))),
        field('卖出日期', formatOutcomeDate(exit.exit_date), OUTCOME_LAYER_SOURCE.exit, !str(exit.exit_date)),
        field('卖出原因', str(exit.exit_reason_text), OUTCOME_LAYER_SOURCE.exit, !str(exit.exit_reason_text)),
      ]
    : isOpenLeg && !isNoTrade
      ? [field('卖出', '持仓中，尚未卖出', OUTCOME_LAYER_SOURCE.exit, false)]
      : [field('卖出', ORIGIN_EMPTY, OUTCOME_LAYER_SOURCE.exit, true)]

  const perfFields = []
  if (Number.isFinite(Number(perf.holding_days))) {
    perfFields.push(
      field('持仓天数', formatOutcomeHoldingDays(perf.holding_days), OUTCOME_LAYER_SOURCE.performance, false),
    )
  }
  if (Number.isFinite(Number(perf.realized_return_pct))) {
    perfFields.push(
      field(
        '实现收益率',
        formatOutcomeReturnPct(perf.realized_return_pct),
        OUTCOME_LAYER_SOURCE.performance,
        false,
      ),
    )
  }

  return {
    entry: { present: entryPresent, fields: entryFields },
    exit: { present: exitPresent, isOpenLeg, fields: exitFields },
    performance: { fields: perfFields },
  }
}

export function buildOutcomeTabView(items, selectedIndex = 0, generatedAt = '') {
  const legs = Array.isArray(items) ? items : []
  const index = legs.length === 0 ? 0 : Math.min(Math.max(0, selectedIndex), legs.length - 1)
  const selected = legs[index] ?? null
  const quality = str(selected?.metadata?.quality) || 'partial'

  return {
    legs: legs.map((leg, i) => ({
      index: i,
      outcomeId: str(leg?.outcome_id),
      status: str(leg?.outcome_status),
      summary: buildOutcomeLegSummary(leg),
      buyFillId: leg?.entry?.buy_fill_id,
      buyPlanId: leg?.entry?.buy_plan_id,
    })),
    selected: selected
      ? {
          raw: selected,
          status: str(selected.outcome_status),
          statusLabel: OUTCOME_STATUS_LABEL[str(selected.outcome_status).toUpperCase()] || selected.outcome_status,
          returnPct: formatOutcomeReturnPct(selected.performance?.realized_return_pct),
          holdingDays: formatOutcomeHoldingDays(selected.performance?.holding_days),
        }
      : null,
    explainSections: selected ? buildExplainSections(selected) : [],
    resultSections: selected ? buildResultSections(selected) : null,
    isEmpty: legs.length === 0,
    quality,
    qualityLabel: outcomeQualityLabel(quality),
    generatedAt: str(generatedAt) || str(selected?.metadata?.as_of),
  }
}
