/**
 * 日频信号回测引擎（MVP）
 * - 输入：日 K bars + 按日信号回调
 * - 成本：佣金、印花税、滑点、整手、T+1
 * - 可选：5 级总仓位上限（按日 modeKey）
 */

import { MARKET_MODE_EXPOSURE_FACTOR, resolveTradingLevel } from './tradingLevelRules.js'

function clamp(n, min, max) {
  return Math.max(min, Math.min(max, n))
}

function roundLot(shares, lot = 100) {
  return Math.floor(shares / Math.max(1, lot)) * Math.max(1, lot)
}

export const DEFAULT_BACKTEST_COST = {
  commissionRate: 0.00025,
  minCommission: 5,
  stampTaxRate: 0.0005, // 卖出
  slippageRate: 0.001,
  lotSize: 100,
  tPlusOne: true,
}

/**
 * @param {object} opts
 * @param {Array<{day:string, open:number, high:number, low:number, close:number}>} opts.bars
 * @param {(i:number, bar:object, bars:object[]) => ({tag?:string, action?:'buy'|'sell', strength?:number}|null)} opts.signalAt
 * @param {(i:number, bar:object) => string} [opts.marketModeAt] - 返回 level1..level5
 * @param {number} [opts.initialCash]
 * @param {object} [opts.cost]
 * @param {boolean} [opts.usePositionDiscipline] - 叠加 5 级仓位上限
 * @param {Record<string, number>} [opts.levelMaxPct] - modeKey -> max exposure
 * @param {number} [opts.maxPositionPct] - 单票上限
 */
export function runDailySignalBacktest(opts) {
  const bars = opts.bars || []
  const signalAt = opts.signalAt
  const marketModeAt = opts.marketModeAt || (() => 'level4')
  const cash0 = Number(opts.initialCash) || 1_000_000
  const cost = { ...DEFAULT_BACKTEST_COST, ...(opts.cost || {}) }
  const useDiscipline = !!opts.usePositionDiscipline
  const levelMaxPct = opts.levelMaxPct || MARKET_MODE_EXPOSURE_FACTOR
  const maxPosPct = Number(opts.maxPositionPct) || 0.15

  let cash = cash0
  let shares = 0
  let avgCost = 0
  let pendingSellable = 0 // T+1：昨日买入今日可卖
  let boughtToday = 0
  const equityCurve = []
  const trades = []
  let peak = cash0
  let maxDrawdown = 0

  const buyFee = (amount) => Math.max(cost.minCommission, amount * cost.commissionRate)
  const sellFee = (amount) =>
    Math.max(cost.minCommission, amount * cost.commissionRate) + amount * cost.stampTaxRate

  for (let i = 0; i < bars.length; i++) {
    const bar = bars[i]
    const close = Number(bar.close)
    if (!(close > 0)) continue

    // 日初：昨日买入转为可卖
    if (cost.tPlusOne) {
      pendingSellable += boughtToday
      boughtToday = 0
    } else {
      pendingSellable = shares
    }

    const rawModeKey = marketModeAt(i, bar) || 'unknown'
    const modeKey = resolveTradingLevel(rawModeKey).key
    const maxExp = useDiscipline ? (levelMaxPct[rawModeKey] ?? levelMaxPct[modeKey] ?? 0.2) : 1
    const equityBefore = cash + shares * close
    const sig = signalAt ? signalAt(i, bar, bars) : null

    if (sig?.action === 'buy' || (sig?.tag && !sig.action && ['买', '强', '突', '趋', '转', '弹'].includes(sig.tag))) {
      const strength = clamp(Number(sig.strength) || 0.5, 0.2, 1)
      const px = close * (1 + cost.slippageRate)
      const roomValue = Math.max(0, equityBefore * maxExp - shares * close)
      const maxByPos = equityBefore * maxPosPct * strength
      const budget = Math.min(cash, roomValue, maxByPos)
      let qty = roundLot(budget / px, cost.lotSize)
      while (qty >= cost.lotSize) {
        const amount = qty * px
        const fee = buyFee(amount)
        if (amount + fee > cash) {
          qty -= cost.lotSize
          continue
        }
        cash -= amount + fee
        const newShares = shares + qty
        avgCost = newShares > 0 ? (avgCost * shares + amount) / newShares : 0
        shares = newShares
        boughtToday += qty
        trades.push({ day: bar.day, side: 'buy', price: px, qty, fee, modeKey, tag: sig.tag || 'buy' })
        break
      }
    }

    if (sig?.action === 'sell' || (sig?.tag && ['止', '减', '冲'].includes(sig.tag))) {
      const sellable = cost.tPlusOne ? pendingSellable : shares
      const pct = clamp(Number(sig.sellPct) || (sig.tag === '止' ? 1 : 0.5), 0.1, 1)
      let qty = roundLot(sellable * pct, cost.lotSize)
      if (qty > sellable) qty = roundLot(sellable, cost.lotSize)
      if (qty >= cost.lotSize && shares >= qty) {
        const px = close * (1 - cost.slippageRate)
        const amount = qty * px
        const fee = sellFee(amount)
        cash += amount - fee
        shares -= qty
        pendingSellable = Math.max(0, pendingSellable - qty)
        if (shares <= 0) {
          shares = 0
          avgCost = 0
        }
        trades.push({ day: bar.day, side: 'sell', price: px, qty, fee, modeKey, tag: sig.tag || 'sell' })
      }
    }

    // 级别 1：强制清仓可卖部分
    if (useDiscipline && modeKey === 'level1' && shares > 0) {
      const sellable = cost.tPlusOne ? pendingSellable : shares
      let qty = roundLot(sellable, cost.lotSize)
      if (qty >= cost.lotSize) {
        const px = close * (1 - cost.slippageRate)
        const amount = qty * px
        const fee = sellFee(amount)
        cash += amount - fee
        shares -= qty
        pendingSellable = Math.max(0, pendingSellable - qty)
        trades.push({ day: bar.day, side: 'sell', price: px, qty, fee, modeKey, tag: 'level1_force' })
      }
    }

    const equity = cash + shares * close
    peak = Math.max(peak, equity)
    const dd = peak > 0 ? (peak - equity) / peak : 0
    maxDrawdown = Math.max(maxDrawdown, dd)
    equityCurve.push({ day: bar.day, equity, cash, shares, close, modeKey })
  }

  const finalEquity = equityCurve.length ? equityCurve[equityCurve.length - 1].equity : cash0
  const totalReturn = cash0 > 0 ? (finalEquity - cash0) / cash0 : 0
  const buyTrades = trades.filter((t) => t.side === 'buy')
  const sellTrades = trades.filter((t) => t.side === 'sell')

  // 简化夏普：日收益标准差
  const rets = []
  for (let i = 1; i < equityCurve.length; i++) {
    const prev = equityCurve[i - 1].equity
    const cur = equityCurve[i].equity
    if (prev > 0) rets.push((cur - prev) / prev)
  }
  const mean = rets.length ? rets.reduce((a, b) => a + b, 0) / rets.length : 0
  const variance = rets.length
    ? rets.reduce((a, b) => a + (b - mean) ** 2, 0) / rets.length
    : 0
  const std = Math.sqrt(variance)
  const sharpe = std > 0 ? (mean / std) * Math.sqrt(252) : 0

  return {
    ok: true,
    initialCash: cash0,
    finalEquity: Math.round(finalEquity * 100) / 100,
    totalReturn,
    totalReturnPct: Math.round(totalReturn * 10000) / 100,
    maxDrawdown,
    maxDrawdownPct: Math.round(maxDrawdown * 10000) / 100,
    sharpe: Math.round(sharpe * 100) / 100,
    tradeCount: trades.length,
    buyCount: buyTrades.length,
    sellCount: sellTrades.length,
    trades,
    equityCurve,
  }
}

/** 将东财 K 线数组转为 bars */
export function eastMoneyKLinesToBars(klineList) {
  const list = Array.isArray(klineList) ? klineList : klineList ? [...klineList] : []
  return list
    .map((k) => ({
      day: String(k.Day || k.day || '').slice(0, 10),
      open: Number(k.Open ?? k.open),
      high: Number(k.High ?? k.high),
      low: Number(k.Low ?? k.low),
      close: Number(k.Close ?? k.close),
      volume: Number(k.Volume ?? k.volume) || 0,
    }))
    .filter((b) => b.day && b.close > 0)
}
