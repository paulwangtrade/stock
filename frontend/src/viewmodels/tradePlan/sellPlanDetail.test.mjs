/**
 * Unit tests: T-sell sell plan detail card (Phase14-A-R1-E).
 * Run: node frontend/src/viewmodels/tradePlan/sellPlanDetail.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'sellPlanDetail.js')).href
const {
  buildSellPlanDetailCard,
  resolvePlanLifecycleTagType,
  resolveSellReason,
  resolveAvailableQty,
} = await import(modUrl)

const samplePlan = {
  generated_at: '2026-08-29T10:15:30+08:00',
  risk: { reasons: ['fallback reason'] },
  items: [
    {
      side: 'sell',
      stock_code: '600519',
      stock_name: '贵州茅台',
      target_volume: 100,
      risk_message: '',
      strategy_name: '',
      entry_rule: '',
    },
  ],
}

{
  const card = buildSellPlanDetailCard({
    plan: samplePlan,
    snapshotPositions: [{ stockCode: '600519', availableQty: 500 }],
  })
  assert.equal(card.stockCode, '600519')
  assert.equal(card.stockName, '贵州茅台')
  assert.equal(card.sellQuantity, 100)
  assert.equal(card.availableQty, 500)
  assert.match(card.createdAtLabel, /2026-08-29/)
}

{
  const item = samplePlan.items[0]
  assert.equal(resolveSellReason({ ...item, risk_message: '止盈减仓' }, samplePlan), '止盈减仓')
  assert.equal(resolveSellReason(item, samplePlan), 'fallback reason')
}

{
  const item = { ...samplePlan.items[0], reason: 'manual sell' }
  assert.equal(resolveSellReason(item, samplePlan), 'manual sell')
  const card = buildSellPlanDetailCard({ plan: { ...samplePlan, items: [item] } })
  assert.equal(card.sellReason, 'manual sell')
}

{
  const item = {
    ...samplePlan.items[0],
    reason: 'exit_review:REVIEW_REQUIRED;LOSS_REVIEW',
    risk_message: 'should not win',
  }
  assert.equal(resolveSellReason(item, samplePlan), 'exit_review:REVIEW_REQUIRED;LOSS_REVIEW')
}

{
  assert.equal(resolveAvailableQty([{ stockCode: '000001', availableQty: 200 }], '000001'), 200)
  assert.equal(resolveAvailableQty([], '000001'), null)
}

{
  assert.equal(resolvePlanLifecycleTagType({ lifecycleLabel: 'Draft', isReady: false, isFrozen: false }), 'warning')
  assert.equal(resolvePlanLifecycleTagType({ lifecycleLabel: 'Ready', isReady: true, isFrozen: false }), 'success')
  assert.notEqual(
    resolvePlanLifecycleTagType({ lifecycleLabel: 'Draft', isReady: false, isFrozen: false }),
    'success',
  )
}

console.log('sellPlanDetail.test.mjs: all passed')
