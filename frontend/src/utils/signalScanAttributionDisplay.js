/**
 * Display helpers for signal-scan attribution (research observation only).
 * Missing horizons stay「数据不足」even if a numeric field leaked through.
 * DEFAULT_* matches signalSettings.js so this file stays runnable under node:test.
 */
const DEFAULT_SCREEN_STRATEGY_ID = 'default'
const DEFAULT_SCREEN_STRATEGY_NAME = '默认参数预设'

export const ATTRIBUTION_DISCLAIMER = '仅研究对照，不构成交易建议，不进入模拟交易计划'
export const RESEARCH_STAT_LABEL = '研究统计，不是收益证明'
export const INSUFFICIENT = '数据不足'
export const LARGE_SAMPLE_LIMIT = 100
export const LARGE_SAMPLE_WARNING = '样本过大，先收窄信号再归因'

const SIGNAL_TAG_LABELS = {
  XS_MOM_TOP: '截面动量',
  MA_TREND: '均线趋势',
  BREAKOUT_N: '突破观察',
}

const REASON_TEXT = {
  no_entry: '没有对照价',
  no_future_bar: '本地没有足够的后续交易日',
  calendar_gap: '日线之间有未确认的交易日缺口',
  bad_asof: '对照日无效',
}

export function horizonCellText(cell) {
  if (!cell || cell.status !== 'ok') return INSUFFICIENT
  const text = String(cell.text || '').trim()
  if (!text || text === INSUFFICIENT) return INSUFFICIENT
  return text
}

export function horizonReasonText(cell) {
  if (!cell || cell.status === 'ok') return ''
  return REASON_TEXT[cell.reason] || '本地日线不足，未计算涨跌'
}

export function horizonTitle(cell) {
  if (!cell || cell.status !== 'ok') return horizonReasonText(cell)
  const parts = []
  if (cell.futureDate) parts.push(String(cell.futureDate))
  if (cell.futureClose != null && cell.futureClose !== '') {
    const n = Number(cell.futureClose)
    if (Number.isFinite(n)) parts.push(`收盘 ${n.toFixed(2)}`)
  }
  return parts.join(' ')
}

export function strategyCell(row, labels) {
  let name = String(row?.strategyName || '').trim()
  let id = String(row?.strategyId || '').trim()
  if (!name && !id) id = DEFAULT_SCREEN_STRATEGY_ID
  const fromList = id && labels && typeof labels === 'object' ? String(labels[id] || '').trim() : ''
  if (!name && fromList) name = fromList
  if (!name && id === DEFAULT_SCREEN_STRATEGY_ID) name = DEFAULT_SCREEN_STRATEGY_NAME
  if (name && id && name !== id) return `${name}（${id}）`
  return name || id || '—'
}

export function buildAttributionSignalTagOptions(tags) {
  return (Array.isArray(tags) ? tags : [])
    .map((tag) => String(tag || '').trim())
    .filter(Boolean)
    .map((tag) => ({
      value: tag,
      label: SIGNAL_TAG_LABELS[tag] ? `${tag} ${SIGNAL_TAG_LABELS[tag]}` : tag,
    }))
}

export function displayName(name) {
  const s = String(name || '').trim()
  return s || '—'
}

export function horizonSummaryText(stat) {
  if (!stat || !Number(stat.complete)) return '完整样本 0'
  const mean = stat.meanText || '—'
  const median = stat.medianText || '—'
  return `完整 ${stat.complete} · 均值 ${mean} · 中位数 ${median}`
}

export function findHorizon(row, horizon) {
  const list = Array.isArray(row?.horizons) ? row.horizons : []
  return list.find((item) => Number(item?.horizon) === Number(horizon)) || null
}
