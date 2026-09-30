/**
 * Run: node frontend/src/utils/signalScanAttributionDisplay.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const mod = await import(pathToFileURL(join(__dir, 'signalScanAttributionDisplay.js')).href)
const {
  ATTRIBUTION_DISCLAIMER,
  RESEARCH_STAT_LABEL,
  LARGE_SAMPLE_WARNING,
  horizonCellText,
  horizonReasonText,
  horizonSummaryText,
  strategyCell,
  displayName,
  findHorizon,
  buildAttributionSignalTagOptions,
  WHATIF_TITLE,
  WHATIF_NOTE_FALLBACK,
  WHATIF_SMALL_SAMPLE,
  WHATIF_IN_SAMPLE_MARK,
  WHATIF_TOGGLE,
  WHATIF_DIFF_LABEL,
  WHATIF_BASE_FALLBACK,
  WHATIF_SCENARIO_FALLBACK,
  whatIfColumns,
  whatIfNote,
  whatIfArmLabel,
  whatIfSlotStat,
  whatIfReturnText,
  whatIfDiffText,
  whatIfDiffRate,
  whatIfCountDiff,
} = mod

assert.equal(ATTRIBUTION_DISCLAIMER, '仅研究对照，不构成交易建议，不进入模拟交易计划')
assert.equal(/买入|推荐|alpha/i.test(ATTRIBUTION_DISCLAIMER + RESEARCH_STAT_LABEL), false)
assert.match(RESEARCH_STAT_LABEL, /不是收益证明/)

assert.equal(horizonCellText(null), '数据不足')
assert.equal(
  horizonCellText({ status: 'insufficient', text: '数据不足', returnRate: 0.5 }),
  '数据不足',
)
assert.equal(horizonCellText({ status: 'ok', text: '+1.00%', returnRate: 0.01 }), '+1.00%')
assert.equal(horizonReasonText({ status: 'insufficient', reason: 'calendar_gap' }), '日线之间有未确认的交易日缺口')

assert.equal(strategyCell({ strategyName: '策略甲', strategyId: 's1' }), '策略甲（s1）')
assert.equal(strategyCell({ strategyId: 's1' }), 's1')
assert.equal(strategyCell({}), '默认参数预设（default）')
assert.equal(strategyCell({ strategyId: 'default' }, { default: '我的预设' }), '我的预设（default）')
assert.equal(strategyCell({ strategyName: '快照名', strategyId: 'default' }, { default: '我的预设' }), '快照名（default）')
assert.equal(LARGE_SAMPLE_WARNING, '样本过大，先收窄信号再归因')
assert.deepEqual(buildAttributionSignalTagOptions(['强', 'XS_MOM_TOP']), [
  { value: '强', label: '强' },
  { value: 'XS_MOM_TOP', label: 'XS_MOM_TOP 截面动量' },
])
assert.equal(displayName(''), '—')
assert.equal(displayName('浦发银行'), '浦发银行')

assert.equal(horizonSummaryText({ complete: 0, meanText: '+9.00%' }), '完整样本 0')
assert.match(horizonSummaryText({ complete: 2, meanText: '+1.00%', medianText: '+1.00%' }), /均值 \+1\.00%/)

const row = { horizons: [{ horizon: 3, status: 'ok', text: '+3.00%' }] }
assert.equal(findHorizon(row, 3).text, '+3.00%')
assert.equal(findHorizon(row, 10), null)

assert.equal(WHATIF_TITLE, '假设沙盘')
assert.match(WHATIF_NOTE_FALLBACK, /对照实验，不是买卖指令/)
assert.match(WHATIF_NOTE_FALLBACK, /不写入交易计划/)
assert.match(WHATIF_SMALL_SAMPLE, /不足 8/)
assert.match(WHATIF_SMALL_SAMPLE, /不能当成规律/)
assert.equal(WHATIF_IN_SAMPLE_MARK, '样本内')
assert.equal(WHATIF_TOGGLE, '纳入假设')
assert.equal(WHATIF_DIFF_LABEL, '差额 · 假设减基线')
assert.equal(WHATIF_BASE_FALLBACK, '基线 · 全部命中等权')
assert.equal(WHATIF_SCENARIO_FALLBACK, '假设 · 勾选特征等权')
assert.equal(/推荐买入|下单指令/.test(WHATIF_NOTE_FALLBACK + WHATIF_SMALL_SAMPLE), false)

const cols = whatIfColumns()
assert.deepEqual(cols.map((item) => item.key), ['1', '3', '10', 'toDate'])
assert.equal(cols[0].inSample, true)
assert.equal(whatIfNote(null), WHATIF_NOTE_FALLBACK)
assert.equal(whatIfNote({ note: '对照实验，不是买卖指令。自定义' }), '对照实验，不是买卖指令。自定义')
assert.equal(whatIfArmLabel({ label: '基线 · 全部命中等权' }, WHATIF_BASE_FALLBACK), '基线 · 全部命中等权')
assert.equal(whatIfArmLabel({}, WHATIF_SCENARIO_FALLBACK), WHATIF_SCENARIO_FALLBACK)

const arm = {
  horizons: [{ horizon: 1, complete: 2, mean: 0.03, meanText: '+3.00%' }],
  toDate: { horizon: 0, complete: 0, mean: 0.5, meanText: '+50.00%' },
}
assert.equal(whatIfSlotStat(arm, '1').meanText, '+3.00%')
assert.equal(whatIfReturnText(whatIfSlotStat(arm, '1')), '完整 2 · 等权 +3.00%')
assert.equal(whatIfReturnText(whatIfSlotStat(arm, 'toDate')), '数据不足')
assert.equal(whatIfReturnText({ complete: 0, mean: 0.5, meanText: '+50.00%' }), '数据不足')
assert.equal(
  whatIfDiffText({ complete: 4, mean: -0.005, meanText: '-0.50%' }, { complete: 2, mean: 0.03, meanText: '+3.00%' }),
  '+3.50%',
)
assert.equal(whatIfDiffText({ complete: 0, mean: 1 }, { complete: 2, mean: 0.03 }), '数据不足')
assert.equal(whatIfDiffRate({ complete: 1, mean: null }, { complete: 1, mean: 0.1 }), null)
assert.equal(whatIfCountDiff({ hitCount: 5 }, { hitCount: 2 }), '-3')
assert.equal(whatIfCountDiff({ hitCount: 1 }, { hitCount: 4 }), '+3')
