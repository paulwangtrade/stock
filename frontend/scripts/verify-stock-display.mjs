import assert from 'node:assert/strict'
import {
  CLICK_KLINE_MODAL,
  UNKNOWN_STOCK_NAME,
  applyStockClickAction,
  looksLikeInternalStockCode,
  toStockDisplay,
} from '../src/utils/stockDisplay.js'

function assertNoInternalCodeOnScreen(model) {
  assert.ok(model)
  assert.doesNotMatch(model.display_name, /^(sh|sz|bj)/i)
  assert.doesNotMatch(model.display_code, /^(sh|sz|bj)/i)
  assert.match(model.display_code, /^\d{6}\.(SH|SZ|BJ|HK)$/)
  assert.equal(model.click_action.type, CLICK_KLINE_MODAL)
  assert.equal(model.click_action.chart_code, model.display_code)
}

{
  const m = toStockDisplay({ stock_code: 'sz000021', stock_name: '深科技' })
  assert.equal(m.market, 'SZ')
  assert.equal(m.symbol, '000021')
  assert.equal(m.display_name, '深科技')
  assert.equal(m.display_code, '000021.SZ')
  assert.equal(m.stock_code, 'sz000021')
  assert.equal(m.click_action.title, '深科技 000021.SZ — 日K')
  assertNoInternalCodeOnScreen(m)
}

{
  const m = toStockDisplay({ market: 'SH', symbol: '600519', name: '贵州茅台' })
  assert.equal(m.display_name, '贵州茅台')
  assert.equal(m.display_code, '600519.SH')
  assert.equal(m.stock_code, 'sh600519')
  assertNoInternalCodeOnScreen(m)
}

{
  const m = toStockDisplay({ stock_code: 'sh601127' })
  assert.equal(m.display_name, UNKNOWN_STOCK_NAME)
  assert.equal(m.display_code, '601127.SH')
  assert.ok(!looksLikeInternalStockCode(m.display_name))
  assertNoInternalCodeOnScreen(m)
}

{
  const m = toStockDisplay({ stock_code: 'sz001232', stock_name: 'sz001232' })
  assert.equal(m.display_name, UNKNOWN_STOCK_NAME)
  assert.equal(m.display_code, '001232.SZ')
}

{
  const m = toStockDisplay({ stock_code: '000021.SZ', name: '深科技' })
  assert.equal(m.market, 'SZ')
  assert.equal(m.display_code, '000021.SZ')
}

{
  const m = toStockDisplay({ stock_code: '600519', name: '贵州茅台' })
  assert.equal(m.display_code, '600519.SH')
  assert.equal(m.stock_code, 'sh600519')
}

{
  const m = toStockDisplay({ symbol: '300750', market: 'sz', name: '宁德时代' })
  assert.equal(m.market, 'SZ')
  assert.equal(m.display_code, '300750.SZ')
}

assert.equal(toStockDisplay(null), null)
assert.equal(toStockDisplay({}), null)
assert.equal(toStockDisplay({ stock_code: 'gb_aapl' }), null)

{
  const model = toStockDisplay({ market: 'SZ', symbol: '000021', name: '深科技' })
  const target = { visible: false, title: '', chartCode: '', stockName: '' }
  assert.equal(applyStockClickAction(model, target), true)
  assert.equal(target.visible, true)
  assert.equal(target.chartCode, '000021.SZ')
  assert.equal(target.stockName, '深科技')
  assert.match(target.title, /日K/)
}

console.log('verify-stock-display: ok')
