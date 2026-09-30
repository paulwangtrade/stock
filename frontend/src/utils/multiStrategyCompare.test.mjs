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
  PRESET_UNIMPLEMENTED_REASON,
  UNWIRED_REASON,
  WIRED_SCAN_ENGINES,
  buildStrategyCatalog,
  concludePresetObservation,
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
  assert.ok(vue.includes('resolveStrategyRole'))
  assert.ok(vue.includes("title: '角色'"))
  assert.ok(vue.includes('未实现独立求值'))
  assert.equal(vue.includes('今日转势买点'), false)
}

const STOLEN_REVERSAL = {
  tag: '转',
  statusText: '今日转势买点 · RSI 67.5',
  latestStatus: { text: '今日转势买点 · RSI 67.5', rsi: 67.5 },
}

function presetEngine(id, name, extra = {}) {
  return {
    strategyId: `preset:${id}`,
    name,
    kind: 'scan_preset',
    wired: true,
    ...extra,
  }
}

{
  const reversalReason = '转势买点 · RSI 67.5'
  const xsmom = concludePresetObservation(presetEngine('ext_xsmom_v1', '截面动量V1', { scanKind: 'xsmom' }), {
    summary: STOLEN_REVERSAL,
  })
  const maTrend = concludePresetObservation(presetEngine('ext_ma_trend_v1', '均线趋势V1'), {
    summary: STOLEN_REVERSAL,
  })
  const volMom = concludePresetObservation(
    presetEngine('ext_vol_mom_v1', '量价动量V1', { scanKind: 'vol_mom', engineStatus: 'planned' }),
    { summary: STOLEN_REVERSAL },
  )
  const extBreakout = concludePresetObservation(
    presetEngine('ext_breakout_v1', '突破观察V1', { scanKind: 'breakout' }),
    { summary: STOLEN_REVERSAL },
  )
  for (const row of [xsmom, maTrend, volMom, extBreakout]) {
    assert.equal(row.verdict, '数据不足')
    assert.equal(row.reason, PRESET_UNIMPLEMENTED_REASON)
    assert.equal(row.reason.includes('转势买点'), false)
    assert.equal(row.reason.includes('67.5'), false)
    assert.notEqual(row.reason, STOLEN_REVERSAL.statusText)
    assert.notEqual(row.reason, reversalReason)
  }

  const ice = concludePresetObservation(
    presetEngine('default', '默认参数预设', { scanKind: 'ice', templateId: 'ice_point' }),
    { summary: STOLEN_REVERSAL },
  )
  const aggressive = concludePresetObservation(presetEngine('user-fast', '激进参数', { scanKind: 'ice' }), {
    summary: STOLEN_REVERSAL,
  })
  assert.equal(ice.verdict, '命中')
  assert.equal(ice.reason.includes('今日转势买点'), false)
  assert.equal(ice.reason.includes('转势买点'), false)
  assert.ok(ice.reason.includes('默认参数预设'))
  assert.ok(ice.reason.includes('本预设规则触发（转）'))
  assert.ok(ice.reason.includes('RSI 67.5'))
  assert.notEqual(ice.reason.replace(/RSI\s+67\.5/g, '').trim(), '')
  assert.notEqual(ice.reason, aggressive.reason)
  assert.ok(aggressive.reason.includes('激进参数'))
  const namedToday = concludePresetObservation(presetEngine('user-today', '今日观察', { scanKind: 'ice' }), {
    summary: STOLEN_REVERSAL,
  })
  assert.equal(namedToday.verdict, '命中')
  assert.equal(namedToday.reason.includes('今日转势买点'), false)
  assert.ok(namedToday.reason.startsWith('今日观察 · 本预设规则触发（转）'))

  const shared = concludePresetObservation(
    presetEngine('alias_rev', '转势别名', { scanKind: 'scan_reversal_v1' }),
    { familyRead: { verdict: '命中', bias: '偏多', reason: reversalReason }, summary: STOLEN_REVERSAL },
  )
  assert.equal(shared.verdict, '命中')
  assert.equal(shared.bias, '偏多')
  assert.match(shared.reason, /^与转势买点同源 · 转势别名/)
  assert.ok(shared.reason.includes('RSI 67.5'))
  assert.equal(shared.reason.includes('今日转势买点'), false)
  assert.notEqual(shared.reason, reversalReason)
  assert.notEqual(shared.reason.replace(/RSI\s+67\.5/g, '').trim(), '与转势买点同源')
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
  const catalog = buildStrategyCatalog({
    settings: {
      activeScreenStrategyId: 'default',
      screenStrategies: [
        { id: 'default', name: '默认参数预设', settings: {} },
        { id: 'ext_xsmom_v1', name: '截面动量V1', settings: {} },
        { id: 'ext_ma_trend_v1', name: '均线趋势V1', settings: {} },
      ],
    },
    stockStrategies: [],
  })
  const byCatalog = Object.fromEntries(catalog.map((item) => [item.strategyId, item]))
  assert.equal(byCatalog['preset:ext_xsmom_v1'].scanKind, 'xsmom')
  assert.equal(byCatalog['preset:ext_ma_trend_v1'].scanKind, 'ma_trend')
  assert.equal(byCatalog['preset:default'].scanKind, 'ice')
  const engines = [
    WIRED_SCAN_ENGINES.find((item) => item.strategyId === 'scan_reduce_v1'),
    WIRED_SCAN_ENGINES.find((item) => item.strategyId === 'scan_reversal_v1'),
    byCatalog['preset:default'],
    byCatalog['preset:ext_xsmom_v1'],
    byCatalog['preset:ext_ma_trend_v1'],
    presetEngine('shared_reduce', '减仓同源别名', { scanKind: 'scan_reduce_v1' }),
  ]
  const rows = evaluateObservationCompare({
    bars,
    engines,
    activeSignalOptions: { ...buildSignalOptions(null), reduceEarlyEnabled: false },
  })
  for (const row of rows) assertRowShape(row)
  const byId = Object.fromEntries(rows.map((row) => [row.strategyId, row]))
  assert.equal(byId.scan_reduce_v1.verdict, '命中')
  assert.match(byId.scan_reduce_v1.reason, /破 MA20/)
  assert.equal(byId['preset:ext_xsmom_v1'].verdict, '数据不足')
  assert.equal(byId['preset:ext_xsmom_v1'].reason, PRESET_UNIMPLEMENTED_REASON)
  assert.equal(byId['preset:ext_ma_trend_v1'].reason, PRESET_UNIMPLEMENTED_REASON)
  assert.equal(byId['preset:ext_xsmom_v1'].reason.includes('破 MA20'), false)
  assert.equal(byId['preset:ext_ma_trend_v1'].reason.includes('转势'), false)
  assert.notEqual(byId['preset:ext_xsmom_v1'].reason, byId.scan_reduce_v1.reason)
  assert.notEqual(byId['preset:default'].reason, byId.scan_reduce_v1.reason)
  assert.equal(byId['preset:default'].reason.includes('破 MA20'), false)
  assert.equal(byId['preset:default'].reason.includes('今日'), false)
  assert.ok(byId['preset:default'].reason.includes('默认参数预设'))
  assert.match(byId['preset:shared_reduce'].reason, /^与破位减仓观察同源 · 减仓同源别名/)
  assert.equal(byId['preset:shared_reduce'].reason.includes('破 MA20'), false)
  if (byId.scan_reduce_v1.verdict === '命中') {
    assert.equal(byId['preset:shared_reduce'].verdict, '命中')
    assert.equal(byId['preset:shared_reduce'].bias, byId.scan_reduce_v1.bias)
  }
}

console.log('multiStrategyCompare.test.mjs ok')
