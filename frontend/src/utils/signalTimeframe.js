/**
 * Phase17.4 — Signal Timeframe Contract（前端内存契约，无 DB 字段）
 * 冰点族默认仅日 K；不改变 computeFullSignals 算法。
 */

/** @typedef {'daily'|'weekly'|'monthly'|'quarterly'|'yearly'|'intraday'|'unknown'} SignalTimeframe */

/** 现网冰点/买卖点 Provider 描述（展示门闩用） */
export const ICE_SIGNAL_DESCRIPTOR = Object.freeze({
  id: 'ice_reversal',
  name: '冰点/买卖点',
  /** @type {SignalTimeframe} */
  timeframe: 'daily',
  timeframeCompat: Object.freeze(['daily']),
})

/**
 * 东财 klt → 规范 timeframe
 * @param {string} klt
 * @returns {SignalTimeframe}
 */
export function kltToSignalTimeframe(klt) {
  switch (String(klt || '').trim()) {
    case '101':
      return 'daily'
    case '102':
      return 'weekly'
    case '103':
      return 'monthly'
    case '104':
      return 'quarterly'
    case '106':
      return 'yearly'
    case '1':
    case '5':
    case '15':
    case '30':
    case '60':
    case '120':
      return 'intraday'
    default:
      return 'unknown'
  }
}

/**
 * @param {{ timeframe?: string, timeframeCompat?: string[] } | null | undefined} signal
 * @param {SignalTimeframe|string} chartTimeframe
 */
export function isSignalCompatibleWithChartTimeframe(signal, chartTimeframe) {
  const chart = String(chartTimeframe || '').trim()
  if (!chart || chart === 'unknown') return false
  const compat = Array.isArray(signal?.timeframeCompat) && signal.timeframeCompat.length
    ? signal.timeframeCompat.map(String)
    : [String(signal?.timeframe || '')].filter(Boolean)
  return compat.includes(chart)
}

/** 冰点 markers 是否应在当前 klt 显示 */
export function isIceSignalVisibleOnKlt(klt) {
  return isSignalCompatibleWithChartTimeframe(ICE_SIGNAL_DESCRIPTOR, kltToSignalTimeframe(klt))
}

export const ICE_SIGNAL_UNSUPPORTED_HINT = '该信号仅支持日K周期'
