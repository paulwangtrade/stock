/** Phase16.27-P0 — mapSnapshotPosition today_pnl / quote_pre_close (no network). */
import { mapSnapshotPosition } from './portfolioSnapshot.ts'

function assert(cond, msg) {
  if (!cond) throw new Error(msg || 'assert failed')
}

const withToday = mapSnapshotPosition({
  stock_code: 'sh600000',
  stock_name: '浦发',
  total_qty: 1000,
  available_qty: 1000,
  locked_qty: 0,
  avg_cost: 10,
  mark_price: 12,
  market_value: 12000,
  pnl: 2000,
  display_price: 20,
  display_quote_source: 'live',
  quote_pre_close: 19,
  today_pnl: 1000,
  position_state: { state: 'S2_AVAILABLE', is_new_position: false },
})
assert(withToday.todayPnl === 1000, 'todayPnl mapped')
assert(withToday.quotePreClose === 19, 'quotePreClose mapped')
assert(withToday.pnl === 2000, 'ledger pnl unchanged')

const missing = mapSnapshotPosition({
  stock_code: 'sz000001',
  total_qty: 100,
  avg_cost: 10,
  mark_price: 11,
  pnl: 100,
  display_price: 12,
  position_state: {},
})
assert(missing.todayPnl === null, 'missing today_pnl → null')
assert(missing.quotePreClose === null, 'missing pre_close → null')

console.log('portfolioSnapshot.todayPnl.selftest: OK')
