/**
 * Unit tests: opportunity user action helpers (Phase14-G1.1).
 * Run: node frontend/src/utils/opportunityUserAction.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'opportunityUserAction.js')).href
const {
  batchKeyFromSnapshot,
  indexOpportunityEntriesBySecucode,
  isOpportunityWatched,
  buildWatchActionPayload,
  buildIgnoreActionPayload,
  buildIgnoreFromWatchlistItem,
} = await import(modUrl)

{
  assert.equal(batchKeyFromSnapshot({ id: 42 }), 'snap:42')
  assert.equal(
    batchKeyFromSnapshot({ tradeDate: '2026-08-18', session: 'close', strategyId: 'default' }),
    '2026-08-18|close|default',
  )
}

{
  const map = indexOpportunityEntriesBySecucode([
    { secucode: '600363.SH', latest_user_action: { action: 'WATCH' } },
  ])
  assert.equal(isOpportunityWatched(map['600363.SH']), true)
  assert.equal(isOpportunityWatched({ latest_user_action: { action: 'VIEW' } }), false)
  assert.equal(isOpportunityWatched({ latest_user_action: { action: 'IGNORE' } }), false)
}

{
  const payload = buildWatchActionPayload({
    snap: { id: 9 },
    row: { SECUCODE: '600363.SH', SIGNAL_TIME: '2026-08-18' },
    signalSummary: { tag: '强' },
  })
  assert.equal(payload.scanBatchKey, 'snap:9')
  assert.equal(payload.secucode, '600363.SH')
  assert.equal(payload.signalTag, '强')
}

{
  const ignorePayload = buildIgnoreActionPayload({
    snap: { id: 9 },
    row: { SECUCODE: '600363.SH', SIGNAL_TIME: '2026-08-18' },
    signalSummary: { tag: '强' },
  })
  assert.equal(ignorePayload.scanBatchKey, 'snap:9')
  assert.equal(ignorePayload.secucode, '600363.SH')
}

{
  const fromList = buildIgnoreFromWatchlistItem({
    opportunity_id: 'opp_abc',
    scan_batch_key: 'snap:9',
    stock_code: 'sh600363',
  })
  assert.equal(fromList.opportunityId, 'opp_abc')
  assert.equal(fromList.scanBatchKey, 'snap:9')
  assert.equal(fromList.stockCode, 'sh600363')
  assert.equal(buildIgnoreFromWatchlistItem({ stock_code: 'sh600363' }), null)
}

console.log('opportunityUserAction.test.mjs: all passed')
