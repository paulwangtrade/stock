/**
 * Selftest: morning materialize UI helpers.
 * Run: node --import ./frontend/scripts/loaders/register-js-ext.mjs frontend/src/api/tradePlans.materializeMorning.selftest.mjs
 */
import assert from 'node:assert/strict'
import {
  PRICING_STAGE_AFTER_CLOSE_INTENT,
  PRICING_STAGE_MORNING_MATERIALIZED,
  buildMaterializeMorningRequestBody,
  canShowMorningMaterializeButton,
  formatMaterializeMorningDisplay,
  inferPricingStageForMaterializeUI,
  normalizeMaterializeMorningResponse,
} from './tradePlansMaterializeMorning.js'

{
  assert.equal(
    canShowMorningMaterializeButton({
      status: 'draft',
      isFrozen: false,
      pricingStage: PRICING_STAGE_AFTER_CLOSE_INTENT,
    }),
    true,
    'draft + after_close + not frozen → show',
  )
  assert.equal(
    canShowMorningMaterializeButton({
      status: 'draft',
      isFrozen: true,
      pricingStage: PRICING_STAGE_AFTER_CLOSE_INTENT,
    }),
    false,
    'frozen → hide',
  )
  assert.equal(
    canShowMorningMaterializeButton({
      status: 'ready',
      isFrozen: false,
      pricingStage: PRICING_STAGE_AFTER_CLOSE_INTENT,
    }),
    false,
    'non-draft → hide',
  )
  assert.equal(
    canShowMorningMaterializeButton({
      status: 'draft',
      isFrozen: false,
      pricingStage: PRICING_STAGE_MORNING_MATERIALIZED,
    }),
    false,
    'already materialized → hide',
  )
  assert.equal(
    canShowMorningMaterializeButton({
      status: 'Draft',
      isFrozen: false,
      pricingStage: PRICING_STAGE_AFTER_CLOSE_INTENT,
    }),
    true,
    'status case-insensitive',
  )
}

{
  assert.equal(
    inferPricingStageForMaterializeUI({ lifecycleStage: 'S1_INTENT_DRAFT' }),
    PRICING_STAGE_AFTER_CLOSE_INTENT,
  )
  assert.equal(
    inferPricingStageForMaterializeUI({ lifecycleStage: 'S2_INTENT_MATERIALIZED' }),
    PRICING_STAGE_MORNING_MATERIALIZED,
  )
  assert.equal(
    inferPricingStageForMaterializeUI({
      blockers: [{ code: 'INTENT_NOT_MATERIALIZED' }],
    }),
    PRICING_STAGE_AFTER_CLOSE_INTENT,
  )
  assert.equal(
    inferPricingStageForMaterializeUI({
      explicitPricingStage: PRICING_STAGE_MORNING_MATERIALIZED,
      lifecycleStage: 'S1_INTENT_DRAFT',
    }),
    PRICING_STAGE_MORNING_MATERIALIZED,
    'explicit wins',
  )
}

{
  assert.deepEqual(buildMaterializeMorningRequestBody(17), { plan_id: 17 })
  assert.deepEqual(buildMaterializeMorningRequestBody('42.9'), { plan_id: 42 })
  assert.deepEqual(buildMaterializeMorningRequestBody(0), { plan_id: 0 })
}

{
  const ok = normalizeMaterializeMorningResponse({
    success: true,
    plan_id: 17,
    materialized_items: 1,
    readiness_ready: false,
    blockers: [{ code: 'QG-E1', rule_code: 'E1', severity: 'block', message: 'quality' }],
    code: 0,
    pricing_stage: 'morning_materialized',
  })
  assert.equal(ok.success, true)
  assert.equal(ok.materialized_items, 1)
  assert.equal(ok.readiness_ready, false)
  assert.equal(ok.blockers.length, 1)
  const disp = formatMaterializeMorningDisplay(ok)
  assert.equal(disp.ok, true)
  assert.equal(disp.level, 'success')
  assert.match(disp.title, /成功/)
  assert.ok(disp.detailLines.some((l) => l.includes('物化条目：1')))
  assert.ok(disp.detailLines.some((l) => l.includes('Blocked')))
  assert.ok(disp.detailLines.some((l) => l.includes('QG-E1')))
}

{
  const zero = normalizeMaterializeMorningResponse({
    success: true,
    plan_id: 17,
    materialized_items: 0,
    readiness_ready: false,
    blockers: [{ code: 'NO_TRADEABLE_ITEMS', message: 'none' }],
    code: 0,
  })
  const zeroDisp = formatMaterializeMorningDisplay(zero)
  assert.equal(zeroDisp.level, 'warning')
  assert.equal(zeroDisp.ok, false)
  assert.match(zeroDisp.title, /未产生可执行项/)
}

{
  const fail = normalizeMaterializeMorningResponse({
    success: false,
    plan_id: 17,
    materialized_items: 0,
    readiness_ready: false,
    blockers: [],
    code: 40910,
    message: 'plan is frozen',
    failed_step: 'precheck',
  })
  const disp = formatMaterializeMorningDisplay(fail)
  assert.equal(disp.ok, false)
  assert.equal(disp.level, 'error')
  assert.match(disp.title, /失败/)
  assert.ok(disp.detailLines.some((l) => l.includes('plan is frozen')))
  assert.ok(disp.detailLines.some((l) => l.includes('precheck')))
}

console.log('tradePlans.materializeMorning.selftest: OK')
