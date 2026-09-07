import assert from 'node:assert/strict'
import test from 'node:test'
import {
  HOME_PLAN_EMPTY_NEXT,
  HOME_PLAN_EMPTY_TODAY,
  buildHomePlanExtraRows,
  buildHomePlanPrimaryRow,
  buildHomePlanSlotModel,
  resolveSameDaySourceBucket,
} from './homePlanDisplay.js'

test('resolveSameDaySourceBucket maps product buckets', () => {
  assert.equal(resolveSameDaySourceBucket('strategy'), 'strategy')
  assert.equal(resolveSameDaySourceBucket('watchlist'), 'watchlist')
  assert.equal(resolveSameDaySourceBucket('manual_sell'), 'manual')
  assert.equal(resolveSameDaySourceBucket('other'), 'unknown')
})

test('buildHomePlanPrimaryRow maps after_close Strategy chip', () => {
  const row = buildHomePlanPrimaryRow({
    id: 60,
    trade_date: '2026-09-07',
    status: 'draft',
    source_session: 'after_close',
    items: [{ stock_code: 'sh600000', stock_name: '浦发' }],
  })
  assert.equal(row.planId, 60)
  assert.equal(row.sourceChip.label, 'Strategy')
  assert.equal(row.itemCount, 1)
  assert.equal(row.statusKey, 'pending_confirm')
  assert.equal(row.stocks.length, 1)
})

test('buildHomePlanPrimaryRow maps t_sell Manual chip', () => {
  const row = buildHomePlanPrimaryRow({
    id: 62,
    trade_date: '2026-09-06',
    status: 'draft',
    source_session: 't_sell',
    items: [],
  })
  assert.equal(row.sourceChip.label, 'Manual')
})

test('buildHomePlanSlotModel dual empty copy', () => {
  const today = buildHomePlanSlotModel('today', { trade_date: '2026-09-06', plan: null }, [])
  const next = buildHomePlanSlotModel('next', { trade_date: '2026-09-07', plan: null }, [])
  assert.equal(today.hasPlan, false)
  assert.equal(today.emptyText, HOME_PLAN_EMPTY_TODAY)
  assert.equal(next.emptyText, HOME_PLAN_EMPTY_NEXT)
  assert.equal(today.title, '今日执行')
  assert.equal(next.title, '下一交易日准备')
})

test('buildHomePlanExtraRows excludes primary and keeps multi-plan', () => {
  const extras = buildHomePlanExtraRows(
    [
      { id: 60, source_session: 'after_close', source: 'strategy', status: 'draft', trade_date: '2026-09-07' },
      { id: 61, source_session: 't_sell', source: 'manual_sell', status: 'draft', trade_date: '2026-09-07' },
    ],
    60,
  )
  assert.equal(extras.length, 1)
  assert.equal(extras[0].planId, 61)
  assert.equal(extras[0].sourceChip.label, 'Manual')
})

test('slot with next after_close hasPlan true', () => {
  const slot = buildHomePlanSlotModel(
    'next',
    {
      trade_date: '2026-09-07',
      plan: {
        id: 60,
        trade_date: '2026-09-07',
        status: 'draft',
        source_session: 'after_close',
        items: [{ stock_code: 'sz000001' }, { stock_code: 'sz000002' }],
      },
    },
    [],
  )
  assert.equal(slot.hasPlan, true)
  assert.equal(slot.primary.itemCount, 2)
  assert.equal(slot.rows.length, 1)
})
