/**
 * Unit tests: T-sell TradePlan UI flow (Phase14-A-R1-C).
 * Run: node frontend/src/viewmodels/tradePlan/tSellFlow.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'tSellFlow.js')).href
const {
  resolveIsTSellFlow,
  resolveTradePlanSourceLabel,
  resolveTSellGuidancePhase,
  resolveTSellStickyCtaKind,
  tSellStickyCtaLabel,
  tSellExecuteDisabledReason,
  canExecuteTSell,
} = await import(modUrl)

{
  assert.equal(resolveIsTSellFlow({ routeFlow: 'sell', sourceSession: '' }), true)
  assert.equal(resolveIsTSellFlow({ routeFlow: '', sourceSession: 't_sell' }), true)
  assert.equal(resolveIsTSellFlow({ routeFlow: '', sourceSession: 'exit_review' }), true)
  assert.equal(resolveIsTSellFlow({ routeFlow: 'buy', sourceSession: 'after_close' }), false)
}

{
  assert.equal(resolveTradePlanSourceLabel('t_sell'), '人工卖出')
  assert.equal(resolveTradePlanSourceLabel('exit_review'), '退出复评')
  assert.equal(resolveTradePlanSourceLabel('after_close'), '盘后计划')
}

{
  assert.equal(
    resolveTSellGuidancePhase({ hasPlan: true, isApproved: false, isFrozen: false, isReady: false }),
    2,
  )
  assert.equal(
    resolveTSellGuidancePhase({ hasPlan: true, isApproved: true, isFrozen: false, isReady: false }),
    3,
  )
  assert.equal(
    resolveTSellGuidancePhase({ hasPlan: true, isApproved: true, isFrozen: true, isReady: false }),
    4,
  )
}

{
  assert.equal(
    resolveTSellStickyCtaKind({ hasPlan: true, isApproved: false, isFrozen: false, isReady: false }),
    'approve',
  )
  assert.equal(
    resolveTSellStickyCtaKind({ hasPlan: true, isApproved: true, isFrozen: false, isReady: false }),
    'freeze',
  )
  assert.equal(
    resolveTSellStickyCtaKind({ hasPlan: true, isApproved: true, isFrozen: true, isReady: false }),
    'execute_sell',
  )
}

{
  assert.equal(tSellStickyCtaLabel('approve'), '批准卖出')
  assert.equal(tSellStickyCtaLabel('execute_sell'), '执行卖出')
}

{
  const blocked = tSellExecuteDisabledReason({
    hasPlan: true,
    isFrozen: false,
    isReady: false,
    executing: false,
    planId: 1,
    hasFill: false,
  })
  assert.match(blocked, /冻结/)
  assert.equal(
    canExecuteTSell({
      hasPlan: true,
      isFrozen: true,
      isReady: false,
      executing: false,
      planId: 9,
      hasFill: false,
    }),
    true,
  )
}

console.log('tSellFlow.test.mjs: all passed')
