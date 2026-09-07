import { GetStockEastMoneyKLine, GetIndustryMoneyRankSina } from '../../wailsjs/go/main/App'
import { getSharedIndexDailyKLine } from './indexKlineCache'
import { buildIndexMa20ByDay, normalizeDayKey, summarizeBuySignal } from './icePointSignals'
import { getSignalOptions } from './signalSettingsStore'
import { buildSignalOptions } from './signalSettings'
import { resolveSignalLastBarIndex } from './tradingSession'
import { eastMoneyCodeVariants, resolveStrategyRowCode } from './stockCode'
import { applyAddPositionToSummary, hasPositionContext } from './addPositionSignals'
import { applyRushReduceToSummary } from './rushReduceSignals'
import { applyCostAwareSellToSummary } from './costAwareSellSignals'
import { fetchMarketStatusSnapshot } from './marketStatusBar'
import { matchSectorFlow, applyHoldingPositionAdjustToSummary } from './holdingPositionAdjust'
import {
  resolveEffectiveStockMarketMode,
  resolveStockMarketSegment,
} from './stockMarketSegment'
import { buildScanRiskAdviceShadow } from './riskintel/scanRiskAdviceShadow'
import { observeLegacyRiskAdvicePair } from './riskintel/scanRiskAdviceShadowEvaluation'
import { buildRiskAdviceProjection } from './riskintel/riskAdviceProjection'
import { getHoldingDecision } from './riskintel/holdingDecisionAdapter'

let indexMa20ByDayCache = null
let indexMa20FetchPromise = null
let industryMoneyRankCache = null
let industryMoneyRankFetchPromise = null
let marketSnapshotCache = { snapshot: null, at: 0 }

/** 日 K 扫描根数：按信号参数动态计算，避免无谓拉 800 根（全市场扫描主要瓶颈） */
export function resolveScanKlineBarCount(extra = {}) {
  const opts = getSignalOptions()
  const crossMa = Number(opts.reversalCrossMaPeriod) || 60
  const box = Number(opts.boxPeriod) || 20
  const maPeriod = Number(opts.maPeriod) || 20
  const need = Math.max(crossMa + 40, box + 35, maPeriod + 25, 120)
  const cap = extra.maxBars ?? 800
  return Math.min(cap, need)
}

const dailyBarsCache = new Map()
const DAILY_BARS_CACHE_MAX = 8000
const DAILY_BARS_CACHE_TTL_MS = 5 * 60 * 1000

function cacheDailyBars(key, bars) {
  if (!bars) return
  if (dailyBarsCache.size >= DAILY_BARS_CACHE_MAX) {
    const first = dailyBarsCache.keys().next().value
    dailyBarsCache.delete(first)
  }
  dailyBarsCache.set(key, { bars, at: Date.now() })
}

function daySortKey(dayStr) {
  const s = String(dayStr || '').trim().replace(/\//g, '-')
  const m = s.match(/^(\d{4})-(\d{2})-(\d{2})/)
  if (m) return Number(m[1] + m[2] + m[3])
  const c = s.match(/^(\d{4})(\d{2})(\d{2})/)
  if (c) return Number(c[1] + c[2] + c[3])
  return 0
}

export function isScannableWatchlistCode(code) {
  if (!code) return false
  const raw = String(code).trim()
  const em = resolveStrategyRowCode({ SECUCODE: raw, SECURITY_CODE: raw }) || raw
  const c = em.toLowerCase()
  if (c.startsWith('hk') || c.startsWith('gb_') || c.startsWith('us')) return false
  return c.startsWith('sh') || c.startsWith('sz') || c.startsWith('bj') || /^\d{6}(\.(sh|sz|bj))?$/i.test(c)
}

async function ensureIndexMa20ByDay() {
  if (indexMa20ByDayCache) return indexMa20ByDayCache
  if (indexMa20FetchPromise) return indexMa20FetchPromise
  indexMa20FetchPromise = (async () => {
    try {
      // 与 MarketStatusBar 共用指数日 K 单例（TTL + in-flight 去重）
      const list = await getSharedIndexDailyKLine('000001.SH', '上证指数', { barCount: 120 })
      const closeByDay = new Map()
      for (const r of list) {
        const k = normalizeDayKey(r.day)
        const c = Number(r.close)
        if (k && Number.isFinite(c)) closeByDay.set(k, c)
      }
      indexMa20ByDayCache = buildIndexMa20ByDay(closeByDay, 20)
    } catch {
      indexMa20ByDayCache = new Map()
    }
    return indexMa20ByDayCache
  })()
  return indexMa20FetchPromise
}

async function ensureIndustryMoneyRank() {
  if (industryMoneyRankCache) return industryMoneyRankCache
  if (industryMoneyRankFetchPromise) return industryMoneyRankFetchPromise
  industryMoneyRankFetchPromise = (async () => {
    try {
      const [industry, concept] = await Promise.all([
        GetIndustryMoneyRankSina('0', 'netamount'),
        GetIndustryMoneyRankSina('1', 'netamount'),
      ])
      industryMoneyRankCache = {
        industry: Array.isArray(industry) ? industry : [],
        concept: Array.isArray(concept) ? concept : [],
      }
    } catch {
      industryMoneyRankCache = { industry: [], concept: [] }
    }
    return industryMoneyRankCache
  })()
  return industryMoneyRankFetchPromise
}

async function resolveScanMarketSnapshot() {
  const now = Date.now()
  if (now - marketSnapshotCache.at < 60_000 && marketSnapshotCache.snapshot) {
    return marketSnapshotCache.snapshot
  }
  try {
    const snapshot = await fetchMarketStatusSnapshot()
    marketSnapshotCache = { snapshot, at: now }
    return snapshot
  } catch {
    return marketSnapshotCache.snapshot
  }
}

async function fetchDailyBars(code, name, barCount = resolveScanKlineBarCount()) {
  const cacheKey = `${code}|${barCount}`
  const cached = dailyBarsCache.get(cacheKey)
  if (cached && Date.now() - cached.at < DAILY_BARS_CACHE_TTL_MS) {
    return cached.bars
  }
  if (cached) dailyBarsCache.delete(cacheKey)

  const variants = eastMoneyCodeVariants(code)
  for (const emCode of variants) {
    try {
      const raw = await GetStockEastMoneyKLine(emCode, name || '', '101', barCount)
      const list = Array.isArray(raw) ? raw : []
      if (!list.length) continue
      const sorted = [...list].sort((a, b) => daySortKey(a.day) - daySortKey(b.day))
      const closes = []
      const opens = []
      const highs = []
      const lows = []
      const volumes = []
      const turnoverRates = []
      const dayKeys = []
      for (const r of sorted) {
        const c = Number(r.close)
        const o = Number(r.open)
        const h = Number(r.high)
        const l = Number(r.low)
        const v = Number(r.volume)
        const tr = Number(r.turnoverRate)
        if (!Number.isFinite(c)) continue
        closes.push(c)
        opens.push(Number.isFinite(o) ? o : c)
        highs.push(Number.isFinite(h) ? h : c)
        lows.push(Number.isFinite(l) ? l : c)
        volumes.push(Number.isFinite(v) ? v : 0)
        turnoverRates.push(Number.isFinite(tr) ? tr : 0)
        dayKeys.push(normalizeDayKey(r.day))
      }
      if (closes.length >= 20) {
        const bars = { closes, opens, highs, lows, volumes, turnoverRates, dayKeys }
        cacheDailyBars(`${code}|${barCount}`, bars)
        cacheDailyBars(`${emCode}|${barCount}`, bars)
        return bars
      }
    } catch {
      /* try next variant */
    }
  }
  return null
}

export function clearDailyBarsScanCache() {
  dailyBarsCache.clear()
}

async function runPool(tasks, concurrency = 3, onTaskDone) {
  const results = []
  let i = 0
  let completed = 0
  async function worker() {
    while (i < tasks.length) {
      const idx = i++
      results[idx] = await tasks[idx]()
      completed++
      onTaskDone?.(completed, tasks.length)
    }
  }
  const workers = Array.from({ length: Math.min(concurrency, tasks.length || 1) }, () => worker())
  await Promise.all(workers)
  return results
}

/** 批量扫描列表最后一根有效 K 线信号（买/强/趋/突/冰/卖） */
export async function scanRowsLastBarSignals(rows, options = {}) {
  const list = (rows || [])
    .map((row) => {
      const code = resolveStrategyRowCode(row) || row?.SECUCODE || row?.code
      const name = row?.SECURITY_NAME_ABBR || row?.name || ''
      return code ? { code, name, key: row?.SECUCODE || code } : null
    })
    .filter((x) => x && isScannableWatchlistCode(x.code))

  const byKey = {}
  if (!list.length) return byKey

  const indexMa20ByDay = await ensureIndexMa20ByDay()
  const barCount = options.barCount ?? resolveScanKlineBarCount()
  const signalOptions = {
    ...getSignalOptions(),
    ...(options.signalSettings ? buildSignalOptions(options.signalSettings) : {}),
    recentBuyDays: 0,
    recentSellDays: 0,
    includeSell: options.includeSell !== false,
  }

  const tasks = list.map(({ code, name, key }) => async () => {
    const bars = await fetchDailyBars(code, name, barCount)
    if (!bars) return [key, { ok: false }]
    const lastIdx = options.useAbsoluteLastBar
      ? Math.max(0, bars.dayKeys.length - 1)
      : resolveSignalLastBarIndex(bars.dayKeys)
    const summary = summarizeBuySignal(
      { ...bars, indexMa20ByDay },
      { ...signalOptions, signalLastIndex: lastIdx },
    )
    return [key, { ok: true, ...summary }]
  })

  const onProgress = options.onProgress
  const pairs = await runPool(tasks, options.concurrency ?? 4, (done, total) => {
    onProgress?.({ phase: 'scan', done, total })
  })
  for (const pair of pairs) {
    if (pair?.[0]) byKey[pair[0]] = pair[1]
  }
  return byKey
}

/** 扫描自选列表，返回有冰/买/强/趋/突/卖提示的股票 */
export async function scanWatchlistSignals(stocks, options = {}) {
  const list = (stocks || []).filter((s) => s?.code && isScannableWatchlistCode(s.code))
  if (!list.length) {
    return { hints: [], byCode: {} }
  }

  const indexMa20ByDay = await ensureIndexMa20ByDay()
  const signalOptions = {
    ...getSignalOptions(),
    recentBuyDays: options.recentBuyDays ?? 0,
    recentSellDays: options.recentSellDays ?? 0,
  }

  const hasAnyPosition = Object.values(options.positionsByCode || {}).some(hasPositionContext)
  const [marketSnapshot, sectorRankLists] = hasAnyPosition
    ? await Promise.all([resolveScanMarketSnapshot(), ensureIndustryMoneyRank()])
    : [null, null]

  const tasks = list.map(({ code, name }) => async () => {
    const bars = await fetchDailyBars(code, name)
    if (!bars) return { code, ok: false }
    const lastIdx = resolveSignalLastBarIndex(bars.dayKeys)
    let summary = summarizeBuySignal(
      { ...bars, indexMa20ByDay },
      { ...signalOptions, signalLastIndex: lastIdx },
    )
    const positionCtx = options.positionsByCode?.[code]
    let riskContext
    let riskAdvice
    let riskAdviceProjection
    let holdingDecision
    if (hasPositionContext(positionCtx)) {
      const marketSegment = resolveStockMarketSegment(code)
      const globalMarketMode = marketSnapshot?.mode
      const segmentMarketMode = marketSnapshot?.markets?.[marketSegment.key]?.mode
      const effectiveMarketMode = resolveEffectiveStockMarketMode(
        globalMarketMode,
        segmentMarketMode,
        marketSegment,
      )
      summary = applyCostAwareSellToSummary(summary, bars, positionCtx, signalOptions)
      summary = applyRushReduceToSummary(summary, bars, positionCtx, signalOptions)
      if (summary.tag !== '冲' && summary.tag !== '减' && summary.tag !== '止') {
        summary = applyAddPositionToSummary(summary, bars, positionCtx, signalOptions)
      }
      const sectorFlow = sectorRankLists
        ? matchSectorFlow(positionCtx.industry, positionCtx.bkName, sectorRankLists)
        : null
      summary = applyHoldingPositionAdjustToSummary(summary, bars, positionCtx, {
        marketModeKey: effectiveMarketMode.key,
        globalMarketMode,
        segmentMarketMode,
        marketSegment,
        sectorFlow,
        profitPct: positionCtx.profitPct,
      })
      // Phase8-0 Step3-A: RiskAdvice shadow dual-write (does not alter legacy holdingAdvice)
      const shadow = buildScanRiskAdviceShadow({
        stockCode: code,
        marketModeKey: effectiveMarketMode.key,
        effectiveMarketMode,
        globalMarketMode,
        segmentMarketMode,
        marketSegment,
        sectorFlow,
        bars,
        lastIdx: summary.recentSignalBar ?? lastIdx,
        asOf: new Date().toISOString(),
      })
      if (shadow.riskAdvice) {
        riskContext = shadow.riskContext
        riskAdvice = shadow.riskAdvice
      }
      observeLegacyRiskAdvicePair(summary.holdingAdvice, riskAdvice ?? null, {
        stockCode: code,
        shadowFailure: shadow.failReason,
      })
      // Phase8.6: read-only RiskAdvice projection (does not overwrite holdingAdvice)
      if (riskAdvice) {
        const projected = buildRiskAdviceProjection(riskAdvice, {
          symbol: code,
          position: positionCtx,
          riskContext,
          meta: { stockCode: code, asOf: riskAdvice.asOf },
        })
        if (projected.projection) riskAdviceProjection = projected.projection
      } else {
        buildRiskAdviceProjection(null, { symbol: code, position: positionCtx })
      }
      // Phase9-A: unified adapter (source locked to legacy; does not change trading)
      holdingDecision = getHoldingDecision({
        holdingAdvice: summary.holdingAdvice,
        riskAdviceProjection,
        symbol: code,
        shadowFailure: shadow.failReason,
      })
    }
    return {
      code,
      name: name || code,
      ok: true,
      tag: summary.tag,
      tagType: summary.tagType,
      statusText: summary.statusText,
      sellPositionPct: summary.sellPositionPct,
      addPositionPct: summary.addPositionPct,
      rushReducePct: summary.rushReducePct,
      sourceTag: summary.sourceTag,
      costPrice: summary.costPrice,
      sellVolume: summary.sellVolume,
      holdingAdvice: summary.holdingAdvice,
      ...(riskAdvice
        ? { riskContext, riskAdvice }
        : {}),
      ...(riskAdviceProjection
        ? { riskAdviceProjection }
        : {}),
      ...(holdingDecision
        ? { holdingDecision }
        : {}),
      sortRank: summary.sortRank,
      daysAgo: summary.recentSignalDaysAgo,
      effectiveSignalDayKey: summary.effectiveSignalDayKey || '',
      buyPriceRange: summary.buyPriceRange,
      latestStatus: summary.latestStatus,
      hasRecentStrictBuy: summary.hasRecentStrictBuy,
      hasRecentBreakout: summary.hasRecentBreakout,
      signalBar: summary.recentSignalBar,
    }
  })

  const rows = await runPool(tasks, options.concurrency ?? 3)
  const byCode = {}
  const hints = []
  for (const row of rows) {
    if (!row?.ok) continue
    byCode[row.code] = row
    if (row.tag && row.daysAgo === 0) hints.push(row)
  }
  hints.sort((a, b) => a.sortRank - b.sortRank || a.name.localeCompare(b.name, 'zh-CN'))
  return { hints, byCode }
}
