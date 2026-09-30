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

export const WHATIF_TITLE = '假设沙盘'
export const WHATIF_NOTE_FALLBACK = '对照实验，不是买卖指令。不写入交易计划，也不下单。'
export const WHATIF_SMALL_SAMPLE = '假设子集不足 8 只，差额不稳定，不能当成规律。'
export const WHATIF_IN_SAMPLE_MARK = '样本内'
export const WHATIF_TOGGLE = '纳入假设'
export const WHATIF_DIFF_LABEL = '差额 · 假设减基线'
export const WHATIF_BASE_FALLBACK = '基线 · 全部命中等权'
export const WHATIF_SCENARIO_FALLBACK = '假设 · 勾选特征等权'

const WHATIF_COLUMNS = [
  { key: '1', label: '+1 交易日', inSample: true },
  { key: '3', label: '+3 交易日', inSample: false },
  { key: '10', label: '+10 交易日', inSample: false },
  { key: 'toDate', label: '迄今', inSample: false },
]

export function whatIfColumns() {
  return WHATIF_COLUMNS.map((item) => ({ ...item }))
}

export function whatIfNote(panel) {
  const note = String(panel?.note || '').trim()
  return note || WHATIF_NOTE_FALLBACK
}

export function whatIfArmLabel(arm, fallback) {
  const label = String(arm?.label || '').trim()
  return label || fallback
}

export function whatIfSlotStat(arm, key) {
  if (!arm) return null
  if (key === 'toDate') return arm.toDate || null
  const list = Array.isArray(arm.horizons) ? arm.horizons : []
  return list.find((item) => String(item?.horizon) === String(key)) || null
}

function finiteMean(stat) {
  if (!stat || !Number(stat.complete)) return null
  if (stat.mean === null || stat.mean === undefined || stat.mean === '') return null
  const n = Number(stat.mean)
  return Number.isFinite(n) ? n : null
}

export function formatResearchPct(rate) {
  if (!Number.isFinite(rate)) return INSUFFICIENT
  const pct = rate * 100
  const text = `${Math.abs(pct).toFixed(2)}%`
  if (pct < 0) return `-${text}`
  return `+${text}`
}

export function whatIfReturnText(stat) {
  if (!stat || !Number(stat.complete)) return INSUFFICIENT
  const mean = stat.meanText || '—'
  return `完整 ${stat.complete} · 等权 ${mean}`
}

export function whatIfDiffRate(baseStat, scenarioStat) {
  const base = finiteMean(baseStat)
  const scenario = finiteMean(scenarioStat)
  if (base == null || scenario == null) return null
  return scenario - base
}

export function whatIfDiffText(baseStat, scenarioStat) {
  const rate = whatIfDiffRate(baseStat, scenarioStat)
  if (rate == null) return INSUFFICIENT
  return formatResearchPct(rate)
}

export function whatIfCountDiff(baseline, scenario) {
  const base = Number(baseline?.hitCount)
  const next = Number(scenario?.hitCount)
  if (!Number.isFinite(base) || !Number.isFinite(next)) return '—'
  const diff = next - base
  if (diff > 0) return `+${diff}`
  return String(diff)
}
