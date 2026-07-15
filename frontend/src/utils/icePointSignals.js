/** RSI(14) 与冰点/买卖点信号（与 K 线图一致） */

import {
  calcSellPositionPct,
  sellPositionHint,
} from './sellPositionRatio'
import { calcBuyPriceRange } from './buyPriceRange'

/** 卖点展示：RSI 超买区停留后下穿 → 止；连续破 MA20 / 破买点低点 → 减 */
export const SELL_TAG_TAKE_PROFIT = '止'
export const SELL_TAG_REDUCE = '减'

/** 主信号展示优先级（高→低）；同日止/减优先「减」；加仅自选持仓场景 */
export const SIGNAL_TAG_PRIORITY = ['减', '止', '冲', '加', '强', '趋', '转', '突', '弹', '买', '冰']
export const SIGNAL_SORT_RANK_NONE = 10
export const SIGNAL_PRIORITY_SUMMARY = '止/减 > 强 > 趋 > 转 > 突 > 弹 > 买'

export function signalSortRank(tag) {
  if (!tag) return SIGNAL_SORT_RANK_NONE
  const i = SIGNAL_TAG_PRIORITY.indexOf(tag)
  return i >= 0 ? i : SIGNAL_SORT_RANK_NONE
}

export function isSellSignalTag(tag) {
  return tag === SELL_TAG_TAKE_PROFIT || tag === SELL_TAG_REDUCE
}

export function calcRSI(closes, period = 14) {
  const out = new Array(closes.length).fill(null)
  for (let i = period; i < closes.length; i++) {
    let gain = 0
    let loss = 0
    for (let j = 0; j < period; j++) {
      const ch = closes[i - j] - closes[i - j - 1]
      if (ch >= 0) gain += ch
      else loss -= ch
    }
    const ag = gain / period
    const al = loss / period
    out[i] = al === 0 ? 100 : 100 - 100 / (1 + ag / al)
  }
  return out
}

function sma(closes, period) {
  const out = new Array(closes.length).fill(null)
  for (let i = period - 1; i < closes.length; i++) {
    let s = 0
    for (let j = 0; j < period; j++) s += closes[i - j]
    out[i] = s / period
  }
  return out
}

function volMa(volumes, period, i) {
  if (!volumes?.length || i < period - 1) return null
  let s = 0
  for (let j = 0; j < period; j++) s += volumes[i - j] || 0
  return s / period
}

/** YYYY-MM-DD */
export function normalizeDayKey(dayStr) {
  const s = String(dayStr || '').trim().replace(/\//g, '-')
  const m = s.match(/^(\d{4})-(\d{2})-(\d{2})/)
  if (m) return `${m[1]}-${m[2]}-${m[3]}`
  const c = s.match(/^(\d{4})(\d{2})(\d{2})/)
  if (c) return `${c[1]}-${c[2]}-${c[3]}`
  return s.slice(0, 10)
}

/** 日期 key → 可排序数字 YYYYMMDD */
export function dayKeyToNum(dayStr) {
  const k = normalizeDayKey(dayStr)
  const m = k.match(/^(\d{4})-(\d{2})-(\d{2})$/)
  if (!m) return 0
  return Number(m[1] + m[2] + m[3])
}

/** 从策略执行时间解析回放截止日 YYYY-MM-DD */
export function parseRunDayKey(createdAt) {
  if (!createdAt) return ''
  const s = String(createdAt).trim()
  const m = s.match(/^(\d{4}-\d{2}-\d{2})/)
  return m ? m[1] : normalizeDayKey(s)
}

/** 截断 K 线序列至指定交易日（含该日） */
export function truncateBarsToDay(bars, dayKey) {
  const target = normalizeDayKey(dayKey)
  if (!target || !bars?.closes?.length) return bars
  const { closes, opens, lows, volumes, dayKeys } = bars
  const targetNum = dayKeyToNum(target)
  let lastIdx = -1
  for (let i = 0; i < dayKeys.length; i++) {
    const n = dayKeyToNum(dayKeys[i])
    if (n > 0 && n <= targetNum) lastIdx = i
  }
  if (lastIdx < 19) return null
  return {
    closes: closes.slice(0, lastIdx + 1),
    opens: (opens || closes).slice(0, lastIdx + 1),
    lows: (lows || closes).slice(0, lastIdx + 1),
    volumes: (volumes || []).slice(0, lastIdx + 1),
    dayKeys: dayKeys.slice(0, lastIdx + 1),
    indexMa20ByDay: bars.indexMa20ByDay,
  }
}

/** 截断指数 MA20 映射至指定交易日（含该日） */
export function truncateIndexMa20ByDay(indexMa20ByDay, dayKey) {
  const targetNum = dayKeyToNum(dayKey)
  if (!targetNum || !indexMa20ByDay?.size) return indexMa20ByDay
  const out = new Map()
  for (const [day, val] of indexMa20ByDay) {
    if (dayKeyToNum(day) <= targetNum) out.set(day, val)
  }
  return out
}

/** 上证等指数：按交易日 key → { close, ma20 } */
export function buildIndexMa20ByDay(indexCloseByDay, maPeriod = 20) {
  const out = new Map()
  if (!indexCloseByDay?.size) return out
  const days = [...indexCloseByDay.keys()].sort()
  const closes = days.map((d) => indexCloseByDay.get(d))
  for (let i = 0; i < days.length; i++) {
    let ma = null
    if (i >= maPeriod - 1) {
      let s = 0
      for (let j = 0; j < maPeriod; j++) s += closes[i - j]
      ma = s / maPeriod
    }
    out.set(days[i], { close: closes[i], ma20: ma })
  }
  return out
}

function lastIceBefore(iceEnter, i, minGap) {
  let found = null
  for (const idx of iceEnter) {
    if (idx < i && i - idx >= minGap) found = idx
  }
  return found
}

function indexBullishOnDay(dayKey, indexMa20ByDay) {
  if (!indexMa20ByDay?.size || !dayKey) return true
  const row = indexMa20ByDay.get(dayKey)
  if (!row || row.ma20 == null) return true
  return row.close > row.ma20
}

/**
 * 冰点：RSI 下穿 30
 * 买点（出冰点）：RSI 上穿 30，且前 lookback 日内曾出现「冰」（RSI 下穿 30）
 * 止盈：RSI 自超买区（默认≥72）连续停留 N 日后下穿 → 止（须浮盈+自高点回撤，见 sellProfitGate）
 * 减仓：首次有效跌破 MA20（默认 1 日确认）+ 可选提前减（破 MA5 走弱/大阴/破近端低点）
 */
export function computeTradeSignals(closes, options = {}) {
  const {
    rsiPeriod = 14,
    iceThreshold = 30,
    overbought = 70,
    lookback = 5,
    maPeriod = 20,
    reduceConfirmDays = 1,
    takeProfitMinOverboughtDays = 3,
  } = options

  const len = closes.length
  const rsi = calcRSI(closes, rsiPeriod)
  const ma20 = sma(closes, maPeriod)
  const iceEnter = []
  const buy = []
  const sellRsi = []
  const sellMa20 = []

  for (let i = 1; i < len; i++) {
    const r = rsi[i]
    const prev = rsi[i - 1]
    if (r == null || prev == null) continue

    if (prev >= iceThreshold && r < iceThreshold) {
      iceEnter.push(i)
    }

    if (prev <= iceThreshold && r > iceThreshold) {
      let hadFormalIce = false
      for (const idx of iceEnter) {
        if (idx < i && i - idx <= lookback) {
          hadFormalIce = true
          break
        }
      }
      if (hadFormalIce) buy.push(i)
    }

    if (prev >= overbought && r < overbought) {
      const minDays = Math.max(1, Math.floor(Number(takeProfitMinOverboughtDays) || 1))
      let streak = 0
      for (let k = i - 1; k >= 0 && streak < minDays; k--) {
        const rk = rsi[k]
        if (rk == null || rk < overbought) break
        streak++
      }
      if (streak >= minDays) sellRsi.push(i)
    }
  }

  const reduceConfirm = Math.max(1, Math.floor(Number(reduceConfirmDays) || 1))
  for (let i = reduceConfirm - 1; i < len; i++) {
    let allBelow = true
    for (let j = 0; j < reduceConfirm; j++) {
      const idx = i - j
      if (ma20[idx] == null || closes[idx] == null || closes[idx] >= ma20[idx]) {
        allBelow = false
        break
      }
    }
    if (!allBelow) continue
    const before = i - reduceConfirm
    if (before >= 0 && ma20[before] != null && closes[before] != null && closes[before] < ma20[before]) {
      continue
    }
    sellMa20.push(i)
  }

  const last = len - 1
  let latestStatus = { text: '—', type: 'default', rsi: null, ma20: null }

  if (last >= 0 && rsi[last] != null) {
    const rl = rsi[last]
    latestStatus.rsi = rl
    if (ma20[last] != null) latestStatus.ma20 = ma20[last]

    if (rl < iceThreshold) {
      latestStatus = {
        ...latestStatus,
        text: `冰点区内 · RSI ${rl.toFixed(1)}`,
        type: 'info',
      }
    } else if (rl > overbought) {
      latestStatus = {
        ...latestStatus,
        text: `超买区 · RSI ${rl.toFixed(1)}`,
        type: 'warning',
      }
    } else {
      let daysSinceBuy = null
      for (let i = last; i >= 0; i--) {
        if (buy.includes(i)) {
          daysSinceBuy = last - i
          break
        }
      }
      if (daysSinceBuy != null && daysSinceBuy <= 15) {
        latestStatus = {
          ...latestStatus,
          text: `出冰点 ${daysSinceBuy} 个交易日前 · RSI ${rl.toFixed(1)}`,
          type: 'success',
        }
      } else {
        latestStatus = {
          ...latestStatus,
          text: `常态 · RSI ${rl.toFixed(1)}`,
          type: 'default',
        }
      }
    }
  }

  return { rsi, ma20, iceEnter, buy, sellRsi, sellMa20, latestStatus }
}

/**
 * 平台突破「突」：箱体整理后放量突破前高，并经 N 日站稳确认（过滤假突破）
 * - 前 boxPeriod 日振幅 ≤ maxRangePct（默认 20%）
 * - 突破日收阳、收盘站上箱体上沿、放量、均线多头初成
 * - 突破日上影线不宜过长（默认 ≤ K 线振幅 55%）
 * - 突破后 confirmDays 个交易日内收盘均不低于箱体上沿，才在突破日标「突」（默认 3 日）
 */
export function computeBreakoutSignals(bars, options = {}) {
  const {
    closes = [],
    opens = [],
    highs = [],
    lows = [],
    volumes = [],
    boxPeriod = 20,
    maxRangePct = 0.2,
    volMult = 1.25,
    volPeriod = 5,
    minGap = 15,
    breakBuffer = 0.005,
    /** 突破后需连续站稳的交易日数（不含突破日本身） */
    confirmDays = 3,
    /** 突破日 K 线上影线占振幅上限；null 关闭 */
    maxUpperWickRatio = 0.55,
  } = { ...bars, ...options }

  const len = closes.length
  const confirm = Math.max(0, Math.floor(Number(confirmDays) || 0))
  if (len < boxPeriod + confirm + 1) return []

  const ma5 = sma(closes, 5)
  const ma10 = sma(closes, 10)
  const ma20 = sma(closes, 20)
  const breakout = []

  function boxRange(breakIdx) {
    const lookStart = breakIdx - boxPeriod
    const lookEnd = breakIdx - 1
    if (lookStart < 0) return null

    let boxHigh = -Infinity
    let boxLow = Infinity
    for (let j = lookStart; j <= lookEnd; j++) {
      const h = highs[j] ?? Math.max(opens[j] ?? closes[j], closes[j])
      const l = lows[j] ?? Math.min(opens[j] ?? closes[j], closes[j])
      boxHigh = Math.max(boxHigh, h)
      boxLow = Math.min(boxLow, l)
    }
    const mid = (boxHigh + boxLow) / 2
    if (mid <= 0 || boxHigh <= boxLow) return null
    if ((boxHigh - boxLow) / mid > maxRangePct) return null
    return { boxHigh, boxLow }
  }

  function passesBreakoutDay(breakIdx, boxHigh) {
    const o = opens[breakIdx]
    const c = closes[breakIdx]
    if (o == null || c == null || c <= o) return false
    if (c <= boxHigh * (1 + breakBuffer)) return false

    const vma = volMa(volumes, volPeriod, breakIdx)
    if (vma != null && vma > 0 && (volumes[breakIdx] || 0) < vma * volMult) return false

    if (ma5[breakIdx] == null || ma10[breakIdx] == null) return false
    if (ma5[breakIdx] < ma10[breakIdx]) return false
    if (ma20[breakIdx] != null && ma10[breakIdx] < ma20[breakIdx] * 0.995) return false

    if (maxUpperWickRatio != null) {
      const h = highs[breakIdx] ?? Math.max(o, c)
      const l = lows[breakIdx] ?? Math.min(o, c)
      const range = h - l
      if (range > 0) {
        const upperWick = h - Math.max(o, c)
        if (upperWick / range > maxUpperWickRatio) return false
      }
    }
    return true
  }

  function holdsAboveBox(breakIdx, boxHigh, throughIdx) {
    const holdFloor = boxHigh
    for (let j = breakIdx + 1; j <= throughIdx; j++) {
      const c = closes[j]
      if (c == null || c < holdFloor) return false
    }
    return true
  }

  for (let breakIdx = boxPeriod; breakIdx < len; breakIdx++) {
    const box = boxRange(breakIdx)
    if (!box || !passesBreakoutDay(breakIdx, box.boxHigh)) continue

    const confirmEnd = breakIdx + confirm
    if (confirmEnd >= len) continue
    if (!holdsAboveBox(breakIdx, box.boxHigh, confirmEnd)) continue

    let dup = false
    for (const b of breakout) {
      if (breakIdx - b < minGap) {
        dup = true
        break
      }
    }
    if (!dup) breakout.push(breakIdx)
  }
  return breakout
}

/**
 * - 近 sellLookback 日内出现过「卖」，站稳起点须晚于卖日
 * - 连续 confirmDays 日收盘 ≥ MA20（信号落在第 confirmDays 日）
 * - 卖后至站稳前曾收于 MA20 下，或自卖点低点反弹 ≥ minReboundPct
 * - 信号日收阳、实体 ≥ minBodyPct、RSI ≤ maxRsi、收 > MA5、MA5 ≥ MA10、MA20 走平/向上
 * - 上影线不宜过长；不与 买/强/趋/突 同日；与「买」相邻 buyAdjacentDays 日不标
 */
export function computeReboundAfterSell(bars, baseSig, skipIndices, options = {}) {
  const { closes = [], opens = [], highs = [], lows = [] } = bars || {}
  const {
    sellLookback = 6,
    minReboundPct = 0.05,
    minBodyPct = 0.015,
    maxRsi = 60,
    minGap = 6,
    buyIndices = [],
    buyAdjacentDays = 2,
    minDaysAfterSell = 1,
    maxUpperWickRatio = 0.5,
    confirmDays = 2,
  } = options

  const confirm = Math.max(1, Math.floor(Number(confirmDays) || 2))

  const len = closes.length
  const ma20 = baseSig?.ma20
  const ma5 = sma(closes, 5)
  const ma10 = sma(closes, 10)
  const rsi = baseSig?.rsi
  const sellSet = new Set([...(baseSig?.sellRsi || []), ...(baseSig?.sellMa20 || [])])
  const skip = skipIndices instanceof Set ? skipIndices : new Set(skipIndices || [])
  const reboundBuy = []

  function lastSellBefore(i) {
    for (let j = i - 1; j >= Math.max(0, i - sellLookback); j--) {
      if (sellSet.has(j)) return j
    }
    return null
  }

  function nearBuySignal(i) {
    for (const b of buyIndices) {
      if (Math.abs(i - b) <= buyAdjacentDays) return true
    }
    return false
  }

  function holdsAboveMa20(fromIdx, throughIdx) {
    for (let j = fromIdx; j <= throughIdx; j++) {
      const c = closes[j]
      const m = ma20?.[j]
      if (c == null || m == null || c < m) return false
    }
    return true
  }

  function wasBelowMa20Since(sellIdx, beforeIdx) {
    for (let j = sellIdx; j < beforeIdx; j++) {
      const c = closes[j]
      const m = ma20?.[j]
      if (c != null && m != null && c < m) return true
    }
    return false
  }

  function passesSignalDayFilters(i) {
    const o = opens[i]
    const c = closes[i]
    const h = highs[i]
    const l = lows[i]
    if (o == null || c == null || c <= o) return false
    if (o <= 0 || (c - o) / o < minBodyPct) return false
    const r = rsi?.[i]
    if (r == null || r > maxRsi) return false

    const m5v = ma5[i]
    const m10v = ma10[i]
    if (m5v == null || c <= m5v) return false
    if (m10v == null || m5v < m10v) return false

    const m20v = ma20?.[i]
    const m20prev = ma20?.[i - 1]
    if (m20v == null || m20prev == null || m20v < m20prev) return false

    if (maxUpperWickRatio != null && h != null && l != null && h > l) {
      const upperWick = (h - c) / (h - l)
      if (upperWick > maxUpperWickRatio) return false
    }
    return true
  }

  for (let i = confirm - 1; i < len; i++) {
    if (skip.has(i)) continue
    if (nearBuySignal(i)) continue
    if (!passesSignalDayFilters(i)) continue

    const standStart = i - confirm + 1
    if (standStart < 1) continue
    if (!holdsAboveMa20(standStart, i)) continue

    const sellIdx = lastSellBefore(i)
    if (sellIdx == null || i - sellIdx < minDaysAfterSell) continue
    if (standStart <= sellIdx) continue

    let lowSince = Infinity
    for (let j = sellIdx; j <= i; j++) {
      const lj = lows[j] ?? closes[j]
      if (lj != null && lj < lowSince) lowSince = lj
    }
    const c = closes[i]
    const reboundPctOk =
      lowSince > 0 &&
      lowSince < Infinity &&
      c != null &&
      (c - lowSince) / lowSince >= minReboundPct
    if (!wasBelowMa20Since(sellIdx, standStart) && !reboundPctOk) continue

    let dup = false
    for (const b of reboundBuy) {
      if (i - b < minGap) {
        dup = true
        break
      }
    }
    if (!dup) reboundBuy.push(i)
  }
  return reboundBuy
}

/**
 * 转势买「转」：收盘上穿指定均线（默认 MA60），前段多数时间在均线下，MA20 走平/向上，收阳
 * 不与 买/强/趋/突 同日；专抓底部反转/金叉，不同于「趋」的回踩买点
 */
export function computeReversalSignals(bars, baseSig, skipIndices, options = {}) {
  if (options.reversalEnabled === false) return []

  const {
    closes = [],
    opens = [],
  } = bars || {}
  const len = closes.length
  const crossPeriod = Math.max(5, Math.floor(Number(options.reversalCrossMaPeriod) || 60))
  if (len < crossPeriod + 1) return []

  const maCross = sma(closes, crossPeriod)
  const ma5 = sma(closes, 5)
  const ma10 = sma(closes, 10)
  const ma20 = baseSig?.ma20 ?? sma(closes, 20)
  const rsi = baseSig?.rsi
  const skip = skipIndices instanceof Set ? skipIndices : new Set(skipIndices || [])

  const minBodyPct = options.reversalMinBodyPct ?? 0.01
  const rsiMin = options.reversalRsiMin ?? 45
  const rsiMax = options.reversalRsiMax ?? 70
  const requireMa20FlatOrUp = options.reversalRequireMa20FlatOrUp !== false
  const ma20Lookback = Math.max(1, Math.floor(Number(options.reversalMa20LookbackDays) || 5))
  const requireMa5AboveMa10 = options.reversalRequireMa5AboveMa10 === true
  const requireCloseAboveMa5 = options.reversalRequireCloseAboveMa5 !== false
  const recentBelowDays = Math.max(1, Math.floor(Number(options.reversalRecentBelowDays) || 5))
  const recentBelowMinRatio = options.reversalRecentBelowMinRatio ?? 0.6
  const minGap = Math.max(1, Math.floor(Number(options.reversalMinGap) || 5))
  const minBelowCount = Math.max(1, Math.ceil(recentBelowDays * recentBelowMinRatio))

  const reversalBuy = []

  for (let i = crossPeriod; i < len; i++) {
    if (skip.has(i)) continue

    const mc = maCross[i]
    const mcPrev = maCross[i - 1]
    const c = closes[i]
    const cPrev = closes[i - 1]
    if (mc == null || mcPrev == null || c == null || cPrev == null) continue
    if (c <= mc || cPrev > mcPrev) continue

    let belowCount = 0
    for (let j = Math.max(0, i - recentBelowDays); j < i; j++) {
      if (closes[j] != null && maCross[j] != null && closes[j] <= maCross[j]) belowCount++
    }
    if (belowCount < minBelowCount) continue

    const o = opens[i]
    if (o == null || c <= o) continue
    if (o <= 0 || (c - o) / o < minBodyPct) continue

    if (requireCloseAboveMa5 && (ma5[i] == null || c <= ma5[i])) continue
    if (requireMa5AboveMa10 && (ma5[i] == null || ma10[i] == null || ma5[i] <= ma10[i])) continue

    const r = rsi?.[i]
    if (r == null || r < rsiMin || r > rsiMax) continue

    if (requireMa20FlatOrUp) {
      const m20 = ma20[i]
      const m20Prev = i >= ma20Lookback ? ma20[i - ma20Lookback] : null
      if (m20 == null || m20Prev == null || m20 < m20Prev) continue
    }

    let dup = false
    for (const t of reversalBuy) {
      if (i - t < minGap) {
        dup = true
        break
      }
    }
    if (!dup) reversalBuy.push(i)
  }
  return reversalBuy
}

/** 震荡/粘合过滤：均线过近、近端有止/减、窄幅横盘时不标趋 */
export function passesTrendChopFilter(i, ctx = {}) {
  const { options = {}, ma5, ma10, closes, highs, lows, sellSet } = ctx
  if (options.trendChopFilterEnabled === false) return true

  const m5 = ma5?.[i]
  const m10 = ma10?.[i]
  const minSpread = options.trendMinMaSpreadPct ?? 0.01
  if (m10 != null && m10 > 0 && m5 != null && (m5 - m10) / m10 < minSpread) {
    return false
  }

  const sellLb = Math.max(0, Math.floor(Number(options.trendChopSellLookback) || 8))
  if (sellSet?.size && sellLb > 0) {
    for (let j = Math.max(0, i - sellLb); j < i; j++) {
      if (sellSet.has(j)) return false
    }
  }

  const rangeDays = Math.max(2, Math.floor(Number(options.trendChopRangeDays) || 5))
  const maxRangePct = options.trendChopMaxRangePct ?? 0.08
  if (maxRangePct > 0) {
    const start = i - rangeDays + 1
    if (start >= 0) {
      let hi = -Infinity
      let lo = Infinity
      for (let j = start; j <= i; j++) {
        const h = highs?.[j] ?? closes?.[j]
        const l = lows?.[j] ?? closes?.[j]
        if (h != null) hi = Math.max(hi, h)
        if (l != null) lo = Math.min(lo, l)
      }
      const mid = (hi + lo) / 2
      if (mid > 0 && hi > lo && (hi - lo) / mid < maxRangePct) return false
    }
  }
  return true
}

function collectEntryBarGroups(filteredBuy, strictBuy, trendBuy, reversalBuy, breakoutBuy, reboundBuy) {
  return {
    buy: filteredBuy || [],
    strong: strictBuy || [],
    trend: trendBuy || [],
    reversal: reversalBuy || [],
    breakout: breakoutBuy || [],
    rebound: reboundBuy || [],
  }
}

/** 各买点类型 → 保护天数（兼容旧版统一 sellProtectAfterEntryDays） */
function resolveSellProtectDays(options = {}) {
  const legacy = options.sellProtectAfterEntryDays
  const fallback = legacy != null ? legacy : 0
  return {
    buy: options.sellProtectAfterBuy ?? fallback,
    strong: options.sellProtectAfterStrong ?? fallback,
    trend: options.sellProtectAfterTrend ?? fallback,
    breakout: options.sellProtectAfterBreakout ?? fallback,
    rebound: options.sellProtectAfterRebound ?? fallback,
    reversal: options.sellProtectAfterReversal ?? fallback,
  }
}

function buildEntryMetaList(groups, lows, closes, protectByType) {
  const out = []
  for (const [type, indices] of Object.entries(groups)) {
    const protect = Math.max(0, Math.floor(Number(protectByType[type]) || 0))
    for (const e of indices) {
      out.push({
        index: e,
        type,
        protect,
        low: lows[e] ?? closes[e],
      })
    }
  }
  return out
}

function nearestEntryBefore(barIndex, entries) {
  let nearest = null
  for (const ent of entries) {
    if (ent.index < barIndex && (!nearest || ent.index > nearest.index)) nearest = ent
  }
  return nearest
}

/** 止：须相对最近买点仍有浮盈，且自阶段高点已有足够回撤 */
function filterTakeProfitWithProfitGate(indices, entryMeta, closes, options) {
  if (options.sellProfitGateEnabled === false) return indices || []

  const minGain = options.takeProfitMinGainPct ?? 0.05
  const lookback = options.sellBreakEntryLowLookback ?? options.recentBuyDays ?? 15
  const minPullback = options.takeProfitMinPullbackFromPeakPct ?? 0.03
  const requirePullback = options.takeProfitRequirePullbackFromPeak !== false

  return (indices || []).filter((i) => {
    const c = closes[i]
    if (c == null) return false

    const ent = nearestEntryBefore(i, entryMeta)
    let entryPrice = null
    let peakFrom = 0

    if (ent && i - ent.index <= lookback) {
      entryPrice = ent.low ?? closes[ent.index]
      peakFrom = ent.index
    } else {
      const start = Math.max(0, i - lookback)
      let refLow = Infinity
      for (let j = start; j < i; j++) {
        if (closes[j] != null) refLow = Math.min(refLow, closes[j])
      }
      entryPrice = Number.isFinite(refLow) ? refLow : null
      peakFrom = start
    }

    if (entryPrice == null || entryPrice <= 0) return false
    if ((c - entryPrice) / entryPrice < minGain) return false

    if (requirePullback) {
      let peak = -Infinity
      for (let j = peakFrom; j <= i; j++) {
        if (closes[j] != null) peak = Math.max(peak, closes[j])
      }
      if (peak > 0 && (peak - c) / peak < minPullback) return false
    }
    return true
  })
}

/** 提前减：MA20 确认前，结构已明显走弱 */
function augmentEarlyReduceSignals(sellMa20, bars, ma20, options = {}) {
  if (options.reduceEarlyEnabled === false) return sellMa20 || []

  const { closes = [], opens = [], lows = [] } = bars || {}
  const len = closes.length
  if (len < 3) return sellMa20 || []

  const set = new Set(sellMa20 || [])
  const ma5 = sma(closes, 5)
  const ma5Days = Math.max(1, Math.floor(Number(options.reduceEarlyMa5Days) || 3))
  const minDrop = options.reduceEarlyMinDropPct ?? 0.045
  const breakLowDays = Math.max(2, Math.floor(Number(options.reduceEarlyBreakLowDays) || 5))
  const uptrendLookback = Math.max(3, Math.floor(Number(options.reduceEarlyUptrendLookback) || 8))
  const requireUptrend = options.reduceEarlyRequireUptrend !== false

  function hadRecentAboveMa20(i) {
    if (!requireUptrend) return true
    for (let j = Math.max(0, i - uptrendLookback); j < i; j++) {
      if (closes[j] != null && ma20[j] != null && closes[j] >= ma20[j]) return true
    }
    return false
  }

  for (let i = 1; i < len; i++) {
    if (set.has(i)) continue
    if (!hadRecentAboveMa20(i)) continue

    const c = closes[i]
    const cPrev = closes[i - 1]
    const mPrev = ma20[i - 1]
    if (c == null || cPrev == null) continue

    // 路径 A：连续 N 日收在 MA5 下（MA20 尚未有效跌破时提前预警）
    if (i >= ma5Days) {
      let belowMa5 = true
      for (let j = 0; j < ma5Days; j++) {
        const idx = i - j
        if (ma5[idx] == null || closes[idx] == null || closes[idx] >= ma5[idx]) {
          belowMa5 = false
          break
        }
      }
      if (belowMa5) {
        const before = i - ma5Days
        const wasAboveMa5 =
          before >= 0 && ma5[before] != null && closes[before] != null && closes[before] >= ma5[before]
        const stillNearMa20 = ma20[i] != null && c >= ma20[i] * 0.985
        if (wasAboveMa5 && stillNearMa20) set.add(i)
      }
    }

    // 路径 B：单日大阴（前一日在 MA20 上）
    if (mPrev != null && cPrev >= mPrev && cPrev > 0) {
      const drop = (cPrev - c) / cPrev
      const o = opens[i]
      if (drop >= minDrop && o != null && c < o) set.add(i)
    }

    // 路径 C：跌破近 N 日低点（前一日仍在 MA20 上）
    if (mPrev != null && cPrev >= mPrev && i >= breakLowDays) {
      let priLo = Infinity
      for (let j = i - breakLowDays; j < i; j++) {
        const lj = lows[j] ?? closes[j]
        if (lj != null) priLo = Math.min(priLo, lj)
      }
      if (Number.isFinite(priLo) && c < priLo) set.add(i)
    }
  }

  return [...set].sort((a, b) => a - b)
}

/**
 * 保护期内默认不标止/减；减若收盘跌破最近买点当日低点仍可保留
 */
function filterSellSignalsAfterEntry(sellIndices, entries, closes, options = {}) {
  if (!sellIndices?.length) return []
  const allowBreakLow = options.sellBreakEntryLowEnablesReduce !== false
  const isReduce = options.sellKind === 'reduce'

  return sellIndices.filter((i) => {
    const ent = nearestEntryBefore(i, entries)
    if (!ent || ent.protect < 1) return true
    const gap = i - ent.index
    if (gap > ent.protect) return true
    if (isReduce && allowBreakLow) {
      const c = closes[i]
      const floor = ent.low
      if (c != null && floor != null && c < floor) return true
    }
    return false
  })
}

/** 同类型信号最小间隔（避免图表密集重复） */
function dedupeSignalIndices(indices, minGap) {
  const gap = Math.max(0, Math.floor(Number(minGap) || 0))
  const sorted = [...(indices || [])].sort((a, b) => a - b)
  if (gap < 1 || sorted.length <= 1) return sorted
  const out = [sorted[0]]
  for (let k = 1; k < sorted.length; k++) {
    if (sorted[k] - out[out.length - 1] >= gap) out.push(sorted[k])
  }
  return out
}

/** 首次收盘跌破最近买点低点 → 补标减（仅破位日，非持续低于期间每日都标） */
function augmentReduceByEntryLowBreak(sellMa20, closes, entries, lookback = 15) {
  const set = new Set(sellMa20 || [])
  const len = closes.length
  for (let i = 1; i < len; i++) {
    let nearest = null
    for (const ent of entries) {
      if (ent.index < i && i - ent.index <= lookback) {
        if (!nearest || ent.index > nearest.index) nearest = ent
      }
    }
    if (!nearest) continue
    const c = closes[i]
    const prev = closes[i - 1]
    const floor = nearest.low
    if (c == null || prev == null || floor == null) continue
    if (c < floor && prev >= floor) set.add(i)
  }
  return [...set].sort((a, b) => a - b)
}

/**
 * 强化买 / 趋势买 / 平台突破（需 OHLCV + 可选上证 MA20）
 * 买（展示）：出冰点且收阳，收盘>MA5、实体≥1%；MA20 下须 MA20 不走弱且 MA5≥MA10
 * 强买：买 + 冰间隔≥4日 + 收阳>MA5 + 放量≥1.25×均量 + 上证MA20上 + 个股>MA20且MA20走平/向上 + 2日不破位确认
 * 趋买：上升趋势中回踩 MA5/MA10 收阳（RSI 45~68）；收盘距 MA5 不超过上限（默认 3%，0=不限制）
 * 趋次日确认（trendRequireNextDayYang）：回踩日满足条件后，须下一交易日收阳才在次日 K 线标「趋」
 * 趋过滤：均线粘合 / 近端有止·减 / 窄幅横盘时不标（chopFilterEnabled）
 * 转：收盘上穿 MA60（可配）且前段多数在均线下，MA20 走平/向上，收阳（转势启动，非回踩）
 * 突：箱体整理后放量突破前高，且突破后 3 日收盘站稳箱体上沿（过滤假突破）
 * 弹：卖后连续 2 日收盘站稳 MA20 确认（近 6 日内曾「卖」，第 2 日标弹）
 */
export function computeFullSignals(bars, options = {}) {
  const {
    closes = [],
    opens = [],
    highs = [],
    lows = [],
    volumes = [],
    dayKeys = [],
    indexMa20ByDay = null,
    lookback = 5,
    iceMinGap = 4,
    requireIndexBull = true,
    volPeriod = 5,
    iceThreshold = 30,
    /** 强买：成交量 ≥ 近均量 × 该倍数 */
    strictVolMult = 1.25,
    /** 强买：出「强」前需连续站稳的交易日数（不含买点日） */
    strongConfirmDays = 2,
    requireStockAboveMa20 = true,
    requireMa20Rising = true,
  } = bars || {}

  const base = computeTradeSignals(closes, { lookback, iceThreshold, ...options })
  const len = closes.length
  const ma5 = sma(closes, 5)
  const ma10 = sma(closes, 10)
  const strictBuy = []
  const trendBuy = []
  const strictSet = new Set()
  const buyMinBodyPct = options.buyMinBodyPct ?? 0.01

  function passesBuyDisplayFilter(i) {
    if (!opens?.length) return true
    const o = opens[i]
    const c = closes[i]
    if (o == null || c == null || c <= o) return false
    if (o <= 0 || (c - o) / o < buyMinBodyPct) return false

    const m5v = ma5[i]
    if (m5v == null || c <= m5v) return false

    const m20v = base.ma20[i]
    const m20prev = i > 0 ? base.ma20[i - 1] : null
    if (m20v != null && c < m20v) {
      if (m20prev == null || m20v < m20prev) return false
      const m10v = ma10[i]
      if (m10v == null || m5v < m10v) return false
    }
    return true
  }

  const filteredBuy = base.buy.filter(passesBuyDisplayFilter)

  const strongConfirm = Math.max(0, Math.floor(Number(strongConfirmDays) || 0))

  function holdsAfterBuy(buyIdx, throughIdx) {
    const floor = lows[buyIdx] ?? closes[buyIdx]
    if (floor == null) return false
    for (let j = buyIdx + 1; j <= throughIdx; j++) {
      const c = closes[j]
      if (c == null || c < floor) return false
    }
    return true
  }

  function passesStrictBuyDay(buyIdx) {
    if (!filteredBuy.includes(buyIdx)) return false
    if (lastIceBefore(base.iceEnter, buyIdx, iceMinGap) == null) return false

    const o = opens[buyIdx]
    const c = closes[buyIdx]
    if (o == null || c == null || c <= o) return false
    if (ma5[buyIdx] == null || c <= ma5[buyIdx]) return false

    const vma = volMa(volumes, volPeriod, buyIdx)
    if (vma != null && vma > 0 && (volumes[buyIdx] || 0) < vma * strictVolMult) return false
    if (requireIndexBull && !indexBullishOnDay(dayKeys[buyIdx], indexMa20ByDay)) return false

    if (requireStockAboveMa20) {
      const m20 = base.ma20[buyIdx]
      if (m20 == null || c <= m20) return false
    }
    if (requireMa20Rising) {
      const m20 = base.ma20[buyIdx]
      const m20Prev = buyIdx > 0 ? base.ma20[buyIdx - 1] : null
      if (m20 == null || m20Prev == null || m20 < m20Prev) return false
    }
    return true
  }

  for (const buyIdx of filteredBuy) {
    const confirmEnd = buyIdx + strongConfirm
    if (confirmEnd >= len) continue
    if (!passesStrictBuyDay(buyIdx)) continue
    if (strongConfirm > 0 && !holdsAfterBuy(buyIdx, confirmEnd)) continue

    let dup = false
    for (const b of strictBuy) {
      if (buyIdx - b < iceMinGap) {
        dup = true
        break
      }
    }
    if (!dup) {
      strictBuy.push(buyIdx)
      strictSet.add(buyIdx)
    }
  }

  const trendSellSet = new Set([...(base.sellRsi || []), ...(base.sellMa20 || [])])
  const trendChopCtx = { options, ma5, ma10, closes, highs, lows, sellSet: trendSellSet }

  for (let i = 20; i < len; i++) {
    if (filteredBuy.includes(i) || strictSet.has(i)) continue
    const trendRsiMin = options.trendRsiMin ?? 45
    const trendRsiMax = options.trendRsiMax ?? 68
    const trendTouchMaPct = options.trendTouchMaPct ?? 0.015
    const trendMaxCloseAboveMa5Pct = options.trendMaxCloseAboveMa5Pct ?? 0.03
    const trendMinUpBars = options.trendMinUpBars ?? 3
    const trendMinGap = options.trendMinGap ?? 5
    const r = base.rsi[i]
    if (r == null || r < trendRsiMin || r > trendRsiMax) continue
    if (ma5[i] == null || ma10[i] == null || base.ma20[i] == null) continue
    if (closes[i - 1] <= base.ma20[i - 1] || ma5[i - 1] <= ma10[i - 1]) continue
    let upBars = 0
    for (let j = Math.max(0, i - 4); j < i; j++) {
      if (closes[j] > base.ma20[j]) upBars++
    }
    if (upBars < trendMinUpBars) continue
    const low = lows[i] ?? closes[i]
    const touchMa5 = low <= ma5[i] * (1 + trendTouchMaPct)
    const touchMa10 = low <= ma10[i] * (1 + trendTouchMaPct)
    if (!touchMa5 && !touchMa10) continue
    const o = opens[i]
    const c = closes[i]
    if (o == null || c == null || c <= o || c <= ma5[i]) continue
    if (trendMaxCloseAboveMa5Pct > 0 && ma5[i] > 0) {
      if ((c - ma5[i]) / ma5[i] > trendMaxCloseAboveMa5Pct) continue
    }
    if (!passesTrendChopFilter(i, trendChopCtx)) continue

    const trendRequireNextDayYang = options.trendRequireNextDayYang === true
    let markIdx = i
    if (trendRequireNextDayYang) {
      const next = i + 1
      if (next >= len) continue
      const nOpen = opens[next]
      const nClose = closes[next]
      if (nOpen == null || nClose == null || nClose <= nOpen) continue
      markIdx = next
    }

    let dup = false
    for (const t of trendBuy) {
      if (markIdx - t < trendMinGap) {
        dup = true
        break
      }
    }
    if (!dup) trendBuy.push(markIdx)
  }

  const breakoutBuy = computeBreakoutSignals(
    { closes, opens, highs, lows, volumes },
    options,
  )

  const skipForReversal = new Set([
    ...filteredBuy,
    ...strictBuy,
    ...trendBuy,
    ...breakoutBuy,
  ])
  const reversalBuy = computeReversalSignals(
    { closes, opens, highs, lows, volumes },
    base,
    skipForReversal,
    options,
  )

  const skipForRebound = new Set([
    ...filteredBuy,
    ...strictBuy,
    ...trendBuy,
    ...reversalBuy,
    ...breakoutBuy,
  ])
  const reboundBuy = computeReboundAfterSell(
    { closes, opens, highs, lows },
    base,
    skipForRebound,
    {
      sellLookback: options.reboundSellLookback ?? 6,
      minReboundPct: options.reboundMinPct ?? 0.05,
      minBodyPct: options.reboundMinBodyPct ?? 0.015,
      maxRsi: options.reboundMaxRsi ?? 60,
      minGap: options.reboundMinGap ?? 6,
      buyIndices: filteredBuy,
      buyAdjacentDays: options.reboundBuyAdjacentDays ?? 2,
      minDaysAfterSell: options.reboundMinDaysAfterSell ?? 1,
      maxUpperWickRatio: options.reboundMaxUpperWickRatio ?? 0.5,
      confirmDays: options.reboundConfirmDays ?? 2,
    },
  )

  const last = len - 1
  let latestStatus = { ...base.latestStatus }
  if (last >= 0) {
    const rl = base.rsi[last]
    const overbought = options.overbought ?? 70
    if (rl != null && rl >= iceThreshold && rl <= overbought) {
      let daysSinceBuy = null
      for (let i = last; i >= 0; i--) {
        if (filteredBuy.includes(i)) {
          daysSinceBuy = last - i
          break
        }
      }
      if (daysSinceBuy != null && daysSinceBuy <= 15) {
        latestStatus = {
          ...latestStatus,
          text: `出冰点 ${daysSinceBuy} 个交易日前 · RSI ${rl.toFixed(1)}`,
          type: 'success',
        }
      } else if (String(base.latestStatus?.text || '').includes('出冰点')) {
        latestStatus = {
          ...latestStatus,
          text: `常态 · RSI ${rl.toFixed(1)}`,
          type: 'default',
        }
      }
    }

    const recentStrict = strictBuy.filter((i) => last - i <= 15).pop()
    const recentTrend = trendBuy.filter((i) => last - i <= 15).pop()
    const recentReversal = reversalBuy.filter((i) => last - i <= 15).pop()
    const recentBreakout = breakoutBuy.filter((i) => last - i <= 15).pop()
    const recentRebound = reboundBuy.filter((i) => last - i <= 15).pop()
    if (recentStrict != null) {
      const days = last - recentStrict
      latestStatus = {
        ...latestStatus,
        text: `${days === 0 ? '今日' : `${days}日前`}强化买点 · RSI ${base.latestStatus.rsi?.toFixed(1) ?? '—'}`,
        type: 'success',
      }
    } else if (recentTrend != null) {
      const days = last - recentTrend
      const trendConfirmNote = options.trendRequireNextDayYang ? ' · 次日确认' : ''
      latestStatus = {
        ...latestStatus,
        text: `${days === 0 ? '今日' : `${days}日前`}趋势买点${trendConfirmNote} · RSI ${base.latestStatus.rsi?.toFixed(1) ?? '—'}`,
        type: 'success',
      }
    } else if (recentReversal != null) {
      const days = last - recentReversal
      latestStatus = {
        ...latestStatus,
        text: `${days === 0 ? '今日' : `${days}日前`}转势买点 · RSI ${base.latestStatus.rsi?.toFixed(1) ?? '—'}`,
        type: 'success',
      }
    } else if (recentBreakout != null) {
      const days = last - recentBreakout
      latestStatus = {
        ...latestStatus,
        text: `${days === 0 ? '今日' : `${days}日前`}平台突破 · RSI ${base.latestStatus.rsi?.toFixed(1) ?? '—'}`,
        type: 'success',
      }
    } else if (recentRebound != null) {
      const days = last - recentRebound
      latestStatus = {
        ...latestStatus,
        text: `${days === 0 ? '今日' : `${days}日前`}卖后站稳MA20 · RSI ${base.latestStatus.rsi?.toFixed(1) ?? '—'}`,
        type: 'warning',
      }
    }
  }

  const sellProtectByType = resolveSellProtectDays(options)
  const entryGroups = collectEntryBarGroups(filteredBuy, strictBuy, trendBuy, reversalBuy, breakoutBuy, reboundBuy)
  const entryMeta = buildEntryMetaList(entryGroups, lows, closes, sellProtectByType)
  const entryLowLookback = options.sellBreakEntryLowLookback ?? options.recentBuyDays ?? 15

  let sellMa20 = [...(base.sellMa20 || [])]
  sellMa20 = augmentEarlyReduceSignals(
    sellMa20,
    { closes, opens, highs, lows },
    base.ma20,
    options,
  )
  if (options.sellBreakEntryLowEnablesReduce !== false) {
    sellMa20 = augmentReduceByEntryLowBreak(sellMa20, closes, entryMeta, entryLowLookback)
  }
  sellMa20 = filterSellSignalsAfterEntry(sellMa20, entryMeta, closes, {
    sellKind: 'reduce',
    sellBreakEntryLowEnablesReduce: options.sellBreakEntryLowEnablesReduce !== false,
  })
  sellMa20 = dedupeSignalIndices(sellMa20, options.sellReduceMinGap ?? 8)
  let sellRsi = dedupeSignalIndices(
    filterSellSignalsAfterEntry(base.sellRsi || [], entryMeta, closes, {
      sellKind: 'takeProfit',
      sellBreakEntryLowEnablesReduce: false,
    }),
    options.sellTakeProfitMinGap ?? 8,
  )
  sellRsi = filterTakeProfitWithProfitGate(sellRsi, entryMeta, closes, options)

  return {
    ...base,
    buy: filteredBuy,
    ma5,
    ma10,
    strictBuy,
    trendBuy,
    reversalBuy,
    breakoutBuy,
    reboundBuy,
    sellRsi,
    sellMa20,
    latestStatus,
  }
}

/** 近期（列表/弹窗用）信号所在 K 线索引 */
export function findRecentSignalBar(sig, lastIndex, tag, options = {}) {
  if (!sig || lastIndex < 0 || !tag) return null
  const recentBuyDays = options.recentBuyDays ?? 15
  const recentSellDays = options.recentSellDays ?? 5
  const strongConfirmDays = options.strongConfirmDays ?? 2
  const breakoutConfirmDays = options.breakoutConfirmDays ?? options.confirmDays ?? 3

  function withinRecent(barIdx) {
    const daysAgo = lastIndex - barIdx
    return daysAgo >= 0 && daysAgo <= recentBuyDays
  }

  if (tag === SELL_TAG_REDUCE) {
    for (let i = lastIndex; i >= Math.max(0, lastIndex - recentSellDays); i--) {
      if ((sig.sellMa20 || []).includes(i)) {
        return { index: i, tag: SELL_TAG_REDUCE, daysAgo: lastIndex - i }
      }
    }
    return null
  }
  if (tag === SELL_TAG_TAKE_PROFIT) {
    const reduceSet = new Set(sig.sellMa20 || [])
    for (let i = lastIndex; i >= Math.max(0, lastIndex - recentSellDays); i--) {
      if ((sig.sellRsi || []).includes(i) && !reduceSet.has(i)) {
        return { index: i, tag: SELL_TAG_TAKE_PROFIT, daysAgo: lastIndex - i }
      }
    }
    return null
  }
  if (tag === '卖') {
    const reduce = findRecentSignalBar(sig, lastIndex, SELL_TAG_REDUCE, options)
    if (reduce) return reduce
    return findRecentSignalBar(sig, lastIndex, SELL_TAG_TAKE_PROFIT, options)
  }
  if (tag === '强') {
    for (let i = (sig.strictBuy || []).length - 1; i >= 0; i--) {
      const buyIdx = sig.strictBuy[i]
      const confirmedAt = buyIdx + strongConfirmDays
      if (withinRecent(confirmedAt)) {
        return { index: buyIdx, tag: '强', daysAgo: lastIndex - confirmedAt, confirmedAt }
      }
    }
    return null
  }
  if (tag === '趋') {
    for (let i = (sig.trendBuy || []).length - 1; i >= 0; i--) {
      const idx = sig.trendBuy[i]
      if (withinRecent(idx)) {
        return { index: idx, tag: '趋', daysAgo: lastIndex - idx }
      }
    }
    return null
  }
  if (tag === '转') {
    for (let i = (sig.reversalBuy || []).length - 1; i >= 0; i--) {
      const idx = sig.reversalBuy[i]
      if (withinRecent(idx)) {
        return { index: idx, tag: '转', daysAgo: lastIndex - idx }
      }
    }
    return null
  }
  if (tag === '突') {
    for (let i = (sig.breakoutBuy || []).length - 1; i >= 0; i--) {
      const breakIdx = sig.breakoutBuy[i]
      const confirmedAt = breakIdx + breakoutConfirmDays
      if (withinRecent(confirmedAt)) {
        return { index: breakIdx, tag: '突', daysAgo: lastIndex - confirmedAt, confirmedAt }
      }
    }
    return null
  }
  if (tag === '弹') {
    for (let i = (sig.reboundBuy || []).length - 1; i >= 0; i--) {
      const idx = sig.reboundBuy[i]
      if (withinRecent(idx)) {
        return { index: idx, tag: '弹', daysAgo: lastIndex - idx }
      }
    }
    return null
  }
  if (tag === '买') {
    for (let i = (sig.buy || []).length - 1; i >= 0; i--) {
      const idx = sig.buy[i]
      if (withinRecent(idx)) {
        return { index: idx, tag: '买', daysAgo: lastIndex - idx }
      }
    }
    return null
  }
  return null
}

/** 按优先级取当前应展示的主信号（与列表标签一致；见 SIGNAL_PRIORITY_SUMMARY） */
export function pickPrimaryRecentSignal(sig, lastIndex, options = {}) {
  const includeSell = options.includeSell !== false
  for (const tag of SIGNAL_TAG_PRIORITY) {
    if (!includeSell && (tag === SELL_TAG_REDUCE || tag === SELL_TAG_TAKE_PROFIT)) continue
    const hit = findRecentSignalBar(sig, lastIndex, tag, options)
    if (hit) return hit
  }
  return null
}

/** 信号综合评分 0–100（标签优先级 + RSI + 信号时效） */
export function calcSignalScore(summary) {
  if (!summary?.ok || !summary.tag) return null
  const baseByTag = { 强: 92, 趋: 84, 加: 86, 转: 80, 突: 78, 弹: 72, 买: 68, 冰: 52, 减: 16, 止: 28, 冲: 22, 卖: 22 }
  const base = baseByTag[summary.tag]
  if (base == null) return null
  let score = base
  const days = summary.recentSignalDaysAgo
  if (days != null && days > 0) {
    score -= Math.min(12, days * 3)
  }
  const rsi = summary.latestStatus?.rsi
  if (Number.isFinite(rsi) && !isSellSignalTag(summary.tag) && summary.tag !== '卖') {
    if (rsi < 25) score += 8
    else if (rsi < 30) score += 5
    else if (rsi < 35) score += 2
  }
  return Math.max(0, Math.min(100, Math.round(score)))
}

/** 策略结果：强化买 / 趋势买 / 买点（可选含卖点） */
export function summarizeBuySignal(bars, options = {}) {
  const recentBuyDays = options.recentBuyDays ?? 15
  const recentSellDays = options.recentSellDays ?? 5
  const includeSell = options.includeSell !== false
  const strongConfirmDays = options.strongConfirmDays ?? 2
  const breakoutConfirmDays = options.breakoutConfirmDays ?? options.confirmDays ?? 3
  const payload = Array.isArray(bars)
    ? {
        closes: bars,
        opens: [],
        highs: [],
        lows: [],
        volumes: [],
        dayKeys: [],
        indexMa20ByDay: options.indexMa20ByDay ?? null,
      }
    : {
        closes: bars?.closes ?? [],
        opens: bars?.opens ?? [],
        highs: bars?.highs ?? [],
        lows: bars?.lows ?? [],
        volumes: bars?.volumes ?? [],
        dayKeys: bars?.dayKeys ?? [],
        indexMa20ByDay: options.indexMa20ByDay ?? bars?.indexMa20ByDay ?? null,
      }
  const sig = computeFullSignals(payload, options)
  const closes = payload.closes
  let last = closes.length - 1
  if (options.signalLastIndex != null && options.signalLastIndex >= 0) {
    last = Math.min(options.signalLastIndex, last)
  }
  const primary = pickPrimaryRecentSignal(sig, last, {
    recentBuyDays,
    recentSellDays,
    includeSell,
    strongConfirmDays,
    breakoutConfirmDays,
  })
  const recentStrict = primary?.tag === '强' ? primary.index : null
  const recentTrend = primary?.tag === '趋' ? primary.index : null
  const recentReversal = primary?.tag === '转' ? primary.index : null
  const recentBreakout = primary?.tag === '突' ? primary.index : null
  const recentRebound = primary?.tag === '弹' ? primary.index : null
  const recentBuy = primary?.tag === '买' ? primary.index : null
  const recentReduce = includeSell && primary?.tag === SELL_TAG_REDUCE ? primary.index : null
  const recentTakeProfit = includeSell && primary?.tag === SELL_TAG_TAKE_PROFIT ? primary.index : null
  const recentSell = recentReduce ?? recentTakeProfit

  const inIce = sig.latestStatus?.type === 'info'
  let tag = primary?.tag || ''
  let tagType = 'default'
  let sellPositionPct = null
  let statusText = sig.latestStatus?.text || '—'
  if (includeSell && recentReduce != null) {
    const days = primary?.daysAgo ?? last - recentReduce
    tag = SELL_TAG_REDUCE
    tagType = 'error'
    sellPositionPct = calcSellPositionPct(tag, sig, recentReduce, payload, options)
    const hint = sellPositionHint(tag, sellPositionPct)
    statusText = `${days === 0 ? '今日' : `${days}日前`}减 · 破 MA20 · RSI ${sig.latestStatus?.rsi?.toFixed(1) ?? '—'}${hint ? ` · ${hint}` : ''}`
  } else if (includeSell && recentTakeProfit != null) {
    const days = primary?.daysAgo ?? last - recentTakeProfit
    tag = SELL_TAG_TAKE_PROFIT
    tagType = 'warning'
    sellPositionPct = calcSellPositionPct(tag, sig, recentTakeProfit, payload, options)
    const hint = sellPositionHint(tag, sellPositionPct)
    statusText = `${days === 0 ? '今日' : `${days}日前`}止 · RSI 回落 · RSI ${sig.latestStatus?.rsi?.toFixed(1) ?? '—'}${hint ? ` · ${hint}` : ''}`
  } else if (recentStrict != null) {
    tag = '强'
    tagType = 'success'
    const days = primary?.daysAgo ?? last - recentStrict
    statusText = `${days === 0 ? '今日' : `${days}日前`}强化买点 · RSI ${sig.latestStatus?.rsi?.toFixed(1) ?? '—'}`
  } else if (recentTrend != null) {
    tag = '趋'
    tagType = 'warning'
    const days = primary?.daysAgo ?? last - recentTrend
    const trendConfirmNote = options.trendRequireNextDayYang ? ' · 次日确认' : ''
    statusText = `${days === 0 ? '今日' : `${days}日前`}趋势买点${trendConfirmNote} · RSI ${sig.latestStatus?.rsi?.toFixed(1) ?? '—'}`
  } else if (recentReversal != null) {
    tag = '转'
    tagType = 'success'
    const days = primary?.daysAgo ?? last - recentReversal
    statusText = `${days === 0 ? '今日' : `${days}日前`}转势买点 · RSI ${sig.latestStatus?.rsi?.toFixed(1) ?? '—'}`
  } else if (recentBreakout != null) {
    tag = '突'
    tagType = 'success'
    const days = primary?.daysAgo ?? last - recentBreakout
    statusText = `${days === 0 ? '今日' : `${days}日前`}平台突破 · RSI ${sig.latestStatus?.rsi?.toFixed(1) ?? '—'}`
  } else if (recentRebound != null) {
    tag = '弹'
    tagType = 'warning'
    const days = primary?.daysAgo ?? last - recentRebound
    statusText = `${days === 0 ? '今日' : `${days}日前`}卖后站稳MA20 · RSI ${sig.latestStatus?.rsi?.toFixed(1) ?? '—'}`
  } else if (recentBuy != null) {
    tag = '买'
    tagType = 'success'
    const days = primary?.daysAgo ?? last - recentBuy
    statusText = `${days === 0 ? '今日' : `${days}日前`}出冰点买点 · RSI ${sig.latestStatus?.rsi?.toFixed(1) ?? '—'}`
  } else if (inIce) {
    tag = '冰'
    tagType = 'info'
    statusText = sig.latestStatus?.text || '冰点区内'
  }

  const summary = {
    ...sig,
    ok: true,
    hasRecentBuy:
      recentStrict != null ||
      recentTrend != null ||
      recentReversal != null ||
      recentBreakout != null ||
      recentRebound != null ||
      recentBuy != null,
    hasRecentStrictBuy: recentStrict != null,
    hasRecentTrendBuy: recentTrend != null,
    hasRecentReversal: recentReversal != null,
    hasRecentBreakout: recentBreakout != null,
    hasRecentRebound: recentRebound != null,
    hasRecentBuyBasic: recentBuy != null,
    hasRecentSell: recentSell != null,
    hasRecentReduce: recentReduce != null,
    hasRecentTakeProfit: recentTakeProfit != null,
    signalLastIndex: last,
    effectiveSignalDayKey: payload.dayKeys?.[last] ? normalizeDayKey(payload.dayKeys[last]) : '',
    recentSignalBar: primary?.index ?? null,
    recentSignalConfirmBar: primary?.confirmedAt ?? primary?.index ?? null,
    recentSignalDaysAgo: primary?.daysAgo ?? null,
    inIce,
    tag,
    tagType,
    sellPositionPct,
    statusText,
    sortRank: signalSortRank(tag || (inIce ? '冰' : '')),
  }
  summary.signalScore = calcSignalScore(summary)
  summary.buyPriceRange = calcBuyPriceRange(summary, payload, options)
  return summary
}
