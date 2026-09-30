import assert from 'node:assert/strict'
import test from 'node:test'
import { STRATEGY_PRESETS } from './technicalIndicators.js'
import { cloneDefaultSignalSettings, mergeSignalSettings } from './signalSettings.js'
import {
  OBSERVATION_CRON_EXPR,
  OBSERVATION_STRATEGIES,
  buildObservationStrategyPayload,
  evaluateObservationStrategy,
  isObservationStrategyId,
  runObservationScanBatch,
} from './observationStrategies.js'

function flatBars(n, price, volume = 100) {
  return {
    opens: Array(n).fill(price),
    highs: Array(n).fill(price),
    lows: Array(n).fill(price * 0.99),
    closes: Array(n).fill(price),
    volumes: Array(n).fill(volume),
  }
}

function risingPullbackBars() {
  const n = 40
  const step = 0.5
  const closes = []
  const opens = []
  const highs = []
  const lows = []
  const volumes = []
  for (let i = 0; i < n; i++) {
    const close = 20 + i * step
    closes.push(close)
    opens.push(close - 0.15)
    highs.push(close + 0.1)
    lows.push(close - 0.2)
    volumes.push(1000)
  }
  // 最后一根收回阳线，低点靠近 20 日均线（约 34.75），收盘仍在均线之上。
  lows[n - 1] = 35
  opens[n - 1] = 36
  closes[n - 1] = 39.5
  highs[n - 1] = 39.8
  return { opens, highs, lows, closes, volumes }
}

function breakoutBars({ volume = 200 } = {}) {
  const n = 21
  const bars = flatBars(n, 10, 100)
  bars.highs = bars.highs.slice()
  bars.closes = bars.closes.slice()
  bars.opens = bars.opens.slice()
  bars.lows = bars.lows.slice()
  bars.volumes = bars.volumes.slice()
  const i = n - 1
  bars.highs[i] = 12
  bars.closes[i] = 11
  bars.opens[i] = 10.2
  bars.lows[i] = 10.1
  bars.volumes[i] = volume
  return bars
}

function bounceBars() {
  const n = 60
  const closes = Array(n).fill(70)
  // 近端先上冲再小幅回落，最后一根阳线反弹；更早的 70 保证这不是 60 日新低。
  const tail = {
    45: 92,
    46: 94,
    47: 96,
    48: 98,
    49: 100,
    50: 99,
    51: 98,
    52: 97,
    53: 96,
    54: 95,
    55: 94,
    56: 93,
    57: 92,
    58: 89,
    59: 90,
  }
  for (const [idx, price] of Object.entries(tail)) closes[Number(idx)] = price
  const opens = closes.map((close) => close - 0.4)
  const highs = closes.map((close, i) => Math.max(close, opens[i]) + 0.3)
  const lows = closes.map((close, i) => Math.min(close, opens[i]) - 0.3)
  opens[59] = 88.6
  highs[59] = 90.4
  lows[59] = 88.2
  return { opens, highs, lows, closes, volumes: Array(n).fill(1000) }
}

test('registry has exactly three observation strategies, distinct from ice presets', () => {
  assert.equal(OBSERVATION_STRATEGIES.length, 3)
  const ids = OBSERVATION_STRATEGIES.map((item) => item.strategyId)
  assert.deepEqual(new Set(ids).size, 3)
  assert.equal(ids.includes('ext_xsmom_v1'), false)
  const iceIds = new Set(STRATEGY_PRESETS.map((item) => item.id))
  for (const item of OBSERVATION_STRATEGIES) {
    assert.equal(item.feedsTradePlan, false)
    assert.equal(item.enableCron, false)
    assert.equal(item.observationOnly, true)
    assert.equal(item.trackA, false)
    assert.equal(iceIds.has(item.strategyId), false)
    assert.equal(isObservationStrategyId(item.strategyId), true)
    assert.ok(item.name)
    assert.ok(item.blurb)
    assert.equal(/胜率|年化|必涨/.test(item.blurb), false)
  }
})

test('signal presets include the three strategies and default feed stays off', () => {
  const settings = cloneDefaultSignalSettings()
  const ids = settings.screenStrategies.map((item) => item.id)
  for (const item of OBSERVATION_STRATEGIES) {
    assert.equal(ids.includes(item.strategyId), true)
    const preset = settings.screenStrategies.find((row) => row.id === item.strategyId)
    assert.equal(preset.feedsTradePlan, false)
    assert.equal(preset.enableCron, false)
    assert.equal(preset.builtin, true)
    assert.equal(preset.trackA, false)
  }
  assert.equal(settings.activeScreenStrategyId, 'default')

  const stripped = mergeSignalSettings({
    screenStrategies: [{ id: 'default', name: '默认参数预设', settings: {} }],
    activeScreenStrategyId: 'default',
  })
  for (const item of OBSERVATION_STRATEGIES) {
    assert.ok(stripped.screenStrategies.some((row) => row.id === item.strategyId))
  }
  const kept = mergeSignalSettings({
    screenStrategies: [
      { id: 'default', name: '默认参数预设', settings: {} },
      {
        id: 'ext_ma_pullback',
        name: '均线趋势回踩',
        feedsTradePlan: true,
        enableCron: true,
        settings: {},
      },
    ],
  })
  const ma = kept.screenStrategies.find((row) => row.id === 'ext_ma_pullback')
  assert.equal(ma.feedsTradePlan, true)
  assert.equal(ma.enableCron, true)
  assert.equal(ma.trackA, false)
})

test('enabling cron does not turn on feedsTradePlan', () => {
  const def = OBSERVATION_STRATEGIES[0]
  const saved = buildObservationStrategyPayload(def, null, { enable: true, feedsTradePlan: false })
  assert.equal(saved.enable, true)
  assert.equal(saved.cronExpr, OBSERVATION_CRON_EXPR)
  const meta = JSON.parse(saved.queryJson)
  assert.equal(meta.feedsTradePlan, false)
  assert.equal(meta.trackA, false)
  assert.equal(meta.strategyId, def.strategyId)

  const fed = buildObservationStrategyPayload(def, { id: 7, queryJson: saved.queryJson, enable: true, cronExpr: saved.cronExpr }, {
    enable: true,
    feedsTradePlan: true,
  })
  assert.equal(JSON.parse(fed.queryJson).feedsTradePlan, true)
  const cronOnly = buildObservationStrategyPayload(def, fed, { enable: false })
  assert.equal(cronOnly.enable, false)
  assert.equal(JSON.parse(cronOnly.queryJson).feedsTradePlan, true)
})

test('ma pullback passes a rising dip and skips bad bars', () => {
  const ok = evaluateObservationStrategy('ext_ma_pullback', risingPullbackBars())
  assert.equal(ok.pass, true, ok.reason)
  assert.match(ok.statusText, /不是交易指令/)

  const far = risingPullbackBars()
  far.lows[far.lows.length - 1] = far.closes[far.closes.length - 1] - 0.2
  const missed = evaluateObservationStrategy('ext_ma_pullback', far)
  assert.equal(missed.pass, false)
  assert.equal(missed.reason, 'no_pullback')

  const short = evaluateObservationStrategy('ext_ma_pullback', flatBars(10, 10))
  assert.equal(short.pass, false)
  assert.equal(short.reason, 'insufficient_bars')

  const broken = risingPullbackBars()
  broken.closes[broken.closes.length - 1] = Number.NaN
  assert.equal(evaluateObservationStrategy('ext_ma_pullback', broken).pass, false)
})

test('volume breakout requires a close above the prior high and heavier volume', () => {
  const ok = evaluateObservationStrategy('ext_vol_breakout', breakoutBars({ volume: 200 }))
  assert.equal(ok.pass, true, ok.reason)

  const quiet = evaluateObservationStrategy('ext_vol_breakout', breakoutBars({ volume: 100 }))
  assert.equal(quiet.pass, false)
  assert.equal(quiet.reason, 'volume_not_confirmed')

  const missing = breakoutBars()
  missing.volumes[3] = 0
  assert.equal(evaluateObservationStrategy('ext_vol_breakout', missing).reason, 'volume_missing')

  const noVol = breakoutBars()
  delete noVol.volumes
  assert.equal(evaluateObservationStrategy('ext_vol_breakout', noVol).reason, 'invalid_bars')
})

test('controlled drawdown bounce skips ice-like new lows', () => {
  const ok = evaluateObservationStrategy('ext_dd_bounce', bounceBars())
  assert.equal(ok.pass, true, ok.reason)

  const iced = bounceBars()
  const last = iced.closes.length - 1
  iced.lows[last] = 60
  const skipped = evaluateObservationStrategy('ext_dd_bounce', iced)
  assert.equal(skipped.pass, false)
  assert.equal(skipped.reason, 'new_low_overlap')

  const deep = bounceBars()
  deep.closes[deep.closes.length - 1] = 70
  deep.opens[deep.opens.length - 1] = 69
  const tooDeep = evaluateObservationStrategy('ext_dd_bounce', deep)
  assert.equal(tooDeep.pass, false)
  assert.equal(tooDeep.reason, 'drawdown_too_deep')
})

test('observation scan emits hits only for the selected strategy id', () => {
  const stock = {
    code: 'sh600036',
    name: '招商银行',
    ...breakoutBars({ volume: 200 }),
  }
  const hit = runObservationScanBatch('ext_vol_breakout', [stock])
  assert.equal(hit.hitTotal, 1)
  assert.equal(hit.items[0].tag, '买')
  assert.equal(hit.items[0].strategy_id, 'ext_vol_breakout')
  assert.match(hit.items[0].statusText, /不是交易指令/)

  const other = runObservationScanBatch('ext_ma_pullback', [stock])
  assert.equal(other.hitTotal, 0)
  assert.equal(runObservationScanBatch('ice_buy', [stock]).hitTotal, 0)
})
