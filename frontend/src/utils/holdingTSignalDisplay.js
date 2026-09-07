/**
 * Phase17.6 — Holding T Signal display labels (observation only).
 */

import {
  T_SIGNAL_BUY_WATCH,
  T_SIGNAL_SELL_WATCH,
  T_SIGNAL_REASON,
} from './holdingTSignal.js'

const REASON_ZH = {
  [T_SIGNAL_REASON.NO_POSITION]: '无持仓',
  [T_SIGNAL_REASON.CANNOT_SELL]: '当前不可卖（T+1/锁定）',
  [T_SIGNAL_REASON.PRICE_STALE]: '行情过期',
  [T_SIGNAL_REASON.NO_5M_BARS]: '缺少 5 分钟 K 线',
  [T_SIGNAL_REASON.INVALID_COST]: '成本价无效',
  [T_SIGNAL_REASON.SHORT_PULLBACK]: '5 分钟短线回撤',
  [T_SIGNAL_REASON.BOUNCE_HINT]: '波动恢复/反弹迹象',
  [T_SIGNAL_REASON.NEAR_COST]: '接近成本区域',
  [T_SIGNAL_REASON.SHORT_RISE]: '5 分钟上涨',
  [T_SIGNAL_REASON.AWAY_FROM_COST]: '偏离成本',
  [T_SIGNAL_REASON.MOMENTUM_FADE]: '短线动能减弱',
  [T_SIGNAL_REASON.SUITABILITY_UNSUITABLE]: '适宜性偏弱（仅观察）',
  [T_SIGNAL_REASON.HEALTH_CAUTION]: '健康等级需关注',
}

export function tSignalTypeLabelZH(type) {
  const t = String(type || '')
  if (t === T_SIGNAL_BUY_WATCH) return 'T买观察'
  if (t === T_SIGNAL_SELL_WATCH) return 'T卖观察'
  return t || '—'
}

export function tSignalReasonLabelZH(code) {
  const c = String(code || '')
  return REASON_ZH[c] || c || '—'
}

export function tSignalLevelLabelZH(level) {
  const l = String(level || '').toLowerCase()
  if (l === 'active') return '较强'
  if (l === 'soft') return '偏弱'
  return l || '—'
}

export function formatTSignalConfidence(c) {
  const n = Number(c)
  if (!Number.isFinite(n)) return '—'
  return `${Math.round(n * 100)}%`
}
