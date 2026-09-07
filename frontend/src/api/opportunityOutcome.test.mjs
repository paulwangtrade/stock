/**
 * Unit tests: OutcomeProjection API client (Phase16-G1).
 * Run: node --experimental-strip-types frontend/src/api/opportunityOutcome.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'opportunityOutcome.ts')).href
const {
  OUTCOME_STATUS,
  OpportunityOutcomeNotFoundError,
  mapOutcomeProjection,
  fetchOpportunityOutcomes,
  fetchOpportunityOutcome,
} = await import(modUrl)

const anchorBlocks = {
  signal: {
    present: true,
    snapshot_id: 16,
    signal_tag: '突',
    signal_price: 42.15,
    signal_time: '2026-09-01T15:00:00+08:00',
    trigger_reason: '突破+放量',
  },
  opportunity: {
    present: true,
    pool_id: 3,
    score: 0.86,
    rank: 3,
    strategy_name: 'trend_breakout',
    pool_source: 'strategy_run',
  },
  decision: {
    decision_status: 'BUY_CANDIDATE',
    candidate_status: 'PLAN_FILLED',
    plan_id: 42,
    plan_item_id: 7,
  },
}

const openFixture = {
  outcome_id: 'out:sz301125:fill:1',
  opportunity_id: 'opp:sz301125:2026-09-02:pool:3',
  stock_code: 'sz301125',
  stock_name: '腾亚精工',
  outcome_status: OUTCOME_STATUS.OPEN,
  ...anchorBlocks,
  entry: {
    present: true,
    buy_fill_id: 1,
    buy_plan_id: 42,
    buy_plan_item_id: 7,
    entry_price: 41,
    entry_qty: 500,
    entry_fee: 12.3,
    entry_date: '2026-09-02',
  },
  exit: { present: false },
  performance: {
    entry_price: 41,
    quantity: 500,
    holding_days: 5,
  },
  metadata: {
    source_type: 'round_trip_fifo',
    quality: 'complete',
    missing: [],
    fifo_policy: 'account_default_v1',
    as_of: '2026-09-02T04:00:00Z',
  },
}

const closedFixture = {
  ...openFixture,
  outcome_id: 'out:sz301125:rt:1:2',
  outcome_status: OUTCOME_STATUS.CLOSED,
  exit: {
    present: true,
    sell_fill_id: 2,
    sell_plan_id: 55,
    exit_price: 44,
    exit_qty: 500,
    exit_date: '2026-09-15',
    exit_channel: 'exit_review',
  },
  performance: {
    entry_price: 41,
    exit_price: 44,
    quantity: 500,
    realized_return_pct: 7.317,
    holding_days: 13,
  },
}

const noTradeFixture = {
  outcome_id: 'out:sz301125:noop:2026-09-02',
  opportunity_id: 'opp:sz301125:2026-09-02:pool:3',
  stock_code: 'sz301125',
  stock_name: '腾亚精工',
  outcome_status: OUTCOME_STATUS.NO_TRADE,
  ...anchorBlocks,
  entry: { present: false },
  exit: { present: false },
  performance: {},
  metadata: {
    source_type: 'no_trade',
    quality: 'partial',
    missing: ['entry_fill', 'exit_fill'],
    as_of: '2026-09-02T04:00:00Z',
  },
}

// OPEN
{
  const row = mapOutcomeProjection(openFixture)
  assert.equal(row.outcome_status, OUTCOME_STATUS.OPEN)
  assert.equal(row.entry.present, true)
  assert.equal(row.entry.buy_fill_id, 1)
  assert.equal(row.entry.entry_qty, 500)
  assert.equal(row.exit.present, false)
  assert.equal(row.performance.holding_days, 5)
  assert.deepEqual(row.metadata.missing, [])
}

// CLOSED
{
  const row = mapOutcomeProjection(closedFixture)
  assert.equal(row.outcome_status, OUTCOME_STATUS.CLOSED)
  assert.equal(row.exit.present, true)
  assert.equal(row.exit.exit_price, 44)
  assert.equal(row.performance.realized_return_pct, 7.317)
  assert.equal(row.performance.holding_days, 13)
}

// NO_TRADE
{
  const row = mapOutcomeProjection(noTradeFixture)
  assert.equal(row.outcome_status, OUTCOME_STATUS.NO_TRADE)
  assert.equal(row.entry.present, false)
  assert.equal(row.signal.present, true)
  assert.equal(row.opportunity.present, true)
  assert.deepEqual(row.metadata.missing, ['entry_fill', 'exit_fill'])
}

// Empty list envelope (status filter miss — HTTP 200)
{
  const originalFetch = globalThis.fetch
  globalThis.fetch = async () => ({
    status: 200,
    ok: true,
    json: async () => ({
      code: 0,
      ok: true,
      items: [],
      generated_at: '2026-09-02T04:00:00Z',
    }),
  })
  try {
    const result = await fetchOpportunityOutcome('sz301125', { status: 'CLOSED' })
    assert.equal(result.items.length, 0)
    assert.equal(result.generated_at, '2026-09-02T04:00:00Z')
  } finally {
    globalThis.fetch = originalFetch
  }
}

// 404 — fetchOpportunityOutcome throws OpportunityOutcomeNotFoundError
{
  const originalFetch = globalThis.fetch
  globalThis.fetch = async () => ({
    status: 404,
    ok: false,
    json: async () => ({
      code: 404,
      ok: false,
      message: 'outcome not found',
      items: [],
      generated_at: '2026-09-02T04:00:00Z',
    }),
  })
  try {
    await assert.rejects(
      () => fetchOpportunityOutcome('sh999999'),
      (err) => err instanceof OpportunityOutcomeNotFoundError && err.status === 404,
    )
  } finally {
    globalThis.fetch = originalFetch
  }
}

// fetchOpportunityOutcomes — list envelope
{
  const originalFetch = globalThis.fetch
  globalThis.fetch = async (url) => {
    assert.match(String(url), /\/api\/opportunities\/outcomes\?trade_date=2026-09-02&limit=5/)
    return {
      status: 200,
      ok: true,
      json: async () => ({
        code: 0,
        ok: true,
        items: [openFixture, closedFixture],
        generated_at: '2026-09-02T04:00:00Z',
      }),
    }
  }
  try {
    const result = await fetchOpportunityOutcomes({ tradeDate: '2026-09-02', limit: 5 })
    assert.equal(result.items.length, 2)
    assert.equal(result.items[0].outcome_status, OUTCOME_STATUS.OPEN)
    assert.equal(result.items[1].outcome_status, OUTCOME_STATUS.CLOSED)
  } finally {
    globalThis.fetch = originalFetch
  }
}

console.log('opportunityOutcome.test.mjs: all passed')
