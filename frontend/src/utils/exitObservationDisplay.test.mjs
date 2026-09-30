import assert from 'node:assert/strict'
import {
  EXIT_OBSERVE_CLASS,
  EXIT_OBSERVATION_DISCLAIMER,
  canOfferExitSellIntent,
  exitSellIntentConfirmCopy,
  readExitObservation,
} from './exitObservationDisplay.js'

const sellable = {
  source: 'paper_sim',
  availableQty: 200,
  totalQty: 200,
  canSell: true,
}

const pending = readExitObservation(null, 'pending')
assert.equal(pending.pending, true)
assert.equal(pending.sellIntentAllowed, false)
assert.equal(canOfferExitSellIntent(pending, sellable), false)

const missing = readExitObservation(null, 'ready')
assert.equal(missing.class, EXIT_OBSERVE_CLASS.INSUFFICIENT)
assert.equal(missing.label, '数据不足')
assert.equal(missing.sellIntentAllowed, false)

const unknown = readExitObservation({ class: 'SELL', label: '卖出', sellIntentAllowed: true }, 'error')
assert.equal(unknown.class, EXIT_OBSERVE_CLASS.INSUFFICIENT)
assert.equal(canOfferExitSellIntent(unknown, sellable), false)

const hold = readExitObservation(
  { class: 'HOLD_OBSERVE', label: '持有观察', reason: '暂无复评信号', sellIntentAllowed: true },
  'ready',
)
assert.equal(hold.label, '持有观察')
assert.equal(hold.sellIntentAllowed, false)
assert.equal(canOfferExitSellIntent(hold, sellable), false)

const reduce = readExitObservation(
  {
    class: 'REDUCE_OBSERVE',
    label: '减仓观察',
    reason: '浮亏进入关注区间',
    sellIntentAllowed: true,
    persistSellPlans: false,
    writesTradePlan: false,
  },
  'ready',
)
assert.equal(canOfferExitSellIntent(reduce, sellable), true)
assert.equal(canOfferExitSellIntent(reduce, { ...sellable, availableQty: 0 }), false)

const poisoned = readExitObservation(
  {
    class: 'FLATTEN_OBSERVE',
    label: '清仓观察',
    reason: '需要重新评估',
    sellIntentAllowed: true,
    writesTradePlan: true,
  },
  'ready',
)
assert.equal(poisoned.class, EXIT_OBSERVE_CLASS.FLATTEN)
assert.equal(poisoned.sellIntentAllowed, false)
assert.equal(canOfferExitSellIntent(poisoned, sellable), false)

const copy = exitSellIntentConfirmCopy(reduce)
assert.match(copy, /非交易指令/)
assert.equal(copy.split('\n')[0], EXIT_OBSERVATION_DISCLAIMER)
assert.match(copy, /不会仅因退出观察标签写入交易计划/)
assert.doesNotMatch(copy, /立即卖出/)

console.log('exitObservationDisplay.test.mjs: ok')
