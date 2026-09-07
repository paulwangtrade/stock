/**
 * Phase16-H0 Explanation UI Kit — shared display helpers (pure functions for tests).
 */
import { buildStockDisplayTitle, toStockDisplayModel } from './stockDisplay.js'

export const EXPLANATION_READONLY_LABEL = '只读'

export const EXPLANATION_PIPELINE_TITLE = '决策链路'

export const EXPLANATION_PIPELINE_CAPTION =
  '发现 → 关注 → 决策 → 计划 → 持仓（各层数据来源独立标注）'

export const EXPLANATION_SOURCE_PREFIX = '字段来源：'

/**
 * Build Drawer title via StockDisplayModel (Phase16.14).
 * Prefer buildExplanationDrawerTitleFromModel when a model is already available.
 * Never produces code(code) when name is missing.
 */
export function buildExplanationDrawerTitle(stockName, stockCode, theme) {
  const model = toStockDisplayModel({
    stock_code: stockCode,
    stock_name: stockName,
  })
  return buildStockDisplayTitle(model, theme)
}

/** @param {{ displayText?: string } | null | undefined} model */
export function buildExplanationDrawerTitleFromModel(model, theme) {
  return buildStockDisplayTitle(model, theme)
}

/** Normalize pipeline steps for ExplanationPipeline. */
export function normalizePipelineSteps(steps) {
  if (!Array.isArray(steps)) return []
  return steps
    .map((step, index) => ({
      key: String(step?.key ?? step?.label ?? index),
      label: String(step?.label ?? '').trim(),
      active: Boolean(step?.active),
    }))
    .filter((step) => step.label)
}

/** Normalize field rows for ExplanationFieldGrid. */
export function normalizeExplanationFields(fields) {
  if (!Array.isArray(fields)) return []
  return fields.map((field, index) => ({
    key: String(field?.key ?? field?.label ?? index),
    label: String(field?.label ?? '').trim(),
    value: field?.value == null ? '' : String(field.value),
    source: field?.source == null ? '' : String(field.source).trim(),
    missing: Boolean(field?.missing),
    wide: Boolean(field?.wide),
    showTag: Boolean(field?.showTag),
    tooltip: field?.tooltip == null ? '' : String(field.tooltip).trim(),
  }))
}

export function isExplanationFieldWide(field) {
  return Boolean(field?.wide)
}

/** Map PortfolioProvenance origin display row → ExplanationFieldGrid fields. */
export function buildProvenanceOriginFields(origin, options = {}) {
  const o = origin && typeof origin === 'object' ? origin : {}
  const priceTooltip = String(options.signalPriceTooltip || '').trim()
  const fields = [
    {
      key: 'strategy',
      label: '来源策略',
      value: o.strategy,
      missing: Boolean(o.strategyMissing),
    },
    {
      key: 'signal_tag',
      label: '信号标签',
      value: o.signalTag,
      missing: Boolean(o.signalTagMissing),
      showTag: !o.signalTagMissing,
    },
    {
      key: 'signal_price',
      label: '信号价格',
      value: o.signalPrice,
      missing: Boolean(o.signalPriceMissing),
      tooltip: priceTooltip,
    },
    {
      key: 'signal_time',
      label: '信号时间',
      value: o.signalTime,
      missing: Boolean(o.signalTimeMissing),
    },
    {
      key: 'snapshot_id',
      label: 'snapshot_id',
      value: o.snapshotId,
      missing: Boolean(o.snapshotIdMissing),
    },
    {
      key: 'buy_reason',
      label: '发现依据',
      value: o.buyReason,
      missing: Boolean(o.buyReasonMissing),
      wide: true,
    },
  ]
  if (!o.selectionReasonMissing) {
    fields.push({
      key: 'selection_reason',
      label: '入选说明',
      value: o.selectionReason,
      missing: false,
      wide: true,
    })
  }
  return fields
}
