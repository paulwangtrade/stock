import assert from 'node:assert/strict'
import test from 'node:test'
import {
  SAME_PLAN_MESSAGE,
  buildUpcomingDisplayContext,
  isTradingDay,
  shanghaiDate,
} from './planContext.js'

function plan(id, tradeDate, status = 'ready') {
  return { id, trade_date: tradeDate, status, freeze: status === 'ready' ? { is_frozen: true } : undefined }
}

function res(planObj, nextTradingDay = '') {
  return {
    ok: !!planObj,
    plan: planObj,
    next_trading_day: nextTradingDay,
    trade_date: planObj?.trade_date || '',
  }
}

test('isTradingDay weekend', () => {
  assert.equal(isTradingDay('2026-08-29'), false)
  assert.equal(isTradingDay('2026-08-28'), true)
})

test('trading day: todaySlot only accepts trade_date == calendar today', () => {
  const now = new Date('2026-08-28T10:00:00+08:00')
  const today = shanghaiDate(now)
  assert.equal(today, '2026-08-28')

  const ctx = buildUpcomingDisplayContext({
    now,
    resToday: res(plan(44, '2026-08-28'), '2026-08-31'),
    resNext: res(plan(45, '2026-08-31'), '2026-08-31'),
  })
  assert.equal(ctx.todaySlot.plan?.id, 44)
  assert.equal(ctx.todaySlot.tradeDate, '2026-08-28')
  assert.equal(ctx.nextSlot.plan?.id, 45)
  assert.equal(ctx.defaultFocus, 'today')
})

test('trading day: upcoming for tomorrow does not fill today slot', () => {
  const now = new Date('2026-08-28T10:00:00+08:00')
  const ctx = buildUpcomingDisplayContext({
    now,
    resToday: res(plan(45, '2026-08-31'), '2026-08-31'),
    resNext: res(plan(45, '2026-08-31'), '2026-08-31'),
  })
  assert.equal(ctx.todaySlot.plan, null)
  assert.equal(ctx.todaySlot.empty, '暂无今日执行计划')
  assert.equal(ctx.nextSlot.plan?.id, 45)
  assert.equal(ctx.defaultFocus, 'next')
})

test('non-trading day: hide today slot and default to next', () => {
  const now = new Date('2026-08-29T10:00:00+08:00')
  const ctx = buildUpcomingDisplayContext({
    now,
    resToday: res(plan(45, '2026-08-31'), '2026-08-31'),
    resNext: res(plan(45, '2026-08-31'), '2026-08-31'),
  })
  assert.equal(ctx.showTodaySlot, false)
  assert.equal(ctx.todaySlot.visible, false)
  assert.equal(ctx.todaySlot.plan, null)
  assert.equal(ctx.nextSlot.plan?.id, 45)
  assert.equal(ctx.defaultFocus, 'next')
})

test('same plan_id: dedupe next slot and show message', () => {
  const now = new Date('2026-08-28T10:00:00+08:00')
  const shared = plan(44, '2026-08-28')
  const ctx = buildUpcomingDisplayContext({
    now,
    resToday: res(shared, '2026-08-31'),
    resNext: res({ ...shared, trade_date: '2026-08-31' }, '2026-08-31'),
  })
  assert.equal(ctx.todaySlot.plan?.id, 44)
  assert.equal(ctx.nextSlot.plan, null)
  assert.equal(ctx.samePlanMessage, SAME_PLAN_MESSAGE)
  assert.equal(ctx.nextSlot.note, SAME_PLAN_MESSAGE)
})
