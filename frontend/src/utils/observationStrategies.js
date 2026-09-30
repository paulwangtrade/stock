/**
 * 内置观察策略（日线筛）。
 * 观察名单 ≠ 交易指令：默认 feedsTradePlan=false，打开定时也不会自动进入模拟交易计划。
 * Track-A / 自动下单保持关闭。数据不足或条件不成立时 fail-closed 跳过。
 *
 * 与冰点模板（ice_buy / ice_watch / ice_technical）分开，也不复制 ext_xsmom_v1
 * （仓库里没有该预设）。
 */

export const OBSERVATION_QUERY_TYPE = 'observation'

/** 用户打开「定时观察」且未填表达式时使用；仅注册 cron，不改 feedsTradePlan。 */
export const OBSERVATION_CRON_EXPR = '0 20 15 * * 1-5'

export const OBSERVATION_STRATEGY_IDS = {
  maPullback: 'ext_ma_pullback',
  volBreakout: 'ext_vol_breakout',
  ddBounce: 'ext_dd_bounce',
}

const MA_PULLBACK = {
  maPeriod: 20,
  slopeLookback: 5,
  touchPct: 0.02,
  pullbackWindow: 3,
  maxPiercePct: 0.03,
}

const VOL_BREAKOUT = {
  lookback: 20,
  volPeriod: 20,
  volMult: 1.5,
}

const DD_BOUNCE = {
  peakLookback: 20,
  minDrawdown: 0.08,
  maxDrawdown: 0.18,
  iceLookback: 60,
  rsiPeriod: 14,
  /** 与冰点阈值对齐：RSI 低于 30 视为冰点重叠，跳过。 */
  iceRsi: 30,
}

export const OBSERVATION_STRATEGIES = [
  {
    strategyId: OBSERVATION_STRATEGY_IDS.maPullback,
    name: '均线趋势回踩',
    blurb:
      '日线收盘站在上升的 20 日均线之上，且近 3 日内低点曾靠近该均线后收回阳线。只作观察名单，不是买卖指令。K 线不足、价格无效或均线未上升时跳过。',
    feedsTradePlan: false,
    enableCron: false,
    observationOnly: true,
    trackA: false,
    queryType: OBSERVATION_QUERY_TYPE,
  },
  {
    strategyId: OBSERVATION_STRATEGY_IDS.volBreakout,
    name: '放量突破确认',
    blurb:
      '收盘价突破此前 20 日最高价，且当日成交量高于此前 20 日均量的 1.5 倍。只作观察名单，不是买卖指令。成交量缺失、为零或未放大时跳过。',
    feedsTradePlan: false,
    enableCron: false,
    observationOnly: true,
    trackA: false,
    queryType: OBSERVATION_QUERY_TYPE,
  },
  {
    strategyId: OBSERVATION_STRATEGY_IDS.ddBounce,
    name: '受控回撤反弹',
    blurb:
      '自近 20 日高点回撤约 8%–18% 后出现阳线反弹，且不是 60 日新低、RSI 不低于 30。只作观察名单，不是买卖指令。回撤过深、过浅，或与冰点新低/超卖重叠时跳过。',
    feedsTradePlan: false,
    enableCron: false,
    observationOnly: true,
    trackA: false,
    queryType: OBSERVATION_QUERY_TYPE,
  },
]

const BY_ID = new Map(OBSERVATION_STRATEGIES.map((item) => [item.strategyId, item]))

export function isObservationStrategyId(strategyId) {
  return BY_ID.has(String(strategyId || '').trim())
}

export function observationStrategyById(strategyId) {
  return BY_ID.get(String(strategyId || '').trim()) || null
}

export function parseObservationMeta(raw) {
  let obj = raw
  if (typeof raw === 'string') {
    const text = raw.trim()
    if (!text) return { strategyId: '', feedsTradePlan: false, observationOnly: true, trackA: false }
    try {
      obj = JSON.parse(text)
    } catch {
      return { strategyId: '', feedsTradePlan: false, observationOnly: true, trackA: false }
    }
  }
  if (!obj || typeof obj !== 'object') {
    return { strategyId: '', feedsTradePlan: false, observationOnly: true, trackA: false }
  }
  return {
    strategyId: String(obj.strategyId || obj.strategy_id || '').trim(),
    feedsTradePlan: obj.feedsTradePlan === true,
    observationOnly: obj.observationOnly !== false,
    trackA: false,
  }
}

export function observationStrategyIdFromRow(row) {
  if (!row || typeof row !== 'object') return ''
  return parseObservationMeta(row.queryJson || row.queryJSON || '').strategyId
}

/**
 * 持久化到「我的策略」。打开定时只改 enable / cronExpr，不把 feedsTradePlan 设为 true。
 */
export function buildObservationStrategyPayload(def, row, patch = {}) {
  const base = observationStrategyById(def?.strategyId) || def
  const current = parseObservationMeta(row?.queryJson || '')
  const enable = patch.enable != null ? !!patch.enable : !!(row?.enable)
  const feedsTradePlan = patch.feedsTradePlan != null ? patch.feedsTradePlan === true : current.feedsTradePlan === true
  const cronExpr = enable
    ? String(row?.cronExpr || '').trim() || OBSERVATION_CRON_EXPR
    : String(row?.cronExpr || '').trim()
  return {
    id: Number(row?.id) || 0,
    name: String(row?.name || base?.name || '').trim(),
    queryType: OBSERVATION_QUERY_TYPE,
    queryText: '',
    queryJson: JSON.stringify({
      strategyId: base.strategyId,
      feedsTradePlan,
      observationOnly: true,
      trackA: false,
    }),
    keyword: '',
    industry: '',
    cronExpr,
    enable,
    pageSize: Number(row?.pageSize) > 0 ? Number(row.pageSize) : 50,
    description: base?.blurb || '',
  }
}

function finitePositive(value) {
  return Number.isFinite(value) && value > 0
}

function alignedLength(bars, { requireVolume = false } = {}) {
  const n = bars?.closes?.length || 0
  if (!n) return 0
  if (!Array.isArray(bars.opens) || bars.opens.length !== n) return -1
  if (!Array.isArray(bars.highs) || bars.highs.length !== n) return -1
  if (!Array.isArray(bars.lows) || bars.lows.length !== n) return -1
  if (requireVolume && (!Array.isArray(bars.volumes) || bars.volumes.length !== n)) return -1
  return n
}

function smaAt(values, index, period) {
  if (index < period - 1) return null
  let sum = 0
  for (let j = 0; j < period; j++) {
    const v = values[index - j]
    if (!finitePositive(v)) return null
    sum += v
  }
  return sum / period
}

function skip(reason) {
  return { pass: false, reason, statusText: '' }
}

function pass(def) {
  return {
    pass: true,
    reason: '',
    statusText: `${def.name} · 观察名单 · 不是交易指令`,
  }
}

function screenMaPullback(bars) {
  const n = alignedLength(bars)
  if (n < 0) return skip('invalid_bars')
  const need = MA_PULLBACK.maPeriod + MA_PULLBACK.slopeLookback
  if (n <= need) return skip('insufficient_bars')
  const i = n - 1
  const ma = smaAt(bars.closes, i, MA_PULLBACK.maPeriod)
  const maPrev = smaAt(bars.closes, i - MA_PULLBACK.slopeLookback, MA_PULLBACK.maPeriod)
  if (ma == null || maPrev == null) return skip('ma_unavailable')
  if (!(ma > maPrev)) return skip('ma_not_rising')
  const close = bars.closes[i]
  const open = bars.opens[i]
  if (!finitePositive(close) || !finitePositive(open)) return skip('invalid_bar')
  if (!(close > ma)) return skip('close_not_above_ma')
  if (!(close > open)) return skip('not_recovery_bar')
  let touched = false
  for (let k = 0; k < MA_PULLBACK.pullbackWindow; k++) {
    const idx = i - k
    const maK = smaAt(bars.closes, idx, MA_PULLBACK.maPeriod)
    const lowK = bars.lows[idx]
    if (maK == null || !finitePositive(lowK)) return skip('invalid_bar')
    const upper = maK * (1 + MA_PULLBACK.touchPct)
    const lower = maK * (1 - MA_PULLBACK.maxPiercePct)
    if (lowK <= upper && lowK >= lower) touched = true
  }
  if (!touched) return skip('no_pullback')
  return pass(observationStrategyById(OBSERVATION_STRATEGY_IDS.maPullback))
}

function screenVolBreakout(bars) {
  const n = alignedLength(bars, { requireVolume: true })
  if (n < 0) return skip('invalid_bars')
  const need = Math.max(VOL_BREAKOUT.lookback, VOL_BREAKOUT.volPeriod) + 1
  if (n < need) return skip('insufficient_bars')
  const i = n - 1
  const close = bars.closes[i]
  const high = bars.highs[i]
  const vol = bars.volumes[i]
  if (!finitePositive(close) || !finitePositive(high) || !finitePositive(vol)) return skip('invalid_bar')
  let priorHigh = -Infinity
  for (let j = i - VOL_BREAKOUT.lookback; j < i; j++) {
    const h = bars.highs[j]
    if (!finitePositive(h)) return skip('invalid_bar')
    if (h > priorHigh) priorHigh = h
  }
  if (!(close > priorHigh)) return skip('no_breakout')
  let volSum = 0
  for (let j = i - VOL_BREAKOUT.volPeriod; j < i; j++) {
    const v = bars.volumes[j]
    if (!finitePositive(v)) return skip('volume_missing')
    volSum += v
  }
  const avg = volSum / VOL_BREAKOUT.volPeriod
  if (!(avg > 0) || !(vol > avg * VOL_BREAKOUT.volMult)) return skip('volume_not_confirmed')
  return pass(observationStrategyById(OBSERVATION_STRATEGY_IDS.volBreakout))
}

function rsiAt(closes, index, period) {
  if (index < period) return null
  let gain = 0
  let loss = 0
  for (let j = 0; j < period; j++) {
    const newer = closes[index - j]
    const older = closes[index - j - 1]
    if (!finitePositive(newer) || !finitePositive(older)) return null
    const ch = newer - older
    if (ch >= 0) gain += ch
    else loss -= ch
  }
  const avgGain = gain / period
  const avgLoss = loss / period
  if (avgLoss === 0) return 100
  return 100 - 100 / (1 + avgGain / avgLoss)
}

function screenDdBounce(bars) {
  const n = alignedLength(bars)
  if (n < 0) return skip('invalid_bars')
  if (n < DD_BOUNCE.iceLookback) return skip('insufficient_bars')
  const i = n - 1
  const close = bars.closes[i]
  const open = bars.opens[i]
  const low = bars.lows[i]
  const prev = bars.closes[i - 1]
  if (!finitePositive(close) || !finitePositive(open) || !finitePositive(low) || !finitePositive(prev)) {
    return skip('invalid_bar')
  }
  let peak = -Infinity
  for (let j = i - DD_BOUNCE.peakLookback; j < i; j++) {
    const h = bars.highs[j]
    if (!finitePositive(h)) return skip('invalid_bar')
    if (h > peak) peak = h
  }
  if (!(peak > 0)) return skip('peak_unavailable')
  const drawdown = (peak - close) / peak
  if (drawdown < DD_BOUNCE.minDrawdown) return skip('drawdown_too_shallow')
  if (drawdown > DD_BOUNCE.maxDrawdown) return skip('drawdown_too_deep')
  if (!(close > prev) || !(close > open)) return skip('not_bounce_bar')
  let priorMin = Infinity
  for (let j = i - (DD_BOUNCE.iceLookback - 1); j < i; j++) {
    const l = bars.lows[j]
    if (!finitePositive(l)) return skip('invalid_bar')
    if (l < priorMin) priorMin = l
  }
  if (low <= priorMin) return skip('new_low_overlap')
  const rsi = rsiAt(bars.closes, i, DD_BOUNCE.rsiPeriod)
  if (rsi == null) return skip('rsi_unavailable')
  if (rsi < DD_BOUNCE.iceRsi) return skip('ice_rsi_overlap')
  return pass(observationStrategyById(OBSERVATION_STRATEGY_IDS.ddBounce))
}

const SCREENS = {
  [OBSERVATION_STRATEGY_IDS.maPullback]: screenMaPullback,
  [OBSERVATION_STRATEGY_IDS.volBreakout]: screenVolBreakout,
  [OBSERVATION_STRATEGY_IDS.ddBounce]: screenDdBounce,
}

/** 在最后一根日线上评估。不通过时 pass=false，reason 为跳过原因。 */
export function evaluateObservationStrategy(strategyId, bars) {
  const id = String(strategyId || '').trim()
  const screen = SCREENS[id]
  if (!screen) return skip('unknown_strategy')
  return screen(bars || {})
}

export function runObservationScanBatch(strategyId, stocks) {
  const id = String(strategyId || '').trim()
  const items = []
  if (!isObservationStrategyId(id)) return { items, hitTotal: 0 }
  for (const stock of stocks || []) {
    const result = evaluateObservationStrategy(id, {
      opens: stock?.opens,
      highs: stock?.highs,
      lows: stock?.lows,
      closes: stock?.closes,
      volumes: stock?.volumes,
    })
    if (!result.pass) continue
    const row = stock?.row || {}
    const last = (stock?.closes?.length || 1) - 1
    const close = stock?.closes?.[last]
    items.push({
      SECUCODE: row.SECUCODE || stock.secucode || stock.code,
      SECURITY_CODE: row.SECURITY_CODE || '',
      SECURITY_NAME_ABBR: row.SECURITY_NAME_ABBR || stock.name || '',
      NEW_PRICE: row.NEW_PRICE ?? (Number.isFinite(close) ? String(close) : ''),
      CHANGE_RATE: row.CHANGE_RATE ?? '',
      HIGH_PRICE: row.HIGH_PRICE ?? '',
      LOW_PRICE: row.LOW_PRICE ?? '',
      PRE_CLOSE_PRICE: row.PRE_CLOSE_PRICE ?? '',
      VOLUME: row.VOLUME ?? '',
      DEAL_AMOUNT: row.DEAL_AMOUNT ?? '',
      TURNOVERRATE: row.TURNOVERRATE ?? '',
      VOLUME_RATIO: row.VOLUME_RATIO ?? '',
      INDUSTRY: row.INDUSTRY ?? '',
      CONCEPT: row.CONCEPT ?? '',
      MARKET: row.MARKET ?? '',
      tag: '买',
      recentSignalDaysAgo: 0,
      statusText: result.statusText,
      sortRank: 1,
      rsi: null,
      strategy_id: id,
      ok: true,
    })
  }
  return { items, hitTotal: items.length }
}
