/**
 * 研究候选策略筛选（纯函数）。
 * Run: node frontend/src/utils/researchCandidateStrategyFilter.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const mod = await import(pathToFileURL(join(__dir, 'researchCandidateStrategyFilter.js')).href)
const {
  researchCandidateStrategyId,
  researchCandidateStrategyLabel,
  strategyNameMapFromPresets,
  buildResearchStrategyFilterOptions,
  filterResearchCandidatesByStrategy,
  researchCandidateStrategyDetailText,
  researchCandidateRowKey,
} = mod

const rows = [
  { id: 'rc:signal:2026-08-18:sh600000', stock_code: 'sh600000', strategy_id: 'ext_xsmom_v1', strategy_name: '截面动量V1', signal_score: 90 },
  { id: 'rc:signal:2026-08-18:sz000001', stock_code: 'sz000001', strategy_id: 'alpha_v1', signal_score: 80 },
  { id: 'rc:signal:2026-08-18:sz300001', stock_code: 'sz300001', signal_score: 70 },
  { id: 'rc:signal:2026-08-18:sh600000', stock_code: 'sh600000', strategy_id: 'beta_v1', strategy_name: '', signal_score: 88 },
]

{
  assert.equal(researchCandidateStrategyId(rows[0]), 'ext_xsmom_v1')
  assert.equal(researchCandidateStrategyId(rows[2]), '')
  assert.equal(researchCandidateStrategyId({ strategyId: '  default  ' }), 'default')
  assert.equal(researchCandidateStrategyId(null), '')
}

{
  const presets = strategyNameMapFromPresets([
    { id: 'default', name: '默认参数预设' },
    { id: 'alpha_v1', name: '不应覆盖空名以外的记录' },
    { id: 'beta_v1', name: '预设Beta' },
  ])
  assert.equal(researchCandidateStrategyLabel(rows[0], presets), '截面动量V1')
  assert.equal(researchCandidateStrategyLabel(rows[1], presets), '不应覆盖空名以外的记录')
  assert.equal(researchCandidateStrategyLabel({ strategy_id: 'beta_v1' }, presets), '预设Beta')
  assert.equal(researchCandidateStrategyLabel({ strategy_id: 'unknown_x' }, presets), 'unknown_x')
  assert.equal(researchCandidateStrategyLabel({ strategy_id: '' }, presets), '未标注')
  assert.equal(researchCandidateStrategyLabel({}, { default: '默认参数预设' }), '未标注')
  assert.equal(researchCandidateStrategyLabel({ strategy_id: 'default' }, presets), '默认参数预设')
  assert.equal(researchCandidateStrategyDetailText(rows[0], presets), '截面动量V1（ext_xsmom_v1）')
  assert.equal(researchCandidateStrategyDetailText(rows[2], presets), '未标注')
  assert.equal(researchCandidateStrategyDetailText({ strategy_id: 'unknown_x' }, presets), 'unknown_x')
}

{
  const options = buildResearchStrategyFilterOptions(rows, { beta_v1: '预设Beta' })
  assert.equal(options[0].label, '全部')
  assert.equal(options[0].value, null)
  assert.equal(options[options.length - 1].label, '未标注')
  assert.equal(options[options.length - 1].value, '')
  assert.deepEqual(
    options.slice(1, -1).map((o) => o.value).sort(),
    ['alpha_v1', 'beta_v1', 'ext_xsmom_v1'],
  )
  assert.equal(options.find((o) => o.value === 'ext_xsmom_v1').label, '截面动量V1')
  assert.equal(options.find((o) => o.value === 'beta_v1').label, '预设Beta')
  const noBlank = buildResearchStrategyFilterOptions([rows[0]], {})
  assert.equal(noBlank.some((o) => o.value === ''), false)
}

{
  assert.equal(filterResearchCandidatesByStrategy(rows, null).length, 4)
  const mom = filterResearchCandidatesByStrategy(rows, 'ext_xsmom_v1')
  assert.equal(mom.length, 1)
  assert.equal(mom[0].stock_code, 'sh600000')
  const unlabeled = filterResearchCandidatesByStrategy(rows, '')
  assert.equal(unlabeled.length, 1)
  assert.equal(unlabeled[0].stock_code, 'sz300001')
  assert.equal(filterResearchCandidatesByStrategy(rows, 'missing').length, 0)
  const both = filterResearchCandidatesByStrategy(rows, 'beta_v1')
  assert.equal(both.length, 1)
  assert.notEqual(researchCandidateRowKey(rows[0]), researchCandidateRowKey(both[0]))
}

console.log('researchCandidateStrategyFilter.test.mjs: all passed')
