/**
 * Unit tests: OutcomeProjection display (Phase16-D4).
 * Run: node frontend/src/utils/opportunityOutcomeDisplay.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'opportunityOutcomeDisplay.js')).href
const {
  OUTCOME_STATUS_LABEL,
  OUTCOME_EMPTY_MESSAGE,
  buildOutcomeLegSummary,
  buildOutcomeTabView,
  defaultLegIndex,
  formatOutcomeReturnPct,
  outcomeStatusTagType,
} = await import(modUrl)

const openLeg = {
  outcome_id: 'out:sz301125:fill:1',
  outcome_status: 'OPEN',
  signal: { present: true, signal_tag: '突' },
  opportunity: { present: true, rank: 3, score: 0.86 },
  decision: { decision_status: 'BUY_CANDIDATE', candidate_status: 'PLAN_FILLED' },
  entry: { present: true, buy_fill_id: 1001, buy_plan_id: 42, entry_price: 41, entry_qty: 500, entry_date: '2026-09-02' },
  exit: { present: false },
  performance: { holding_days: 5 },
  metadata: { quality: 'complete', as_of: '2026-09-02T04:00:00Z' },
}

const closedLeg = {
  outcome_id: 'out:sz301125:rt:1:2',
  outcome_status: 'CLOSED',
  signal: { present: true, signal_tag: '突' },
  opportunity: { present: true },
  decision: { decision_status: 'BUY_CANDIDATE', candidate_status: 'PLAN_FILLED' },
  entry: { present: true, buy_fill_id: 1001, entry_date: '2026-09-02' },
  exit: { present: true, exit_price: 44, exit_qty: 500, exit_date: '2026-09-15' },
  performance: { realized_return_pct: 8.32, holding_days: 13 },
  metadata: { quality: 'complete' },
}

const noTradeLeg = {
  outcome_id: 'out:sz301125:noop:2026-09-02',
  outcome_status: 'NO_TRADE',
  signal: { present: true, signal_tag: '突' },
  opportunity: { present: true },
  decision: { decision_status: 'NOT_IN_PLAN', candidate_status: 'NOT_IN_POOL' },
  entry: { present: false },
  exit: { present: false },
  performance: {},
  metadata: { quality: 'partial', as_of: '2026-09-02T04:00:00Z' },
}

// OPEN
assert.match(buildOutcomeLegSummary(openLeg), /持仓中/)
assert.equal(outcomeStatusTagType('OPEN'), 'info')
{
  const view = buildOutcomeTabView([openLeg], 0, '2026-09-02T04:00:00Z')
  assert.equal(view.selected.statusLabel, OUTCOME_STATUS_LABEL.OPEN)
  assert.equal(view.explainSections.length, 4)
  assert.equal(view.resultSections.entry.present, true)
  assert.equal(view.resultSections.exit.isOpenLeg, true)
  const openExit = view.resultSections.exit.fields.find((f) => f.label === '卖出')
  assert.equal(openExit?.value, '持仓中，尚未卖出')
  assert.equal(view.resultSections.performance.fields.length, 1)
}

// CLOSED
assert.match(buildOutcomeLegSummary(closedLeg), /已平仓/)
assert.match(buildOutcomeLegSummary(closedLeg), /\+8\.32%/)
assert.equal(formatOutcomeReturnPct(8.32), '+8.32%')
assert.equal(outcomeStatusTagType('CLOSED'), 'success')
{
  const view = buildOutcomeTabView([closedLeg], 0)
  assert.equal(view.selected.returnPct, '+8.32%')
  assert.equal(view.resultSections.exit.present, true)
  assert.equal(view.resultSections.performance.fields.length, 2)
  const retField = view.resultSections.performance.fields.find((f) => f.label === '实现收益率')
  assert.equal(retField?.value, '+8.32%')
}

// NO_TRADE
assert.equal(OUTCOME_STATUS_LABEL.NO_TRADE, '无成交')
assert.equal(buildOutcomeLegSummary(noTradeLeg), '无成交')
{
  const view = buildOutcomeTabView([noTradeLeg], 0)
  assert.equal(view.selected.status, 'NO_TRADE')
  assert.equal(view.resultSections.entry.present, false)
}

// Empty items (200 empty list — not 404)
{
  const view = buildOutcomeTabView([])
  assert.equal(view.isEmpty, true)
  assert.equal(OUTCOME_EMPTY_MESSAGE, '暂无交易结果投影')
}

// Multi-leg default index prefers OPEN
assert.equal(defaultLegIndex([closedLeg, openLeg]), 1)

// Align with provenance trade fillId
assert.equal(
  defaultLegIndex([closedLeg, openLeg], [{ fillId: 1001 }]),
  0,
)

// Tab view multi-leg
{
  const view = buildOutcomeTabView([openLeg, closedLeg], 1)
  assert.equal(view.legs.length, 2)
  assert.equal(view.selected.statusLabel, '已平仓')
}

console.log('opportunityOutcomeDisplay.test.mjs: all passed')
