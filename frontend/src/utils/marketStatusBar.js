import { normalizeDayKey } from './icePointSignals'
import { getSharedIndexDailyKLine } from './indexKlineCache'
import { resolveMarketPositionCap } from './positionPolicy'
import { resolveMarketMode, toDisplayTradingLevel } from './tradingLevelRules'
import {
  STOCK_MARKET_SEGMENTS,
  STOCK_MARKET_SEGMENT_ORDER,
} from './stockMarketSegment'
import {
  getChinaTodayKey,
  isAShareMarketOpenNow,
  isBeforeAShareMarketOpen,
  resolveEffectiveSignalDayKey,
  resolveSignalLastBarIndex,
} from './tradingSession'

const WEEKDAY_NUM = { Mon: 1, Tue: 2, Wed: 3, Thu: 4, Fri: 5, Sat: 6, Sun: 0 }
const CN_TZ = 'Asia/Shanghai'

function getChinaTimeParts(date = new Date()) {
  const fmt = new Intl.DateTimeFormat('en-US', {
    timeZone: CN_TZ,
    weekday: 'short',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
  const parts = {}
  for (const p of fmt.formatToParts(date)) {
    if (p.type !== 'literal') parts[p.type] = p.value
  }
  return parts
}

function getChinaHm(date = new Date()) {
  const { hour, minute } = getChinaTimeParts(date)
  return Number(hour) * 60 + Number(minute)
}

function isWeekend(date = new Date()) {
  const { weekday } = getChinaTimeParts(date)
  const w = WEEKDAY_NUM[weekday]
  return !w || w >= 6
}

/** 交易时段标签（盘前 / 盘中 / 午间休市 / 盘后 / 休市） */
export function resolveMarketSessionLabel(date = new Date()) {
  if (isWeekend(date)) return { key: 'closed', label: '休市', inSession: false }
  const hm = getChinaHm(date)
  if (hm < 9 * 60 + 30) return { key: 'pre', label: '盘前', inSession: false }
  if (hm >= 11 * 60 + 30 && hm < 13 * 60) return { key: 'lunch', label: '午间休市', inSession: false }
  if (hm >= 15 * 60) return { key: 'post', label: '盘后', inSession: false }
  return { key: 'trading', label: '盘中', inSession: true }
}

function daySortKey(dayStr) {
  const s = String(dayStr || '').trim().replace(/\//g, '-')
  const m = s.match(/^(\d{4})-(\d{2})-(\d{2})/)
  if (m) return Number(m[1] + m[2] + m[3])
  const c = s.match(/^(\d{4})(\d{2})(\d{2})/)
  if (c) return Number(c[1] + c[2] + c[3])
  return 0
}

function parseChangePercent(bar) {
  const raw = bar?.ChangePercent ?? bar?.changePercent
  const n = Number(raw)
  if (Number.isFinite(n)) return n
  const close = Number(bar?.close)
  const open = Number(bar?.open)
  if (Number.isFinite(close) && Number.isFinite(open) && open > 0) return ((close - open) / open) * 100
  return null
}

function movingAverage(values, period, index) {
  if (index < period - 1) return null
  let sum = 0
  for (let i = index - period + 1; i <= index; i++) {
    const value = Number(values[i])
    if (!Number.isFinite(value)) return null
    sum += value
  }
  return sum / period
}

function previousAverage(values, lookback, index) {
  if (index < 1) return null
  let sum = 0
  let count = 0
  for (let i = Math.max(0, index - lookback); i < index; i++) {
    const value = Number(values[i])
    if (Number.isFinite(value) && value > 0) {
      sum += value
      count++
    }
  }
  return count ? sum / count : null
}

export { resolveMarketMode } from './tradingLevelRules'

/** 顶栏轮询间隔：盘中 30s，盘前 60s，其余 5min */
export function getMarketStatusRefreshIntervalMs(date = new Date()) {
  if (isAShareMarketOpenNow(date)) return 30_000
  if (isBeforeAShareMarketOpen(date)) return 60_000
  return 300_000
}

async function fetchIndexMetrics(index, date = new Date()) {
  const todayKey = getChinaTodayKey(date)
  const emptyMode = resolveMarketMode({})
  const empty = {
    ok: false, close: null, ma5: null, ma10: null, ma20: null, ma60: null,
    volumeRatio: null, changePct: null, effectiveDay: '', isTodayBar: false, mode: emptyMode,
  }
  try {
    // barCount 与 watchlistSignalScan 对齐，便于 000001.SH 等同 key 复用
    const raw = await getSharedIndexDailyKLine(index.indexCode, index.indexName, { barCount: 120 })
    const list = (Array.isArray(raw) ? raw : [])
      .filter((row) => Number.isFinite(Number(row?.close)))
      .sort((a, b) => daySortKey(a.day) - daySortKey(b.day))
    if (!list.length) return empty

    const dayKeys = list.map((r) => normalizeDayKey(r.day))
    const lastIdx = resolveSignalLastBarIndex(dayKeys)
    if (lastIdx < 0) return empty

    const closes = list.map((r) => Number(r.close))
    const volumes = list.map((r) => Number(r.volume) || 0)
    const effectiveDay = resolveEffectiveSignalDayKey(dayKeys)
    const bar = list[lastIdx]
    const close = closes[lastIdx]
    const ma5 = movingAverage(closes, 5, lastIdx)
    const ma10 = movingAverage(closes, 10, lastIdx)
    const ma20 = movingAverage(closes, 20, lastIdx)
    const ma60 = movingAverage(closes, 60, lastIdx)
    const prevMa20 = movingAverage(closes, 20, lastIdx - 5)
    const avgVolume = previousAverage(volumes, 5, lastIdx)
    const volumeRatio = avgVolume > 0 && volumes[lastIdx] > 0 ? volumes[lastIdx] / avgVolume : null
    const volumeExpanding = lastIdx >= 1 && volumes[lastIdx] >= volumes[lastIdx - 1]
    const ma20Rising = Number.isFinite(prevMa20) && Number.isFinite(ma20) && ma20 >= prevMa20
    const changePct = parseChangePercent(bar)
    const mode = resolveMarketMode({ close, ma5, ma10, ma20, ma60, volumeRatio, volumeExpanding, ma20Rising })

    return {
      ok: true,
      close,
      ma5,
      ma10,
      ma20,
      ma60,
      volumeRatio,
      changePct: Number.isFinite(changePct) ? changePct : null,
      effectiveDay,
      isTodayBar: effectiveDay === todayKey,
      mode,
    }
  } catch {
    return empty
  }
}

function resolveGlobalMarketMode(markets) {
  const shMode = markets.shMain?.mode
  const szMode = markets.szMain?.mode
  if (!shMode?.level && !szMode?.level) return resolveMarketMode({})

  const shLevel = Number(shMode?.level) || 3
  const szLevel = Number(szMode?.level) || 3
  const level = Math.min(shLevel, szLevel)
  const source = shLevel <= szLevel ? STOCK_MARKET_SEGMENTS.shMain : STOCK_MARKET_SEGMENTS.szMain
  const sourceMode = shLevel <= szLevel ? shMode : szMode
  return {
    ...sourceMode,
    key: `level${level}`,
    level,
    label: `总${toDisplayTradingLevel(level)}级 ${sourceMode?.name || ''}`.trim(),
    reason: `沪主板${toDisplayTradingLevel(shLevel)}级、深主板${toDisplayTradingLevel(szLevel)}级，按较弱一侧控制总仓位（${source.name}）`,
  }
}

/** 五类市场指数 -> 分市场 5 级模型；沪深主板较弱一侧约束账户总仓位。 */
let lastMarketStatusSnapshot = null

export function getLastMarketStatusSnapshot() {
  return lastMarketStatusSnapshot
}

export function getLastMarketModeKey() {
  return lastMarketStatusSnapshot?.mode?.key || 'unknown'
}

export async function fetchMarketStatusSnapshot(date = new Date()) {
  const session = resolveMarketSessionLabel(date)
  try {
    const entries = await Promise.all(
      STOCK_MARKET_SEGMENT_ORDER.map(async (key) => {
        const segment = STOCK_MARKET_SEGMENTS[key]
        return [key, await fetchIndexMetrics(segment, date)]
      }),
    )
    const markets = Object.fromEntries(entries)
    const shIndex = markets.shMain
    const mode = resolveGlobalMarketMode(markets)
    lastMarketStatusSnapshot = {
      ok: Object.values(markets).some((row) => row.ok),
      session,
      shIndex,
      markets,
      mode,
      positionCap: resolveMarketPositionCap(mode.key),
      error: Object.values(markets).some((row) => row.ok) ? '' : '暂无市场指数 K 线',
      updatedAt: Date.now(),
    }
    return lastMarketStatusSnapshot
  } catch (e) {
    const mode = resolveMarketMode({})
    lastMarketStatusSnapshot = {
      ok: false,
      session,
      shIndex: null,
      markets: {},
      mode,
      positionCap: resolveMarketPositionCap(mode.key),
      error: e?.message || '加载失败',
      updatedAt: Date.now(),
    }
    return lastMarketStatusSnapshot
  }
}
