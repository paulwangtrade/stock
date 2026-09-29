import assert from 'node:assert/strict'
import { runDailySignalBacktest } from '../src/utils/backtestEngine.js'
import {
  identityAllowsRun,
  resolveStockIdentity,
  toStockSuggestions,
} from '../src/utils/signalBacktestIdentity.js'
import {
  buildEquityChartOption,
  buildPriceTradeChartOption,
} from '../src/utils/signalBacktestCharts.js'

const basics = [
  { name: '晶合集成', ts_code: '688137.SH', symbol: '688137' },
  { name: '贵州茅台', ts_code: '600519.SH', symbol: '600519' },
  { name: '平安银行', ts_code: '000001.SZ', symbol: '000001' },
  { name: '招商银行', ts_code: '600036.SH', symbol: '600036' },
]

{
  const hit = resolveStockIdentity('688137', basics)
  assert.equal(hit.status, 'unique')
  assert.equal(hit.name, '晶合集成')
  assert.equal(hit.klineCode, '688137')
  assert.equal(identityAllowsRun(hit), true)
}

{
  const hit = resolveStockIdentity('sh688137', basics)
  assert.equal(hit.status, 'unique')
  assert.equal(hit.name, '晶合集成')
  assert.equal(hit.klineCode, '688137')
}

{
  const hit = resolveStockIdentity('688137.SH', basics)
  assert.equal(hit.status, 'unique')
  assert.equal(hit.symbol, '688137')
}

{
  const hit = resolveStockIdentity('晶合集成', basics)
  assert.equal(hit.status, 'unique')
  assert.equal(hit.klineCode, '688137')
  assert.equal(toStockSuggestions(hit)[0].value, '688137')
}

{
  const hit = resolveStockIdentity('  贵州茅台  ', basics)
  assert.equal(hit.status, 'unique')
  assert.equal(hit.symbol, '600519')
}

{
  const hit = resolveStockIdentity('银行', basics)
  assert.equal(hit.status, 'ambiguous')
  assert.equal(identityAllowsRun(hit), false)
  assert.ok(hit.matches.length >= 2)
  assert.match(hit.message, /点选/)
}

{
  const hit = resolveStockIdentity('平安银行', basics)
  assert.equal(hit.status, 'unique')
  assert.equal(hit.symbol, '000001')
}

{
  const hit = resolveStockIdentity('不存在的公司xyz', basics)
  assert.equal(hit.status, 'unknown')
  assert.equal(hit.klineCode, '')
  assert.match(hit.message, /没有找到/)
  assert.equal(identityAllowsRun(hit), false)
}

{
  const hit = resolveStockIdentity('601012', [])
  assert.equal(hit.status, 'code_only')
  assert.equal(hit.klineCode, '601012')
  assert.equal(identityAllowsRun(hit), true)
  assert.match(hit.message, /仍可按代码回测/)
}

{
  const hit = resolveStockIdentity('', basics)
  assert.equal(hit.status, 'empty')
  assert.equal(identityAllowsRun(hit), false)
}

{
  const dup = resolveStockIdentity('688137', [
    { Name: '晶合集成', TsCode: '688137.SH', Symbol: '688137' },
    { name: '晶合集成', ts_code: '688137.SH', symbol: '688137' },
  ])
  assert.equal(dup.status, 'unique')
  assert.equal(dup.matches.length, 1)
}

const bars = []
let price = 20
for (let i = 0; i < 80; i++) {
  price = price * (i % 9 === 0 ? 1.03 : 0.997)
  bars.push({
    day: `2024-03-${String((i % 28) + 1).padStart(2, '0')}`,
    open: price,
    high: price * 1.01,
    low: price * 0.99,
    close: price,
    volume: 1000 + i,
  })
}

const primary = runDailySignalBacktest({
  bars,
  initialCash: 1000000,
  usePositionDiscipline: true,
  signalAt: (i) => {
    if (i === 10) return { action: 'buy', strength: 0.8, tag: '趋' }
    if (i === 40) return { action: 'sell', sellPct: 1, tag: '止' }
    return null
  },
  marketModeAt: () => 'level4',
})
const compare = runDailySignalBacktest({
  bars,
  initialCash: 1000000,
  usePositionDiscipline: false,
  signalAt: (i) => (i === 12 ? { action: 'buy', strength: 1, tag: '趋' } : null),
  marketModeAt: () => 'level4',
})

const equity = buildEquityChartOption(primary, compare, { showCompare: true, dark: true })
assert.equal(equity.series[0].name, '本次回测')
assert.equal(equity.series[0].data.length, primary.equityCurve.length)
assert.equal(equity.series[0].data[0], Number(primary.equityCurve[0].equity))
assert.equal(equity.series[1].name, '无纪律对照')
assert.equal(equity.series[1].data.length, primary.equityCurve.length)
assert.equal(equity.xAxis.data[3], primary.equityCurve[3].day)
assert.equal(equity.series[0].markLine.data[0].yAxis, primary.initialCash)

const solo = buildEquityChartOption(primary, compare, { showCompare: false })
assert.equal(solo.series.length, 1)

const priceChart = buildPriceTradeChartOption(primary, { dark: false })
const closeSeries = priceChart.series.find((item) => item.name === '收盘价')
const buySeries = priceChart.series.find((item) => item.name === '买入')
const sellSeries = priceChart.series.find((item) => item.name === '卖出')
assert.deepEqual(closeSeries.data, primary.equityCurve.map((point) => Number(point.close)))
assert.equal(buySeries.data.length, primary.trades.filter((trade) => trade.side === 'buy').length)
assert.equal(sellSeries.data.length, primary.trades.filter((trade) => trade.side === 'sell').length)
assert.ok(buySeries.data.length >= 1)
assert.equal(buySeries.data[0].value[0], primary.trades.find((trade) => trade.side === 'buy').day)
assert.equal(typeof priceChart.tooltip.formatter, 'function')

const empty = buildEquityChartOption(null, null)
assert.equal(empty.series[0].data.length, 0)
const emptyPrice = buildPriceTradeChartOption({ equityCurve: [], trades: [] })
assert.equal(emptyPrice.series[0].data.length, 0)

console.log('信号回测图表与股票识别校验通过')
