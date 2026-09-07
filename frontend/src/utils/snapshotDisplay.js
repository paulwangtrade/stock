/**
 * Phase14 UX-ISSUE-002 + Phase16.25: snapshot banner / history labels (pure functions).
 * trade_date = business trading day; created_at = generation wall time.
 */
import { normalizeTradeDate } from './opportunityListMetrics.js'

const CN_TZ = 'Asia/Shanghai'
const WEEKDAY_NUM = { Mon: 1, Tue: 2, Wed: 3, Thu: 4, Fri: 5, Sat: 6, Sun: 0 }

const NON_TRADING_DAY_TIP =
  '当前为非交易日，已自动使用最近交易日数据生成快照。'

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

/** Shanghai calendar YYYY-MM-DD */
export function getChinaTodayKey(date = new Date()) {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: CN_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(date)
}

/** A 股交易日 9:30 前（含周末则 false）— 与 tradingSession.js 一致 */
function isBeforeAShareMarketOpen(date = new Date()) {
  const { weekday, hour, minute } = getChinaTimeParts(date)
  if (!WEEKDAY_NUM[weekday] || WEEKDAY_NUM[weekday] >= 6) return false
  const hm = Number(hour) * 60 + Number(minute)
  return hm < 9 * 60 + 30
}

/** Session suffix for market cutoff line. */
export function snapshotSessionCloseLabel(session) {
  return String(session || '').trim() === 'midday' ? '午盘' : '收盘'
}

function parseDayKeyParts(dayKey) {
  const s = normalizeTradeDate(dayKey)
  const m = s.match(/^(\d{4})-(\d{2})-(\d{2})/)
  if (!m) return null
  return { y: Number(m[1]), mo: Number(m[2]), d: Number(m[3]) }
}

function formatDayKey({ y, mo, d }) {
  return `${y}-${String(mo).padStart(2, '0')}-${String(d).padStart(2, '0')}`
}

function addCalendarDays(dayKey, delta) {
  const p = parseDayKeyParts(dayKey)
  if (!p) return ''
  const dt = new Date(p.y, p.mo - 1, p.d)
  dt.setDate(dt.getDate() + delta)
  return formatDayKey({ y: dt.getFullYear(), mo: dt.getMonth() + 1, d: dt.getDate() })
}

function weekdayFromDayKey(dayKey) {
  const p = parseDayKeyParts(dayKey)
  if (!p) return -1
  return new Date(p.y, p.mo - 1, p.d).getDay()
}

function isWeekendDayKey(dayKey) {
  const wd = weekdayFromDayKey(dayKey)
  return wd === 0 || wd === 6
}

function previousWeekdayDayKey(dayKey) {
  let cur = dayKey
  for (let i = 0; i < 8; i++) {
    cur = addCalendarDays(cur, -1)
    if (!isWeekendDayKey(cur)) return cur
  }
  return cur
}

function parseCreatedAtDate(createdAt) {
  if (createdAt == null || createdAt === '') return null
  if (createdAt instanceof Date && !Number.isNaN(createdAt.getTime())) return createdAt
  const s = String(createdAt).trim()
  if (!s) return null
  const d = new Date(s)
  return Number.isNaN(d.getTime()) ? null : d
}

function isSameScanDayAsCreated(tradeDate, createdAt) {
  const scanDay = normalizeTradeDate(tradeDate)
  const d = parseCreatedAtDate(createdAt)
  if (!scanDay || !d) return false
  return getChinaTodayKey(d) === scanDay
}

/**
 * Market data cutoff — for legacy rows where trade_date was a calendar weekend.
 * New snaps (Phase16.25) already store a real trading day; then this returns tradeDate.
 */
export function resolveMarketCutoffDate(tradeDate, { createdAt } = {}) {
  const scanDay = normalizeTradeDate(tradeDate)
  if (!scanDay) return ''

  const wd = weekdayFromDayKey(scanDay)
  if (wd === 6) return addCalendarDays(scanDay, -1)
  if (wd === 0) return addCalendarDays(scanDay, -2)

  if (
    createdAt != null &&
    isSameScanDayAsCreated(scanDay, createdAt) &&
    isBeforeAShareMarketOpen(parseCreatedAtDate(createdAt))
  ) {
    return previousWeekdayDayKey(scanDay)
  }

  return scanDay
}

/** M-D without leading zeros, e.g. 2026-08-29 → 8-29 */
export function formatCompactMonthDay(tradeDate) {
  const p = parseDayKeyParts(normalizeTradeDate(tradeDate))
  if (!p) return String(tradeDate || '').trim()
  return `${p.mo}-${p.d}`
}

/** Format created_at as Shanghai `YYYY-MM-DD HH:mm` (generation time). */
export function formatSnapshotCreatedAt(createdAt) {
  const d = parseCreatedAtDate(createdAt)
  if (!d) return ''
  const parts = {}
  for (const p of new Intl.DateTimeFormat('en-CA', {
    timeZone: CN_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).formatToParts(d)) {
    if (p.type !== 'literal') parts[p.type] = p.value
  }
  if (!parts.year) return ''
  const hour = String(parts.hour || '00').padStart(2, '0')
  return `${parts.year}-${parts.month}-${parts.day} ${hour}:${parts.minute}`
}

/**
 * Tip when snapshot business day differs from generation calendar day
 * (non-trading day generation → PrevTradingDay trade_date).
 */
export function buildNonTradingDaySnapshotTip({ tradeDate, createdAt, now = new Date() } = {}) {
  const td = normalizeTradeDate(tradeDate)
  if (!td) return ''
  const created = parseCreatedAtDate(createdAt)
  const genDay = created ? getChinaTodayKey(created) : getChinaTodayKey(now)
  if (genDay && td !== genDay) return NON_TRADING_DAY_TIP
  return ''
}

/** History select label: `2026-09-04 收盘快照 · 策略 (47只)` */
export function buildSnapshotHistoryLabel(snap) {
  const tradeDate = normalizeTradeDate(snap?.tradeDate)
  if (!tradeDate) return '—'
  const closeLabel = snapshotSessionCloseLabel(snap?.session)
  const name = String(snap?.strategyName || '').trim()
  const hits = Number(snap?.hitTotal)
  const hitPart = Number.isFinite(hits) ? ` (${hits}只)` : ''
  const namePart = name ? ` · ${name}` : ''
  return `${tradeDate} ${closeLabel}快照${namePart}${hitPart}`
}

/**
 * @param {{ tradeDate?: string, session?: string, scannedTotal?: number, hitTotal?: number, createdAt?: unknown }} snap
 */
export function buildSnapshotMetaDisplay(snap) {
  if (!snap) return null
  const tradeDate = normalizeTradeDate(snap.tradeDate)
  if (!tradeDate) return null

  const marketCutoffDate = resolveMarketCutoffDate(tradeDate, { createdAt: snap.createdAt })
  const closeLabel = snapshotSessionCloseLabel(snap.session)
  const scanned = Number(snap.scannedTotal) || 0
  const hits = Number(snap.hitTotal) || 0
  const statsLine = `扫 ${scanned.toLocaleString('zh-CN')} 只 · 命中 ${hits.toLocaleString('zh-CN')} 只`

  const createdLabel = formatSnapshotCreatedAt(snap.createdAt)
  // Phase16.25: business day vs generation time
  const businessLine = `${tradeDate} ${closeLabel}快照`
  const marketLine = `业务交易日：${businessLine}`
  const generatedLine = createdLabel ? `生成于 ${createdLabel}` : '生成于 —'
  const compactLine = createdLabel
    ? `${businessLine}（${generatedLine}）`
    : businessLine
  const nonTradingTip = buildNonTradingDaySnapshotTip({
    tradeDate,
    createdAt: snap.createdAt,
  })

  return {
    marketCutoffDate,
    closeLabel,
    businessLine,
    marketLine,
    generatedLine,
    statsLine,
    compactLine,
    nonTradingTip,
  }
}
