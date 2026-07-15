import assert from 'node:assert/strict'
import {
  LEVEL_SEMANTICS_VERSION,
  MARKET_LEVEL_IMPACT,
  MARKET_MODE_EXPOSURE_FACTOR,
  calculateModelPositionCap,
  isNewEntryBlockedLevel,
  resolveMarketMode,
  resolveTradingLevel,
  toDisplayTradingLevel,
} from '../src/utils/tradingLevelRules.js'
import { calcBuyPositionPlan } from '../src/utils/buyPositionSizing.js'
import {
  resolveEffectiveStockMarketMode,
  resolveStockMarketSegment,
} from '../src/utils/stockMarketSegment.js'

const cases = [
  {
    expected: 'level5',
    input: { close: 110, ma5: 108, ma10: 105, ma20: 100, ma60: 95, volumeRatio: 1.3, volumeExpanding: true, ma20Rising: true },
  },
  {
    expected: 'level4',
    input: { close: 104, ma5: 103, ma10: 104.5, ma20: 100, ma60: 96, volumeRatio: 0.9, volumeExpanding: false, ma20Rising: true },
  },
  {
    expected: 'level3',
    input: { close: 99, ma5: 101, ma10: 100, ma20: 100, ma60: 98, volumeRatio: 0.95, volumeExpanding: false, ma20Rising: false },
  },
  {
    expected: 'level2',
    input: { close: 97, ma5: 98, ma10: 99, ma20: 100, ma60: 95, volumeRatio: 1.1, volumeExpanding: true, ma20Rising: false },
  },
  {
    expected: 'level1',
    input: { close: 92, ma5: 94, ma10: 96, ma20: 99, ma60: 95, volumeRatio: 1.4, volumeExpanding: true, ma20Rising: false },
  },
]

for (const row of cases) {
  assert.equal(resolveMarketMode(row.input).key, row.expected)
}

assert.equal(LEVEL_SEMANTICS_VERSION, 2)
assert.equal(calculateModelPositionCap('level1', 1).pct, 0)
assert.equal(calculateModelPositionCap('level2', 1).rule.rangeLabel, '0%-10%')
assert.equal(calculateModelPositionCap('level3', 1).rule.rangeLabel, '0%-20%')
assert.equal(calculateModelPositionCap('level4', 1).rule.rangeLabel, '40%-60%')
assert.equal(calculateModelPositionCap('level5', 1).rule.rangeLabel, '80%-100%')
assert.equal(calculateModelPositionCap('level5', 0.85).pct, 0.85)
assert.equal(toDisplayTradingLevel(1), 1)
assert.equal(toDisplayTradingLevel(3), 3)
assert.equal(toDisplayTradingLevel(5), 5)
assert.equal(resolveTradingLevel('attack').key, 'level5')
assert.equal(resolveTradingLevel('neutral').key, 'level3')
assert.equal(resolveTradingLevel('defense').key, 'level2')
assert.equal(MARKET_MODE_EXPOSURE_FACTOR.attack, 1)
assert.equal(MARKET_MODE_EXPOSURE_FACTOR.defense, 0.1)

assert.equal(resolveStockMarketSegment('600519.SH').key, 'shMain')
assert.equal(resolveStockMarketSegment('000001.SZ').key, 'szMain')
assert.equal(resolveStockMarketSegment('688001.SH').key, 'star')
assert.equal(resolveStockMarketSegment('300750.SZ').key, 'chinext')
assert.equal(resolveStockMarketSegment('920002.BJ').key, 'beijing')
assert.equal(resolveStockMarketSegment('832000.BJ').key, 'beijing')
assert.equal(resolveStockMarketSegment('430047').key, 'beijing')

const effective = resolveEffectiveStockMarketMode(
  { key: 'level4', level: 4, name: '轻仓试探' },
  { key: 'level2', level: 2, name: '战略撤退' },
  resolveStockMarketSegment('300750.SZ'),
)
assert.equal(effective.level, 2)
assert.match(effective.reason, /创业板2级/)

const automation = {
  accountEquity: 100000,
  riskPerTradePct: 0.01,
  maxPositionPct: 1,
  maxTotalExposurePct: 1,
  minLotSize: 100,
  tagConfidence: { 买: 1 },
  blockNewEntriesOnDefense: true,
  requireChecklistForBuyConfirm: false,
}
const buyCtx = {
  summary: { tag: '买' },
  buyPriceRange: { signalLow: 9.5 },
  livePrice: 10,
  automation,
  totalExposureValue: 0,
}
assert.equal(calcBuyPositionPlan({ ...buyCtx, marketModeKey: 'level1' }).ok, false)
assert.equal(calcBuyPositionPlan({ ...buyCtx, marketModeKey: 'level5' }).ok, true)
assert.equal(isNewEntryBlockedLevel(1), true)
assert.equal(isNewEntryBlockedLevel(2), true)
assert.equal(isNewEntryBlockedLevel(4), false)
assert.ok(MARKET_LEVEL_IMPACT[5] > MARKET_LEVEL_IMPACT[1])

console.log('等级语义 v2、仓位模型与持仓风控规则校验通过')
