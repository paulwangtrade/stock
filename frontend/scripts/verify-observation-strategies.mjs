/**
 * 内置观察策略：开关互不牵连，卡片保留运行入口，主表排除 observation。
 * 用法: node scripts/verify-observation-strategies.mjs
 */
import fs from 'fs'
import {
  OBSERVATION_STRATEGIES,
  buildObservationStrategyPayload,
  observationStrategyIdFromRow,
  parseObservationMeta,
} from '../src/utils/observationStrategies.js'

function assert(cond, msg) {
  if (!cond) {
    console.error('FAIL', msg)
    process.exit(1)
  }
}

const ids = OBSERVATION_STRATEGIES.map((item) => item.strategyId)
assert(ids.join(',') === 'ext_ma_pullback,ext_vol_breakout,ext_dd_bounce', 'three built-in ids')

for (const card of OBSERVATION_STRATEGIES) {
  const cronOnly = buildObservationStrategyPayload(card, { id: 7, pageSize: 40 }, {
    enable: true,
    feedsTradePlan: false,
  })
  const meta = JSON.parse(cronOnly.queryJson)
  assert(cronOnly.queryType === 'observation', 'query type')
  assert(cronOnly.enable === true, 'cron enable')
  assert(cronOnly.cronExpr === '0 5 15 * * 1-5', 'default cron')
  assert(meta.feedsTradePlan === false, 'cron does not feed')
  assert(meta.strategyId === card.strategyId, 'strategy id')
  assert(cronOnly.id === 7, 'row id kept for RunStockStrategy')

  const feedOnly = buildObservationStrategyPayload(card, { id: 7, cronExpr: '0 5 15 * * 1-5' }, {
    enable: false,
    feedsTradePlan: true,
  })
  const feedMeta = JSON.parse(feedOnly.queryJson)
  assert(feedOnly.enable === false, 'feed does not enable cron')
  assert(feedOnly.cronExpr === '', 'feed clears cron')
  assert(feedMeta.feedsTradePlan === true, 'explicit feed')

  const omitted = buildObservationStrategyPayload(card, null, { enable: true })
  assert(JSON.parse(omitted.queryJson).feedsTradePlan === false, 'omitted feed stays false')
  assert(omitted.id === 0, 'missing row has no id')
}

assert(parseObservationMeta('{"feedsTradePlan":"true"}').feedsTradePlan === false, 'string true is not feed')
assert(parseObservationMeta('{"feedsTradePlan":1}').feedsTradePlan === false, 'numeric 1 is not feed')
assert(parseObservationMeta('{"feedsTradePlan":true,"strategyId":"ext_dd_bounce"}').feedsTradePlan === true, 'boolean true')
assert(
  observationStrategyIdFromRow({ queryText: 'ext_vol_breakout', queryJson: '' }) === 'ext_vol_breakout',
  'id from query text',
)

const vue = fs.readFileSync(new URL('../src/components/stockStrategyManager.vue', import.meta.url), 'utf8')
assert(vue.includes('runNow(card.row)'), 'card run calls RunStockStrategy path')
assert(vue.includes('定时观察'), 'cron switch label')
assert(vue.includes('纳入模拟交易计划'), 'feed switch label')
assert(vue.includes("excludeQueryType: 'observation'"), 'main list excludes observation')
assert(vue.includes("queryType: 'observation'"), 'observation list query')
assert(!vue.includes('内置观察策略（PR16）'), 'static PR16 card is not the panel')

console.log('ok observation strategies', ids.join(', '))
