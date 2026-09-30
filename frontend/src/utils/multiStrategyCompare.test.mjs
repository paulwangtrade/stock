/**
 * 多策略对照：只读求值，不产生交易建议。
 * Run: node frontend/src/utils/multiStrategyCompare.test.mjs
 */
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { register } from 'node:module'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

register(pathToFileURL(join(dirname(fileURLToPath(import.meta.url)), 'extensionlessResolveHook.mjs')).href, {
  parentURL: import.meta.url,
})

const __dir = dirname(fileURLToPath(import.meta.url))
const { buildSignalOptions } = await import(pathToFileURL(join(__dir, 'signalSettings.js')).href)
const {
  COMPARE_FOOTER_TEXT,
  UNWIRED_REASON,
  WIRED_SCAN_ENGINES,
  buildStrategyCatalog,
  defaultSelectedStrategyIds,
  evaluateObservationCompare,
  klineRowsToScanBars,
} = await import(pathToFileURL(join(__dir, 'multiStrategyCompare.js')).href)

const VERDICTS = new Set(['命中', '未命中', '数据不足', '未接入'])
const BIASES = new Set(['偏多', '偏空', '中性', '—'])

function flatBars(n, price, dayStart = 20240102) {
  const closes = []
  const opens = []
  const highs = []
  const lows = []
  const volumes = []
  const dayKeys = []
  for (let i = 0; i < n; i++) {
    closes.push(price)
    opens.push(price)
    highs.push(price)
    lows.push(price)
    volumes.push(1000)
    const day = String(dayStart + i)
    dayKeys.push(`${day.slice(0, 4)}-${day.slice(4, 6)}-${day.slice(6, 8)}`)
  }
  return { closes, opens, highs, lows, volumes, dayKeys, indexMa20ByDay: null }
}

function assertRowShape(row) {
  assert.ok(VERDICTS.has(row.verdict), row.verdict)
  assert.ok(BIASES.has(row.bias), row.bias)
  assert.equal(Object.hasOwn(row, 'action'), false)
  assert.equal(Object.hasOwn(row, 'side'), false)
  assert.equal(Object.hasOwn(row, 'order'), false)
  assert.equal(Object.hasOwn(row, 'tradePlan'), false)
  assert.equal(Object.hasOwn(row, 'signalScore'), false)
  assert.equal(Object.hasOwn(row, 'hitCount'), false)
}

assert.equal(COMPARE_FOOTER_TEXT, '仅观察对照 · 不进入模拟交易计划 · 票数不等于交易信号')

{
  const catalog = buildStrategyCatalog({ settings: null, stockStrategies: [{ id: 7, name: '我的条件', queryType: 'eastmoney_nl' }] })
  const ids = defaultSelectedStrategyIds(catalog)
  for (const engine of WIRED_SCAN_ENGINES) {
    assert.ok(ids.includes(engine.strategyId))
  }
  assert.ok(ids.includes('preset:default'))
  assert.equal(ids.includes('pack:ice_buy'), false)
  assert.equal(ids.includes('ss:7'), false)
  const pack = catalog.find((item) => item.strategyId === 'pack:ice_buy')
  assert.equal(pack.wired, false)
  assert.equal(pack.kind, 'unwired')
}

{
  const bars = flatBars(40, 10)
  const catalog = buildStrategyCatalog({ settings: null, stockStrategies: [] })
  const engines = catalog.filter((item) =>
    ['scan_reduce_v1', 'scan_reversal_v1', 'pack:ice_buy', 'preset:default'].includes(item.strategyId),
  )
  const rows = evaluateObservationCompare({
    bars,
    engines,
    activeSignalOptions: buildSignalOptions(null),
  })
  assert.equal(Array.isArray(rows), true)
  assert.equal(rows.recommendation, undefined)
  for (const row of rows) assertRowShape(row)
  const byId = Object.fromEntries(rows.map((row) => [row.strategyId, row]))
  assert.equal(byId.scan_reduce_v1.verdict, '未命中')
  assert.equal(byId.scan_reduce_v1.bias, '—')
  assert.equal(byId.scan_reversal_v1.verdict, '数据不足')
  assert.equal(byId['pack:ice_buy'].verdict, '未接入')
  assert.equal(byId['pack:ice_buy'].reason, UNWIRED_REASON)
  assert.equal(byId['pack:ice_buy'].bias, '—')
  assert.equal(byId['preset:default'].verdict, '未命中')
  assert.equal(byId['preset:default'].bias, '—')
}

{
  const bars = flatBars(30, 10)
  bars.closes[28] = 9
  bars.closes[29] = 9
  bars.opens[28] = 9
  bars.opens[29] = 9
  bars.highs[28] = 9.2
  bars.highs[29] = 9.2
  bars.lows[28] = 8.8
  bars.lows[29] = 8.8
  const engines = WIRED_SCAN_ENGINES.filter((item) =>
    item.strategyId === 'scan_reduce_v1' || item.strategyId === 'scan_ice_v1',
  )
  const rows = evaluateObservationCompare({
    bars,
    engines,
    activeSignalOptions: { ...buildSignalOptions(null), reduceEarlyEnabled: false },
  })
  const byId = Object.fromEntries(rows.map((row) => [row.strategyId, row]))
  assert.equal(byId.scan_reduce_v1.verdict, '命中')
  assert.equal(byId.scan_reduce_v1.bias, '偏空')
  assert.match(byId.scan_reduce_v1.reason, /破 MA20/)
  assert.equal(byId.scan_reduce_v1.dataDay, bars.dayKeys[29])
  assert.equal(byId.scan_ice_v1.verdict, '命中')
  assert.equal(byId.scan_ice_v1.bias, '中性')
  for (const row of rows) assertRowShape(row)
}

{
  const rows = evaluateObservationCompare({
    bars: null,
    engines: [WIRED_SCAN_ENGINES[0], { strategyId: 'pack:ice_buy', name: '冰点超跌·出坑买点', kind: 'unwired', wired: false }],
    activeSignalOptions: {},
  })
  assert.equal(rows[0].verdict, '数据不足')
  assert.equal(rows[1].verdict, '未接入')
}

{
  const bars = klineRowsToScanBars([
    { Day: '2024-01-03', Open: 11, High: 12, Low: 10, Close: 11, Volume: 5 },
    { day: '2024-01-02', open: 10, high: 10, low: 9, close: 10, volume: 4 },
  ])
  assert.deepEqual(bars.closes, [10, 11])
  assert.equal(bars.dayKeys[0], '2024-01-02')
}

{
  const vue = readFileSync(join(__dir, '../components/MultiStrategyComparePanel.vue'), 'utf8')
  const shell = readFileSync(join(__dir, '../components/researchIndex.vue'), 'utf8')
  assert.ok(vue.includes('多策略对照'))
  assert.ok(vue.includes('开始对照'))
  assert.ok(vue.includes('COMPARE_FOOTER_TEXT'))
  assert.equal(vue.includes('会诊'), false)
  assert.equal(vue.includes('TradePlan'), false)
  assert.equal(vue.includes('RunStockStrategy'), false)
  assert.ok(shell.includes('name="多策略对照"'))
  assert.equal(shell.includes('会诊'), false)
}

console.log('multiStrategyCompare.test.mjs ok')
