/**
 * Unit tests: OpportunityProjection display (Phase16-C2.1).
 * Run: node frontend/src/utils/opportunityProjectionDisplay.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'opportunityProjectionDisplay.js')).href
const {
  buildOpportunityProjectionView,
  buildProjectionPipeline,
  isProjectionLayerActive,
  PROJECTION_LAYER_SOURCE,
} = await import(modUrl)

const signalOnlyFixture = {
  opportunity_id: 'opp:sh600000:2026-09-01',
  stock_code: 'sh600000',
  stock_name: '浦发银行',
  trade_date: '2026-09-01',
  signal: {
    present: true,
    signal_tag: '强',
    signal_price: 8.5,
    signal_time: '2026-09-01T15:00:00+08:00',
    trigger_reason: '趋势增强',
  },
  opportunity: { present: false },
  decision: { decision_status: 'NOT_IN_PLAN', candidate_status: 'NOT_IN_POOL' },
  trade_plan: { present: false, frozen: false },
  portfolio: { holding_status: 'NOT_HELD' },
  metadata: { source_type: 'signal_snapshot', quality: 'partial', updated_at: '2026-09-01T04:00:00Z' },
}

const signalPoolFixture = {
  ...signalOnlyFixture,
  stock_code: 'sz301125',
  stock_name: '腾亚精工',
  trade_date: '2026-09-02',
  signal: {
    present: true,
    signal_tag: '突',
    signal_price: 42.15,
    signal_time: '2026-09-01T15:00:00+08:00',
    trigger_reason: '突破+放量',
  },
  opportunity: { present: true, score: 0.86, rank: 3, strategy_source: 'run:1' },
  decision: { decision_status: 'WATCH', candidate_status: 'IN_POOL' },
}

const fullLoopFixture = {
  ...signalPoolFixture,
  decision: {
    decision_status: 'BUY_CANDIDATE',
    candidate_status: 'PLAN_PENDING',
    plan_id: 42,
  },
  trade_plan: {
    present: true,
    plan_id: 42,
    plan_status: 'draft',
    trade_date: '2026-09-02',
    frozen: false,
  },
  portfolio: { holding_status: 'HELD', position_qty: 500 },
  metadata: { source_type: 'candidate_pool', quality: 'complete', updated_at: '2026-09-02T04:00:00Z' },
}

const researchOnlyFixture = {
  opportunity_id: 'opp:sh600519:2026-09-01',
  stock_code: 'sh600519',
  stock_name: '贵州茅台',
  trade_date: '2026-09-01',
  signal: { present: true, signal_tag: '强', signal_price: 1800, signal_time: '2026-09-01T15:00:00+08:00' },
  opportunity: { present: false },
  decision: { decision_status: 'NOT_IN_PLAN', candidate_status: 'NOT_IN_POOL' },
  trade_plan: { present: false, frozen: false },
  portfolio: { holding_status: 'NOT_HELD' },
  research: { research_id: 'res:sh600519', tags: ['research_only', 'rsi'] },
  metadata: { source_type: 'research_candidate', quality: 'partial', updated_at: '2026-09-01T04:00:00Z' },
}

function section(view, id) {
  return view.sections.find((s) => s.id === id)
}

// Case1: Signal only — Pool absent
{
  const view = buildOpportunityProjectionView(signalOnlyFixture)
  assert.ok(view)
  assert.equal(section(view, 'signal').present, true)
  assert.equal(section(view, 'signal').fields[0].source, PROJECTION_LAYER_SOURCE.signal)
  assert.equal(section(view, 'opportunity').present, false)
  assert.equal(isProjectionLayerActive(signalOnlyFixture, 'signal'), true)
  assert.equal(isProjectionLayerActive(signalOnlyFixture, 'opportunity'), false)
  const pipeline = buildProjectionPipeline(signalOnlyFixture)
  assert.equal(pipeline.find((p) => p.key === 'signal').active, true)
  assert.equal(pipeline.find((p) => p.key === 'opportunity').active, false)
}

// Case2: Signal + Pool
{
  const view = buildOpportunityProjectionView(signalPoolFixture)
  assert.equal(section(view, 'signal').fields[0].value, '突')
  assert.equal(section(view, 'opportunity').present, true)
  const oppFields = section(view, 'opportunity').fields
  const scoreField = oppFields.find((f) => f.label === 'score')
  assert.ok(scoreField, 'score field present')
  assert.equal(scoreField.value, '0.8600')
  assert.equal(scoreField.source, PROJECTION_LAYER_SOURCE.opportunity)
  const poolSource = oppFields.find((f) => f.label === 'pool_source')
  assert.ok(poolSource, 'pool_source field present')
  assert.equal(section(view, 'decision').fields[0].value, '观察')
}

// Case3: Full loop
{
  const view = buildOpportunityProjectionView(fullLoopFixture)
  assert.equal(section(view, 'decision').fields[0].value, '买入候选')
  assert.equal(section(view, 'trade_plan').present, true)
  assert.equal(section(view, 'trade_plan').fields[0].value, '42')
  assert.equal(section(view, 'portfolio').fields[0].value, '已持仓')
  assert.equal(view.quality, 'complete')
}

// Case4: Research only overlay — no Pool, research section present
{
  const view = buildOpportunityProjectionView(researchOnlyFixture)
  assert.equal(section(view, 'opportunity').present, false)
  assert.equal(section(view, 'research').present, true)
  assert.match(section(view, 'research').fields[0].value, /research_only/)
  assert.equal(section(view, 'research').fields[0].source, PROJECTION_LAYER_SOURCE.research)
  assert.notEqual(section(view, 'decision').fields[0].value, '买入候选')
}

console.log('opportunityProjectionDisplay.test.mjs: all passed')
