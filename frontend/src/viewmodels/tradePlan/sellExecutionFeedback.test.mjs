/**
 * Unit tests: sell execution feedback (Phase14-A-R1-D).
 * Run: node frontend/src/viewmodels/tradePlan/sellExecutionFeedback.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'sellExecutionFeedback.js')).href
const {
  T_SELL_EXEC_STATUS,
  resolveTSellExecutionStatus,
  buildSellExecutionFeedback,
  SELL_CASH_NOTICE,
} = await import(modUrl)

{
  assert.equal(resolveTSellExecutionStatus({ executing: true }), T_SELL_EXEC_STATUS.executing)
  assert.equal(
    resolveTSellExecutionStatus({ isApproved: false, isFrozen: false }),
    T_SELL_EXEC_STATUS.draft,
  )
  assert.equal(
    resolveTSellExecutionStatus({ isApproved: true, isFrozen: false }),
    T_SELL_EXEC_STATUS.approved,
  )
  assert.equal(
    resolveTSellExecutionStatus({ isApproved: true, isFrozen: true }),
    T_SELL_EXEC_STATUS.frozen,
  )
  assert.equal(resolveTSellExecutionStatus({ hasFill: true }), T_SELL_EXEC_STATUS.filled)
}

{
  const fb = buildSellExecutionFeedback({
    plan: {
      items: [{ side: 'sell', stock_code: 'sz000001', status: 'filled', filled_volume: 100 }],
    },
    lifecycle: { firstFillAt: '2026-08-28T10:00:00Z' },
    dashboardFills: [],
    lastRun: { filledCount: 1 },
  })
  assert.equal(fb.executedQuantity, 100)
  assert.equal(fb.hasFill, true)
  assert.equal(fb.cashNotice, SELL_CASH_NOTICE)
}

console.log('sellExecutionFeedback.test.mjs: all passed')
