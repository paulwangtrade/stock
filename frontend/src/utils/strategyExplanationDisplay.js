/**
 * Phase16.15-A — Strategy Explanation DTO → display helpers.
 * Pure display layer: no API calls, no mutations, no business logic.
 * Phase16.18-B2: price field labels via priceDisplay.
 * Phase16.21-D: status labels via statusDisplay.
 */
import {
  formatPriceValue,
  formatPriceWithContext,
  priceColumnTitle,
  PRICE_KIND,
} from './priceDisplay.js'
import { formatStatus } from './statusDisplay.js'

const EMPTY = '—'

/**
 * Phase16.20-A / 16.21-D — user-facing Strategy Explanation status (never raw "degraded"/blank).
 * @param {unknown} status
 * @returns {string}
 */
export function strategyExplanationStatusLabel(status) {
  return formatStatus(status, 'explanation').label
}

/**
 * @param {unknown} status
 * @returns {'success'|'warning'|'error'|'info'|'default'}
 */
export function strategyExplanationStatusType(status) {
  return formatStatus(status, 'explanation').type
}

/**
 * Short hint under status — degraded is NOT a hard failure.
 * @param {unknown} status
 * @returns {string}
 */
export function strategyExplanationStatusHint(status) {
  return formatStatus(status, 'explanation').tooltip
}

function str(v) {
  if (v == null) return ''
  return String(v).trim()
}

function num(v, digits = 1) {
  const n = Number(v)
  return Number.isFinite(n) ? n.toFixed(digits) : ''
}

/**
 * Convert an Explanation DTO (from /api/product/strategy-explanation)
 * into ExplanationTable row objects.
 *
 * All fields from Signal / Risk / Entry / Citations / Disclaimers are preserved.
 * Each row: { key, field, content, missing, kind, group, emphasis, tooltip, source }
 *
 * @param {object} explanation - Explanation struct (strategyexplain/types.go)
 * @returns {Array} rows compatible with ExplanationTable.vue
 */
export function buildStrategyExplanationRows(explanation) {
  if (!explanation || typeof explanation !== 'object') return []
  const sections = explanation.sections || {}
  const signal = sections.signal || {}
  const risk = sections.risk || {}
  const entry = sections.entry || {}
  const exit = sections.exit || null
  const citations = Array.isArray(explanation.citations) ? explanation.citations : []
  const disclaimers = Array.isArray(explanation.disclaimers) ? explanation.disclaimers : []

  const rows = []

  // ── Signal ──────────────────────────────────────────────────────────────
  rows.push({
    key: 'signal_tag',
    field: '信号类型',
    content: str(signal.tag) || EMPTY,
    missing: !str(signal.tag),
    kind: str(signal.tag) ? 'tag' : 'text',
    group: 'signal',
    emphasis: 'normal',
    tooltip: '',
    source: '',
  })
  rows.push({
    key: 'signal_score',
    field: '评分',
    content: num(signal.score) || EMPTY,
    missing: !Number.isFinite(Number(signal.score)),
    kind: 'text',
    group: 'signal',
    emphasis: 'normal',
    tooltip: '策略模型综合评分，不代表收益预测',
    source: '',
  })
  if (Number(signal.snapshot_ref) > 0) {
    rows.push({
      key: 'signal_snapshot_ref',
      field: '扫描引用',
      content: `#${Number(signal.snapshot_ref)}`,
      missing: false,
      kind: 'text',
      group: 'signal',
      emphasis: 'muted',
      tooltip: '信号扫描快照编号（只读）',
      source: '',
    })
  }
  if (str(signal.narrative)) {
    rows.push({
      key: 'signal_narrative',
      field: '信号说明',
      content: str(signal.narrative),
      missing: false,
      kind: 'text',
      group: 'signal',
      emphasis: 'normal',
      tooltip: '',
      source: '',
    })
  }
  if (str(signal.missing_reason)) {
    rows.push({
      key: 'signal_missing_reason',
      field: '信号缺失原因',
      content: str(signal.missing_reason),
      missing: true,
      kind: 'text',
      group: 'signal',
      emphasis: 'muted',
      tooltip: '',
      source: '',
    })
  }

  // ── Risk ─────────────────────────────────────────────────────────────────
  rows.push({
    key: 'risk_code',
    field: '风险代码',
    content: str(risk.risk_code) || EMPTY,
    missing: !str(risk.risk_code),
    kind: 'text',
    group: 'risk',
    emphasis: str(risk.risk_code) ? 'warn' : 'normal',
    tooltip: '',
    source: '',
  })
  rows.push({
    key: 'risk_message',
    field: '风险说明',
    content: str(risk.risk_message) || EMPTY,
    missing: !str(risk.risk_message),
    kind: 'text',
    group: 'risk',
    emphasis: 'normal',
    tooltip: '',
    source: '',
  })
  if (str(risk.plan_status)) {
    const st = formatStatus(risk.plan_status, 'plan')
    rows.push({
      key: 'risk_plan_status',
      field: '计划状态',
      content: st.label || EMPTY,
      missing: !st.label || st.label === '—',
      kind: 'text',
      group: 'risk',
      emphasis: 'muted',
      tooltip: st.tooltip || '',
      source: '',
    })
  }
  if (risk.accepted != null) {
    rows.push({
      key: 'risk_accepted',
      field: '风险接受',
      content: risk.accepted ? '已接受' : '未接受',
      missing: false,
      kind: 'text',
      group: 'risk',
      emphasis: risk.accepted ? 'normal' : 'warn',
      tooltip: '',
      source: '',
    })
  }
  if (str(risk.narrative)) {
    rows.push({
      key: 'risk_narrative',
      field: '风险详述',
      content: str(risk.narrative),
      missing: false,
      kind: 'text',
      group: 'risk',
      emphasis: 'normal',
      tooltip: '',
      source: '',
    })
  }

  // ── Entry ─────────────────────────────────────────────────────────────────
  rows.push({
    key: 'entry_strategy_name',
    field: '策略',
    content: str(entry.strategy_name) || EMPTY,
    missing: !str(entry.strategy_name),
    kind: 'text',
    group: 'entry',
    emphasis: 'normal',
    tooltip: '',
    source: '',
  })
  if (str(entry.entry_rule)) {
    rows.push({
      key: 'entry_rule',
      field: '入场规则',
      content: str(entry.entry_rule),
      missing: false,
      kind: 'text',
      group: 'entry',
      emphasis: 'normal',
      tooltip: '',
      source: '',
    })
  }
  if (Number.isFinite(Number(entry.ref_price)) && Number(entry.ref_price) > 0) {
    const refCtx = formatPriceWithContext(entry.ref_price, PRICE_KIND.ref, {
      source: str(entry.ref_source),
      asOf: str(entry.ref_as_of),
      digits: 2,
    })
    rows.push({
      key: 'entry_ref_price',
      field: priceColumnTitle(PRICE_KIND.ref),
      content: refCtx.value,
      missing: false,
      kind: 'text',
      group: 'entry',
      emphasis: 'normal',
      tooltip: refCtx.tooltip,
      source: str(entry.ref_source),
    })
  }
  if (str(entry.thesis_intact_note)) {
    rows.push({
      key: 'entry_thesis_intact',
      field: '逻辑完整性',
      content: str(entry.thesis_intact_note),
      missing: false,
      kind: 'text',
      group: 'entry',
      emphasis: 'normal',
      tooltip: '',
      source: '',
    })
  }
  if (str(entry.narrative)) {
    rows.push({
      key: 'entry_narrative',
      field: '入场说明',
      content: str(entry.narrative),
      missing: false,
      kind: 'text',
      group: 'entry',
      emphasis: 'normal',
      tooltip: '',
      source: '',
    })
  }

  // ── Exit (optional) ──────────────────────────────────────────────────────
  if (exit && str(exit.mode) && str(exit.mode) !== 'none' && str(exit.mode) !== 'unavailable') {
    if (str(exit.narrative)) {
      rows.push({
        key: 'exit_narrative',
        field: '退出建议',
        content: str(exit.narrative),
        missing: false,
        kind: 'text',
        group: 'exit',
        emphasis: 'warn',
        tooltip: '',
        source: '',
      })
    }
  }

  // ── Citations ─────────────────────────────────────────────────────────────
  citations.forEach((c, i) => {
    const label = str(c?.label) || str(c?.ref) || `依据 ${i + 1}`
    const ref = str(c?.ref)
    const kind = str(c?.kind)
    rows.push({
      key: `citation_${i}`,
      field: i === 0 ? '决策依据' : '',
      content: kind ? `${label}${ref && ref !== label ? ' · ' + ref : ''}` : label,
      missing: false,
      kind: 'text',
      group: 'evidence',
      emphasis: 'muted',
      tooltip: ref || '',
      source: '',
    })
  })

  // ── Disclaimers ───────────────────────────────────────────────────────────
  disclaimers.forEach((d, i) => {
    rows.push({
      key: `disclaimer_${i}`,
      field: i === 0 ? '免责声明' : '',
      content: str(d),
      missing: false,
      kind: 'text',
      group: 'disclaimer',
      emphasis: 'muted',
      tooltip: '',
      source: '',
    })
  })

  return rows
}

/**
 * Convert a cache entry { itemId, code, name, explanation, planItemStatus?, limitPrice? }
 * into a flat summary row for TradePlanExplanationSummaryTable.
 *
 * @param {object} entry
 * @returns {object|null}
 */
export function toExplanationSummaryRow(entry) {
  if (!entry || typeof entry !== 'object') return null
  const expl = entry.explanation || {}
  const sections = expl.sections || {}
  const signal = sections.signal || {}
  const risk = sections.risk || {}
  const entry_ = sections.entry || {}

  const signalScore = Number.isFinite(Number(signal.score)) ? Number(signal.score) : null
  const refPrice =
    Number.isFinite(Number(entry_.ref_price)) && Number(entry_.ref_price) > 0
      ? Number(entry_.ref_price)
      : null
  const refCtx = formatPriceWithContext(refPrice, PRICE_KIND.ref, {
    source: str(entry_.ref_source),
    asOf: str(entry_.ref_as_of),
    digits: 2,
  })

  const explStatus = formatStatus(expl.status, 'explanation')
  const planSt = formatStatus(entry.planItemStatus, 'plan_item')
  const limit = Number(entry.limitPrice)
  const execKey =
    Number.isFinite(limit) && limit > 0
      ? 'ready'
      : String(entry.planItemStatus || '').toLowerCase() === 'filled'
        ? 'executed'
        : 'waiting_intent'
  const execSt = formatStatus(execKey, 'execution')

  return {
    itemId: entry.itemId,
    code: str(entry.code) || EMPTY,
    name: str(entry.name) || EMPTY,
    headline: str(expl.headline),
    status: str(expl.status),
    explainStatusLabel: explStatus.label || EMPTY,
    explainStatusType: explStatus.type,
    explainStatusTooltip: explStatus.tooltip || '',
    planStatusLabel: planSt.label || EMPTY,
    execStatusLabel: execSt.label || EMPTY,
    execStatusType: execSt.type,
    execStatusTooltip: execSt.tooltip || '',
    signalTag: str(signal.tag),
    signalScore,
    signalScoreLabel: signalScore != null ? String(Math.round(signalScore)) : EMPTY,
    riskCode: str(risk.risk_code),
    riskMessage: str(risk.risk_message),
    riskAccepted: risk.accepted,
    strategyName: str(entry_.strategy_name) || str(entry_.entry_rule) || EMPTY,
    entryRule: str(entry_.entry_rule),
    refPrice,
    refPriceLabel: refPrice != null ? refCtx.value : EMPTY,
    refPriceTooltip: refPrice != null ? refCtx.tooltip : '',
    refSource: str(entry_.ref_source),
    refAsOf: str(entry_.ref_as_of),
    isMissingExplain: String(expl.status || '').toLowerCase() === 'missing',
  }
}
