import assert from 'node:assert/strict'
import {
  buildDashboardPlanContext,
  isTradingDay,
  nextTradingDayString,
  resolvePlanContextActive,
  shanghaiDate,
} from '../src/utils/planContext.js'

assert.equal(isTradingDay('2026-08-20'), true) // Thursday
assert.equal(isTradingDay('2026-08-21'), true) // Friday
assert.equal(isTradingDay('2026-08-22'), false) // Saturday
assert.equal(isTradingDay('2026-08-23'), false) // Sunday
assert.equal(nextTradingDayString('2026-08-20'), '2026-08-21')
assert.equal(nextTradingDayString('2026-08-21'), '2026-08-24')
assert.equal(nextTradingDayString('2026-08-22'), '2026-08-24')

const morning = new Date('2026-08-20T10:00:00+08:00')
const preClose = new Date('2026-08-20T14:59:59+08:00')
const close = new Date('2026-08-20T15:00:00+08:00')
const evening = new Date('2026-08-20T16:30:00+08:00')
const saturday = new Date('2026-08-22T10:00:00+08:00')
const sunday = new Date('2026-08-23T20:00:00+08:00')

assert.equal(shanghaiDate(morning), '2026-08-20')
assert.equal(resolvePlanContextActive(morning), 'current')
assert.equal(resolvePlanContextActive(preClose), 'current')
assert.equal(resolvePlanContextActive(close), 'next')
assert.equal(resolvePlanContextActive(evening), 'next')
assert.equal(resolvePlanContextActive(saturday), 'next')
assert.equal(resolvePlanContextActive(sunday), 'next')

const todayPlan = {
  id: 44,
  trade_date: '2026-08-20',
  status: 'draft',
  items: [{ stock_code: 'sz000021', stock_name: '深科技' }],
}
const nextPlan = {
  id: 45,
  trade_date: '2026-08-21',
  status: 'draft',
  items: [{ stock_code: 'sz000858', stock_name: '五粮液' }],
}
const resCurrent = {
  ok: true,
  trade_date: '2026-08-20',
  next_trading_day: '2026-08-21',
  plan: todayPlan,
}
const resNext = {
  ok: true,
  trade_date: '2026-08-21',
  next_trading_day: '2026-08-24',
  plan: nextPlan,
}

{
  const ctx = buildDashboardPlanContext({ now: morning, resCurrent, resNext })
  assert.equal(ctx.active, 'current')
  assert.equal(ctx.active_plan.kind, 'current')
  assert.equal(ctx.active_plan.title, '今日交易计划')
  assert.equal(ctx.active_plan.plan?.id, 44)
  assert.equal(ctx.current_plan.plan?.id, 44)
  assert.equal(ctx.next_plan.plan?.id, 45)
}

{
  const ctx = buildDashboardPlanContext({ now: close, resCurrent, resNext })
  assert.equal(ctx.active, 'next')
  assert.equal(ctx.active_plan.kind, 'next')
  assert.equal(ctx.active_plan.title, '下一个交易日计划')
  assert.equal(ctx.active_plan.plan?.id, 45)
  assert.equal(ctx.current_plan.plan?.id, 44, 'today draft still loaded, not displayed')
}

{
  const ctx = buildDashboardPlanContext({ now: saturday, resCurrent, resNext })
  assert.equal(ctx.active, 'next')
  assert.equal(ctx.current_plan.plan, null, 'Saturday must not treat a weekday plan as today')
  assert.equal(ctx.next_plan.trade_date, '2026-08-24')
}

{
  const weekendCurrent = {
    ok: true,
    trade_date: '2026-08-22',
    next_trading_day: '2026-08-24',
    plan: { id: 50, trade_date: '2026-08-24', status: 'draft', items: [] },
  }
  const weekendNext = {
    ok: true,
    trade_date: '2026-08-24',
    next_trading_day: '2026-08-25',
    plan: { id: 50, trade_date: '2026-08-24', status: 'draft', items: [] },
  }
  const ctx = buildDashboardPlanContext({ now: saturday, resCurrent: weekendCurrent, resNext: weekendNext })
  assert.equal(ctx.active, 'next')
  assert.equal(ctx.current_plan.plan, null)
  assert.equal(ctx.active_plan.plan?.id, 50)
  assert.equal(ctx.active_plan.trade_date, '2026-08-24')
}

{
  const ctx = buildDashboardPlanContext({
    now: evening,
    resCurrent,
    resNext: { ok: false, plan: null, next_trading_day: '2026-08-24' },
  })
  assert.equal(ctx.active, 'next')
  assert.equal(ctx.active_plan.plan, null)
  assert.match(ctx.active_plan.empty_text, /下一/)
}

console.log('verify-plan-context: ok')
