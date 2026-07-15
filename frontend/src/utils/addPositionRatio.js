/** 加仓动态比例（参考值，须已有持仓且浮盈，非投资建议） */

function clamp(n, min, max) {
  return Math.max(min, Math.min(max, n))
}

function roundPct01(p) {
  return Math.round(p * 20) / 20
}

/**
 * @param {'强'|'趋'|'突'} sourceTag
 * @returns {number} 0.15 ~ 0.35
 */
export function calcAddPositionPct(sourceTag, barIndex, bars, sig, options = {}) {
  const closes = bars?.closes ?? []
  const rsi = sig?.rsi ?? []
  const ma20 = sig?.ma20 ?? []
  const i = barIndex
  if (i < 0 || !closes[i]) return 0.25

  let pct = sourceTag === '强' ? 0.28 : sourceTag === '突' ? 0.24 : 0.22

  const r = rsi[i]
  if (r != null) {
    if (r >= 60) pct -= 0.04
    else if (r <= 48) pct += 0.03
  }

  const c = closes[i]
  const m = ma20[i]
  if (c != null && m != null && m > 0) {
    const above = (c - m) / m
    if (above >= 0.06) pct -= 0.03
    else if (above <= 0.015) pct += 0.02
  }

  const cap = options.addMaxPct ?? 0.35
  const floor = options.addMinPct ?? 0.15
  return roundPct01(clamp(pct, floor, cap))
}

export function formatAddPositionPct(pct) {
  if (pct == null || !Number.isFinite(pct)) return ''
  const n = Math.round(pct * 100)
  if (n <= 0) return ''
  return `${n}%`
}

export function addPositionHint(pct) {
  const s = formatAddPositionPct(pct)
  if (!s) return ''
  return `建议加仓 ${s}`
}
