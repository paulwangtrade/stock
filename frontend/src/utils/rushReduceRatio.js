/** 早减动态仓位比例（冲高回落提前减仓，参考值，非投资建议） */

function clamp(n, min, max) {
  return Math.max(min, Math.min(max, n))
}

function roundPct01(p) {
  return Math.round(p * 20) / 20
}

/**
 * 早减：阳后阴 / 冲高回落，建议减仓比例约 25%～55%
 */
export function calcRushReducePct(barIndex, bars, sig, options = {}) {
  const closes = bars?.closes ?? []
  const opens = bars?.opens ?? []
  const highs = bars?.highs ?? []
  const rsi = sig?.rsi ?? []
  const i = barIndex
  if (i < 1 || !closes[i]) return 0.35

  const c = closes[i]
  const cPrev = closes[i - 1]
  let pct = 0.32

  if (cPrev > 0) {
    const drop = (cPrev - c) / cPrev
    pct += Math.min(0.15, drop * 2)
  }

  const rPrev = rsi[i - 1]
  if (rPrev != null) {
    if (rPrev >= 72) pct += 0.1
    else if (rPrev >= 65) pct += 0.06
  }

  const hPrev = highs[i - 1] ?? cPrev
  const oPrev = opens[i - 1]
  if (hPrev > 0 && oPrev != null && cPrev > oPrev) {
    const upperWick = (hPrev - cPrev) / (hPrev - Math.min(oPrev, cPrev) || hPrev)
    if (upperWick >= 0.35) pct += 0.08
  }

  const cap = options.rushReduceMaxPct ?? 0.55
  const floor = options.rushReduceMinPct ?? 0.25
  return roundPct01(clamp(pct, floor, cap))
}

export function formatRushReducePct(pct) {
  if (pct == null || !Number.isFinite(pct)) return ''
  const n = Math.round(pct * 100)
  if (n <= 0) return ''
  return `${n}%`
}

export function rushReduceHint(pct) {
  const s = formatRushReducePct(pct)
  if (!s) return ''
  return `建议早减 ${s}`
}
