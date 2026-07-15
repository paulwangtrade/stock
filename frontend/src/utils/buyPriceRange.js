/** 买点标签 · 买入价（信号参考 + 次日/延后回踩区间，非投资建议） */

export const BUY_ENTRY_TAGS = new Set(['强', '趋', '转', '突', '买', '弹'])

const MAX_RANGE_PCT = 0.018
const DEFER_RANGE_PCT = 0.022
/** 信号当日：次日区间上沿相对出信号价的小幅高开容忍 */
const TOMORROW_HEADROOM_PCT = 0.008

export function formatPriceTick(n) {
  if (!Number.isFinite(n)) return '—'
  const abs = Math.abs(n)
  if (abs >= 100) return n.toFixed(2)
  if (abs >= 1) return n.toFixed(2)
  return n.toFixed(3)
}

export function formatBuyPriceRangeText(range) {
  if (!range) return '—'
  if (range.text) return range.text
  if (range.instantText) return range.instantText
  return '—'
}

export function buyPriceRangeLabel(range) {
  if (!range) return '买入区间'
  if (range.daysAgo === 0) {
    return range.deferMode === 'wait' ? '明日参考' : '今/明日参考'
  }
  if (range.daysAgo > 0) return '延后参考'
  return '买入区间'
}

/** 上沿锚定在 hi，只从下方收窄区间 */
function tightenRangeFromHigh(lo, hi, maxPct = MAX_RANGE_PCT) {
  if (!Number.isFinite(lo) || !Number.isFinite(hi)) return null
  lo = Math.min(lo, hi)
  if (lo <= 0 || hi <= 0 || lo >= hi) return null
  const maxSpan = hi * maxPct
  if (hi - lo > maxSpan) lo = hi - maxSpan
  if (lo >= hi) return null
  return { low: lo, high: hi }
}

function smaAt(closes, idx, period = 20) {
  if (!closes?.length || idx < period - 1) return null
  let s = 0
  for (let j = 0; j < period; j++) s += closes[idx - j]
  return s / period
}

function tomorrowRangeHigh(instantPrice) {
  return instantPrice * (1 + TOMORROW_HEADROOM_PCT)
}

/** 强/突 用确认完成日；其余用标信号日 */
function resolveInstantBarIndex(summary) {
  const tag = summary?.tag
  const confirmBar = summary.recentSignalConfirmBar
  const sigBar = summary.recentSignalBar
  if ((tag === '强' || tag === '突') && confirmBar != null && confirmBar >= 0) {
    return confirmBar
  }
  return sigBar
}

function priceStatus(nowClose, instantPrice, rangeHigh) {
  const hi = rangeHigh ?? instantPrice
  if (nowClose == null || instantPrice == null || !Number.isFinite(instantPrice) || instantPrice <= 0) {
    return 'unknown'
  }
  if (nowClose >= instantPrice * 0.992 && nowClose <= hi * 1.002) return 'inZone'
  if (nowClose > hi * 1.005) return 'above'
  if (nowClose < instantPrice * 0.985) return 'below'
  return 'near'
}

function rangeText(range) {
  if (!range) return null
  return `${formatPriceTick(range.low)}~${formatPriceTick(range.high)}`
}

/** 信号后仍可考虑的回踩上沿 */
function calcDeferRange(summary, bars, instantBar, instantPrice, sigLow, last, rangeHigh) {
  const lows = bars?.lows ?? []
  const daysAgo = summary.recentSignalDaysAgo ?? Math.max(0, last - instantBar)
  const ma20 = smaAt(bars?.closes ?? [], last, 20)
  const hi = rangeHigh ?? instantPrice

  let lo = sigLow
  for (let i = instantBar; i <= last; i++) {
    const l = lows[i]
    if (Number.isFinite(l)) lo = Math.min(lo, l)
  }
  if (ma20 != null && ma20 > 0) lo = Math.min(lo, ma20)

  const maxPct = daysAgo === 0 ? DEFER_RANGE_PCT : MAX_RANGE_PCT
  return tightenRangeFromHigh(lo, hi, maxPct)
}

/**
 * 买入价 = 出信号瞬间价（该 K 线收盘价）
 * 信号当日：展示区间上沿含次日小幅高开容忍，明日仍可落入区间买入
 */
export function calcBuyPriceRange(summary, bars, _options = {}) {
  const tag = summary?.tag
  if (!BUY_ENTRY_TAGS.has(tag)) return null

  const closes = bars?.closes ?? []
  const lows = bars?.lows ?? []
  const opens = bars?.opens ?? []
  const last = summary.signalLastIndex ?? closes.length - 1
  if (last < 0 || closes[last] == null) return null

  const instantBar = resolveInstantBarIndex(summary)
  if (instantBar == null || instantBar < 0 || instantBar > last || closes[instantBar] == null) {
    return null
  }

  const instantPrice = closes[instantBar]
  const sigLow = lows[instantBar] ?? Math.min(opens[instantBar] ?? instantPrice, instantPrice)
  const nowClose = closes[last]
  const daysAgo = summary.recentSignalDaysAgo ?? Math.max(0, last - instantBar)

  const rangeHigh = daysAgo === 0 ? tomorrowRangeHigh(instantPrice) : instantPrice
  const idealPullback = tightenRangeFromHigh(sigLow, instantPrice)
  const displayPullback = tightenRangeFromHigh(sigLow, rangeHigh)
  const deferPullback = calcDeferRange(summary, bars, instantBar, instantPrice, sigLow, last, rangeHigh)
  const status = priceStatus(nowClose, instantPrice, rangeHigh)

  const idealText = rangeText(idealPullback)
  const displayTextRaw = rangeText(displayPullback)
  const deferText = rangeText(deferPullback)

  let displayText = daysAgo === 0 ? displayTextRaw : idealText
  let deferMode = 'same'
  let statusHint = ''

  if (status === 'above') {
    deferMode = 'wait'
    displayText = deferText || displayTextRaw || idealText
    statusHint =
      daysAgo === 0
        ? `今日可不追，明日等回踩 ${displayText || '区间附近'}`
        : `现价偏高，等回踩 ${displayText || '区间附近'}`
  } else if (status === 'below') {
    deferMode = daysAgo === 0 ? 'todayOrTomorrow' : 'defer'
    displayText = deferText || displayTextRaw || idealText
    statusHint = `现价${formatPriceTick(nowClose)}低于出信号价，区间仍有效`
  } else if (status === 'inZone' || status === 'near') {
    deferMode = daysAgo === 0 ? 'todayOrTomorrow' : 'same'
    displayText = displayTextRaw || idealText || deferText
    statusHint =
      daysAgo === 0
        ? `明日仍可买，价格落入 ${displayText || '参考区间'} 即可`
        : '现价接近出信号价'
  } else if (daysAgo > 0 && deferText) {
    deferMode = 'defer'
    displayText = deferText
    statusHint = `${daysAgo}日前出信号，仍可等回踩该区间`
  }

  const tagNote =
    tag === '强' || tag === '突'
      ? '出信号价=确认完成当日收盘价'
      : '出信号价=标信号当日收盘价'

  const instantText =
    status === 'above'
      ? `${formatPriceTick(instantPrice)} ↑`
      : status === 'below'
        ? `${formatPriceTick(instantPrice)} ↓`
        : formatPriceTick(instantPrice)

  const ma20 = smaAt(closes, last, 20)
  const tomorrowHighNote =
    daysAgo === 0 && rangeHigh > instantPrice
      ? `明日上限≈${formatPriceTick(rangeHigh)}（出信号价+0.8%）`
      : ''

  return {
    tag,
    daysAgo,
    instantPrice,
    rangeHigh,
    instantBar,
    signalClose: instantPrice,
    signalLow: sigLow,
    low: displayPullback?.low ?? deferPullback?.low ?? sigLow,
    high: displayPullback?.high ?? rangeHigh,
    text: displayText,
    signalRefText: idealText,
    deferText,
    instantText,
    mode: status,
    deferMode,
    extended: status === 'above',
    note: [
      tagNote,
      idealText ? `理想贴近 ${idealText}` : '',
      tomorrowHighNote,
      deferText && deferText !== displayTextRaw && deferText !== idealText ? `深回踩 ${deferText}` : '',
      ma20 != null ? `MA20≈${formatPriceTick(ma20)}` : '',
      statusHint,
    ]
      .filter(Boolean)
      .join(' · '),
  }
}
