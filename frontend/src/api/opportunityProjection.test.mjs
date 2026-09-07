/**
 * Unit tests: OpportunityProjection API client (Phase16-C2.0).
 * Run: node --experimental-strip-types frontend/src/api/opportunityProjection.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'opportunityProjection.ts')).href
const {
  DECISION_STATUS,
  OpportunityProjectionNotFoundError,
  mapOpportunityProjection,
  fetchOpportunityProjections,
  fetchOpportunityProjection,
} = await import(modUrl)

const case1Fixture = {
  opportunity_id: 'opp:sz301125:2026-09-02',
  stock_code: 'sz301125',
  stock_name: '腾亚精工',
  trade_date: '2026-09-02',
  signal: {
    present: true,
    snapshot_id: 16,
    session: 'close',
    strategy_id: 'trend_breakout',
    signal_tag: '突',
    signal_price: 42.15,
    signal_time: '2026-09-01T15:00:00+08:00',
    signal_status: 'frozen',
    trigger_reason: '突破+放量',
    schema_version: 'signal_event.v1',
  },
  opportunity: {
    present: true,
    pool_id: 3,
    score: 0.86,
    rank: 3,
    strategy_source: 'run:1',
    strategy_name: 'trend_breakout',
    pool_source: 'strategy_run',
  },
  decision: {
    decision_status: 'WATCH',
    candidate_status: 'IN_POOL',
  },
  trade_plan: { present: false, frozen: false },
  portfolio: { holding_status: 'NOT_HELD' },
  metadata: {
    source_type: 'candidate_pool',
    quality: 'partial',
    missing: ['trade_plan'],
    updated_at: '2026-09-02T04:00:00Z',
  },
}

const case2Fixture = {
  ...case1Fixture,
  decision: {
    decision_status: 'BUY_CANDIDATE',
    candidate_status: 'PLAN_PENDING',
    plan_id: 42,
    plan_item_id: 7,
    item_status: 'pending',
    target_amount: 100000,
    decision_provider: 'fixed',
  },
  trade_plan: {
    present: true,
    plan_id: 42,
    plan_status: 'draft',
    item_status: 'pending',
    trade_date: '2026-09-02',
    frozen: false,
  },
  portfolio: { holding_status: 'HELD', position_qty: 500 },
  metadata: {
    source_type: 'candidate_pool',
    quality: 'complete',
    updated_at: '2026-09-02T04:00:00Z',
  },
}

const case3Fixture = {
  opportunity_id: 'opp:sh600519:2026-09-01',
  stock_code: 'sh600519',
  stock_name: '贵州茅台',
  trade_date: '2026-09-01',
  signal: {
    present: true,
    snapshot_id: 17,
    signal_tag: '强',
    signal_price: 1800,
    signal_time: '2026-09-01T15:00:00+08:00',
  },
  opportunity: { present: false },
  decision: {
    decision_status: 'NOT_IN_PLAN',
    candidate_status: 'NOT_IN_POOL',
  },
  trade_plan: { present: false, frozen: false },
  portfolio: { holding_status: 'NOT_HELD' },
  research: {
    research_id: 'res:sh600519',
    status: 'active',
    tags: ['research_only'],
    signal_score: 0.72,
    explain_summary: 'RSI 超卖观察',
  },
  metadata: {
    source_type: 'research_candidate',
    quality: 'partial',
    missing: ['opportunity'],
    updated_at: '2026-09-01T04:00:00Z',
  },
}

// Case1: Signal + Pool → WATCH
{
  const row = mapOpportunityProjection(case1Fixture)
  assert.equal(row.stock_code, 'sz301125')
  assert.equal(row.signal.present, true)
  assert.equal(row.signal.signal_tag, '突')
  assert.equal(row.opportunity.present, true)
  assert.equal(row.opportunity.score, 0.86)
  assert.equal(row.decision.decision_status, DECISION_STATUS.WATCH)
  assert.equal(row.trade_plan.present, false)
}

// Case2: Signal + Pool + TradePlan → BUY_CANDIDATE + HELD
{
  const row = mapOpportunityProjection(case2Fixture)
  assert.equal(row.decision.decision_status, DECISION_STATUS.BUY_CANDIDATE)
  assert.equal(row.trade_plan.present, true)
  assert.equal(row.trade_plan.plan_id, 42)
  assert.equal(row.portfolio.holding_status, 'HELD')
  assert.equal(row.portfolio.position_qty, 500)
}

// Case3: Research only — no Pool, not BUY_CANDIDATE
{
  const row = mapOpportunityProjection(case3Fixture)
  assert.equal(row.signal.present, true)
  assert.equal(row.opportunity.present, false)
  assert.notEqual(row.decision.decision_status, DECISION_STATUS.BUY_CANDIDATE)
  assert.ok(row.research)
  assert.equal(row.research.research_id, 'res:sh600519')
  assert.deepEqual(row.research.tags, ['research_only'])
}

// Case4: 404 — fetchOpportunityProjection throws OpportunityProjectionNotFoundError
{
  const originalFetch = globalThis.fetch
  globalThis.fetch = async () => ({
    status: 404,
    ok: false,
    json: async () => ({
      code: 404,
      ok: false,
      message: 'projection not found',
      items: [],
      total: 0,
      generated_at: '2026-09-02T04:00:00Z',
    }),
  })
  try {
    await assert.rejects(
      () => fetchOpportunityProjection('sh999999', { tradeDate: '2026-09-02' }),
      (err) => err instanceof OpportunityProjectionNotFoundError && err.status === 404,
    )
  } finally {
    globalThis.fetch = originalFetch
  }
}

// fetchOpportunityProjections — list envelope
{
  const originalFetch = globalThis.fetch
  globalThis.fetch = async (url) => {
    assert.match(String(url), /\/api\/opportunities\/projections\?trade_date=2026-09-02&limit=10/)
    return {
      status: 200,
      ok: true,
      json: async () => ({
        code: 0,
        ok: true,
        items: [case1Fixture],
        total: 1,
        generated_at: '2026-09-02T04:00:00Z',
      }),
    }
  }
  try {
    const result = await fetchOpportunityProjections({ tradeDate: '2026-09-02', limit: 10 })
    assert.equal(result.total, 1)
    assert.equal(result.items.length, 1)
    assert.equal(result.items[0].decision.decision_status, DECISION_STATUS.WATCH)
    assert.equal(result.generated_at, '2026-09-02T04:00:00Z')
  } finally {
    globalThis.fetch = originalFetch
  }
}

console.log('opportunityProjection.test.mjs: all passed')
