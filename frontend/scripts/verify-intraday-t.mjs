import assert from 'node:assert/strict'
import {
  calculateRSI,
  calculateVWAP,
  evaluateIntradayTSignal,
  isContinuousAuction,
  isIntradayTWindow,
  roundToBoardLot,
} from '../src/utils/intradayTSignals.js'

const vwap = calculateVWAP([
  { high: 10, low: 10, close: 10, volume: 100, amount: 1000 },
  { high: 12, low: 12, close: 12, volume: 300, amount: 3600 },
])
assert.equal(vwap, 11.5)
assert.equal(calculateVWAP([
  { high: 10, low: 10, close: 10, volume: 10, amount: 10_000 },
  { high: 12, low: 12, close: 12, volume: 30, amount: 36_000 },
]), 11.5)

const rsi = calculateRSI([
  44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.1, 45.42,
  45.84, 46.08, 45.89, 46.03, 45.61, 46.28, 46.28,
])
assert.ok(Math.abs(rsi - 70.46) < 0.1)

assert.equal(roundToBoardLot(99), 0)
assert.equal(roundToBoardLot(250), 200)
assert.equal(roundToBoardLot(1050 * 0.25), 200)
assert.equal(isContinuousAuction('2026-07-14 09:45'), true)
assert.equal(isContinuousAuction('2026-07-14 12:00'), false)
assert.equal(isIntradayTWindow('2026-07-14 12:00'), false)
assert.equal(isIntradayTWindow('2026-07-14 14:20'), true)

function makeBars(prices, volumeAt = () => 1000) {
  return prices.map((price, index) => ({
    day: `2026-07-14 ${String(10 + Math.floor(index / 12)).padStart(2, '0')}:${String((index % 12) * 5).padStart(2, '0')}`,
    open: price - 0.02,
    high: price + 0.03,
    low: price - 0.03,
    close: price,
    volume: volumeAt(index),
    amount: price * volumeAt(index),
  }))
}

const rising = makeBars(
  Array.from({ length: 20 }, (_, index) => 10 + index * 0.04),
  (index) => (index === 19 ? 1500 : 1000),
)
const sellSignal = evaluateIntradayTSignal({
  rows: rising,
  holdingVolume: 1050,
  costPrice: 9.5,
  state: { rounds: 0, soldQty: 0, boughtBackQty: 0 },
  now: '2026-07-14 11:30',
})
assert.equal(sellSignal.action, 'T出')
assert.equal(sellSignal.suggestedVolume, 200)
assert.ok(sellSignal.confidence >= 70)
assert.ok(sellSignal.reasons.length > 0)
assert.ok(sellSignal.invalidation)
assert.equal(sellSignal.requiresSellableConfirmation, true)
assert.match(sellSignal.reasons.join(''), /人工确认可卖/)

const falling = makeBars(Array.from({ length: 20 }, (_, index) => 11 - index * 0.05))
const buySignal = evaluateIntradayTSignal({
  rows: falling,
  holdingVolume: 1050,
  state: { rounds: 1, soldQty: 200, boughtBackQty: 0 },
  now: '2026-07-14 11:30',
})
assert.equal(buySignal.action, 'T入')
assert.equal(buySignal.suggestedVolume, 200)

const blocked = evaluateIntradayTSignal({
  rows: rising,
  holdingVolume: 1050,
  state: { rounds: 2, soldQty: 400, boughtBackQty: 400 },
  now: '2026-07-14 11:30',
})
assert.equal(blocked.action, '观察')
assert.match(blocked.reasons.join(''), /两轮/)

console.log('分钟做T引擎校验通过')
