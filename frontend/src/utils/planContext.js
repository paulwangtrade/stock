/**
 * Phase12-D2-C Home Plan Context (display-only).
 * Does not change GET /api/tradeplans/upcoming, Freeze, or execution.
 * Calendar matches backend tradingcalendar.Default (weekend-only).
 */

export const PLAN_TZ = 'Asia/Shanghai'
export const PLAN_SWITCH_HOUR = 15
export const PLAN_SWITCH_MINUTE = 0
export const PLAN_SWITCH_SECOND = 0

const TITLE_CURRENT = '今日交易计划'
const TITLE_NEXT = '下一个交易日计划'

function pad2(n) {
  return String(n).padStart(2, '0')
}

/** Shanghai wall-clock parts for a Date. */
export function shanghaiWall(now = new Date()) {
  const dtf = new Intl.DateTimeFormat('en-US', {
    timeZone: PLAN_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
  })
  const parts = {}
  for (const p of dtf.formatToParts(now)) {
    if (p.type !== 'literal') parts[p.type] = p.value
  }
  const date = `${parts.year}-${parts.month}-${parts.day}`
  return {
    date,
    hour: Number(parts.hour),
    minute: Number(parts.minute),
    second: Number(parts.second),
  }
}

export function shanghaiDate(now = new Date()) {
  return shanghaiWall(now).date
}

/** Weekend-only, same as tradingcalendar.Default with nil Holidays. */
export function isTradingDay(ymd) {
  const m = String(ymd || '').trim().match(/^(\d{4})-(\d{2})-(\d{2})$/)
  if (!m) return false
  const wd = new Date(Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]))).getUTCDay()
  return wd !== 0 && wd !== 6
}

function addUtcDays(ymd, n) {
  const m = String(ymd || '').trim().match(/^(\d{4})-(\d{2})-(\d{2})$/)
  if (!m) return ''
  const dt = new Date(Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]) + n))
  return `${dt.getUTCFullYear()}-${pad2(dt.getUTCMonth() + 1)}-${pad2(dt.getUTCDate())}`
}

/** Next trading day strictly after ymd (weekend skip). Fallback if API omits next_trading_day. */
export function nextTradingDayString(ymd) {
  let cur = addUtcDays(ymd, 1)
  for (let i = 0; i < 14; i++) {
    if (isTradingDay(cur)) return cur
    cur = addUtcDays(cur, 1)
  }
  return ''
}

function secondsOfDay(wall) {
  return wall.hour * 3600 + wall.minute * 60 + wall.second
}

const SWITCH_SECONDS = PLAN_SWITCH_HOUR * 3600 + PLAN_SWITCH_MINUTE * 60 + PLAN_SWITCH_SECOND

export function isBeforePlanSwitch(now = new Date()) {
  return secondsOfDay(shanghaiWall(now)) < SWITCH_SECONDS
}

/** current = trading day and before 15:00; otherwise next. */
export function resolvePlanContextActive(now = new Date()) {
  const wall = shanghaiWall(now)
  if (isTradingDay(wall.date) && secondsOfDay(wall) < SWITCH_SECONDS) return 'current'
  return 'next'
}

function pickExactPlan(res, requiredDate) {
  const plan = res?.ok && res.plan ? res.plan : null
  if (!plan) return null
  const td = String(plan.trade_date || '').trim()
  if (!requiredDate || td !== requiredDate) return null
  return plan
}

function pickOnOrAfterPlan(res, minDate) {
  const plan = res?.ok && res.plan ? res.plan : null
  if (!plan) return null
  const td = String(plan.trade_date || '').trim()
  if (minDate && td && td < minDate) return null
  return plan
}

function slot(kind, tradeDate, title, plan, emptyText) {
  return {
    kind,
    trade_date: tradeDate || '',
    title,
    plan,
    empty_text: emptyText,
  }
}

/**
 * Assemble current_plan / next_plan / active_plan from two upcoming responses.
 * @param {{ now?: Date, resCurrent?: object, resNext?: object }} input
 */
export function buildDashboardPlanContext(input = {}) {
  const now = input.now instanceof Date ? input.now : new Date()
  const wall = shanghaiWall(now)
  const today = wall.date
  const active = resolvePlanContextActive(now)
  const apiNext = String(input.resCurrent?.next_trading_day || '').trim()
  let nextDay = apiNext || nextTradingDayString(today)
  if (!nextDay || nextDay <= today) {
    nextDay = nextTradingDayString(today)
  }

  const current_plan = slot(
    'current',
    today,
    TITLE_CURRENT,
    pickExactPlan(input.resCurrent, today),
    '暂无交易计划',
  )
  const next_plan = slot(
    'next',
    nextDay,
    TITLE_NEXT,
    pickOnOrAfterPlan(input.resNext, nextDay),
    '下一个交易日暂无交易计划',
  )
  const active_plan = active === 'current' ? current_plan : next_plan

  return {
    as_of: now.toISOString(),
    timezone: PLAN_TZ,
    today,
    active,
    current_plan,
    next_plan,
    active_plan,
  }
}
