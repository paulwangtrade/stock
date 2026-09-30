/**
 * 入场未命中观察：近端命中日期与当前参考价。只读，不是委托。
 * Run: node frontend/src/utils/entryMissObservation.test.mjs
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
const { WIRED_SCAN_ENGINES, evaluateObservationCompare } = await import(
  pathToFileURL(join(__dir, 'multiStrategyCompare.js')).href
)
const {
  ENTRY_MISS_FOOTER,
  ENTRY_MISS_LOOKBACK_DAYS,
  ENTRY_MISS_NO_UNIQUE_PRICE,
  buildEntryMissObservation,
  buildMaCrossMissObservation,
  maCrossBacktestSignal,
} = await import(pathToFileURL(join(__dir, 'entryMissObservation.js')).href)

function dayKey(index) {
  const day = 20240102 + index
  const text = String(day)
  return `${text.slice(0, 4)}-${text.slice(4, 6)}-${text.slice(6, 8)}`
}

function barsFromCloses(closes, shape) {
  const opens = []
  const highs = []
  const lows = []
  const volumes = []
  const dayKeys = []
  closes.forEach((close, index) => {
    const spec = shape ? shape(close, index) : null
    opens.push(spec?.open ?? close)
    highs.push(spec?.high ?? close)
    lows.push(spec?.low ?? close)
    volumes.push(spec?.volume ?? 1000)
    dayKeys.push(dayKey(index))
  })
  return { closes: [...closes], opens, highs, lows, volumes, dayKeys, indexMa20ByDay: null }
}

function flatBars(n, price) {
  return barsFromCloses(Array.from({ length: n }, () => price))
}

function assertObservationOnly(miss) {
  assert.equal(miss.observationOnly, true)
  assert.equal(Object.hasOwn(miss, 'action'), false)
  assert.equal(Object.hasOwn(miss, 'side'), false)
  assert.equal(Object.hasOwn(miss, 'order'), false)
  assert.equal(Object.hasOwn(miss, 'tradePlan'), false)
  assert.equal(JSON.stringify(miss).includes('TradePlan'), false)
}

assert.ok(ENTRY_MISS_LOOKBACK_DAYS >= 20 && ENTRY_MISS_LOOKBACK_DAYS <= 60)
assert.equal(ENTRY_MISS_NO_UNIQUE_PRICE, '无法给出唯一价')
assert.equal(ENTRY_MISS_FOOTER.includes('不是买卖指令'), true)

{
  const bars = flatBars(40, 10)
  const engine = WIRED_SCAN_ENGINES.find((item) => item.strategyId === 'scan_strong_v1')
  const reduce = WIRED_SCAN_ENGINES.find((item) => item.strategyId === 'scan_reduce_v1')
  const rows = evaluateObservationCompare({
    bars,
    engines: [engine, reduce],
    activeSignalOptions: buildSignalOptions(null),
  })
  const byId = Object.fromEntries(rows.map((row) => [row.strategyId, row]))
  assert.equal(byId.scan_strong_v1.verdict, '未命中')
  assert.equal(byId.scan_strong_v1.entryMiss.priceMode, 'unavailable')
  assert.match(byId.scan_strong_v1.entryMiss.priceText, /无法给出唯一价/)
  assert.match(byId.scan_strong_v1.entryMiss.hitsText, /近40个交易日无命中/)
  assertObservationOnly(byId.scan_strong_v1.entryMiss)
  assert.equal(byId.scan_reduce_v1.entryMiss, null)
}

{
  const miss = buildEntryMissObservation({
    roleId: 'entry',
    tag: '强',
    bars: flatBars(30, 10),
    familySig: { strictBuy: [10, 20] },
    signalOptions: { strongConfirmDays: 2 },
    verdict: '未命中',
    lookbackDays: 25,
  })
  assert.deepEqual(
    miss.hits.map((item) => item.index),
    [22, 12],
  )
  assert.equal(miss.hits[0].date, dayKey(22))
  assert.equal(miss.hits[0].price, 10)
  assert.match(miss.hitsText, new RegExp(`${dayKey(22)} 10\\.00`))
  assert.match(miss.priceText, /无法给出唯一价/)
  assert.equal(
    buildEntryMissObservation({
      roleId: 'entry',
      tag: '强',
      bars: flatBars(30, 10),
      familySig: { strictBuy: [10] },
      verdict: '命中',
    }),
    null,
  )
  assert.equal(
    buildEntryMissObservation({
      roleId: 'reduce',
      tag: '减',
      bars: flatBars(30, 10),
      familySig: { sellMa20: [20] },
      verdict: '未命中',
    }),
    null,
  )
}

{
  const closes = []
  for (let i = 0; i < 21; i++) closes.push(10)
  const highs = closes.map((close, index) => (index < 20 ? (index % 2 === 0 ? 10.4 : 9.6) : close))
  const lows = closes.map((close, index) => (index < 20 ? (index % 2 === 0 ? 9.7 : 10.1) : close))
  const bars = barsFromCloses(closes, (close, index) => ({
    high: index < 20 ? Math.max(highs[index], close) : close,
    low: index < 20 ? Math.min(lows[index], close) : close,
  }))
  let boxHigh = -Infinity
  let boxLow = Infinity
  for (let j = 0; j < 20; j++) {
    boxHigh = Math.max(boxHigh, bars.highs[j])
    boxLow = Math.min(boxLow, bars.lows[j])
  }
  assert.ok(boxHigh > boxLow)
  const engine = WIRED_SCAN_ENGINES.find((item) => item.strategyId === 'scan_breakout_v1')
  const rows = evaluateObservationCompare({
    bars,
    engines: [engine],
    activeSignalOptions: { ...buildSignalOptions(null), boxPeriod: 20, maxRangePct: 0.2, breakBuffer: 0.005, minGap: 15 },
  })
  const miss = rows[0].entryMiss
  assert.equal(rows[0].verdict, '未命中')
  assert.equal(miss.priceMode, 'price')
  const raw = boxHigh * 1.005
  assert.ok(miss.triggerPrice > raw)
  assert.ok(miss.triggerPrice - raw < 0.02)
  assert.match(miss.priceText, /参考价/)
  assert.match(miss.priceText, /还差/)
  assert.match(miss.priceText, /确认站稳另计/)
  assert.equal(miss.priceText.includes('无法给出唯一价'), false)
  assertObservationOnly(miss)
}

{
  const wide = barsFromCloses(
    Array.from({ length: 25 }, () => 10),
    (_close, index) => ({
      high: index === 10 ? 20 : 10,
      low: index === 10 ? 5 : 10,
    }),
  )
  const miss = buildEntryMissObservation({
    roleId: 'entry',
    tag: '突',
    bars: wide,
    familySig: { breakoutBuy: [] },
    signalOptions: { boxPeriod: 20, maxRangePct: 0.2, breakBuffer: 0.005, minGap: 15 },
    verdict: '未命中',
  })
  assert.equal(miss.priceMode, 'unavailable')
  assert.equal(miss.priceText, '无法给出唯一价')
  assert.equal(miss.triggerPrice, null)
}

{
  const closes = Array.from({ length: 34 }, () => 20)
  closes[30] = 10
  closes[31] = 10
  closes[32] = 20
  closes[33] = 17
  const bars = barsFromCloses(closes)
  const options = {
    ...buildSignalOptions(null),
    reduceConfirmDays: 2,
    maPeriod: 20,
    reboundConfirmDays: 2,
    reboundSellLookback: 6,
    reboundMinDaysAfterSell: 1,
  }
  const direct = buildEntryMissObservation({
    roleId: 'entry',
    tag: '弹',
    bars,
    signalOptions: options,
    verdict: '未命中',
  })
  assert.equal(direct.priceMode, 'price')
  assert.ok(direct.triggerPrice > 17)
  assert.match(direct.priceText, /参考价/)
  assert.match(direct.priceText, /还差/)
  assert.match(direct.priceText, /收回 MA20/)
  assertObservationOnly(direct)

  const blocked = buildEntryMissObservation({
    roleId: 'entry',
    tag: '弹',
    bars: flatBars(40, 10),
    signalOptions: options,
    verdict: '未命中',
  })
  assert.equal(blocked.priceMode, 'unavailable')
  assert.equal(blocked.priceText, '无法给出唯一价')
}

{
  for (const tag of ['趋', '买', '转']) {
    const miss = buildEntryMissObservation({
      roleId: 'entry',
      tag,
      bars: flatBars(30, 10),
      familySig: {},
      verdict: '未命中',
    })
    assert.match(miss.priceText, /无法给出唯一价/)
    assert.equal(miss.triggerPrice, null)
  }
  const preset = buildEntryMissObservation({
    roleId: 'entry',
    composite: true,
    bars: flatBars(30, 10),
    signalOptions: buildSignalOptions(null),
    verdict: '未命中',
  })
  assert.equal(preset.priceMode, 'unavailable')
  assert.equal(preset.priceText, '无法给出唯一价')
  assert.match(preset.hitsText, /近40个交易日/)
}

{
  const closes = Array.from({ length: 40 }, () => 10)
  const flat = buildMaCrossMissObservation({ closes, dayKeys: closes.map((_, i) => dayKey(i)) })
  assert.equal(flat.priceMode, 'unavailable')
  assert.match(flat.priceText, /无法给出唯一价/)
  assert.match(flat.hitsText, /无命中/)
  assertObservationOnly(flat)

  const down = []
  for (let i = 0; i < 40; i++) down.push(50 - i * 0.4)
  const last = down.length - 1
  assert.equal(maCrossBacktestSignal(down, last)?.action === 'buy', false)
  const miss = buildMaCrossMissObservation({ closes: down, dayKeys: down.map((_, i) => dayKey(i)) })
  if (miss.priceMode === 'price') {
    const trial = down.slice()
    trial[last] = miss.triggerPrice
    assert.equal(maCrossBacktestSignal(trial, last)?.action, 'buy')
    const below = down.slice()
    below[last] = Math.round((miss.triggerPrice - 0.02) * 100) / 100
    if (below[last] > 0) {
      assert.notEqual(maCrossBacktestSignal(below, last)?.action, 'buy')
    }
  } else {
    assert.match(miss.priceText, /无法给出唯一价/)
  }
  assert.equal(Object.hasOwn(miss, 'tradePlan'), false)
}

{
  const compareVue = readFileSync(join(__dir, '../components/MultiStrategyComparePanel.vue'), 'utf8')
  const backtestVue = readFileSync(join(__dir, '../components/SignalBacktestPanel.vue'), 'utf8')
  assert.ok(compareVue.includes("title: '未命中观察'"))
  assert.ok(compareVue.includes('ENTRY_MISS_FOOTER'))
  assert.ok(compareVue.includes('无法给出唯一价'))
  assert.equal(compareVue.includes('TradePlan'), false)
  assert.ok(backtestVue.includes('buildMaCrossMissObservation'))
  assert.ok(backtestVue.includes('不是买卖指令'))
  assert.equal(backtestVue.includes('TradePlan'), false)
}

console.log('entryMissObservation.test.mjs ok')
