/**
 * node --experimental-loader ./frontend/scripts/esm-js-ext-loader.mjs --test frontend/src/utils/nextDaySetupWatch.test.mjs
 */
import assert from 'node:assert/strict'
import {
  NEXT_DAY_SETUP_DISCLAIMER,
  NEXT_DAY_SETUP_TAG_SUPPORT,
  NEXT_DAY_SETUP_UNAVAILABLE,
  evaluateBreakoutHighSetup,
  evaluateMa20ReclaimSetup,
  evaluateNextDaySetups,
  formatSetupDistance,
  formatSetupTrigger,
  unavailableSetup,
} from './nextDaySetupWatch.js'

function flatPlatform({ n = 25, lastClose = 10.15, high = 10.2, low = 9.9 } = {}) {
  return {
    closes: Array.from({ length: n }, (_, i) => (i === n - 1 ? lastClose : 10)),
    opens: Array(n).fill(9.95),
    highs: Array(n).fill(high),
    lows: Array(n).fill(low),
    volumes: Array(n).fill(1000),
  }
}

const breakoutOpts = {
  boxPeriod: 20,
  maxRangePct: 0.2,
  breakBuffer: 0.005,
  confirmDays: 2,
  volMult: 1.25,
  volPeriod: 5,
  minGap: 15,
  setupMaxDistancePct: 0.05,
}

{
  const row = evaluateBreakoutHighSetup(flatPlatform(), breakoutOpts)
  assert.ok(row, 'close just under box high should be a breakout setup')
  assert.equal(row.tag, '突')
  assert.equal(row.engine, 'breakout_high')
  assert.equal(row.priceMode, 'price')
  assert.equal(row.confirmed, false)
  assert.equal(row.orderIntent, false)
  assert.equal(row.observationOnly, true)
  assert.equal(row.disclaimer, NEXT_DAY_SETUP_DISCLAIMER)
  assert.ok(row.triggerPrice > 10.15)
  assert.ok(Math.abs(row.triggerPrice - 10.2 * 1.005) < 1e-6)
  assert.equal(formatSetupTrigger(row), row.triggerPrice.toFixed(2))
  assert.match(formatSetupDistance(row), /^还差 /)
  assert.match(row.gapText, /不是已确认信号/)
  assert.doesNotMatch(row.summary, /买入|委托|下单/)
}

{
  assert.equal(evaluateBreakoutHighSetup(flatPlatform({ lastClose: 9.5 }), breakoutOpts), null)
  assert.equal(evaluateBreakoutHighSetup(flatPlatform({ lastClose: 10.3 }), breakoutOpts), null)
}

{
  const bars = flatPlatform()
  bars.closes = bars.closes.map((c, i, arr) => (i === arr.length - 1 ? c : 12 - i * 0.08))
  assert.equal(evaluateBreakoutHighSetup(bars, breakoutOpts), null)
}

{
  const n = 22
  const closes = Array(n).fill(10)
  closes[20] = 9.5
  closes[21] = 10.05
  const bars = {
    closes,
    opens: closes.map((c) => c - 0.05),
    highs: closes.map((c) => c + 0.1),
    lows: closes.map((c) => c - 0.1),
    volumes: Array(n).fill(1000),
  }
  const opts = {
    maPeriod: 20,
    reboundConfirmDays: 2,
    reboundSellLookback: 6,
    reboundMinDaysAfterSell: 1,
    reduceConfirmDays: 1,
    setupMaxDistancePct: 0.05,
  }
  const row = evaluateMa20ReclaimSetup(bars, opts)
  assert.ok(row, 'one day short of MA20 stand after a break should be a reclaim setup')
  assert.equal(row.tag, '弹')
  assert.equal(row.engine, 'ma20_reclaim')
  assert.equal(row.priceMode, 'price')
  assert.equal(row.confirmed, false)
  assert.equal(row.orderIntent, false)
  assert.ok(row.triggerPrice > 0)
  assert.match(row.gapText, /无法/)
  assert.equal(row.disclaimer, NEXT_DAY_SETUP_DISCLAIMER)
  const both = evaluateNextDaySetups(bars, { ...opts, ...breakoutOpts })
  assert.ok(both.some((item) => item.engine === 'ma20_reclaim'))
  assert.ok(both.every((item) => item.orderIntent === false && item.confirmed === false))
}

{
  const trend = NEXT_DAY_SETUP_TAG_SUPPORT.find((item) => item.tag === '趋')
  const strong = NEXT_DAY_SETUP_TAG_SUPPORT.find((item) => item.tag === '强')
  const buy = NEXT_DAY_SETUP_TAG_SUPPORT.find((item) => item.tag === '买')
  const rev = NEXT_DAY_SETUP_TAG_SUPPORT.find((item) => item.tag === '转')
  assert.equal(trend.priceMode, 'gap')
  assert.equal(trend.active, false)
  assert.match(trend.note, /无法给出唯一价/)
  for (const row of [strong, buy, rev]) {
    assert.equal(row.priceMode, 'unavailable')
    assert.equal(row.active, false)
    assert.match(row.note, /无法给出唯一价/)
  }
  const active = NEXT_DAY_SETUP_TAG_SUPPORT.filter((item) => item.active)
  assert.deepEqual(active.map((item) => item.tag), ['突', '弹'])
  assert.ok(active.every((item) => item.priceMode === 'price'))

  const closed = unavailableSetup('趋')
  closed.triggerPrice = 12.3
  assert.equal(formatSetupTrigger(closed), NEXT_DAY_SETUP_UNAVAILABLE)
  assert.equal(formatSetupTrigger({ priceMode: 'gap', triggerPrice: 9 }), NEXT_DAY_SETUP_UNAVAILABLE)
  assert.equal(formatSetupTrigger({ priceMode: 'price', triggerPrice: 0 }), NEXT_DAY_SETUP_UNAVAILABLE)
  assert.equal(closed.orderIntent, false)
  assert.equal(closed.confirmed, false)
}
