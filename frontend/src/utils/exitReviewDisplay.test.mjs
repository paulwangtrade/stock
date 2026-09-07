/**
 * Unit tests: exit review display helpers (Phase14-D2).
 * Run: node frontend/src/utils/exitReviewDisplay.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'exitReviewDisplay.js')).href
const {
  EXIT_EVAL_FILTER,
  EXIT_EVAL_STATE,
  buildExitReviewRiskFactors,
  exitEvalStateRank,
  exitReasonLabel,
  exitReviewDecisionLabel,
  exitReviewActionButtonProps,
  filterExitEvalByMode,
  findHoldingEvalRow,
  hasLatestExitOutcome,
  latestExitOutcomeSummary,
  shouldShowExitReviewAction,
  sortExitEvalHoldings,
} = await import(modUrl)

function row(code, state, reasons = []) {
  return { stockCode: code, evaluation: { state, reasonCodes: reasons } }
}

{
  assert.equal(exitEvalStateRank('REVIEW_REQUIRED'), 3)
  assert.equal(exitEvalStateRank('WATCH'), 2)
  assert.equal(exitEvalStateRank('NORMAL'), 1)
}

{
  assert.equal(shouldShowExitReviewAction(row('a', 'REVIEW_REQUIRED')), true)
  assert.equal(shouldShowExitReviewAction(row('b', 'WATCH')), true)
  assert.equal(shouldShowExitReviewAction(row('c', 'NORMAL')), false)
}

{
  const props = exitReviewActionButtonProps('REVIEW_REQUIRED')
  assert.equal(props.type, 'primary')
  assert.equal(props.quaternary, true)
  assert.equal(exitReviewActionButtonProps('NORMAL').text, true)
}

{
  assert.equal(exitReasonLabel('TIME_REVIEW'), '持有时间关注')
  assert.equal(exitReasonLabel('LOSS_REVIEW'), '亏损关注')
  assert.equal(exitReasonLabel('PLAN_REVIEW'), '计划变化关注')
}

{
  const sorted = sortExitEvalHoldings([
    row('sz000003', 'NORMAL'),
    row('sz000001', 'REVIEW_REQUIRED'),
    row('sz000002', 'WATCH'),
  ])
  assert.deepEqual(
    sorted.map((r) => r.stockCode),
    ['sz000001', 'sz000002', 'sz000003'],
  )
}

{
  const all = [row('a', 'NORMAL'), row('b', 'WATCH'), row('c', 'REVIEW_REQUIRED')]
  assert.equal(filterExitEvalByMode(all, EXIT_EVAL_FILTER.ALL).length, 3)
  assert.deepEqual(
    filterExitEvalByMode(all, EXIT_EVAL_FILTER.WATCH_PLUS).map((r) => r.stockCode),
    ['b', 'c'],
  )
  assert.deepEqual(
    filterExitEvalByMode(all, EXIT_EVAL_FILTER.REVIEW_REQUIRED).map((r) => r.stockCode),
    ['c'],
  )
}

{
  const evalView = {
    holdings: [{ stockCode: 'SZ000001', totalVolume: 100 }],
  }
  assert.equal(findHoldingEvalRow(evalView, 'sz000001')?.totalVolume, 100)
  assert.equal(findHoldingEvalRow(evalView, 'sh600000'), null)
}

{
  const rf = buildExitReviewRiskFactors({
    holdingDays: 25,
    unrealizedReturn: -0.12,
    reasonCodes: ['TIME_REVIEW', 'LOSS_REVIEW'],
    state: EXIT_EVAL_STATE.REVIEW_REQUIRED,
    policy: { maxHoldingDays: 20, lossWatchThreshold: -0.05, lossReviewThreshold: -0.1 },
    lots: [{ context: { plan: { planStatus: 'ready' } } }],
  })
  assert.equal(rf.triggeredCount, 2)
  assert.equal(rf.factors.length, 3)
  assert.equal(rf.factors[0].triggered, true)
  assert.equal(rf.factors[1].triggered, true)
  assert.equal(rf.factors[2].triggered, false)
  assert.match(rf.summaryLine, /2 项因素触线/)
}

{
  const rf = buildExitReviewRiskFactors({
    holdingDays: 5,
    unrealizedReturn: null,
    reasonCodes: [],
    state: EXIT_EVAL_STATE.NORMAL,
    policy: { maxHoldingDays: 20, lossWatchThreshold: -0.05, lossReviewThreshold: -0.1 },
    lots: [],
  })
  assert.equal(rf.triggeredCount, 0)
  assert.match(rf.summaryLine, /暂无复评信号/)
}

{
  assert.equal(exitReviewDecisionLabel('HOLD'), '继续持有')
  assert.equal(exitReviewDecisionLabel('WATCH'), '加入观察')
  assert.equal(exitReviewDecisionLabel('CREATE_SELL_PLAN'), '生成卖出计划')
  assert.equal(hasLatestExitOutcome({ latestOutcome: { id: 'x', decision: 'HOLD' } }), true)
  assert.equal(hasLatestExitOutcome({}), false)
  assert.match(
    latestExitOutcomeSummary({
      latestOutcome: { id: 'x', decision: 'HOLD', reviewTime: '2026-08-29T10:00:00+08:00' },
    }),
    /记录：继续持有/,
  )
}

console.log('exitReviewDisplay.test.mjs: all passed')
