/** Phase16.27-P1 — snapshot valuation fields mapping (no network). */
import { mapSnapshotPosition } from './portfolioSnapshot.ts'

function assert(cond, msg) {
  if (!cond) throw new Error(msg || 'assert failed')
}

const row = mapSnapshotPosition({
  stock_code: 'sh600000',
  stock_name: '浦发',
  total_qty: 1000,
  available_qty: 1000,
  locked_qty: 0,
  avg_cost: 10,
  mark_price: 12,
  market_value: 12000,
  pnl: 2000,
  pnl_percent: 0.2,
  display_price: 20,
  display_quote_source: 'live',
  display_market_value: 20000,
  quote_pre_close: 19,
  today_pnl: 1000,
  position_state: { state: 'S2_AVAILABLE', is_new_position: false },
})

assert(row.marketValue === 12000, 'market_value mapped (ledger)')
assert(row.pnlPercent === 0.2, 'pnl_percent mapped')
assert(row.markPrice === 12, 'mark_price mapped')
assert(row.displayPrice === 20, 'display_price mapped')
assert(row.pnl === 2000, 'pnl mapped')
assert(row.todayPnl === 1000, 'today_pnl mapped')
assert(row.marketValue !== row.displayMarketValue, 'ledger MV ≠ display MV')

const noQuote = mapSnapshotPosition({
  stock_code: 'sz000001',
  total_qty: 100,
  avg_cost: 10,
  mark_price: 11,
  market_value: 1100,
  pnl: 100,
  pnl_percent: 0.1,
  display_quote_source: 'persisted',
  display_price: 11,
  position_state: {},
})
assert(noQuote.todayPnl === null, 'no today_pnl')
assert(noQuote.marketValue === 1100, 'market value still from mark')

console.log('portfolioSnapshot.valuationDisplay.selftest: OK')
