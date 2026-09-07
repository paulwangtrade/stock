/**
 * Phase16.14 ExplanationTable row helpers (display-only).
 */
import { normalizeExplanationFields } from './explanationKit.js'
import {
  ORIGIN_EMPTY,
  ORIGIN_SIGNAL_PRICE_TOOLTIP,
} from './tradePlanOriginDisplay.js'

export const EXPLANATION_TABLE_COLUMNS = {
  field: '字段',
  content: '内容',
}

/**
 * Map legacy ExplanationFieldGrid fields → ExplanationTable rows.
 * @param {Array} fields
 * @param {{ labelFn?: Function, group?: string, showSource?: boolean }} [options]
 */
export function fieldsToExplanationTableRows(fields, options = {}) {
  const labelFn = typeof options.labelFn === 'function' ? options.labelFn : null
  const group = options.group ? String(options.group) : ''
  return normalizeExplanationFields(fields).map((f) => {
    const fieldLabel = labelFn ? labelFn(f.label) || f.label : f.label
    return {
      key: f.key,
      field: fieldLabel,
      content: f.value,
      missing: Boolean(f.missing),
      kind: f.showTag ? 'tag' : 'text',
      source: f.source || '',
      tooltip: f.tooltip || '',
      group,
      emphasis: 'normal',
      meta: {},
    }
  })
}

/**
 * TradePlan Origin display row → table rows (保留全部原字段).
 * @param {object} row buildOriginItemDisplay result
 */
export function buildOriginExplanationRows(row) {
  const r = row && typeof row === 'object' ? row : {}
  const strategyLabel = r.strategyNameLabel || r.strategyName || ORIGIN_EMPTY
  const scoreLabel = r.scoreLabel || r.score || ORIGIN_EMPTY
  const sourceLabel = r.sourceChipLabel || ORIGIN_EMPTY
  return [
    {
      key: 'source_chip',
      field: '来源',
      content: sourceLabel,
      missing: !r.sourceChipLabel || r.sourceBucket === 'unknown',
      kind: 'tag',
      group: 'meta',
    },
    {
      key: 'strategy_name',
      field: '策略',
      content: strategyLabel === '—' ? ORIGIN_EMPTY : strategyLabel,
      missing: Boolean(r.strategyNameMissing) || strategyLabel === '—' || strategyLabel === ORIGIN_EMPTY,
      kind: 'text',
      group: 'meta',
    },
    {
      key: 'score',
      field: '评分',
      content: scoreLabel === '—' ? ORIGIN_EMPTY : scoreLabel,
      missing: Boolean(r.scoreMissing) || scoreLabel === '—' || scoreLabel === ORIGIN_EMPTY,
      kind: 'text',
      group: 'meta',
    },
    {
      key: 'source_reason',
      field: '发现依据',
      content: r.sourceReason ?? ORIGIN_EMPTY,
      missing: Boolean(r.sourceReasonMissing),
      kind: 'text',
      group: 'signal',
    },
    {
      key: 'selection_reason',
      field: '入选说明',
      content: r.selectionReason ?? ORIGIN_EMPTY,
      missing: Boolean(r.selectionReasonMissing),
      kind: 'text',
      group: 'opportunity',
    },
    {
      key: 'signal_time',
      field: '信号时间',
      content: r.signalTime ?? ORIGIN_EMPTY,
      missing: Boolean(r.signalTimeMissing),
      kind: 'text',
      group: 'signal',
    },
    {
      key: 'signal_price',
      field: '信号价格',
      content: r.signalPrice ?? ORIGIN_EMPTY,
      missing: Boolean(r.signalPriceMissing),
      kind: 'text',
      tooltip: ORIGIN_SIGNAL_PRICE_TOOLTIP,
      group: 'signal',
    },
    {
      key: 'signal_tag',
      field: '信号标签',
      content: r.signalTag ?? ORIGIN_EMPTY,
      missing: Boolean(r.signalTagMissing),
      kind: r.signalTagMissing ? 'text' : 'tag',
      group: 'signal',
    },
  ]
}

/**
 * Filter + normalize rows for ExplanationTable.
 */
export function normalizeExplanationTableRows(rows, options = {}) {
  if (!Array.isArray(rows)) return []
  const groups = Array.isArray(options.groups) ? options.groups.filter(Boolean) : null
  return rows
    .map((row, index) => {
      const r = row && typeof row === 'object' ? row : {}
      const kind = String(r.kind || 'text').trim() || 'text'
      return {
        key: String(r.key ?? r.field ?? index),
        field: String(r.field ?? r.label ?? '').trim(),
        content: r.content == null && r.value == null ? '' : String(r.content ?? r.value),
        missing: Boolean(r.missing),
        kind,
        source: r.source == null ? '' : String(r.source).trim(),
        tooltip: r.tooltip == null ? '' : String(r.tooltip).trim(),
        group: r.group == null ? '' : String(r.group).trim(),
        emphasis: r.emphasis || 'normal',
        meta: r.meta && typeof r.meta === 'object' ? r.meta : {},
      }
    })
    .filter((r) => r.field)
    .filter((r) => !groups || !groups.length || groups.includes(r.group))
}
