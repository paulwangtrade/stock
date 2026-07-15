/** 止/减 动态仓位比例（参考值，非投资建议） */

function clamp(n, min, max) {
  return Math.max(min, Math.min(max, n))
}

/** 四舍五入到 5% 档位 */
function roundPct01(p) {
  return Math.round(p * 20) / 20
}

function volMa(volumes, period, i) {
  if (!volumes?.length || i < period - 1) return null
  let s = 0
  for (let j = 0; j < period; j++) s += volumes[i - j] || 0
  return s / period
}

function peakRsiBefore(rsi, endIdx, lookback = 5) {
  let peak = rsi[endIdx]
  if (peak == null) return 70
  const start = Math.max(0, endIdx - lookback + 1)
  for (let j = start; j <= endIdx; j++) {
    const v = rsi[j]
    if (v != null && v > peak) peak = v
  }
  return peak
}

/**
 * 止：RSI 超买回落 — 前高 RSI 越高、单日回落越大 → 建议止盈比例越高
 * 范围约 20%～65%
 */
export function calcTakeProfitPositionPct(sig, barIndex, bars, options = {}) {
  const overbought = options.overbought ?? 70
  const rsi = sig?.rsi || []
  const closes = bars?.closes || []
  const ma20 = sig?.ma20 || []
  const i = barIndex
  if (i < 1 || rsi[i] == null || rsi[i - 1] == null) return 0.3

  const peak = peakRsiBefore(rsi, i - 1, 5)
  const drop = rsi[i - 1] - rsi[i]

  let pct = 0.22 + Math.min(0.33, Math.max(0, (peak - overbought) / 25) * 0.33)
  pct += Math.min(0.12, Math.max(0, drop - 2) / 10 * 0.12)

  const c = closes[i]
  const m = ma20[i]
  if (c != null && m != null && m > 0 && c < m) pct += 0.08

  return roundPct01(clamp(pct, 0.2, 0.65))
}

/**
 * 减：破 MA20 — 跌破幅度越大、放量越明显、MA20 下行 → 建议减仓比例越高
 * 范围约 30%～85%
 */
export function calcReducePositionPct(sig, barIndex, bars, options = {}) {
  const rsi = sig?.rsi || []
  const closes = bars?.closes || []
  const volumes = bars?.volumes || []
  const ma20 = sig?.ma20 || []
  const volPeriod = options.volPeriod ?? 5
  const i = barIndex
  if (i < 0 || !closes[i] || !ma20[i] || ma20[i] <= 0) return 0.4

  const breakPct = Math.max(0, (ma20[i] - closes[i]) / ma20[i])
  let pct = 0.32 + Math.min(0.38, (breakPct / 0.07) * 0.38)

  const vma = volMa(volumes, volPeriod, i)
  if (vma != null && vma > 0 && volumes[i]) {
    const vm = volumes[i] / vma
    if (vm >= 2) pct += 0.12
    else if (vm >= 1.4) pct += 0.07
  }

  if (i >= 5 && ma20[i] != null && ma20[i - 5] != null && ma20[i] < ma20[i - 5]) {
    pct += 0.08
  }

  const r = rsi[i]
  if (r != null) {
    if (r < 35) pct += 0.1
    else if (r < 45) pct += 0.05
  }

  return roundPct01(clamp(pct, 0.3, 0.85))
}

export function calcSellPositionPct(tag, sig, barIndex, bars, options = {}) {
  if (tag === '止') {
    return calcTakeProfitPositionPct(sig, barIndex, bars, options)
  }
  if (tag === '减') {
    return calcReducePositionPct(sig, barIndex, bars, options)
  }
  return null
}

export function formatSellPositionPct(pct) {
  if (pct == null || !Number.isFinite(pct)) return ''
  return `${Math.round(pct * 100)}%`
}

export function sellPositionHint(tag, pct) {
  if (pct == null || !Number.isFinite(pct)) return ''
  const s = formatSellPositionPct(pct)
  if (tag === '止') return `建议止盈 ${s}`
  if (tag === '减') return `建议减仓 ${s}`
  return ''
}
