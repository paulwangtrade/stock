/**
 * Phase17.6 Holding T Signal v0 — frontend mirror of BuildHoldingTSignal.
 * Observation WATCH only; no orders. Keep thresholds aligned with
 * backend/papertrading/holding_t_signal.go.
 */

export const T_SIGNAL_BUY_WATCH = 'T_BUY_WATCH'
export const T_SIGNAL_SELL_WATCH = 'T_SELL_WATCH'

export const T_SIGNAL_LEVEL_ACTIVE = 'active'
export const T_SIGNAL_LEVEL_SOFT = 'soft'

export const T_SIGNAL_REASON = Object.freeze({
  NO_POSITION: 'NO_POSITION',
  CANNOT_SELL: 'CANNOT_SELL',
  PRICE_STALE: 'PRICE_STALE',
  NO_5M_BARS: 'NO_5M_BARS',
  INVALID_COST: 'INVALID_COST',
  SHORT_PULLBACK: 'SHORT_PULLBACK',
  BOUNCE_HINT: 'BOUNCE_HINT',
  NEAR_COST: 'NEAR_COST',
  SHORT_RISE: 'SHORT_RISE',
  AWAY_FROM_COST: 'AWAY_FROM_COST',
  MOMENTUM_FADE: 'MOMENTUM_FADE',
  SUITABILITY_UNSUITABLE: 'SUITABILITY_UNSUITABLE',
  HEALTH_CAUTION: 'HEALTH_CAUTION',
})

const MIN_BARS = 6
const NEAR_COST_MAX = 0.012
const AWAY_COST_MIN = 0.008
const PULLBACK_MIN = 0.004
const RISE_MIN = 0.005
const BOUNCE_MIN = 0.002
const LOOKBACK = 8

/**
 * @param {object} in_
 * @returns {{ signals: object[], notes: string[] }}
 */
export function buildHoldingTSignal(in_ = {}) {
  const notes = []
  const signals = []
  const code = String(in_.stockCode || in_.stock_code || '').trim()
  const posID = String(in_.positionId || in_.position_id || '').trim()
  const hasPos = !!(in_.hasPosition || in_.has_position || Number(in_.availableQty || in_.available_qty) > 0)
  if (!hasPos) {
    notes.push(T_SIGNAL_REASON.NO_POSITION)
    return { signals, notes }
  }

  const fresh = String(in_.freshness || 'UNKNOWN').toUpperCase()
  if (fresh === 'STALE') {
    notes.push(T_SIGNAL_REASON.PRICE_STALE)
    return { signals, notes }
  }

  const rawBars = Array.isArray(in_.bars) ? in_.bars : []
  const bars = rawBars
    .map((b) => ({
      time: String(b.time || b.day || b.Time || ''),
      high: Number(b.high ?? b.High ?? b.close ?? b.Close),
      low: Number(b.low ?? b.Low ?? b.close ?? b.Close),
      close: Number(b.close ?? b.Close),
    }))
    .filter((b) => Number.isFinite(b.close) && b.close > 0)

  if (bars.length < MIN_BARS) {
    notes.push(T_SIGNAL_REASON.NO_5M_BARS)
    return { signals, notes }
  }

  const cost = Number(in_.costPrice ?? in_.cost_price)
  if (!(cost > 0)) {
    notes.push(T_SIGNAL_REASON.INVALID_COST)
    return { signals, notes }
  }

  let canSell = !!(in_.canSell ?? in_.can_sell)
  const avail = Number(in_.availableQty ?? in_.available_qty)
  if (Number.isFinite(avail) && avail <= 0 && !canSell) canSell = false

  const last = bars[bars.length - 1]
  const px = last.close
  const sigTime = (last.time || '').trim() || new Date().toISOString()

  const suit = String(in_.suitabilityLevel || in_.suitability_level || '').toLowerCase()
  const grade = String(in_.healthGrade || in_.health_grade || '').toUpperCase()

  let baseConf = 0.55
  if (fresh === 'UNKNOWN') baseConf -= 0.08
  if (suit === 'caution') baseConf -= 0.08
  if (suit === 'unsuitable') baseConf -= 0.2
  if (grade === 'C' || grade === 'D') baseConf -= 0.08
  if (!grade) baseConf -= 0.04

  const win = bars.length > LOOKBACK ? bars.slice(-LOOKBACK) : bars
  let localHigh = win[0].high
  let localLow = win[0].low
  for (const b of win) {
    if (b.high > localHigh) localHigh = b.high
    if (b.low < localLow || !(localLow > 0)) localLow = b.low
  }
  const pullback = localHigh > 0 ? (localHigh - px) / localHigh : 0
  const riseFromLow = localLow > 0 ? (px - localLow) / localLow : 0
  const distCost = (px - cost) / cost
  const absDistCost = Math.abs(distCost)

  const prev = bars[bars.length - 2]
  const bounce =
    px > prev.low &&
    (px >= prev.close || (prev.low > 0 && (px - prev.low) / prev.low >= BOUNCE_MIN))

  let momFade = false
  if (riseFromLow >= RISE_MIN * 0.8) {
    if (px < prev.close) momFade = true
    if (prev.high > prev.low && px < (prev.high + prev.low) / 2) momFade = true
  }

  if (canSell) {
    const buyReasons = []
    let buyScore = 0
    if (pullback >= PULLBACK_MIN) {
      buyReasons.push(T_SIGNAL_REASON.SHORT_PULLBACK)
      buyScore++
    }
    if (bounce) {
      buyReasons.push(T_SIGNAL_REASON.BOUNCE_HINT)
      buyScore++
    }
    if (absDistCost <= NEAR_COST_MAX) {
      buyReasons.push(T_SIGNAL_REASON.NEAR_COST)
      buyScore++
    }
    if (buyScore >= 2 && distCost <= AWAY_COST_MIN + 0.004) {
      if (suit === 'unsuitable') buyReasons.push(T_SIGNAL_REASON.SUITABILITY_UNSUITABLE)
      if (grade === 'C' || grade === 'D') buyReasons.push(T_SIGNAL_REASON.HEALTH_CAUTION)
      const conf = clamp01(baseConf + 0.12 * (buyScore - 1))
      const level = buyScore >= 3 && conf >= 0.55 ? T_SIGNAL_LEVEL_ACTIVE : T_SIGNAL_LEVEL_SOFT
      signals.push({
        stock_code: code,
        position_id: posID,
        signal_type: T_SIGNAL_BUY_WATCH,
        signal_time: sigTime,
        signal_price: px,
        level,
        reasons: buyReasons,
        confidence: conf,
      })
    }
  } else {
    notes.push(T_SIGNAL_REASON.CANNOT_SELL)
  }

  const sellReasons = []
  let sellScore = 0
  if (riseFromLow >= RISE_MIN || (prev.close > 0 && (px - prev.close) / prev.close >= RISE_MIN * 0.6)) {
    sellReasons.push(T_SIGNAL_REASON.SHORT_RISE)
    sellScore++
  }
  if (distCost >= AWAY_COST_MIN) {
    sellReasons.push(T_SIGNAL_REASON.AWAY_FROM_COST)
    sellScore++
  }
  if (momFade) {
    sellReasons.push(T_SIGNAL_REASON.MOMENTUM_FADE)
    sellScore++
  }
  if (sellScore >= 2) {
    if (suit === 'unsuitable') sellReasons.push(T_SIGNAL_REASON.SUITABILITY_UNSUITABLE)
    if (grade === 'C' || grade === 'D') sellReasons.push(T_SIGNAL_REASON.HEALTH_CAUTION)
    const conf = clamp01(baseConf + 0.12 * (sellScore - 1))
    const level = sellScore >= 3 && conf >= 0.55 ? T_SIGNAL_LEVEL_ACTIVE : T_SIGNAL_LEVEL_SOFT
    signals.push({
      stock_code: code,
      position_id: posID,
      signal_type: T_SIGNAL_SELL_WATCH,
      signal_time: sigTime,
      signal_price: px,
      level,
      reasons: sellReasons,
      confidence: conf,
    })
  }

  return { signals, notes }
}

/**
 * Map HoldingTSignal → ChartMarker (BUY/SELL) for HoldingT SVG only.
 * @param {object[]} signals
 * @returns {import('./chartMarkers.js').ChartMarker[]}
 */
export function holdingTSignalsToChartMarkers(signals) {
  return (Array.isArray(signals) ? signals : []).map((s) => {
    const typ = String(s.signal_type || s.signalType || '')
    const type = typ === T_SIGNAL_SELL_WATCH ? 'SELL' : 'BUY'
    const reasons = Array.isArray(s.reasons) ? s.reasons.join(',') : ''
    const timeRaw = String(s.signal_time || s.signalTime || '')
    const hhmm = timeRaw.match(/\d{1,2}:\d{2}/)?.[0] || timeRaw.slice(0, 5)
    return {
      time: hhmm,
      price: Number(s.signal_price ?? s.signalPrice) || 0,
      type,
      reason: reasons,
    }
  })
}

function clamp01(v) {
  if (!(v >= 0)) return 0
  if (v > 1) return 1
  return Math.round(v * 100) / 100
}
