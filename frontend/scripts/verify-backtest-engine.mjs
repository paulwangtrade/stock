import assert from 'node:assert/strict'
import { runDailySignalBacktest, eastMoneyKLinesToBars } from '../src/utils/backtestEngine.js'

const bars = []
let price = 100
for (let i = 0; i < 60; i++) {
  price = price * (i % 7 === 0 ? 1.02 : 0.995)
  bars.push({
    day: `2024-${String(Math.floor(i / 28) + 1).padStart(2, '0')}-${String((i % 28) + 1).padStart(2, '0')}`,
    open: price,
    high: price * 1.01,
    low: price * 0.99,
    close: price,
  })
}

const result = runDailySignalBacktest({
  bars,
  initialCash: 100000,
  usePositionDiscipline: true,
  signalAt: (i) => {
    if (i === 10) return { action: 'buy', strength: 0.8, tag: '强' }
    if (i === 40) return { action: 'sell', sellPct: 1, tag: '止' }
    return null
  },
  marketModeAt: () => 'level4',
})

assert.equal(result.ok, true)
assert.ok(result.tradeCount >= 2)
assert.ok(result.equityCurve.length > 0)
assert.ok(Number.isFinite(result.sharpe))

const forceResult = runDailySignalBacktest({
  bars: bars.slice(0, 4),
  initialCash: 100000,
  usePositionDiscipline: true,
  maxPositionPct: 0.5,
  signalAt: (i) => (i === 0 ? { action: 'buy', strength: 1, tag: '强' } : null),
  marketModeAt: (i) => (i === 0 ? 'attack' : 'level1'),
})
assert.equal(forceResult.trades[0]?.modeKey, 'level5')
assert.ok(forceResult.trades.some((trade) => trade.tag === 'level1_force'))
assert.equal(forceResult.equityCurve.at(-1)?.shares, 0)

const mapped = eastMoneyKLinesToBars([{ Day: '2024-01-02', Open: 1, High: 2, Low: 1, Close: 1.5, Volume: 100 }])
assert.equal(mapped.length, 1)
assert.equal(mapped[0].close, 1.5)

console.log('回测引擎校验通过')
