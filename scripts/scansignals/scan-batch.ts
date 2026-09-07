/**
 * 全市场信号批量扫描（stdin/stdout JSON），与前端 icePointSignals 同源。
 * 构建：npx esbuild scripts/scansignals/scan-batch.ts --bundle --platform=neutral --format=iife --global-name=SignalScanBatch --outfile=backend/data/signal_scan_bundle.js
 */
import { calcBuyPriceRange } from '../../frontend/src/utils/buyPriceRange'
import {
  buildIndexMa20ByDay,
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
}

export function runSignalScanBatch(input: ScanInput) {
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
