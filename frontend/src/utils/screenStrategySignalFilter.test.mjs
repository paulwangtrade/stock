/**
 * 机会页：策略切换后信号筛选项与已选标签。
 * Run: node frontend/src/utils/screenStrategySignalFilter.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const { signalTagsForScreenStrategy, reconcileScreenSignalTagSelection, ICE_SCREEN_SIGNAL_TAGS } =
  await import(pathToFileURL(join(__dir, 'screenStrategySignalFilter.js')).href)

const xsmom = { id: 'ext_xsmom_v1', name: '截面动量V1', scanKind: 'xsmom' }
const ice = { id: 'default', name: '默认参数预设', scanKind: 'ice' }
const iceVariant = { id: 'preset-ice-2', name: '冰点变体', scanKind: 'ice' }
const maTrend = { id: 'ext_ma_trend_v1', scanKind: 'ma_trend' }
const planned = { id: 'ext_vol_mom_v1', scanKind: 'vol_mom' }

{
  const tags = signalTagsForScreenStrategy(xsmom)
  assert.deepEqual(tags, ['XS_MOM_TOP'])
  assert.equal(tags.includes('趋'), false)
  assert.equal(tags.includes('强'), false)
}

assert.deepEqual(signalTagsForScreenStrategy(ice), ICE_SCREEN_SIGNAL_TAGS)
assert.deepEqual(signalTagsForScreenStrategy(null), ICE_SCREEN_SIGNAL_TAGS)
assert.deepEqual(signalTagsForScreenStrategy(iceVariant), ICE_SCREEN_SIGNAL_TAGS)
assert.deepEqual(signalTagsForScreenStrategy(maTrend), ['MA_TREND'])
assert.deepEqual(signalTagsForScreenStrategy(planned), [])
assert.deepEqual(signalTagsForScreenStrategy({ id: 'custom-xsmom', scanKind: 'xsmom' }), ['XS_MOM_TOP'])

assert.deepEqual(reconcileScreenSignalTagSelection(['趋', '强'], signalTagsForScreenStrategy(xsmom)), [
  'XS_MOM_TOP',
])
assert.deepEqual(reconcileScreenSignalTagSelection([], signalTagsForScreenStrategy(xsmom)), ['XS_MOM_TOP'])

assert.deepEqual(
  reconcileScreenSignalTagSelection(['强', 'XS_MOM_TOP'], signalTagsForScreenStrategy(ice)),
  ['强'],
)
assert.deepEqual(
  reconcileScreenSignalTagSelection(['趋', '强'], signalTagsForScreenStrategy(iceVariant)),
  ['趋', '强'],
)
assert.deepEqual(reconcileScreenSignalTagSelection([], signalTagsForScreenStrategy(ice)), [])

assert.deepEqual(
  reconcileScreenSignalTagSelection(['XS_MOM_TOP'], signalTagsForScreenStrategy(maTrend)),
  ['MA_TREND'],
)
assert.deepEqual(reconcileScreenSignalTagSelection(['趋', '强'], signalTagsForScreenStrategy(planned)), [])

console.log('screenStrategySignalFilter.test.mjs: all passed')
