/**
 * 入场策略当前未命中时的只读观察：近 N 个交易日曾命中的日期与收盘价，
 * 以及当前 K 线能否收成唯一参考价。
 *
 * 不是买卖指令，不写 TradePlan。
 * 路径依赖或截面条件无法收成一个价时，固定写「无法给出唯一价」。
 */
import {
  computeBreakoutSignals,
  computeFullSignals,
  computeReboundAfterSell,
  computeTradeSignals,
  pickPrimaryRecentSignal,
} from './icePointSignals.js'

/** 回看交易日数。约 20–60，默认 40。调用方可传入别的正整数。 */
export const ENTRY_MISS_LOOKBACK_DAYS = 40

export const ENTRY_MISS_NO_UNIQUE_PRICE = '无法给出唯一价'

export const ENTRY_MISS_FOOTER =
  '未命中观察只回看近端曾命中的日期与收盘价；参考价是当前 K 线的观察门槛，不是买卖指令。'

const NO_UNIQUE_PRICE_BY_TAG = {
  强: '无法给出唯一价（冰点路径与指数环境）',
  趋: '无法给出唯一价（回踩、收阳与 RSI 绑在同一根 K 线）',
  买: '无法给出唯一价（RSI 上穿路径）',
  转: '无法给出唯一价（RSI 与均线下占比无法收成唯一价）',
}

function resolveLookback(n) {
  const v = Math.floor(Number(n))
  if (!Number.isFinite(v) || v <= 0) return ENTRY_MISS_LOOKBACK_DAYS
  return v
}

function smaAt(closes, period, index) {
  if (index < period - 1) return null
  let s = 0
  for (let j = 0; j < period; j++) {
    const v = closes[index - j]
    if (v == null || !Number.isFinite(v)) return null
    s += v
  }
  return s / period
}

function roundPrice(v) {
  return Math.round(Number(v) * 100) / 100
}

/** strict：收盘须高于该价；否则收盘不低于该价。按 0.01 取整。 */
function quotePrice(raw, strict) {
  if (!Number.isFinite(raw)) return null
  const cents = raw * 100
  const px = strict ? Math.floor(cents + 1e-6) / 100 + 0.01 : Math.ceil(cents - 1e-6) / 100
  if (!(px > 0)) return null
  return roundPrice(px)
}

function unavailable(text) {
  return {
    priceMode: 'unavailable',
    triggerPrice: null,
    priceText: text || ENTRY_MISS_NO_UNIQUE_PRICE,
  }
}

function priceGate(trigger, close) {
  const shown = roundPrice(trigger)
  const px = Number(close)
  if (!(shown > 0) || !(px > 0)) return unavailable()
  const distancePct = (shown - px) / px
  const abs = Math.abs(distancePct * 100).toFixed(2)
  let gap = '与参考价持平'
  if (distancePct > 1e-8) gap = `还差 ${abs}%`
  else if (distancePct < -1e-8) gap = `收盘已高于参考价 ${abs}%`
  return {
    priceMode: 'price',
    triggerPrice: shown,
    priceText: `参考价 ${shown.toFixed(2)} · ${gap}`,
  }
}

function formatHits(hits, lookbackDays) {
  if (!hits.length) return `近${lookbackDays}个交易日无命中`
  const shown = hits.slice(0, 12)
  const parts = shown.map((item) => {
    const date = item.date || '日期缺失'
    const price = Number.isFinite(Number(item.price)) ? Number(item.price).toFixed(2) : '—'
    return `${date} ${price}`
  })
  const more = hits.length > shown.length ? ` 等${hits.length}次` : ''
  return `近${lookbackDays}个交易日命中：${parts.join('、')}${more}`
}

function indicesForTag(sig, tag) {
  if (!sig) return []
  if (tag === '强') return sig.strictBuy || []
  if (tag === '趋') return sig.trendBuy || []
  if (tag === '转') return sig.reversalBuy || []
  if (tag === '突') return sig.breakoutBuy || []
  if (tag === '弹') return sig.reboundBuy || []
  if (tag === '买') return sig.buy || []
  return []
}

/** 对照里「命中」落在确认日，而不是信号数组里更早的那一根。 */
function verdictDay(tag, index, options) {
  if (tag === '强') {
    const confirm = Math.max(0, Math.floor(Number(options?.strongConfirmDays ?? 2) || 0))
    return index + confirm
  }
  if (tag === '突') {
    const confirm = Math.max(0, Math.floor(Number(options?.breakoutConfirmDays ?? options?.confirmDays ?? 3) || 0))
    return index + confirm
  }
  return index
}

function hitFromDay(bars, day) {
  const price = bars?.closes?.[day]
  if (!Number.isFinite(Number(price))) return null
  return {
    date: String(bars?.dayKeys?.[day] || ''),
    price: Number(price),
    index: day,
  }
}

function collectTagHits(sig, tag, bars, last, lookbackDays, options) {
  const from = Math.max(0, last - lookbackDays + 1)
  const seen = new Set()
  const hits = []
  for (const index of indicesForTag(sig, tag)) {
    const day = verdictDay(tag, index, options)
    if (day < from || day >= last || seen.has(day)) continue
    const hit = hitFromDay(bars, day)
    if (!hit) continue
    seen.add(day)
    hits.push(hit)
  }
  hits.sort((a, b) => b.index - a.index)
  return hits
}

function collectCompositeHits(sig, bars, last, lookbackDays, options) {
  const from = Math.max(0, last - lookbackDays + 1)
  const iceThreshold = Number(options?.iceThreshold ?? 30)
  const read = {
    recentBuyDays: 0,
    recentSellDays: 0,
    includeSell: true,
    strongConfirmDays: options?.strongConfirmDays ?? 2,
    breakoutConfirmDays: options?.breakoutConfirmDays ?? options?.confirmDays ?? 3,
  }
  const hits = []
  for (let day = last - 1; day >= from; day--) {
    const primary = pickPrimaryRecentSignal(sig, day, read)
    const tagged = primary && primary.daysAgo === 0 && primary.tag && primary.tag !== '冰'
    const rsi = sig?.rsi?.[day]
    const inIce = rsi != null && Number(rsi) < iceThreshold
    if (!tagged && !inIce) continue
    const hit = hitFromDay(bars, day)
    if (hit) hits.push(hit)
  }
  return hits
}

function boxBefore(bars, breakIdx, boxPeriod) {
  const lookStart = breakIdx - boxPeriod
  const lookEnd = breakIdx - 1
  if (lookStart < 0) return null
  const opens = bars.opens || []
  const highs = bars.highs || []
  const lows = bars.lows || []
  const closes = bars.closes || []
  let boxHigh = -Infinity
  let boxLow = Infinity
  for (let j = lookStart; j <= lookEnd; j++) {
    const c = closes[j]
    const o = opens[j] ?? c
    const h = highs[j] ?? (o != null && c != null ? Math.max(o, c) : c)
    const l = lows[j] ?? (o != null && c != null ? Math.min(o, c) : c)
    if (h == null || l == null || !Number.isFinite(h) || !Number.isFinite(l)) return null
    boxHigh = Math.max(boxHigh, h)
    boxLow = Math.min(boxLow, l)
  }
  return { boxHigh, boxLow }
}

/**
 * 突：当前 K 线若作为突破日，收盘须站上「不含当日的箱体上沿 × (1+缓冲)」。
 * 箱体不成立，或近端已有突破占着间隔，则无法给出唯一价。
 */
function breakoutCurrentGate(bars, options) {
  const closes = bars?.closes || []
  const i = closes.length - 1
  const boxPeriod = Math.max(2, Math.floor(Number(options?.boxPeriod) || 20))
  const maxRangePct = Number(options?.maxRangePct ?? 0.2)
  const breakBuffer = Number(options?.breakBuffer ?? 0.005)
  const box = boxBefore(bars, i, boxPeriod)
  if (!box) return unavailable()
  const mid = (box.boxHigh + box.boxLow) / 2
  if (!(mid > 0) || !(box.boxHigh > box.boxLow)) return unavailable()
  if ((box.boxHigh - box.boxLow) / mid > maxRangePct) return unavailable()

  const minGap = Math.max(1, Math.floor(Number(options?.minGap) || 15))
  const confirmed = computeBreakoutSignals(
    {
      closes,
      opens: bars.opens || [],
      highs: bars.highs || [],
      lows: bars.lows || [],
      volumes: bars.volumes || [],
    },
    options || {},
  )
  for (const b of confirmed) {
    if (b < i && i - b < minGap) return unavailable()
  }

  const buffer = Number.isFinite(breakBuffer) ? breakBuffer : 0
  const shown = quotePrice(box.boxHigh * (1 + buffer), true)
  if (shown == null) return unavailable()
  const gate = priceGate(shown, closes[i])
  if (gate.priceMode !== 'price') return gate
  const confirm = Math.max(0, Math.floor(Number(options?.breakoutConfirmDays ?? options?.confirmDays ?? 3) || 0))
  if (confirm <= 0) return gate
  return { ...gate, priceText: `${gate.priceText}（突破参考价，确认站稳另计）` }
}

function aboveMa(closes, maPeriod, index) {
  const ma = smaAt(closes, maPeriod, index)
  const close = closes[index]
  return ma != null && close != null && close >= ma
}

/**
 * 弹：只在「还差今天这一根收盘站上 MA」时给出唯一解。
 * 卖后路径或确认天数对不上，无法给出唯一价。
 */
function reboundCurrentGate(bars, options) {
  const closes = bars?.closes || []
  const i = closes.length - 1
  const period = Math.max(2, Math.floor(Number(options?.maPeriod) || 20))
  const confirm = Math.max(1, Math.floor(Number(options?.reboundConfirmDays) || 2))
  const need = confirm - 1
  if (i < period || i < need) return unavailable()

  if (need > 0) {
    for (let k = 1; k <= need; k++) {
      if (!aboveMa(closes, period, i - k)) return unavailable()
    }
    const before = i - need - 1
    if (before >= 0 && aboveMa(closes, period, before)) return unavailable()
  }

  const base = computeTradeSignals(closes, { ...(options || {}), maPeriod: period })
  const sellSet = new Set([...(base.sellRsi || []), ...(base.sellMa20 || [])])
  const sellLookback = Math.max(1, Math.floor(Number(options?.reboundSellLookback) || 6))
  const minDays = Math.max(0, Math.floor(Number(options?.reboundMinDaysAfterSell) || 1))
  let sellIdx = null
  for (let j = i - 1; j >= Math.max(0, i - sellLookback); j--) {
    if (sellSet.has(j)) {
      sellIdx = j
      break
    }
  }
  if (sellIdx == null || i - sellIdx < minDays) return unavailable()
  const standStart = i - confirm + 1
  if (standStart <= sellIdx) return unavailable()

  const rebound = computeReboundAfterSell(
    {
      closes,
      opens: bars?.opens || [],
      highs: bars?.highs || [],
      lows: bars?.lows || [],
    },
    base,
    new Set(),
    {
      sellLookback,
      minDaysAfterSell: minDays,
      confirmDays: confirm,
      minReboundPct: options?.reboundMinPct ?? 0.05,
      minBodyPct: options?.reboundMinBodyPct ?? 0.015,
      maxRsi: options?.reboundMaxRsi ?? 60,
      minGap: options?.reboundMinGap ?? 6,
      maxUpperWickRatio: options?.reboundMaxUpperWickRatio ?? 0.5,
    },
  )
  const minGap = Math.max(1, Math.floor(Number(options?.reboundMinGap) || 6))
  for (const b of rebound) {
    if (b < i && i - b < minGap) return unavailable()
  }

  const from = i - (period - 1)
  if (from < 0) return unavailable()
  let sum = 0
  for (let j = from; j < i; j++) {
    if (closes[j] == null || !Number.isFinite(closes[j])) return unavailable()
    sum += closes[j]
  }
  const shown = quotePrice(sum / (period - 1), false)
  if (shown == null) return unavailable()
  const gate = priceGate(shown, closes[i])
  if (gate.priceMode !== 'price') return gate
  return { ...gate, priceText: `${gate.priceText}（收回 MA${period} 的收盘门槛）` }
}

function priceForTag(tag, bars, options, composite) {
  if (composite || !tag) return unavailable()
  if (tag === '突') return breakoutCurrentGate(bars, options)
  if (tag === '弹') return reboundCurrentGate(bars, options)
  return unavailable(NO_UNIQUE_PRICE_BY_TAG[tag] || ENTRY_MISS_NO_UNIQUE_PRICE)
}

function signalPayload(bars) {
  return {
    closes: bars.closes || [],
    opens: bars.opens || [],
    highs: bars.highs || [],
    lows: bars.lows || [],
    volumes: bars.volumes || [],
    dayKeys: bars.dayKeys || [],
    indexMa20ByDay: bars.indexMa20ByDay || null,
  }
}

/**
 * @returns {null | {
 *   lookbackDays: number,
 *   hits: { date: string, price: number, index: number }[],
 *   hitsText: string,
 *   priceMode: 'price' | 'unavailable',
 *   triggerPrice: number | null,
 *   priceText: string,
 *   observationOnly: true,
 * }}
 */
export function buildEntryMissObservation({
  roleId,
  tag = '',
  composite = false,
  bars,
  signalOptions,
  familySig = null,
  verdict,
  lookbackDays,
} = {}) {
  if (verdict !== '未命中') return null
  if (roleId !== 'entry') return null
  const closes = bars?.closes || []
  const last = closes.length - 1
  if (last < 0) return null

  const window = resolveLookback(lookbackDays)
  const options = signalOptions || {}
  let sig = familySig
  if (composite || !sig) {
    if (closes.length >= 20) {
      sig = computeFullSignals(signalPayload(bars), options)
    }
  }

  const hits = composite
    ? collectCompositeHits(sig, bars, last, window, options)
    : collectTagHits(sig, tag, bars, last, window, options)
  const price = priceForTag(tag, bars, options, composite)
  return {
    lookbackDays: window,
    hits,
    hitsText: formatHits(hits, window),
    priceMode: price.priceMode,
    triggerPrice: price.triggerPrice,
    priceText: price.priceText,
    observationOnly: true,
  }
}

function maCrossParts(closes, i) {
  if (i < 1) return null
  const ma5 = smaAt(closes, 5, i)
  const ma10 = smaAt(closes, 10, i)
  const ma20 = smaAt(closes, 20, i)
  const prevMa5 = smaAt(closes, 5, i - 1)
  const prevMa10 = smaAt(closes, 10, i - 1)
  if (ma5 == null || ma10 == null || prevMa5 == null || prevMa10 == null) return null
  return { ma5, ma10, ma20, prevMa5, prevMa10 }
}

/** 与信号回测面板同一条 MA5/MA10 示意规则。 */
export function maCrossBacktestSignal(closes, i) {
  const parts = maCrossParts(closes, i)
  if (!parts) return null
  const close = closes[i]
  if (
    parts.prevMa5 < parts.prevMa10 &&
    parts.ma5 >= parts.ma10 &&
    parts.ma20 != null &&
    close > parts.ma20
  ) {
    return { action: 'buy', strength: 0.7, tag: '趋' }
  }
  if (parts.prevMa5 > parts.prevMa10 && parts.ma5 <= parts.ma10) {
    return { action: 'sell', sellPct: 1, tag: '止' }
  }
  return null
}

/**
 * 信号回测示意买入在最新 K 线未出现时的观察。
 * 前一日尚未形成死叉下方的金叉前提时，无法给出唯一价。
 */
export function buildMaCrossMissObservation({ closes, dayKeys, lookbackDays, minBarIndex = 20 } = {}) {
  const series = closes || []
  const last = series.length - 1
  const minIndex = Math.max(20, Math.floor(Number(minBarIndex) || 20))
  if (last < minIndex) return null
  if (maCrossBacktestSignal(series, last)?.action === 'buy') return null

  const window = resolveLookback(lookbackDays)
  const from = Math.max(minIndex, last - window + 1)
  const hits = []
  for (let day = last - 1; day >= from; day--) {
    if (maCrossBacktestSignal(series, day)?.action !== 'buy') continue
    const hit = hitFromDay({ closes: series, dayKeys: dayKeys || [] }, day)
    if (hit) hits.push(hit)
  }

  const parts = maCrossParts(series, last)
  let price = unavailable('无法给出唯一价（前一日均线前提不成立）')
  if (parts && parts.prevMa5 < parts.prevMa10 && parts.ma20 != null) {
    let sum4 = 0
    let sum9 = 0
    let sum19 = 0
    let ok = true
    for (let j = 1; j <= 19; j++) {
      const v = series[last - j]
      if (v == null || !Number.isFinite(v)) {
        ok = false
        break
      }
      if (j <= 4) sum4 += v
      if (j <= 9) sum9 += v
      sum19 += v
    }
    if (ok) {
      const crossPx = quotePrice(sum9 - 2 * sum4, false)
      const ma20Px = quotePrice(sum19 / 19, true)
      const shown = Math.max(crossPx || 0, ma20Px || 0)
      if (shown > 0) price = priceGate(shown, series[last])
    }
  }

  return {
    lookbackDays: window,
    hits,
    hitsText: formatHits(hits, window),
    priceMode: price.priceMode,
    triggerPrice: price.triggerPrice,
    priceText: price.priceText,
    observationOnly: true,
  }
}
