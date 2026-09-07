import test from 'node:test'
import assert from 'node:assert/strict'
import {
  collectQuoteCodesFromRows,
  isValidCachedQuote,
  planQuoteRefresh,
  parseLiveQuoteResponse,
  mergeQuoteIntoCache,
  resolveQuoteRefreshState,
  shouldRefreshSnapshotQuotes,
} from './opportunityQuoteRefresh.js'

test('collectQuoteCodesFromRows dedupes by resolved code', () => {
  const rows = [
    { SECUCODE: '600363.SH' },
    { SECUCODE: '600363.SH' },
    { SECUCODE: '000001.SZ' },
  ]
  const codes = collectQuoteCodesFromRows(rows, (row) => row.SECUCODE.toLowerCase())
  assert.deepEqual(codes, ['600363.sh', '000001.sz'])
})

test('planQuoteRefresh skips valid cached quotes', () => {
  const cache = new Map([
    ['sh600363', { price: 28.5, changeRate: 1.2 }],
  ])
  const plan = planQuoteRefresh(['sh600363', 'sz000001'], cache, new Set())
  assert.deepEqual(plan.skippedCached, ['sh600363'])
  assert.deepEqual(plan.toFetch, ['sz000001'])
})

test('planQuoteRefresh skips in-flight codes', () => {
  const inFlight = new Set(['sz000001'])
  const plan = planQuoteRefresh(['sz000001', 'sh600000'], new Map(), inFlight)
  assert.deepEqual(plan.skippedInFlight, ['sz000001'])
  assert.deepEqual(plan.toFetch, ['sh600000'])
})

test('duplicate quote skip: same code requested twice uses cache on second pass', () => {
  const cache = new Map()
  const inFlight = new Set()

  const first = planQuoteRefresh(['sh600363'], cache, inFlight)
  assert.deepEqual(first.toFetch, ['sh600363'])

  const nextCache = mergeQuoteIntoCache(cache, 'sh600363', { price: 10, changeRate: 0.5 })
  const second = planQuoteRefresh(['sh600363'], nextCache, inFlight)
  assert.deepEqual(second.toFetch, [])
  assert.deepEqual(second.skippedCached, ['sh600363'])
})

test('resolveQuoteRefreshState reports pending and skip counts', () => {
  const cache = new Map([['a', { price: 1 }]])
  const state = resolveQuoteRefreshState(['a', 'b', 'c'], cache, new Set(['c']))
  assert.equal(state.pendingCount, 1)
  assert.equal(state.skipCount, 2)
  assert.deepEqual(state.toFetch, ['b'])
})

test('parseLiveQuoteResponse accepts valid quote', () => {
  const entry = parseLiveQuoteResponse({ code: 0, price: 12.3, changePercent: -0.5 })
  assert.deepEqual(entry, { price: 12.3, changeRate: -0.5 })
})

test('shouldRefreshSnapshotQuotes only for snapshot with rows', () => {
  assert.equal(shouldRefreshSnapshotQuotes({ signalDataSource: 'snapshot', filteredRowCount: 3 }), true)
  assert.equal(shouldRefreshSnapshotQuotes({ signalDataSource: 'live', filteredRowCount: 3 }), false)
  assert.equal(shouldRefreshSnapshotQuotes({ signalDataSource: 'snapshot', filteredRowCount: 0 }), false)
})

console.log('opportunityQuoteRefresh.test.mjs: all passed')
