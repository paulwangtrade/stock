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
const TITLE_TODAY_EXEC = '今日执行计划'
const TITLE_NEXT_EXEC = '下一交易日计划'
const EMPTY_TODAY_EXEC = '暂无今日执行计划'
const EMPTY_NEXT_EXEC = '暂无下一交易日计划'
const SAME_PLAN_MESSAGE = '当前计划已与下一交易日计划一致'

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

function planId(plan) {
  const id = Math.trunc(Number(plan?.id) || 0)
  return id > 0 ? id : 0
}

function resolveNextTradingDay(resToday, today) {
  const apiNext = String(resToday?.next_trading_day || '').trim()
  let nextDay = apiNext || nextTradingDayString(today)
  if (!nextDay || nextDay <= today) {
    nextDay = nextTradingDayString(today)
  }
  return nextDay
}

function resolveNextSlotPlan(resToday, resNext, nextDay) {
  if (!nextDay) return null
  return (
    pickExactPlan(resNext, nextDay)
    || pickOnOrAfterPlan(resNext, nextDay)
    || pickExactPlan(resToday, nextDay)
    || pickOnOrAfterPlan(resToday, nextDay)
  )
}

/**
 * TradePlanUpcoming dual-slot display context (presentation-only).
 * Does not alter upcoming API semantics — filters/maps responses for UI slots.
 *
 * @param {{ now?: Date, resToday?: object, resNext?: object }} input
 */
export function buildUpcomingDisplayContext(input = {}) {
  const now = input.now instanceof Date ? input.now : new Date()
  const today = shanghaiDate(now)
  const calendarIsTradingDay = isTradingDay(today)
  const nextDay = resolveNextTradingDay(input.resToday, today)

  const todayPlan = calendarIsTradingDay ? pickExactPlan(input.resToday, today) : null
  const nextPlanRaw = resolveNextSlotPlan(input.resToday, input.resNext, nextDay)

  const todayId = planId(todayPlan)
  const nextId = planId(nextPlanRaw)
  const samePlan = todayId > 0 && nextId > 0 && todayId === nextId

  const todaySlot = {
    kind: 'today',
    tradeDate: calendarIsTradingDay ? today : '',
    title: TITLE_TODAY_EXEC,
    plan: todayPlan,
    empty: calendarIsTradingDay && !todayPlan ? EMPTY_TODAY_EXEC : '',
    visible: calendarIsTradingDay,
  }

  const nextSlot = {
    kind: 'next',
    tradeDate: nextDay || '',
    title: TITLE_NEXT_EXEC,
    plan: samePlan ? null : nextPlanRaw,
    empty: !samePlan && !nextPlanRaw ? EMPTY_NEXT_EXEC : '',
    visible: true,
    note: samePlan ? SAME_PLAN_MESSAGE : '',
  }

  let defaultFocus = 'next'
  if (calendarIsTradingDay && todayPlan) {
    defaultFocus = 'today'
  } else if (nextPlanRaw) {
    defaultFocus = 'next'
  } else if (calendarIsTradingDay) {
    defaultFocus = 'today'
  }

  return {
    as_of: now.toISOString(),
    timezone: PLAN_TZ,
    today,
    calendarIsTradingDay,
    nextTradingDay: nextDay,
    showTodaySlot: calendarIsTradingDay,
    todaySlot,
    nextSlot,
    samePlanId: samePlan ? todayId : null,
    samePlanMessage: samePlan ? SAME_PLAN_MESSAGE : '',
    defaultFocus,
  }
}

export {
  TITLE_TODAY_EXEC,
  TITLE_NEXT_EXEC,
  EMPTY_TODAY_EXEC,
  EMPTY_NEXT_EXEC,
  SAME_PLAN_MESSAGE,
}
