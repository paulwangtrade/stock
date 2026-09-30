/**
 * 次日 setup 观察（T 收盘 → T+1 一步之遥）。
 *
 * 只读观察，不是已确认信号，不生成交易计划或委托。
 * 与收盘快照 hits 分开：本模块的结果不得写入 SignalScanHit。
 *
 * 价 vs 缺口（见 NEXT_DAY_SETUP_TAG_SUPPORT）：
 * - 突 breakout_high：可给出参考价（箱体上沿 × (1+缓冲)）
 * - 弹 ma20_reclaim：可给出参考价（次日收盘 ≥ 次日 MA20 的唯一解）
 * - 趋 / 强 / 买 / 转：路径、截面或 RSI，无法给出唯一价（本期不按股输出）
 */

import {
  computeBreakoutSignals,
  computeReboundAfterSell,
  computeTradeSignals,
} from './icePointSignals.js'

export const NEXT_DAY_SETUP_DISCLAIMER = '若触及可能形成，不保证，非买卖指令'

export const NEXT_DAY_SETUP_UNAVAILABLE = '无法给出唯一价'

/** 距 T 收盘超过该比例则不算「一步」（默认 5%） */
export const NEXT_DAY_SETUP_MAX_DISTANCE_PCT = 0.05

export const NEXT_DAY_SETUP_ENGINE_LABEL = {
  breakout_high: '突破前高',
  ma20_reclaim: '收回 MA20',
}

/**
 * 哪些标签能给出参考价。active=true 的才会在 T 收盘扫描里按股输出。
 * price：唯一参考价；gap / unavailable：无法给出唯一价。
 */
export const NEXT_DAY_SETUP_TAG_SUPPORT = [
  {
    tag: '突',
    engine: 'breakout_high',
    priceMode: 'price',
    active: true,
    note: '参考价 = 近箱体上沿 × (1+突破缓冲)，由 T 及之前的高点唯一确定。放量、收阳、上影、确认站稳只作条件缺口。均线未在 T 收盘排好多头时不列入。',
  },
  {
    tag: '弹',
    engine: 'ma20_reclaim',
    priceMode: 'price',
    active: true,
    note: '还差 1 个收盘站上 MA20、且近端已有止/减时，参考价 = 滚入次日的近 19 日收盘均值（收盘 ≥ 次日 MA20 的唯一解）。RSI、实体、上影无法给出唯一价。',
  },
  {
    tag: '趋',
    engine: 'trend_next_day',
    priceMode: 'gap',
    active: false,
    note: '回踩触及均线、收阳与 RSI 区间绑在同一根 K 线上，无法给出唯一价。',
  },
  {
    tag: '强',
    engine: '',
    priceMode: 'unavailable',
    active: false,
    note: '冰点路径、指数环境与确认日，无法给出唯一价。',
  },
  {
    tag: '买',
    engine: '',
    priceMode: 'unavailable',
    active: false,
    note: 'RSI 上穿路径，无法给出唯一价。',
  },
  {
    tag: '转',
    engine: '',
    priceMode: 'unavailable',
    active: false,
    note: '上穿均线可逆，但 RSI 与均线下占比无法给出唯一价。',
  },
]

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

function smaSeries(closes, period) {
  const out = new Array(closes.length).fill(null)
  for (let i = period - 1; i < closes.length; i++) {
    out[i] = smaAt(closes, period, i)
  }
  return out
}

function roundPrice(v) {
  return Math.round(v * 10000) / 10000
}

function maxDistancePct(options) {
  const n = Number(options?.setupMaxDistancePct)
  if (Number.isFinite(n) && n > 0) return n
  return NEXT_DAY_SETUP_MAX_DISTANCE_PCT
}

function baseWatch(partial) {
  return {
    confirmed: false,
    orderIntent: false,
    observationOnly: true,
    disclaimer: NEXT_DAY_SETUP_DISCLAIMER,
    statusText: '未确认 · 次日观察',
    triggerPrice: null,
    distancePct: null,
    conditionGaps: [],
    gapText: '',
    ...partial,
  }
}

/**
 * 非价规则的失败关闭结果（不按股扫描；供对照与测试）。
 * 即使调用方塞了数字，也不保留触发价。
 */
export function unavailableSetup(tag) {
  const row = NEXT_DAY_SETUP_TAG_SUPPORT.find((item) => item.tag === tag)
  return baseWatch({
    engine: row?.engine || '',
    tag: tag || '',
    priceMode: 'unavailable',
    triggerPrice: null,
    gapText: NEXT_DAY_SETUP_UNAVAILABLE,
    summary: row?.note || NEXT_DAY_SETUP_UNAVAILABLE,
    conditionGaps: [NEXT_DAY_SETUP_UNAVAILABLE],
  })
}

export function formatSetupTrigger(row) {
  if (!row || row.priceMode !== 'price') return NEXT_DAY_SETUP_UNAVAILABLE
  const px = Number(row.triggerPrice)
  if (!Number.isFinite(px) || px <= 0) return NEXT_DAY_SETUP_UNAVAILABLE
  return px.toFixed(2)
}

export function formatSetupDistance(row) {
  if (!row || row.priceMode !== 'price') return '—'
  const d = Number(row.distancePct)
  if (!Number.isFinite(d)) return '—'
  const abs = Math.abs(d * 100).toFixed(2)
  if (d > 0) return `还差 ${abs}%`
  if (d < 0) return `收盘已高于参考价 ${abs}%`
  return '与参考价持平'
}

export function priceModeLabel(mode) {
  if (mode === 'price') return '可给出参考价'
  if (mode === 'gap') return '仅条件缺口'
  return NEXT_DAY_SETUP_UNAVAILABLE
}

/**
 * 突：T+1 若收盘站上「截至 T 的箱体上沿 × (1+缓冲)」，才进入突破日。
 * 箱体用 T+1 突破日会回看的那一段（含 T）。
 */
export function evaluateBreakoutHighSetup(bars, options = {}) {
  const closes = bars?.closes || []
  const opens = bars?.opens || []
  const highs = bars?.highs || []
  const lows = bars?.lows || []
  const len = closes.length
  const i = len - 1
  const boxPeriod = Math.max(2, Math.floor(Number(options.boxPeriod) || 20))
  const maxRangePct = Number(options.maxRangePct ?? 0.2)
  const breakBuffer = Number(options.breakBuffer ?? 0.005)
  if (i < boxPeriod - 1) return null

  const lookStart = i + 1 - boxPeriod
  const lookEnd = i
  if (lookStart < 0) return null

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
  const mid = (boxHigh + boxLow) / 2
  if (!(mid > 0) || !(boxHigh > boxLow)) return null
  if ((boxHigh - boxLow) / mid > maxRangePct) return null

  const trigger = boxHigh * (1 + (Number.isFinite(breakBuffer) ? breakBuffer : 0))
  const close = closes[i]
  if (!(close > 0) || !(trigger > close)) return null
  const distancePct = (trigger - close) / close
  if (distancePct > maxDistancePct(options)) return null

  const ma5 = smaAt(closes, 5, i)
  const ma10 = smaAt(closes, 10, i)
  const ma20 = smaAt(closes, 20, i)
  if (ma5 == null || ma10 == null || ma5 < ma10) return null
  if (ma20 != null && ma10 < ma20 * 0.995) return null

  const minGap = Math.max(1, Math.floor(Number(options.minGap) || 15))
  const confirmed = computeBreakoutSignals(
    { closes, opens, highs, lows, volumes: bars?.volumes || [] },
    options,
  )
  for (const b of confirmed) {
    if (i + 1 - b < minGap) return null
  }

  const volPeriod = Math.max(1, Math.floor(Number(options.volPeriod) || 5))
  const volMult = options.volMult ?? 1.25
  const confirm = Math.max(0, Math.floor(Number(options.confirmDays ?? options.breakoutConfirmDays) || 0))
  const gaps = [
    '次日须收阳（收盘 > 开盘），开盘在 T 收盘未知',
    `次日成交量须 ≥ 近 ${volPeriod} 日均量 × ${volMult}（含次日量，无法在 T 锁定）`,
    '突破日上影线占比须达标',
    '均线多头按 T 收盘核对，次日均线会随收盘变动',
  ]
  if (confirm > 0) {
    gaps.push(`已确认「突」还须突破后再站稳 ${confirm} 日；本行只是突破参考价，不是已确认信号`)
  }

  return baseWatch({
    engine: 'breakout_high',
    tag: '突',
    priceMode: 'price',
    triggerPrice: roundPrice(trigger),
    closeT: close,
    distancePct,
    asOfIndex: i,
    conditionGaps: gaps,
    gapText: gaps.join('；'),
    summary: `次日收盘站上 ${roundPrice(trigger)} 可能进入平台突破日`,
  })
}

/**
 * 弹：连续 confirmDays 日收盘 ≥ MA20 的前一日。
 * 参考价是次日收盘不低于次日 MA20 的唯一解。
 * 近端须已有止/减（与弹相同的卖后前提）；RSI 等仍是缺口。
 */
export function evaluateMa20ReclaimSetup(bars, options = {}) {
  const closes = bars?.closes || []
  const opens = bars?.opens || []
  const highs = bars?.highs || []
  const lows = bars?.lows || []
  const len = closes.length
  const i = len - 1
  const period = Math.max(2, Math.floor(Number(options.maPeriod) || 20))
  const confirm = Math.max(1, Math.floor(Number(options.reboundConfirmDays) || 2))
  const need = confirm - 1
  if (i < period || i < need) return null

  const ma = smaSeries(closes, period)
  const above = (j) => closes[j] != null && ma[j] != null && closes[j] >= ma[j]

  if (need === 0) {
    if (above(i)) return null
  } else {
    for (let k = 0; k < need; k++) {
      if (!above(i - k)) return null
    }
    const before = i - need
    if (before < 0 || above(before)) return null
  }

  const base = computeTradeSignals(closes, { ...options, maPeriod: period })
  const sellSet = new Set([...(base.sellRsi || []), ...(base.sellMa20 || [])])
  const sellLookback = Math.max(1, Math.floor(Number(options.reboundSellLookback) || 6))
  const minDays = Math.max(0, Math.floor(Number(options.reboundMinDaysAfterSell) || 1))
  const signalDay = i + 1
  let sellIdx = null
  for (let j = signalDay - 1; j >= Math.max(0, signalDay - sellLookback); j--) {
    if (sellSet.has(j)) {
      sellIdx = j
      break
    }
  }
  if (sellIdx == null || signalDay - sellIdx < minDays) return null
  const standStart = need > 0 ? i - need + 1 : signalDay
  if (need > 0 && standStart <= sellIdx) return null

  const from = i - (period - 2)
  if (from < 0) return null
  let sum = 0
  for (let j = from; j <= i; j++) {
    if (closes[j] == null || !Number.isFinite(closes[j])) return null
    sum += closes[j]
  }
  const trigger = sum / (period - 1)
  const close = closes[i]
  if (!(trigger > 0) || !(close > 0)) return null
  const distancePct = (trigger - close) / close
  if (distancePct > maxDistancePct(options)) return null

  const rebound = computeReboundAfterSell(
    { closes, opens, highs, lows },
    base,
    new Set(),
    {
      sellLookback,
      minDaysAfterSell: minDays,
      confirmDays: confirm,
      minReboundPct: options.reboundMinPct ?? 0.05,
      minBodyPct: options.reboundMinBodyPct ?? 0.015,
      maxRsi: options.reboundMaxRsi ?? 60,
      minGap: options.reboundMinGap ?? 6,
      maxUpperWickRatio: options.reboundMaxUpperWickRatio ?? 0.5,
    },
  )
  if (rebound.includes(i)) return null

  const gaps = [
    '次日须收阳且实体达标',
    '次日 RSI、MA5≥MA10、MA20 走平/向上、上影线无法在 T 收盘收成唯一价',
  ]
  if (distancePct <= 0) {
    gaps.push('参考价是次日收盘不低于该价才补上站稳；跌破则这一步不成立')
  } else {
    gaps.push('参考价是次日收盘达到该价才回到 MA20 之上')
  }

  const px = roundPrice(trigger)
  const summary =
    distancePct > 0
      ? `次日收盘 ≥ ${px} 可能补上 MA20 站稳的最后一日`
      : `次日收盘不低于 ${px} 可能补上 MA20 站稳的最后一日`

  return baseWatch({
    engine: 'ma20_reclaim',
    tag: '弹',
    priceMode: 'price',
    triggerPrice: px,
    closeT: close,
    distancePct,
    asOfIndex: i,
    conditionGaps: gaps,
    gapText: gaps.join('；'),
    summary,
  })
}

/** 本期只跑可逆出唯一价的两则：突破前高、收回 MA20。 */
export function evaluateNextDaySetups(bars, options = {}) {
  const out = []
  const breakout = evaluateBreakoutHighSetup(bars, options)
  if (breakout) out.push(breakout)
  const reclaim = evaluateMa20ReclaimSetup(bars, options)
  if (reclaim) out.push(reclaim)
  return out
}
