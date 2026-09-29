/**
 * 全市场信号批量扫描（stdin/stdout JSON），与前端 icePointSignals 同源。
 * strategyId=ext_xsmom_v1（或非 default 且 scanMode=xsmom）改走截面动量，不改冰点路径。
 * 构建：npx esbuild scripts/scansignals/scan-batch.ts --bundle --platform=neutral --format=iife --global-name=SignalScanBatch --outfile=backend/data/signal_scan_bundle.js
 */
import { calcBuyPriceRange } from '../../frontend/src/utils/buyPriceRange'
import {
  buildIndexMa20ByDay,
  calcRSI,
  summarizeBuySignal,
} from '../../frontend/src/utils/icePointSignals'
import {
  buildSignalOptions,
  cloneDefaultSignalSettings,
  parseSignalParams,
} from '../../frontend/src/utils/signalSettings'
import { SCREEN_SNAPSHOT_SIGNAL_TAG_SET } from '../../frontend/src/utils/signalTagConstants'

type StockInput = {
  code: string
  name: string
  secucode?: string
  closes: number[]
  opens: number[]
  highs: number[]
  lows: number[]
  volumes: number[]
  dayKeys: string[]
  lastBarIndex?: number
  row?: Record<string, unknown>
}

type ScanInput = {
  stocks?: StockInput[]
  indexClose?: Record<string, number>
  signalParamsJson?: string
  includeSell?: boolean
  /** 空 / default = 冰点；ext_xsmom_v1 = 截面动量。不在此写库。 */
  strategyId?: string
}

/** 观察用截面动量。禁止把该 id 写成 default，避免覆盖冰点快照。 */
export const XS_MOM_STRATEGY_ID = 'ext_xsmom_v1'
export const XS_MOM_TAG = 'XS_MOM_TOP'
/** 简单收益回看交易日：ret = close[t] / close[t-N] - 1 */
const XS_MOM_DEFAULT_LOOKBACK = 20
/** 当前扫描批次（对本策略即整份已准备名单）按收益取前 N 名 */
const XS_MOM_DEFAULT_TOP_N = 20

type XsMomParams = {
  lookback: number
  topN: number
  /** <=0 表示不按 RSI 否决。默认关闭：强动量本身常伴随高 RSI。 */
  rsiMax: number
  scanMode: string
}

function readXsMomParams(signalParamsJson?: string): XsMomParams {
  const out: XsMomParams = {
    lookback: XS_MOM_DEFAULT_LOOKBACK,
    topN: XS_MOM_DEFAULT_TOP_N,
    rsiMax: 0,
    scanMode: '',
  }
  if (!signalParamsJson) return out
  try {
    const raw = JSON.parse(signalParamsJson) as Record<string, unknown>
    const lookback = Number(raw?.xsmomLookback)
    const topN = Number(raw?.xsmomTopN)
    const rsiMax = Number(raw?.xsmomRsiMax)
    if (Number.isFinite(lookback) && lookback >= 1) out.lookback = Math.floor(lookback)
    if (Number.isFinite(topN) && topN >= 1) out.topN = Math.floor(topN)
    if (Number.isFinite(rsiMax) && rsiMax > 0) out.rsiMax = rsiMax
    out.scanMode = String(raw?.scanMode || raw?.xsmomMode || '').trim()
  } catch {
    /* 坏 JSON 用默认，不让整批扫描失败 */
  }
  return out
}

function isCrossSectionMomentumScan(strategyId: string | undefined, scanMode: string): boolean {
  const sid = String(strategyId || '').trim()
  // default / 省略必须保持冰点，即使 params 里写了动量模式。
  if (!sid || sid === 'default') return false
  if (sid === XS_MOM_STRATEGY_ID) return true
  return scanMode === XS_MOM_STRATEGY_ID || scanMode === 'xsmom'
}

function finitePositive(n: unknown): n is number {
  return typeof n === 'number' && Number.isFinite(n) && n > 0
}

function lookbackReturn(closes: number[], lastIdx: number, lookback: number): number | null {
  if (!Array.isArray(closes) || lastIdx < lookback || lastIdx >= closes.length) return null
  const now = closes[lastIdx]
  const prev = closes[lastIdx - lookback]
  if (!finitePositive(now) || !finitePositive(prev)) return null
  const ret = now / prev - 1
  if (!Number.isFinite(ret)) return null
  return ret
}

function latestRsi(closes: number[], lastIdx: number, period: number): number | null {
  if (!Array.isArray(closes) || lastIdx < 0 || period < 2) return null
  try {
    const series = calcRSI(closes.slice(0, lastIdx + 1), period)
    const v = series?.[series.length - 1]
    return typeof v === 'number' && Number.isFinite(v) ? v : null
  } catch {
    return null
  }
}

function rsiPeriodFromParams(signalParamsJson?: string): number {
  try {
    const settings = signalParamsJson ? parseSignalParams(signalParamsJson) : cloneDefaultSignalSettings()
    const p = Number(settings?.common?.rsiPeriod)
    if (Number.isFinite(p) && p >= 2) return Math.floor(p)
  } catch {
    /* default */
  }
  return 14
}

type MomCandidate = {
  stock: StockInput
  lastIdx: number
  ret: number
  rsi: number | null
  price: number
}

function runCrossSectionMomentumBatch(input: ScanInput, params: XsMomParams) {
  const stocks = input?.stocks || []
  const rsiPeriod = rsiPeriodFromParams(input?.signalParamsJson)
  const eligible: MomCandidate[] = []
  for (const s of stocks) {
    try {
      const closes = s?.closes
      if (!Array.isArray(closes) || closes.length === 0) continue
      const lastIdx =
        s.lastBarIndex != null && s.lastBarIndex >= 0
          ? Math.min(s.lastBarIndex, closes.length - 1)
          : closes.length - 1
      if (lastIdx < 0) continue
      const ret = lookbackReturn(closes, lastIdx, params.lookback)
      // 样本不足、价格非法或收益非正：跳过该股，不中断批次。
      if (ret == null || ret <= 0) continue
      const rsi = latestRsi(closes, lastIdx, rsiPeriod)
      if (params.rsiMax > 0 && rsi != null && rsi >= params.rsiMax) continue
      eligible.push({ stock: s, lastIdx, ret, rsi, price: closes[lastIdx] })
    } catch {
      continue
    }
  }
  eligible.sort((a, b) => {
    if (a.ret !== b.ret) return b.ret - a.ret
    return String(a.stock.code || '').localeCompare(String(b.stock.code || ''))
  })
  const picked = eligible.slice(0, params.topN)
  const items: Array<Record<string, unknown>> = []
  if (!SCREEN_SNAPSHOT_SIGNAL_TAG_SET.has(XS_MOM_TAG)) {
    return { items, hitTotal: 0 }
  }
  picked.forEach((c, i) => {
    const s = c.stock
    const row = s.row || {}
    const rank = i + 1
    const pct = (c.ret * 100).toFixed(2)
    const rsiText = c.rsi == null ? '—' : c.rsi.toFixed(1)
    const dayKey = s.dayKeys?.[c.lastIdx] || ''
    items.push({
      SECUCODE: row.SECUCODE || s.secucode || s.code,
      SECURITY_CODE: row.SECURITY_CODE || '',
      SECURITY_NAME_ABBR: row.SECURITY_NAME_ABBR || s.name || '',
      NEW_PRICE: row.NEW_PRICE ?? '',
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
      tag: XS_MOM_TAG,
      recentSignalDaysAgo: 0,
      statusText: `截面动量 · ${params.lookback}日收益 +${pct}% · 排名 ${rank}/${eligible.length} · RSI ${rsiText}`,
      sortRank: 1000 - i,
      rsi: c.rsi,
      schema_version: 'signal_event.v1',
      signal_price: c.price,
      signal_time: dayKey || undefined,
      signal_price_source: 'kline_close',
      signal_days_ago: 0,
      signal_bar_role: 'signal',
      signal_bar_index: c.lastIdx,
      signal_price_status: 'frozen',
      ok: true,
    })
  })
  return { items, hitTotal: items.length }
}

export function runSignalScanBatch(input: ScanInput) {
  const mom = readXsMomParams(input?.signalParamsJson)
  if (isCrossSectionMomentumScan(input?.strategyId, mom.scanMode)) {
    return runCrossSectionMomentumBatch(input, mom)
  }
  return runIcePointScanBatch(input)
}

function runIcePointScanBatch(input: ScanInput) {
  const stocks = input?.stocks || []
  const indexMa20ByDay = buildIndexMa20ByDay(new Map(Object.entries(input?.indexClose || {})), 20)
  let baseSettings = cloneDefaultSignalSettings()
  if (input?.signalParamsJson) {
    try {
      baseSettings = parseSignalParams(input.signalParamsJson)
    } catch {
      /* use default */
    }
  }
  const options = {
    ...buildSignalOptions(baseSettings),
    recentBuyDays: 0,
    recentSellDays: 0,
    includeSell: input?.includeSell !== false,
  }

  const items: Array<Record<string, unknown>> = []
  for (const s of stocks) {
    const bars = {
      closes: s.closes,
      opens: s.opens,
      highs: s.highs,
      lows: s.lows,
      volumes: s.volumes,
      dayKeys: s.dayKeys,
      indexMa20ByDay,
    }
    const lastIdx =
      s.lastBarIndex != null && s.lastBarIndex >= 0
        ? Math.min(s.lastBarIndex, s.closes.length - 1)
        : s.closes.length - 1
    if (lastIdx < 0) continue
    const summary = summarizeBuySignal(bars, { ...options, signalLastIndex: lastIdx })
    if (!summary?.tag || !SCREEN_SNAPSHOT_SIGNAL_TAG_SET.has(summary.tag)) continue
    const row = s.row || {}
    const buyRange = calcBuyPriceRange(summary, bars, { ...options, signalLastIndex: lastIdx })
    const tag = summary.tag
    const isConfirmBar = tag === '强' || tag === '突'
    const signalBarIndex =
      buyRange?.instantBar ?? summary.recentSignalConfirmBar ?? summary.recentSignalBar ?? null
    const signalDaysAgo = summary.recentSignalDaysAgo ?? buyRange?.daysAgo ?? 0
    const signalTime =
      signalBarIndex != null && s.dayKeys?.[signalBarIndex] ? s.dayKeys[signalBarIndex] : ''
    const signalPrice =
      buyRange?.instantPrice != null && Number.isFinite(buyRange.instantPrice) && buyRange.instantPrice > 0
        ? buyRange.instantPrice
        : null
    items.push({
      SECUCODE: row.SECUCODE || s.secucode || s.code,
      SECURITY_CODE: row.SECURITY_CODE || '',
      SECURITY_NAME_ABBR: row.SECURITY_NAME_ABBR || s.name || '',
      NEW_PRICE: row.NEW_PRICE ?? '',
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
      tag: summary.tag,
      recentSignalDaysAgo: summary.recentSignalDaysAgo,
      statusText: summary.statusText,
      sortRank: summary.sortRank,
      rsi: summary.latestStatus?.rsi ?? null,
      schema_version: signalPrice != null ? 'signal_event.v1' : undefined,
      signal_price: signalPrice,
      signal_time: signalTime || undefined,
      signal_price_source: signalPrice != null ? (isConfirmBar ? 'kline_close_confirm' : 'kline_close') : undefined,
      signal_days_ago: signalDaysAgo,
      signal_bar_role: signalPrice != null ? (isConfirmBar ? 'confirm' : 'signal') : undefined,
      signal_bar_index: summary.recentSignalBar ?? signalBarIndex ?? undefined,
      confirm_bar_index: isConfirmBar ? (summary.recentSignalConfirmBar ?? signalBarIndex ?? undefined) : undefined,
      signal_price_status: signalPrice != null ? 'frozen' : undefined,
      ok: true,
    })
  }
  items.sort((a, b) => {
    const ra = Number(a.sortRank) || 0
    const rb = Number(b.sortRank) || 0
    if (ra !== rb) return rb - ra
    return String(a.SECURITY_NAME_ABBR || '').localeCompare(String(b.SECURITY_NAME_ABBR || ''), 'zh-CN')
  })
  return { items, hitTotal: items.length }
}

// IIFE 导出供 Go goja / Node 调用
declare global {
  // eslint-disable-next-line no-var
  var SignalScanBatch: { runSignalScanBatch: typeof runSignalScanBatch }
}
if (typeof globalThis !== 'undefined') {
  ;(globalThis as unknown as { SignalScanBatch: { runSignalScanBatch: typeof runSignalScanBatch } }).SignalScanBatch = {
    runSignalScanBatch,
  }
}
