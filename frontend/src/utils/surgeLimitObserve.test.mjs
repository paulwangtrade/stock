/**
 * 大涨/涨停降权：只观察，不产生买卖指令。
 * Run: node frontend/src/utils/surgeLimitObserve.test.mjs
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
const {
  SURGE_OBSERVE_COPY,
  SURGE_OBSERVE_LABEL,
  annotateSurgeObservation,
  assessPriceSurge,
  assessQuotedMove,
  decorateCompareRows,
  resolveAshareLimitBand,
  resolveOpportunitySurgeBadge,
} = await import(join(__dir, 'surgeLimitObserve.js'))

function ramp(start, last, n = 25) {
  const closes = []
  for (let i = 0; i < n - 1; i++) closes.push(start)
  closes.push(last)
  return closes
}

function assertNoOrderFields(row) {
  for (const key of ['action', 'side', 'order', 'tradePlan', 'signalScore', 'hitCount']) {
    assert.equal(Object.hasOwn(row, key), false, key)
  }
}

{
  assert.equal(resolveAshareLimitBand({ code: '600519', name: '贵州茅台' }).limitPct, 0.1)
  assert.equal(resolveAshareLimitBand({ code: '000001.SZ', name: '平安银行' }).limitPct, 0.1)
  assert.equal(resolveAshareLimitBand({ code: '300750', name: '宁德时代' }).limitPct, 0.2)
  assert.equal(resolveAshareLimitBand({ code: '688981', name: '中芯国际' }).limitPct, 0.2)
  assert.equal(resolveAshareLimitBand({ code: '830799', name: '艾融软件' }).limitPct, 0.3)
  assert.equal(resolveAshareLimitBand({ code: '600000', name: '*ST岩石' }).kind, 'st')
  assert.equal(resolveAshareLimitBand({ code: '600000', name: '*ST岩石' }).limitPct, 0.05)
  assert.equal(resolveAshareLimitBand({ code: '600519', name: '' }).known, false)
  assert.equal(resolveAshareLimitBand({ code: 'AAPL', name: '苹果' }).known, false)
}

{
  const limit = assessPriceSurge({ closes: ramp(10, 11), code: '600519', name: '贵州茅台' })
  assert.equal(limit.state, 'limit_up')
  const near = assessPriceSurge({ closes: ramp(10, 10.97), code: '600519', name: '贵州茅台' })
  assert.equal(near.state, 'limit_up')
  const large = assessPriceSurge({ closes: ramp(10, 10.8), code: '600519', name: '贵州茅台' })
  assert.equal(large.state, 'large_move')
  const calm = assessPriceSurge({ closes: ramp(10, 10.3), code: '600519', name: '贵州茅台' })
  assert.equal(calm.state, 'clear')
}

{
  const mid = assessPriceSurge({ closes: ramp(10, 11.2), code: '300750', name: '宁德时代' })
  assert.equal(mid.state, 'clear')
  const large = assessPriceSurge({ closes: ramp(10, 11.5), code: '300750', name: '宁德时代' })
  assert.equal(large.state, 'large_move')
  const limit = assessPriceSurge({ closes: ramp(10, 12), code: '688981', name: '中芯国际' })
  assert.equal(limit.state, 'limit_up')
}

{
  const stLimit = assessPriceSurge({ closes: ramp(10, 10.5), code: '600000', name: 'ST岩石' })
  assert.equal(stLimit.state, 'limit_up')
  const stLarge = assessPriceSurge({ closes: ramp(10, 10.36), code: '600000', name: 'ST岩石' })
  assert.equal(stLarge.state, 'large_move')
  const stCalm = assessPriceSurge({ closes: ramp(10, 10.2), code: '600000', name: 'ST岩石' })
  assert.equal(stCalm.state, 'clear')
}

{
  const recent = []
  for (let i = 0; i < 20; i++) recent.push(10)
  recent.push(11)
  recent.push(11)
  const surge = assessPriceSurge({ closes: recent, code: '600519', name: '贵州茅台' })
  assert.equal(surge.state, 'limit_up')
  assert.equal(surge.reason, 'recent_limit')
}

{
  const unknownHot = assessPriceSurge({ closes: ramp(10, 10.6), code: '600519', name: '' })
  assert.equal(unknownHot.state, 'unknown')
  assert.equal(unknownHot.failClosed, true)
  const unknownCalm = assessPriceSurge({ closes: ramp(10, 10.2), code: '600519', name: '' })
  assert.equal(unknownCalm.state, 'unknown')
  assert.equal(unknownCalm.failClosed, false)
  const missing = assessPriceSurge({ closes: [10], code: '600519', name: '贵州茅台' })
  assert.equal(missing.state, 'unknown')
  assert.equal(missing.failClosed, true)
}

{
  const surge = assessPriceSurge({ closes: ramp(10, 11), code: '600519', name: '贵州茅台' })
  const hit = annotateSurgeObservation(
    { strategyId: 'scan_strong_v1', verdict: '命中', bias: '偏多', reason: '强化买点' },
    { roleId: 'entry', surge },
  )
  assert.equal(hit.verdict, '命中')
  assert.equal(hit.surgeObserve.label, SURGE_OBSERVE_LABEL)
  assert.equal(hit.surgeObserve.copy, SURGE_OBSERVE_COPY)
  assert.equal(hit.surgeObserve.demote, true)
  assert.equal(hit.displayPriority, 2)
  assert.match(hit.reason, /动量末端观察/)
  assertNoOrderFields(hit)

  const confirm = annotateSurgeObservation(
    { verdict: '命中', reason: '与均线趋势同源' },
    { roleId: 'confirm', surge },
  )
  assert.equal(confirm.surgeObserve.demote, true)

  const reduce = annotateSurgeObservation({ verdict: '命中', reason: '破 MA20' }, { roleId: 'reduce', surge })
  assert.equal(reduce.surgeObserve, null)
  assert.equal(reduce.displayPriority, 0)

  const exit = annotateSurgeObservation({ verdict: '命中', reason: 'RSI 回落' }, { roleId: 'exit', surge })
  assert.equal(exit.surgeObserve, null)

  const miss = annotateSurgeObservation({ verdict: '未命中', reason: '未形成' }, { roleId: 'entry', surge })
  assert.equal(miss.surgeObserve, null)
  assert.equal(miss.displayPriority, 1)
}

{
  const rows = decorateCompareRows(
    [
      { strategyId: 'scan_reduce_v1', verdict: '命中', reason: '破 MA20' },
      { strategyId: 'scan_strong_v1', verdict: '命中', reason: '强化买点' },
      { strategyId: 'scan_ice_v1', verdict: '未命中', reason: '未形成' },
      { strategyId: 'ext_ma_trend_v1', verdict: '命中', reason: '趋势确认' },
    ],
    {
      closes: ramp(10, 11),
      code: '600519',
      name: '贵州茅台',
      roleOf(id) {
        if (id === 'scan_reduce_v1') return 'reduce'
        if (id === 'ext_ma_trend_v1') return 'confirm'
        if (id === 'scan_strong_v1') return 'entry'
        return 'other'
      },
    },
  )
  assert.deepEqual(rows.map((row) => row.strategyId), [
    'scan_reduce_v1',
    'scan_ice_v1',
    'scan_strong_v1',
    'ext_ma_trend_v1',
  ])
  assert.equal(rows[2].surgeObserve.label, '动量末端观察')
  assert.equal(rows[3].surgeObserve.label, '动量末端观察')
  assert.equal(rows[0].surgeObserve, null)
}

{
  const quoted = assessQuotedMove({ changeRate: 9.9, code: '600519', name: '贵州茅台' })
  assert.equal(quoted.state, 'limit_up')
  const fromPrice = assessQuotedMove({
    preClose: 10,
    last: 10.8,
    changeRate: 0,
    code: '600519',
    name: '贵州茅台',
  })
  assert.equal(fromPrice.state, 'large_move')

  const badge = resolveOpportunitySurgeBadge({
    strategyId: 'default',
    tag: '强',
    code: '600519',
    name: '贵州茅台',
    changeRate: '10.02',
  })
  assert.equal(badge.label, SURGE_OBSERVE_LABEL)
  assert.equal(badge.copy, SURGE_OBSERVE_COPY)
  assert.equal(badge.demote, true)

  const confirmBadge = resolveOpportunitySurgeBadge({
    strategyId: 'ext_ma_trend_v1',
    strategyMeta: { kind: 'scan_preset', scanKind: 'ma_trend', templateId: 'ext_ma_trend_v1' },
    tag: '趋',
    code: '300750',
    name: '宁德时代',
    changeRate: 15,
  })
  assert.equal(confirmBadge.label, SURGE_OBSERVE_LABEL)

  assert.equal(resolveOpportunitySurgeBadge({
    strategyId: 'default',
    tag: '减',
    code: '600519',
    name: '贵州茅台',
    changeRate: 10,
  }), null)
  assert.equal(resolveOpportunitySurgeBadge({
    strategyId: 'default',
    tag: '强',
    code: '600519',
    name: '贵州茅台',
    changeRate: 1.2,
  }), null)
  const unknown = resolveOpportunitySurgeBadge({
    strategyId: 'default',
    tag: '买',
    code: '600519',
    name: '',
    changeRate: 6,
  })
  assert.equal(unknown.state, 'unknown')
  assert.equal(unknown.demote, true)
}

{
  const root = join(__dir, '..')
  const panel = readFileSync(join(root, 'components/MultiStrategyComparePanel.vue'), 'utf8')
  const list = readFileSync(join(root, 'components/allStockList.vue'), 'utf8')
  assert.match(panel, /SURGE_OBSERVE_COPY/)
  assert.match(panel, /SURGE_OBSERVE_LABEL/)
  assert.match(panel, /decorateCompareRows/)
  assert.equal(panel.includes('TradePlan'), false)
  assert.equal(panel.includes('signalScore'), false)
  assert.match(list, /SURGE_OBSERVE_COPY/)
  assert.match(list, /resolveOpportunitySurgeBadge/)
  assert.equal(list.includes('CreateOrder'), false)
}

assert.equal(SURGE_OBSERVE_COPY, '不是买卖指令；大涨后确认≠安全买点')
console.log('surgeLimitObserve.test.mjs ok')
