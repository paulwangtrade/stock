const LOT_SIZE = 100
const MAX_T_OUT_RATIO = 0.25

function finite(value, fallback = 0) {
  const n = Number(value)
  return Number.isFinite(n) ? n : fallback
}

function valueOf(row, ...keys) {
  for (const key of keys) {
    if (row?.[key] !== undefined && row?.[key] !== null && row?.[key] !== '') return row[key]
  }
  return undefined
}

function timeKey(value) {
  const text = String(value || '').trim().replace(/\//g, '-')
  const matched = text.match(/(\d{4})-(\d{1,2})-(\d{1,2})[ T](\d{1,2}):(\d{2})/)
  if (matched) {
    return Number(
      `${matched[1]}${matched[2].padStart(2, '0')}${matched[3].padStart(2, '0')}${matched[4].padStart(2, '0')}${matched[5]}`,
    )
  }
  return Number(text.replace(/\D/g, '').slice(0, 12)) || 0
}

export function normalizeIntradayBars(rows = []) {
  return (Array.isArray(rows) ? rows : [])
    .map((row) => {
      const day = String(valueOf(row, 'day', 'Day', 'time', 'Time') || '')
      const close = finite(valueOf(row, 'close', 'Close', 'price', 'Price'), NaN)
      if (!day || !Number.isFinite(close) || close <= 0) return null
      const open = finite(valueOf(row, 'open', 'Open'), close)
      const high = finite(valueOf(row, 'high', 'High'), Math.max(open, close))
      const low = finite(valueOf(row, 'low', 'Low'), Math.min(open, close))
      return {
        day,
        open,
        high,
        low,
        close,
        volume: Math.max(0, finite(valueOf(row, 'volume', 'Volume'))),
        amount: Math.max(0, finite(valueOf(row, 'amount', 'Amount'))),
      }
    })
    .filter(Boolean)
    .sort((a, b) => timeKey(a.day) - timeKey(b.day))
}

export function calculateVWAP(rows = []) {
  let totalAmount = 0
  let totalVolume = 0
  for (const row of rows) {
    const volume = Math.max(0, finite(row?.volume))
    if (!volume) continue
    const amount = Math.max(0, finite(row?.amount))
    const typical = (finite(row?.high, row?.close) + finite(row?.low, row?.close) + finite(row?.close)) / 3
    // 东方财富 K 线常见 volume=手、amount=元；也兼容测试/其他源的 volume=股。
    const impliedScale = typical > 0 ? amount / (typical * volume) : 1
    const normalizedAmount = impliedScale >= 50 && impliedScale <= 150 ? amount / 100 : amount
    totalAmount += normalizedAmount > 0 ? normalizedAmount : typical * volume
    totalVolume += volume
  }
  return totalVolume > 0 ? totalAmount / totalVolume : null
}

export function calculateMA(values = [], period = 5) {
  if (!Array.isArray(values) || values.length < period || period <= 0) return null
  const sample = values.slice(-period).map(Number)
  if (!sample.every(Number.isFinite)) return null
  return sample.reduce((sum, value) => sum + value, 0) / period
}

/** Wilder RSI；不足 period + 1 根时返回 null。 */
export function calculateRSI(values = [], period = 14) {
  if (!Array.isArray(values) || values.length < period + 1 || period <= 0) return null
  const sample = values.map(Number)
  if (!sample.every(Number.isFinite)) return null
  let gains = 0
  let losses = 0
  for (let i = 1; i <= period; i++) {
    const change = sample[i] - sample[i - 1]
    if (change > 0) gains += change
    else losses -= change
  }
  let averageGain = gains / period
  let averageLoss = losses / period
  for (let i = period + 1; i < sample.length; i++) {
    const change = sample[i] - sample[i - 1]
    averageGain = (averageGain * (period - 1) + Math.max(0, change)) / period
    averageLoss = (averageLoss * (period - 1) + Math.max(0, -change)) / period
  }
  if (averageLoss === 0) return averageGain > 0 ? 100 : 50
  const rs = averageGain / averageLoss
  return 100 - 100 / (1 + rs)
}

export function roundToBoardLot(volume, lotSize = LOT_SIZE) {
  const lot = Math.max(1, Math.floor(finite(lotSize, LOT_SIZE)))
  return Math.max(0, Math.floor(finite(volume) / lot) * lot)
}

export function isIntradayTWindow(value) {
  const text = value instanceof Date
    ? new Intl.DateTimeFormat('zh-CN', {
        timeZone: 'Asia/Shanghai',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false,
      }).format(value)
    : String(value || '')
  const matched = text.match(/(\d{1,2}):(\d{2})/)
  if (!matched) return false
  const minute = Number(matched[1]) * 60 + Number(matched[2])
  return (minute >= 10 * 60 && minute <= 11 * 60 + 30)
    || (minute >= 13 * 60 && minute <= 14 * 60 + 30)
}

export function isContinuousAuction(value) {
  const text = value instanceof Date
    ? new Intl.DateTimeFormat('zh-CN', {
        timeZone: 'Asia/Shanghai',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false,
      }).format(value)
    : String(value || '')
  const matched = text.match(/(\d{1,2}):(\d{2})/)
  if (!matched) return false
  const minute = Number(matched[1]) * 60 + Number(matched[2])
  return (minute >= 9 * 60 + 30 && minute <= 11 * 60 + 30)
    || (minute >= 13 * 60 && minute <= 15 * 60)
}

function dayKey(value) {
  const matched = String(value || '').replace(/\//g, '-').match(/(\d{4})-(\d{1,2})-(\d{1,2})/)
  return matched ? `${matched[1]}-${matched[2].padStart(2, '0')}-${matched[3].padStart(2, '0')}` : ''
}

function latestTimeLabel(day) {
  const time = String(day || '').match(/\d{1,2}:\d{2}(?::\d{2})?/)?.[0]
  return time ? (time.split(':').length === 2 ? `${time}:00` : time) : '--:--:--'
}

function pctDistance(value, base) {
  return base > 0 ? (value - base) / base : 0
}

export function computeIntradayTMetrics(rawRows = []) {
  const bars = normalizeIntradayBars(rawRows)
  if (!bars.length) return { bars: [], todayBars: [] }
  const latest = bars[bars.length - 1]
  const latestDay = dayKey(latest.day)
  const todayBars = bars.filter((bar) => dayKey(bar.day) === latestDay)
  const closes = todayBars.map((bar) => bar.close)
  const volumes = todayBars.map((bar) => bar.volume)
  const previousVolumes = volumes.slice(Math.max(0, volumes.length - 21), -1).filter((v) => v > 0)
  const averageVolume = previousVolumes.length
    ? previousVolumes.reduce((sum, value) => sum + value, 0) / previousVolumes.length
    : 0
  return {
    bars,
    todayBars,
    latest,
    date: latestDay,
    time: latestTimeLabel(latest.day),
    price: latest.close,
    vwap: calculateVWAP(todayBars),
    ma5: calculateMA(closes, 5),
    rsi14: calculateRSI(closes, 14),
    intradayHigh: Math.max(...todayBars.map((bar) => bar.high)),
    intradayLow: Math.min(...todayBars.map((bar) => bar.low)),
    volumeRatio: averageVolume > 0 ? latest.volume / averageVolume : null,
  }
}

/**
 * 纯分钟做 T 提示引擎。state 仅代表页面中由用户记录的当日已执行量。
 * @returns {{ action: 'T出'|'T入'|'观察', referencePrice: number|null, suggestedVolume: number, confidence: number, reasons: string[], invalidation: string }}
 */
export function evaluateIntradayTSignal({
  rows = [],
  holdingVolume = 0,
  sellableVolume = null,
  costPrice = 0,
  state = {},
  now,
} = {}) {
  const metrics = computeIntradayTMetrics(rows)
  const costProfitPct = finite(costPrice) > 0 && Number.isFinite(metrics.price)
    ? (metrics.price / finite(costPrice) - 1) * 100
    : null
  const base = {
    action: '观察',
    triggerTime: metrics.time || '--:--',
    referencePrice: metrics.price ?? null,
    suggestedVolume: 0,
    confidence: 0,
    reasons: [],
    invalidation: '指标条件变化或超出对应连续竞价提示时段后失效',
    requiresSellableConfirmation: sellableVolume == null,
    metrics: { ...metrics, costProfitPct },
  }
  if (!metrics.latest || !Number.isFinite(metrics.vwap) || !Number.isFinite(metrics.ma5) || !Number.isFinite(metrics.rsi14)) {
    return { ...base, reasons: ['当日 5 分钟数据不足，至少需要 15 根有效价格数据'] }
  }

  const inWindow = isIntradayTWindow(now || metrics.latest.day)
  const inContinuousAuction = isContinuousAuction(now || metrics.latest.day)
  const rounds = Math.max(0, Math.floor(finite(state.rounds)))
  const soldQty = roundToBoardLot(state.soldQty)
  const boughtBackQty = roundToBoardLot(state.boughtBackQty)
  const outstandingQty = Math.max(0, soldQty - boughtBackQty)
  const confirmedBase = sellableVolume == null
    ? finite(holdingVolume)
    : Math.min(finite(holdingVolume), finite(sellableVolume))
  const maxSellQty = roundToBoardLot(confirmedBase * MAX_T_OUT_RATIO)
  const priceVsVwap = pctDistance(metrics.price, metrics.vwap)
  const range = Math.max(0, metrics.intradayHigh - metrics.intradayLow)
  const rangePosition = range > 0 ? (metrics.price - metrics.intradayLow) / range : 0.5
  const volumeRatio = Number.isFinite(metrics.volumeRatio) ? metrics.volumeRatio : 1

  if (outstandingQty > 0) {
    if (!inWindow) {
      return { ...base, reasons: ['T入仅在 10:00-11:30、13:00-14:30 提示'] }
    }
    const checks = [
      priceVsVwap <= -0.003,
      metrics.price <= metrics.ma5,
      metrics.rsi14 <= 42,
      rangePosition <= 0.45,
    ]
    const score = checks.filter(Boolean).length
    if (score >= 3) {
      return {
        ...base,
        action: 'T入',
        suggestedVolume: outstandingQty,
        confidence: Math.min(92, 55 + score * 8 + (volumeRatio >= 1.1 ? 5 : 0)),
        reasons: [
          `现价低于 VWAP ${(Math.abs(priceVsVwap) * 100).toFixed(2)}%`,
          `RSI14 ${metrics.rsi14.toFixed(1)}，价格位于日内区间 ${Math.round(rangePosition * 100)}%`,
          `仅回补当日已记录 T 出的 ${outstandingQty} 股`,
        ],
        invalidation: `价格重新站上 VWAP ${metrics.vwap.toFixed(2)} 且 RSI14 回到 50 上方`,
      }
    }
    return {
      ...base,
      confidence: Math.min(70, 30 + score * 10),
      reasons: [`尚有 ${outstandingQty} 股待回补，但回落条件未形成`],
      invalidation: '收盘前仍未出现回补条件时，不追价并按自身交易纪律处理隔夜风险',
    }
  }

  if (rounds >= 2) {
    return { ...base, reasons: ['今日已记录两轮，达到每日上限'] }
  }
  if (maxSellQty < LOT_SIZE) {
    return { ...base, reasons: ['底仓的 25% 不足 100 股，不能给出整手 T 出量'] }
  }
  if (!inContinuousAuction) {
    return { ...base, reasons: ['当前不在连续竞价时段'] }
  }

  const checks = [
    priceVsVwap >= 0.003,
    metrics.price >= metrics.ma5,
    metrics.rsi14 >= 68,
    rangePosition >= 0.72,
    volumeRatio >= 1.05,
  ]
  const score = checks.filter(Boolean).length
  if (score >= 4 && checks[0] && checks[2]) {
    return {
      ...base,
      action: 'T出',
      suggestedVolume: maxSellQty,
      confidence: Math.min(92, 50 + score * 8),
      reasons: [
        `现价高于 VWAP ${(priceVsVwap * 100).toFixed(2)}%`,
        `RSI14 ${metrics.rsi14.toFixed(1)}，价格位于日内区间 ${Math.round(rangePosition * 100)}%`,
        `量比 ${volumeRatio.toFixed(2)}，建议量不超过底仓 25%`,
        sellableVolume == null ? '无法确认真实可卖量，请人工确认可卖' : `已按可卖数量 ${finite(sellableVolume)} 股约束`,
        finite(costPrice) > 0 ? `现价相对成本收益 ${((metrics.price / finite(costPrice) - 1) * 100).toFixed(2)}%` : '未录入有效持仓成本',
      ],
      invalidation: `价格跌回 VWAP ${metrics.vwap.toFixed(2)} 或 RSI14 跌破 60`,
    }
  }
  return {
    ...base,
    confidence: Math.min(70, 25 + score * 9),
    reasons: ['高位、VWAP、RSI 与量能条件未形成保守共振'],
  }
}

export const INTRADAY_T_LIMITS = Object.freeze({
  lotSize: LOT_SIZE,
  maxSellRatio: MAX_T_OUT_RATIO,
  maxRounds: 2,
  startTime: '10:00',
  endTime: '14:30',
})
