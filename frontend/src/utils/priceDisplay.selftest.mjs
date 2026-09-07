import assert from 'node:assert/strict'
import {
  formatPriceLabel,
  formatPriceValue,
  formatPriceWithContext,
  priceSourceLabel,
  PRICE_KIND,
} from './priceDisplay.js'

assert.equal(priceSourceLabel('last'), '最新行情价')
assert.equal(priceSourceLabel('mark_price'), '持仓估值价')
assert.equal(priceSourceLabel('signal'), '信号触发价')
assert.equal(priceSourceLabel('ref_price'), '策略参考价')
assert.equal(priceSourceLabel('cost'), '持仓成本价')
assert.equal(formatPriceValue(14.5), '14.50')
assert.equal(formatPriceValue(0), '—')
assert.equal(formatPriceValue(null), '—')
assert.ok(formatPriceLabel(14.5, PRICE_KIND.ref).includes('策略参考价'))
assert.ok(formatPriceLabel(14.5, PRICE_KIND.ref, { asOf: '2026-09-03' }).includes('2026-09-03'))
const ctx = formatPriceWithContext(14.07, 'ref', { source: 'kline_close', asOf: '2026-09-03' })
assert.equal(ctx.label, '策略参考价')
assert.equal(ctx.value, '14.07')
assert.ok(ctx.tooltip.includes('kline_close'))
assert.ok(!String(ctx.value).includes('undefined'))
assert.ok(!String(ctx.value).includes('NaN'))
console.log('priceDisplay.selftest: OK')
