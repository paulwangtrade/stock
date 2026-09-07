/**
 * Unit tests: TradePlan Origin display (Phase14-G2.2 + Phase16.28-B).
 * Run: node frontend/src/utils/tradePlanOriginDisplay.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'tradePlanOriginDisplay.js')).href
const {
  ORIGIN_API_MISSING,
  ORIGIN_EMPTY,
  ORIGIN_REASON_EMPTY,
  ORIGIN_SIGNAL_PRICE_FOOTER,
  ORIGIN_SIGNAL_PRICE_TOOLTIP,
  ORIGIN_TABLE_DASH,
  buildOriginItemDisplay,
  buildOriginPanelModel,
  formatOriginField,
  formatOriginSignalPrice,
  isOriginFieldMissing,
  originSourceChipMeta,
  resolveOriginReasonSummary,
  resolveOriginSourceBucket,
} = await import(modUrl)

{
  assert.equal(isOriginFieldMissing('missing'), true)
  assert.equal(isOriginFieldMissing('MISSING'), true)
  assert.equal(isOriginFieldMissing(''), true)
  assert.equal(isOriginFieldMissing(null), true)
  assert.equal(isOriginFieldMissing('动量策略 v1 扫描命中'), false)
}

{
  assert.equal(formatOriginField('missing'), ORIGIN_EMPTY)
  assert.equal(formatOriginField('  策略入选  '), '策略入选')
}

{
  assert.equal(formatOriginSignalPrice('missing'), ORIGIN_EMPTY)
  assert.equal(formatOriginSignalPrice('12.34'), '12.34')
  assert.equal(formatOriginSignalPrice('28.5'), '28.50')
}

{
  const d = buildOriginItemDisplay(
    {
      stock_code: 'sz000001',
      strategy_name: '动量策略 v1',
      score: '0.82',
      source_reason: '动量策略 v1 扫描命中「突」信号（2026-08-28）',
      selection_reason: '候选池排名第 2，综合分 0.82，纳入计划（上限 5 只）',
      signal_time: '2026-08-28T15:00:00+08:00',
      signal_price: '12.34',
      signal_tag: '突',
    },
    { sourceSession: 'after_close', stockName: '平安银行' },
  )
  assert.equal(d.stockCode, 'sz000001')
  assert.equal(d.stockName, '平安银行')
  assert.equal(d.signalTagLabel, '突')
  assert.equal(d.signalPrice, '12.34')
  assert.equal(d.strategyNameLabel, '动量策略 v1')
  assert.equal(d.scoreLabel, '0.82')
  assert.equal(d.sourceChipLabel, 'Strategy')
  assert.equal(d.sourceBucket, 'strategy')
  assert.equal(d.sourceReasonMissing, false)
  assert.match(d.reasonSummary, /候选池排名/)
  assert.ok(d.reasonSummary.length <= 28 || d.reasonSummary.endsWith('…'))
}

{
  const d = buildOriginItemDisplay({
    stock_code: 'sh600000',
    strategy_name: ORIGIN_API_MISSING,
    score: ORIGIN_API_MISSING,
    source_reason: ORIGIN_API_MISSING,
    selection_reason: ORIGIN_API_MISSING,
    signal_time: ORIGIN_API_MISSING,
    signal_price: ORIGIN_API_MISSING,
    signal_tag: ORIGIN_API_MISSING,
  })
  assert.equal(d.sourceReason, ORIGIN_EMPTY)
  assert.equal(d.selectionReason, ORIGIN_EMPTY)
  assert.equal(d.signalTime, ORIGIN_EMPTY)
  assert.equal(d.signalPrice, ORIGIN_EMPTY)
  assert.equal(d.signalTag, ORIGIN_EMPTY)
  assert.equal(d.strategyNameLabel, ORIGIN_TABLE_DASH)
  assert.equal(d.scoreLabel, ORIGIN_TABLE_DASH)
  assert.equal(d.signalTagLabel, ORIGIN_TABLE_DASH)
  assert.equal(d.reasonSummary, ORIGIN_REASON_EMPTY)
  assert.equal(d.sourceChipLabel, '未知来源')
  assert.equal(d.sourceReasonMissing, true)
  assert.equal(d.signalTagMissing, true)
}

{
  assert.equal(resolveOriginSourceBucket({ sourceSession: 'watchlist' }), 'watchlist')
  assert.equal(resolveOriginSourceBucket({ strategyName: 'follow' }), 'manual')
  assert.equal(resolveOriginSourceBucket({ sourceSession: 'after_close' }), 'strategy')
  const chip = originSourceChipMeta({ sourceSession: 'watchlist' })
  assert.equal(chip.label, 'Watchlist')
}

{
  assert.equal(resolveOriginReasonSummary({ selection_reason: '短理由' }), '短理由')
  assert.equal(
    resolveOriginReasonSummary({
      selection_reason: 'missing',
      source_reason: '发现依据文案',
    }),
    '发现依据文案',
  )
  assert.equal(resolveOriginReasonSummary({}), ORIGIN_REASON_EMPTY)
}

{
  const panel = buildOriginPanelModel(
    [
      {
        stock_code: 'a',
        source_reason: ORIGIN_API_MISSING,
        selection_reason: ORIGIN_API_MISSING,
        signal_time: ORIGIN_API_MISSING,
        signal_price: ORIGIN_API_MISSING,
        signal_tag: ORIGIN_API_MISSING,
      },
      {
        stock_code: 'b',
        source_reason: '策略入选',
        selection_reason: ORIGIN_API_MISSING,
        signal_time: ORIGIN_API_MISSING,
        signal_price: ORIGIN_API_MISSING,
        signal_tag: ORIGIN_API_MISSING,
        strategy_name: '默认策略',
        score: '0.55',
      },
    ],
    { sourceSession: 'after_close', nameByCode: { b: '股票B' } },
  )
  assert.equal(panel.items.length, 2)
  assert.equal(panel.hasAnyData, true)
  assert.equal(panel.items[1].stockName, '股票B')
  assert.equal(panel.items[1].sourceChipLabel, 'Strategy')
  assert.equal(panel.items[1].strategyNameLabel, '默认策略')
}

{
  const panel = buildOriginPanelModel([
    {
      stock_code: 'x',
      source_reason: ORIGIN_API_MISSING,
      selection_reason: ORIGIN_API_MISSING,
      signal_time: ORIGIN_API_MISSING,
      signal_price: ORIGIN_API_MISSING,
      signal_tag: ORIGIN_API_MISSING,
    },
  ])
  assert.equal(panel.hasAnyData, false)
}

assert.match(ORIGIN_SIGNAL_PRICE_FOOTER, /买入价/)
assert.match(ORIGIN_SIGNAL_PRICE_FOOTER, /委托价/)
assert.match(ORIGIN_SIGNAL_PRICE_FOOTER, /成交价/)
assert.match(ORIGIN_SIGNAL_PRICE_TOOLTIP, /非买入价/)

console.log('tradePlanOriginDisplay.test.mjs: all passed')
