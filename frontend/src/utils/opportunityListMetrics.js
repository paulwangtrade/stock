/** Phase14-D/E1：机会列表信号价 / 快照价 / 现价指标（纯函数） */

/** 信号触发价列 Tooltip（frozen / 默认） */
export const SIGNAL_PRICE_TOOLTIP =
  '信号触发价：出信号 K 线收盘价，用于衡量信号以来涨跌；非买入价、非委托价、非成交价。'

/** derived / buyPriceRange 回退 */
export const SIGNAL_PRICE_DERIVED_TOOLTIP =
  '信号触发价（推算）：本次扫描由 K 线推算，非历史快照冻结价；非买入价、非委托价、非成交价。'

/** 机会视图页脚（Phase14-E1；Phase16.19-B2 对齐 priceDisplay 语义） */
export const OPPORTUNITY_PRICE_FOOTER =
  '以下价格均为行情参考，不构成买卖建议。信号触发价指出信号当日 K 线收盘价；最新行情价为展示 overlay；均与买入价、委托价、成交价无关。'

export const SIGNAL_PRICE_STATUS = {
  FROZEN: 'frozen',
  MISSING: 'missing',
  DERIVED: 'derived',
}

export function normalizeSignalPriceStatus(status) {
  const s = String(status || '').trim().toLowerCase()
  if (s === SIGNAL_PRICE_STATUS.DERIVED) return SIGNAL_PRICE_STATUS.DERIVED
  if (s === SIGNAL_PRICE_STATUS.MISSING) return SIGNAL_PRICE_STATUS.MISSING
  if (s === SIGNAL_PRICE_STATUS.FROZEN) return SIGNAL_PRICE_STATUS.FROZEN
  return s || SIGNAL_PRICE_STATUS.FROZEN
}

export function isDerivedSignalPriceStatus(status) {
  return normalizeSignalPriceStatus(status) === SIGNAL_PRICE_STATUS.DERIVED
}

export function isMissingSignalPriceStatus(status) {
  return normalizeSignalPriceStatus(status) === SIGNAL_PRICE_STATUS.MISSING
}

export function signalPriceTooltipForStatus(status) {
  return isDerivedSignalPriceStatus(status) ? SIGNAL_PRICE_DERIVED_TOOLTIP : SIGNAL_PRICE_TOOLTIP
}

/** 「信号以来涨跌」列在 signal_price 缺失时的说明 */
export function vsSignalReturnMissingHint(signalPrice, signalPriceStatus) {
  if (signalPrice != null) return null
  if (isMissingSignalPriceStatus(signalPriceStatus)) {
    return '信号价未记录，无法计算信号以来涨跌'
  }
  return '信号价缺失，无法计算信号以来涨跌'
}

export function parseNumericPrice(value) {
  if (value == null || value === '') return null
  const n = Number(String(value).replace(/,/g, ''))
  return Number.isFinite(n) && n > 0 ? n : null
}

export function formatPctWithSign(value, digits = 2) {
  if (value == null || !Number.isFinite(Number(value))) return '—'
  const n = Number(value)
  const sign = n >= 0 ? '+' : ''
  return `${sign}${n.toFixed(digits)}%`
}

export function calcReturnSinceSignal(latestPrice, signalPrice) {
  const latest = parseNumericPrice(latestPrice)
  const signal = parseNumericPrice(signalPrice)
  if (latest == null || signal == null) return null
  return ((latest - signal) / signal) * 100
}

export function normalizeTradeDate(day) {
  const s = String(day || '').trim().replace(/\//g, '-')
  const m = s.match(/^(\d{4})-(\d{2})-(\d{2})/)
  return m ? `${m[1]}-${m[2]}-${m[3]}` : s.slice(0, 10)
}

/** 从快照 hit 或 live summary 解析信号价/信号时间 */
export function resolveSignalFields(row, signalSummary) {
  const rowPrice = parseNumericPrice(row?.SIGNAL_PRICE ?? row?.signal_price)
  const rowTime = normalizeTradeDate(row?.SIGNAL_TIME ?? row?.signal_time ?? '')
  if (rowPrice != null) {
    const rawStatus = row?.SIGNAL_PRICE_STATUS ?? row?.signal_price_status ?? SIGNAL_PRICE_STATUS.FROZEN
    return {
      signalPrice: rowPrice,
      signalTime: rowTime,
      signalPriceStatus: normalizeSignalPriceStatus(rawStatus),
      signalDaysAgo: row?.signal_days_ago ?? row?.SIGNAL_DAYS_AGO ?? null,
    }
  }

  const summaryPrice = parseNumericPrice(signalSummary?.signalPrice)
  if (summaryPrice != null) {
    const summaryTime = normalizeTradeDate(signalSummary?.signalTime ?? '')
    return {
      signalPrice: summaryPrice,
      signalTime: summaryTime,
      signalPriceStatus: normalizeSignalPriceStatus(
        signalSummary?.signalPriceStatus ?? SIGNAL_PRICE_STATUS.FROZEN,
      ),
      signalDaysAgo: signalSummary?.recentSignalDaysAgo ?? signalSummary?.signal_days_ago ?? null,
    }
  }

  const range = signalSummary?.buyPriceRange
  const rangePrice = parseNumericPrice(range?.instantPrice)
  if (rangePrice != null) {
    return {
      signalPrice: rangePrice,
      signalTime: '',
      signalPriceStatus: SIGNAL_PRICE_STATUS.DERIVED,
      signalDaysAgo: signalSummary?.recentSignalDaysAgo ?? range?.daysAgo ?? null,
    }
  }

  const fallbackTime = rowTime || normalizeTradeDate(signalSummary?.signalTime ?? '')
  return {
    signalPrice: null,
    signalTime: fallbackTime,
    signalPriceStatus: normalizeSignalPriceStatus(
      row?.SIGNAL_PRICE_STATUS ??
        row?.signal_price_status ??
        signalSummary?.signalPriceStatus ??
        SIGNAL_PRICE_STATUS.MISSING,
    ),
    signalDaysAgo: signalSummary?.recentSignalDaysAgo ?? null,
  }
}

export function resolveLatestPrice(row, liveQuote, { preferLive = true } = {}) {
  const live = parseNumericPrice(liveQuote?.price)
  if (preferLive && live != null) return live
  return parseNumericPrice(row?.NEW_PRICE ?? row?.SNAPSHOT_PRICE ?? row?.LIVE_PRICE)
}

export function resolveLiveChangeRate(row, liveQuote, { isSnapshotView = false } = {}) {
  const liveRate = liveQuote?.changeRate
  if (liveRate != null && Number.isFinite(Number(liveRate))) {
    return Number(liveRate)
  }
  if (!isSnapshotView) {
    const rowRate = parseNumericPrice(row?.CHANGE_RATE)
    if (rowRate != null) return rowRate
    const raw = row?.CHANGE_RATE
    if (raw != null && raw !== '') {
      const n = Number(String(raw).replace(/,/g, ''))
      if (Number.isFinite(n)) return n
    }
  }
  return null
}

export function resolveSnapshotChange(row, snapshotTradeDate) {
  const rateRaw = row?.SNAPSHOT_CHANGE_RATE ?? row?.CHANGE_RATE
  if (rateRaw == null || rateRaw === '') return { rate: null, date: '' }
  const n = Number(String(rateRaw).replace(/,/g, ''))
  const date = normalizeTradeDate(row?.SNAPSHOT_TRADE_DATE ?? snapshotTradeDate ?? '')
  return {
    rate: Number.isFinite(n) ? n : null,
    date,
  }
}

export function enrichHitToRow(hit, snapTradeDate = '') {
  const signalPrice = hit?.signal_price ?? hit?.SignalPrice
  const signalTime = hit?.signal_time ?? hit?.SignalTime ?? ''
  return {
    SECUCODE: hit.SECUCODE,
    SECURITY_CODE: hit.SECURITY_CODE,
    SECURITY_NAME_ABBR: hit.SECURITY_NAME_ABBR,
    NEW_PRICE: hit.NEW_PRICE,
    CHANGE_RATE: hit.CHANGE_RATE,
    HIGH_PRICE: hit.HIGH_PRICE,
    LOW_PRICE: hit.LOW_PRICE,
    PRE_CLOSE_PRICE: hit.PRE_CLOSE_PRICE,
    VOLUME: hit.VOLUME,
    DEAL_AMOUNT: hit.DEAL_AMOUNT,
    TURNOVERRATE: hit.TURNOVERRATE,
    VOLUME_RATIO: hit.VOLUME_RATIO,
    INDUSTRY: hit.INDUSTRY,
    CONCEPT: hit.CONCEPT,
    MARKET: hit.MARKET,
    SIGNAL_PRICE: signalPrice,
    SIGNAL_TIME: signalTime,
    SIGNAL_PRICE_STATUS: hit?.signal_price_status ?? hit?.SignalPriceStatus ?? '',
    SNAPSHOT_PRICE: hit.NEW_PRICE,
    SNAPSHOT_CHANGE_RATE: hit.CHANGE_RATE,
    SNAPSHOT_TRADE_DATE: normalizeTradeDate(snapTradeDate),
  }
}

export function enrichHitToSummary(hit) {
  return {
    ok: true,
    tag: hit.tag,
    recentSignalDaysAgo: hit.recentSignalDaysAgo ?? hit.daysAgo ?? null,
    statusText: hit.statusText,
    sortRank: hit.sortRank ?? 0,
    latestStatus: { rsi: hit.rsi ?? null },
    signalPrice: hit?.signal_price ?? hit?.SignalPrice ?? null,
    signalTime: hit?.signal_time ?? hit?.SignalTime ?? '',
    signalPriceStatus: hit?.signal_price_status ?? hit?.SignalPriceStatus ?? '',
  }
}
