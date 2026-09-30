/**
 * Trade-universe gate: cron enable must not imply feedsTradePlan.
 *   node scripts/verify-trade-universe-gate.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const gateUrl = pathToFileURL(join(__dir, '../src/utils/tradeUniverseGate.js')).href
const presetUrl = pathToFileURL(join(__dir, '../src/utils/technicalIndicators.js')).href

const {
  FEEDS_TRADE_PLAN_HINT,
  FEEDS_TRADE_PLAN_CONFIRM,
  UNHOOKED_TRADE_UNIVERSE_MESSAGE,
  defaultStrategyGate,
  buildStrategyGatePayload,
  needsFeedsTradePlanConfirm,
  tradeUniverseBadge,
  presetAdmissionLabel,
  presetFeedsTradePlanDefault,
} = await import(gateUrl)
const { STRATEGY_PRESETS } = await import(presetUrl)

const fresh = defaultStrategyGate()
assert.equal(fresh.enable, false)
assert.equal(fresh.feedsTradePlan, false)

const cronOnly = buildStrategyGatePayload({
  enable: true,
  cronExpr: '0 35 9 * * 1-5',
  feedsTradePlan: false,
})
assert.equal(cronOnly.enable, true)
assert.equal(cronOnly.feedsTradePlan, false, 'enable cron alone must not feed TradePlan')

const cronWithoutExpr = buildStrategyGatePayload({
  enable: true,
  cronExpr: '',
  feedsTradePlan: true,
})
assert.equal(cronWithoutExpr.enable, false, 'cron stays off without an expression')
assert.equal(cronWithoutExpr.feedsTradePlan, true, 'feedsTradePlan is independent of cron')

assert.equal(needsFeedsTradePlanConfirm({ isNew: true, previousFeedsTradePlan: false, nextFeedsTradePlan: true }), true)
assert.equal(needsFeedsTradePlanConfirm({ isNew: false, previousFeedsTradePlan: false, nextFeedsTradePlan: true }), true)
assert.equal(needsFeedsTradePlanConfirm({ isNew: false, previousFeedsTradePlan: true, nextFeedsTradePlan: true }), false)
assert.equal(needsFeedsTradePlanConfirm({ isNew: true, previousFeedsTradePlan: false, nextFeedsTradePlan: false }), false)

assert.equal(tradeUniverseBadge(false), '观察')
assert.equal(tradeUniverseBadge(true), '可进模拟计划')
assert.match(FEEDS_TRADE_PLAN_HINT, /仅模拟盘/)
assert.match(FEEDS_TRADE_PLAN_HINT, /非实盘/)
assert.match(FEEDS_TRADE_PLAN_HINT, /非自动下单/)
assert.match(FEEDS_TRADE_PLAN_CONFIRM, /模拟/)
assert.match(FEEDS_TRADE_PLAN_CONFIRM, /不会自动下单/)
assert.match(UNHOOKED_TRADE_UNIVERSE_MESSAGE, /交易宇宙未挂接任何策略/)

assert.ok(STRATEGY_PRESETS.length >= 1)
for (const preset of STRATEGY_PRESETS) {
  assert.equal(presetFeedsTradePlanDefault(preset), false, `${preset.id} must default feedsTradePlan false`)
  const payload = buildStrategyGatePayload({
    enable: preset.enable,
    cronExpr: preset.cronExpr,
    feedsTradePlan: preset.feedsTradePlan,
  })
  assert.equal(payload.feedsTradePlan, false, `${preset.id} create payload must not feed TradePlan`)
}
const ice = STRATEGY_PRESETS.find((p) => p.id === 'ice_buy')
assert.ok(ice)
assert.equal(ice.enable, true)
assert.equal(ice.feedsTradePlan, false)
assert.match(presetAdmissionLabel(ice), /新建默认观察/)
assert.match(presetAdmissionLabel(STRATEGY_PRESETS.find((p) => p.id === 'ice_watch')), /默认观察/)

console.log('verify-trade-universe-gate: ok')
