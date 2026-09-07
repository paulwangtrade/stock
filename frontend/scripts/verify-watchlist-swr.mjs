import assert from 'node:assert/strict'
import {
  buildWatchlistPlaceholders,
  followedToPlaceholderRow,
  markPendingQuotesFailed,
  normalizeFollowCode,
  clearQuoteFlags,
} from '../src/utils/watchlistSWR.js'

assert.equal(normalizeFollowCode('usAAPL'), 'gb_aapl')
assert.equal(normalizeFollowCode('sh600000'), 'sh600000')

const row = followedToPlaceholderRow({
  StockCode: 'sh600000',
  Name: '浦发银行',
  Price: 10.5,
  Sort: 3,
  CostPrice: 9,
  Volume: 100,
})
assert.equal(row['股票代码'], 'sh600000')
assert.equal(row['股票名称'], '浦发银行')
assert.equal(row['当前价格'], '10.5')
assert.equal(row.quotePending, true)
assert.equal(row.quoteFailed, false)
assert.ok(row.key.endsWith('_sh600000'))

const emptyPrice = followedToPlaceholderRow({ StockCode: 'sz000001', Name: '平安银行', Price: 0 })
assert.equal(emptyPrice['当前价格'], '--')
assert.equal(emptyPrice.quotePending, true)

const { codes, rowsByKey } = buildWatchlistPlaceholders([
  { StockCode: 'sh600000', Name: 'A', Sort: 1, Price: 1 },
  { StockCode: 'sz000001', Name: 'B', Sort: 2, Price: 0 },
])
assert.deepEqual(codes, ['sh600000', 'sz000001'])
assert.equal(Object.keys(rowsByKey).length, 2)

const failed = markPendingQuotesFailed(rowsByKey)
for (const k of Object.keys(failed)) {
  assert.equal(failed[k].quotePending, false)
  assert.equal(failed[k].quoteFailed, true)
  assert.ok(failed[k]['当前价格'] === '--' || failed[k]['当前价格'] === '1')
}

const cleared = clearQuoteFlags({ ...row, quotePending: true })
assert.equal(cleared.quotePending, false)
assert.equal(cleared.quoteFailed, false)

// 失败降级不得清空列表
assert.equal(Object.keys(failed).length, 2)

console.log('verify-watchlist-swr: ok')
