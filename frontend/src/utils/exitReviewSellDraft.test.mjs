/**
 * Unit tests: Exit Review → sell draft helpers (Phase14-M1).
 * Run: node frontend/src/utils/exitReviewSellDraft.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'exitReviewSellDraft.js')).href
const { buildExitReviewSellDraftRow, buildExitReviewSellReason, EXIT_REVIEW_SELL_ACTOR } =
  await import(modUrl)

{
  const row = buildExitReviewSellDraftRow({
    stockCode: 'sz000001',
    stockName: '平安银行',
    availableQty: 800,
    positionState: 'AVAILABLE',
    canSell: true,
  })
  assert.equal(row.stockCode, 'sz000001')
  assert.equal(row.stockName, '平安银行')
  assert.equal(row.availableQty, 800)
  assert.equal(row.positionState, 'AVAILABLE')
  assert.equal(row.canSell, true)
  assert.equal(row.source, 'paper_sim')
}

{
  const reason = buildExitReviewSellReason({
    exitState: 'REVIEW_REQUIRED',
    reasonCodes: ['LOSS_REVIEW', 'TIME_REVIEW'],
  })
  assert.match(reason, /^exit_review:REVIEW_REQUIRED/)
  assert.match(reason, /LOSS_REVIEW,TIME_REVIEW/)
  assert.match(reason, /亏损关注/)
}

{
  const reason = buildExitReviewSellReason({
    exitState: 'WATCH',
    reasonCodes: [],
    userNote: '等待财报',
  })
  assert.equal(reason, 'exit_review:WATCH;note=等待财报')
}

assert.equal(EXIT_REVIEW_SELL_ACTOR, 'ui:exit-review-sell')

console.log('exitReviewSellDraft.test.mjs: all passed')
