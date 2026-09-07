/**
 * Unit tests: Opportunity decision badge display (Phase16-D4).
 * Run: node frontend/src/utils/opportunityDecisionBadgeDisplay.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'opportunityDecisionBadgeDisplay.js')).href
const {
  TIER,
  TIER_LABEL,
  tierFromProjection,
  tierTagType,
  buildDecisionBadgeView,
  formatTradePlanLabel,
  decisionStatusLabel,
  buildProjectionMap,
  resolveProjectionBatchLimit,
} = await import(modUrl)

const poolWatch = {
  opportunity: { present: true, rank: 3 },
  decision: { decision_status: 'WATCH' },
  signal: { present: true, signal_tag: '突' },
  trade_plan: { present: false, frozen: false },
  metadata: { quality: 'complete' },
}

const tradeCandidate = {
  ...poolWatch,
  decision: { decision_status: 'BUY_CANDIDATE' },
  trade_plan: { present: true, plan_id: 42, frozen: false },
}

// D4-1: BUY_CANDIDATE → 交易候选
assert.equal(tierFromProjection(tradeCandidate), TIER.TRADE_CANDIDATE)
const v1 = buildDecisionBadgeView(tradeCandidate, '')
assert.equal(v1.tagLabel, TIER_LABEL.trade_candidate)
assert.equal(v1.tagType, 'warning')
assert.ok(v1.tooltipLines.some((l) => l.includes('Decision: 买入候选')))

// D4-2: Pool + WATCH → 池候选
assert.equal(tierFromProjection(poolWatch), TIER.POOL_CANDIDATE)
const v2 = buildDecisionBadgeView(poolWatch, '')
assert.equal(v2.tagLabel, TIER_LABEL.pool_candidate)
assert.equal(v2.tagType, 'info')

// D4-3: batch miss → 普通信号; row signal in tooltip
const v3 = buildDecisionBadgeView(null, '突')
assert.equal(v3.tagLabel, TIER_LABEL.signal_only)
assert.ok(v3.tooltipLines.some((l) => l === 'Signal: 突'))
assert.ok(v3.tooltipLines.some((l) => l === 'Opportunity Rank: 未入池'))

// REJECT in pool still pool_candidate tier
const rejectPool = {
  ...poolWatch,
  decision: { decision_status: 'REJECT' },
}
assert.equal(tierFromProjection(rejectPool), TIER.POOL_CANDIDATE)
assert.equal(decisionStatusLabel('REJECT'), '拒绝')

// D4-4: trade_plan.present
assert.equal(formatTradePlanLabel({ present: true, plan_id: 1, frozen: false }), '已生成')
assert.equal(formatTradePlanLabel({ present: true, plan_id: 1, frozen: true }), '已生成（已冻结）')
assert.equal(formatTradePlanLabel({ present: false, frozen: false }), '未生成')
const v4 = buildDecisionBadgeView(tradeCandidate, '')
assert.ok(v4.tooltipLines.some((l) => l === 'TradePlan: 已生成'))

// D4-5: loading
const v5 = buildDecisionBadgeView(null, '', { loading: true })
assert.equal(v5.tagLabel, '…')
assert.equal(v5.clickable, false)

// error / retry
const vErr = buildDecisionBadgeView(null, '', { error: true })
assert.equal(vErr.tagLabel, '—')
assert.equal(vErr.retry, true)

// buildProjectionMap
const map = buildProjectionMap([
  { stock_code: 'SZ301125', opportunity_id: 'a' },
  { stock_code: 'sh600000', opportunity_id: 'b' },
])
assert.equal(map.size, 2)
assert.ok(map.has('sz301125'))
assert.ok(map.has('sh600000'))

// resolveProjectionBatchLimit
assert.equal(resolveProjectionBatchLimit(50, 20), 50)
assert.equal(resolveProjectionBatchLimit(200, 20), 100)
assert.equal(resolveProjectionBatchLimit(0, 30), 30)

console.log('opportunityDecisionBadgeDisplay.test.mjs: all passed')
