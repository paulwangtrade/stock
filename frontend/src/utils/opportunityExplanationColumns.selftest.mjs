/** Phase16.28 — opportunity explanation column helpers (no network). */
import {
  buildSignalSnapshotDisplayProjection,
  formatOpportunityTableCell,
  resolveOpportunityReasonSummary,
  resolveOpportunitySourceBucket,
  resolveOpportunityStrategyLabel,
  opportunitySourceChipMeta,
} from './opportunityExplanationColumns.js'

function assert(cond, msg) {
  if (!cond) throw new Error(msg || 'assert failed')
}

const strategyProj = {
  metadata: { source_type: 'candidate_pool' },
  opportunity: {
    present: true,
    pool_source: 'strategy_run',
    strategy_name: '趋势突破',
    strategy_source: '趋势突破@v1',
  },
  signal: {
    present: true,
    trigger_reason: '突破近20日高点且量能放大，符合趋势跟踪入选条件',
    signal_tag: '突破',
  },
}

assert(resolveOpportunitySourceBucket(strategyProj) === 'strategy', 'strategy bucket')
assert(opportunitySourceChipMeta(strategyProj).label === 'Strategy', 'Strategy label')
assert(resolveOpportunityStrategyLabel(strategyProj) === '趋势突破', 'strategy name')
assert(resolveOpportunityReasonSummary(strategyProj, 12).endsWith('…'), 'truncate')
assert(formatOpportunityTableCell('') === '—', 'empty cell')

const followProj = {
  metadata: { source_type: 'candidate_pool' },
  opportunity: { present: true, pool_source: 'follow' },
  signal: { present: false },
}
assert(resolveOpportunitySourceBucket(followProj) === 'manual', 'follow → manual')

const empty = resolveOpportunitySourceBucket(null)
assert(empty === 'unknown', 'null → unknown')
assert(resolveOpportunityReasonSummary(followProj) === '', 'no reason')

const snapFallback = buildSignalSnapshotDisplayProjection(
  { tag: '转', statusText: '今日趋势买点 · RSI 60' },
  '默认策略',
)
assert(snapFallback, 'snapshot fallback built')
assert(resolveOpportunitySourceBucket(snapFallback) === 'strategy', 'signal_snapshot → strategy')
assert(resolveOpportunityStrategyLabel(snapFallback) === '默认策略', 'fallback strategy name')
assert(resolveOpportunityReasonSummary(snapFallback).includes('趋势'), 'fallback reason')
assert(buildSignalSnapshotDisplayProjection({}) == null, 'empty summary → null')

console.log('opportunityExplanationColumns.selftest: OK')
