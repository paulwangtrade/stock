/**
 * Unit tests: intent materialize preflight (Phase14-A-R0-C).
 * Run: node frontend/src/viewmodels/tradePlan/intentPreflight.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'intentPreflight.js')).href
const {
  PREFLIGHT_BLOCK_NO_SELECTED,
  analyzeIntentItem,
  buildIntentMaterializePreflight,
  evaluateMaterializeOutcome,
  formatMaterializeOutcomeDisplay,
  hasPriceAnchor,
  hasTargetVolume,
} = await import(modUrl)

function item(partial) {
  return {
    stock_code: 'sz000001',
    stock_name: '测试',
    side: 'buy',
    status: 'pending',
    intent_status: '',
    ref_price: 0,
    target_volume: 0,
    ...partial,
  }
}

function plan(items, extra = {}) {
  return {
    status: 'draft',
    risk: { passed: true },
    freeze: { is_frozen: false },
    items,
    ...extra,
  }
}

{
  assert.equal(hasPriceAnchor({ ref_price: 10.5 }), true)
  assert.equal(hasPriceAnchor({ ref_price: 0 }), false)
  assert.equal(hasTargetVolume({ target_volume: 100 }), true)
  assert.equal(hasTargetVolume({ target_volume: 0 }), false)
}

{
  const row = analyzeIntentItem(item({ intent_status: 'selected', ref_price: 9.8 }))
  assert.equal(row.isSelected, true)
  assert.equal(row.hasPriceAnchor, true)
  assert.equal(row.hasTargetVolume, false)
}

{
  const pf = buildIntentMaterializePreflight(plan([]))
  assert.equal(pf.selectedCount, 0)
  assert.equal(pf.canMaterialize, false)
  assert.equal(pf.blockReason, PREFLIGHT_BLOCK_NO_SELECTED)
}

{
  const pf = buildIntentMaterializePreflight(
    plan([
      item({ intent_status: 'selected', ref_price: 10 }),
      item({ stock_code: 'sz000002', intent_status: 'selected', ref_price: 0 }),
      item({ stock_code: 'sh600000', side: 'sell', intent_status: 'selected', ref_price: 10 }),
    ]),
  )
  assert.equal(pf.selectedCount, 2, 'sell row excluded')
  assert.equal(pf.canMaterialize, true)
  assert.equal(pf.blockReason, '')
  assert.equal(pf.selectedMissingAnchorCount, 1)
  assert.match(pf.warnReason, /缺少参考价/)
}

{
  const pf = buildIntentMaterializePreflight(
    plan([item({ intent_status: 'selected', ref_price: 10 })]),
    { isFrozen: true },
  )
  assert.equal(pf.canMaterialize, false)
}

{
  const pf = buildIntentMaterializePreflight(
    plan([item({ intent_status: 'priced', ref_price: 10, target_volume: 200 })]),
  )
  assert.equal(pf.selectedCount, 0)
  assert.equal(pf.pricedCount, 1)
  assert.equal(pf.canMaterialize, false)
  assert.equal(pf.blockReason, PREFLIGHT_BLOCK_NO_SELECTED)
}

{
  const outcome = evaluateMaterializeOutcome({
    success: true,
    materialized_items: 0,
    readiness_ready: false,
    blockers: [{ code: 'NO_TRADEABLE_ITEMS', message: 'none' }],
  })
  assert.equal(outcome.level, 'warning')
  assert.equal(outcome.ok, false)
  assert.match(outcome.title, /未产生可执行项/)
}

{
  const outcome = evaluateMaterializeOutcome({
    success: true,
    materialized_items: 2,
    readiness_ready: true,
    blockers: [],
  })
  assert.equal(outcome.level, 'success')
  assert.equal(outcome.ok, true)
  assert.match(outcome.toastMessage, /物化 2 项/)
}

{
  const outcome = evaluateMaterializeOutcome({
    success: false,
    materialized_items: 0,
    message: 'plan is frozen',
    failed_step: 'precheck',
  })
  assert.equal(outcome.level, 'error')
  assert.match(outcome.toastMessage, /plan is frozen/)
}

{
  const disp = formatMaterializeOutcomeDisplay({
    success: true,
    materialized_items: 0,
    readiness_ready: false,
    blockers: [{ code: 'QG-E1', message: 'quality gate' }],
  })
  assert.equal(disp.level, 'warning')
  assert.equal(disp.ok, false)
  assert.match(disp.title, /未产生可执行项/)
  assert.ok(disp.detailLines.some((l) => l.includes('物化条目：0')))
}

{
  const disp = formatMaterializeOutcomeDisplay({
    success: true,
    materialized_items: 3,
    readiness_ready: true,
    blockers: [],
  })
  assert.equal(disp.level, 'success')
  assert.equal(disp.ok, true)
  assert.match(disp.title, /成功/)
}

console.log('intentPreflight.test: OK')
