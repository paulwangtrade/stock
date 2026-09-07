/**
 * Phase13 TradePlan Presentation mapper unit tests (P0)
 *   node scripts/verify-tradeplan-presentation.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, '../src/viewmodels/tradePlanPresentation.js')).href
const {
  TRADEPLAN_PRESENTATION_SCHEMA,
  buildTradePlanPresentation,
  isExecutionCandidate,
  isBlockedCandidate,
  formatBlockReason,
} = await import(modUrl)

function item(partial) {
  return {
    stock_code: 'sz000001',
    stock_name: '平安银行',
    side: 'buy',
    priority: 1,
    target_amount: 100000,
    status: 'pending',
    score: 1,
    risk_code: '',
    risk_message: '',
    strategy_name: 's',
    ref_price: 0,
    limit_price: 0,
    target_volume: 0,
    entry_rule: '',
    intent_status: '',
    ...partial,
  }
}

// --- classifiers ---
assert.equal(isExecutionCandidate(item({ status: 'pending' })), true)
assert.equal(isExecutionCandidate(item({ status: 'filled' })), true)
assert.equal(isExecutionCandidate(item({ status: 'skipped' })), false)
assert.equal(isExecutionCandidate(item({ status: 'PENDING' })), true)
assert.equal(
  isExecutionCandidate(item({ status: 'pending', intent_status: 'gap_skip' })),
  false,
)

assert.equal(isBlockedCandidate(item({ status: 'skipped' })), true)
assert.equal(isBlockedCandidate(item({ status: 'error' })), true)
assert.equal(isBlockedCandidate(item({ status: 'pending', intent_status: 'gap_skip' })), true)
assert.equal(isBlockedCandidate(item({ status: 'pending' })), false)

assert.ok(formatBlockReason(item({ status: 'skipped', risk_code: 'CASH_INSUFFICIENT', risk_message: '现金不足' })).includes('CASH_INSUFFICIENT'))
assert.ok(formatBlockReason(item({ intent_status: 'gap_skip' })).includes('gap_skip'))

// --- mapper: mixed plan (over-generation shape) ---
const plan = {
  id: 42,
  trade_date: '2026-08-25',
  status: 'draft',
  pool_id: 9,
  items: [
    item({ stock_code: 'sz000001', status: 'pending', priority: 1 }),
    item({ stock_code: 'sz000002', status: 'pending', priority: 2 }),
    item({
      stock_code: 'sz000003',
      status: 'skipped',
      priority: 3,
      risk_code: 'SINGLE_NAME_EXCEEDED',
      risk_message: '单票超限',
    }),
    item({
      stock_code: 'sz000004',
      status: 'skipped',
      priority: 4,
      risk_code: 'CASH_INSUFFICIENT',
      risk_message: '现金不足',
    }),
    item({ stock_code: 'sz000005', status: 'pending', intent_status: 'gap_skip', priority: 5 }),
  ],
}

const view = buildTradePlanPresentation(plan)

assert.equal(view.schemaVersion, TRADEPLAN_PRESENTATION_SCHEMA)
assert.equal(view.planId, 42)
assert.equal(view.poolId, 9)

// Must NOT treat raw items.length as execution count
assert.equal(plan.items.length, 5)
assert.equal(view.summary.rawItemCount, 5)
assert.equal(view.executionCandidates.length, 2)
assert.equal(view.summary.executionCount, 2)
assert.equal(view.blockedCandidates.length, 3)
assert.equal(view.summary.blockedCount, 3)

// research empty without pool
assert.equal(view.researchCandidates.length, 0)
assert.equal(view.summary.researchCount, 0)
assert.equal(view.summary.researchEmpty, true)
assert.equal(view.summary.researchEmptyReason, 'pool_not_loaded')

assert.deepEqual(
  view.executionCandidates.map((r) => r.stock_code),
  ['sz000001', 'sz000002'],
)

const blockedCodes = view.blockedCandidates.map((r) => r.stock_code)
assert.ok(blockedCodes.includes('sz000003'))
assert.ok(blockedCodes.includes('sz000004'))
assert.ok(blockedCodes.includes('sz000005'))

const single = view.blockedCandidates.find((r) => r.stock_code === 'sz000003')
assert.ok(String(single.block_reason).includes('SINGLE_NAME_EXCEEDED'))

// counts API used by UI must not equal raw length when skipped present
assert.notEqual(view.executionCandidates.length, plan.items.length)

// --- with injected pool (P1 hook) ---
const withPool = buildTradePlanPresentation(plan, {
  poolItems: [{ stock_code: 'sh600000', stock_name: '浦发', score: 88, rank: 1 }],
})
assert.equal(withPool.researchCandidates.length, 1)
assert.equal(withPool.summary.researchCount, 1)
assert.equal(withPool.summary.researchEmpty, false)
assert.equal(withPool.researchCandidates[0].stock_code, 'sh600000')

// --- null plan ---
const empty = buildTradePlanPresentation(null)
assert.equal(empty.executionCandidates.length, 0)
assert.equal(empty.blockedCandidates.length, 0)
assert.equal(empty.summary.rawItemCount, 0)

console.log('Phase13 TradePlan Presentation P0')
console.log(
  `  execution=${view.summary.executionCount} blocked=${view.summary.blockedCount} research=${view.summary.researchCount} raw=${view.summary.rawItemCount}`,
)
console.log('verify-tradeplan-presentation: passed')
