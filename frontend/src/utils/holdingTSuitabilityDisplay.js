/**
 * Phase17.1 — Holding T-Suitability display helpers (UI only).
 * Not a trade signal; no buy/sell actions.
 */

export const T_SUIT_LEVEL = {
  suitable: 'suitable',
  caution: 'caution',
  unsuitable: 'unsuitable',
}

export const T_SUIT_LEVEL_LABEL = {
  suitable: '可考虑',
  caution: '观察',
  unsuitable: '条件不足',
}

export const T_SUIT_REASON_LABEL = {
  LOCKED_POSITION: 'T+1 锁仓 / 不可卖',
  NO_SELLABLE: '无可卖数量',
  PRICE_STALE: '行情过期',
  LOW_VOLATILITY: '近窗波动偏低',
  VOLATILITY_UNKNOWN: '波动数据不足',
  HEALTH_WATCH: '健康等级 C · 增加观察',
  HEALTH_RISK: '健康等级 D · 风险提示',
  HEALTH_UNKNOWN: '暂无健康评分',
  PARTIAL_LOCKED: '部分仓位锁定（仍可卖）',
}

export function tSuitLevelLabel(level) {
  const k = String(level || '').trim().toLowerCase()
  return T_SUIT_LEVEL_LABEL[k] || '—'
}

export function tSuitLevelTagType(level) {
  const k = String(level || '').trim().toLowerCase()
  if (k === 'suitable') return 'success'
  if (k === 'caution') return 'warning'
  if (k === 'unsuitable') return 'error'
  return 'default'
}

export function tSuitLevelEmoji(level) {
  const k = String(level || '').trim().toLowerCase()
  if (k === 'suitable') return '🟢'
  if (k === 'caution') return '🟡'
  if (k === 'unsuitable') return '🔴'
  return ''
}

export function tSuitReasonLabelZH(code) {
  const k = String(code || '').trim()
  return T_SUIT_REASON_LABEL[k] || k || '—'
}

export function formatTSuitEvalTime(value) {
  if (value == null || value === '') return '—'
  const d = value instanceof Date ? value : new Date(String(value))
  if (Number.isNaN(d.getTime())) return String(value)
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function freshnessLabelZH(v) {
  const s = String(v || '').trim().toUpperCase()
  if (s === 'FRESH') return '新鲜'
  if (s === 'STALE') return '过期'
  if (s === 'UNKNOWN') return '未知'
  return s || '—'
}

export function volatilityLabelZH(v) {
  const s = String(v || '').trim().toUpperCase()
  if (s === 'ACTIVE') return '活跃'
  if (s === 'NORMAL') return '正常'
  if (s === 'LOW') return '偏低'
  if (s === 'UNKNOWN') return '未知'
  return s || '—'
}
