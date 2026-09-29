/**
 * 参数预设目录：内置合并、新建弹窗动作、不覆盖 default 旋钮。
 * Run: npx esbuild scripts/verify-signal-presets.mjs --bundle --platform=node --format=esm --outfile=/tmp/verify-signal-presets.mjs && node /tmp/verify-signal-presets.mjs
 */
import assert from 'node:assert/strict'
import {
  BUILTIN_SCREEN_STRATEGIES,
  ICE_POINT_TEMPLATE_ID,
  SIGNAL_PARAM_SECTIONS,
  applyCreatePreset,
  mergeSignalSettings,
  SNAPSHOT_SCAN_OUT_OF_SCOPE_NOTE,
} from '../src/utils/signalSettings.js'
import { SCREEN_SNAPSHOT_SIGNAL_TAGS } from '../src/utils/signalTagConstants.js'

const builtinIds = [
  'default',
  'ext_xsmom_v1',
  'ext_ma_trend_v1',
  'ext_breakout_v1',
  'ext_vol_mom_v1',
  'ext_ma_pullback_v1',
  'ext_meanrev_watch_v1',
]

function testMergeKeepsDefaultKnobsAndAddsBuiltins() {
  const merged = mergeSignalSettings({
    screenStrategies: [
      {
        id: 'default',
        name: '被改名',
        settings: { common: { rsiPeriod: 9 } },
      },
      {
        id: 'strategy-user',
        name: '我的冰点',
        usageNote: '只看强买',
        templateId: ICE_POINT_TEMPLATE_ID,
        settings: { common: { rsiPeriod: 11 } },
      },
    ],
    activeScreenStrategyId: 'strategy-user',
  })
  assert.deepEqual(merged.screenStrategies.map((item) => item.id).slice(0, builtinIds.length), builtinIds)
  const def = merged.screenStrategies.find((item) => item.id === 'default')
  assert.equal(def.name, '默认参数预设')
  assert.equal(def.builtin, true)
  assert.equal(def.settings.common.rsiPeriod, 9)
  assert.match(def.usageNote, /冰点/)
  const user = merged.screenStrategies.find((item) => item.id === 'strategy-user')
  assert.equal(user.builtin, false)
  assert.equal(user.usageNote, '只看强买')
  assert.equal(user.settings.common.rsiPeriod, 11)
  assert.equal(merged.activeScreenStrategyId, 'strategy-user')
  const planned = merged.screenStrategies.find((item) => item.id === 'ext_vol_mom_v1')
  assert.equal(planned.engineStatus, 'planned')
  assert.match(planned.usageNote, /尚未接入/)
  const xsmom = merged.screenStrategies.find((item) => item.id === 'ext_xsmom_v1')
  assert.equal(xsmom.name, '截面动量V1')
  assert.match(xsmom.usageNote, /非买卖指令/)
}

function testCreateIceCopiesKnobs() {
  const base = mergeSignalSettings({
    screenStrategies: [{
      id: 'default',
      name: '默认参数预设',
      settings: { common: { rsiPeriod: 9 } },
    }],
  })
  const before = base.screenStrategies.length
  const created = applyCreatePreset(base, {
    name: '宽松冰点',
    templateId: ICE_POINT_TEMPLATE_ID,
    usageNote: '阈值放宽后的观察',
    copyFromId: 'default',
    id: 'strategy-test-ice',
  })
  assert.equal(created.ok, true)
  assert.equal(created.action, 'created')
  assert.equal(created.strategyId, 'strategy-test-ice')
  assert.equal(created.settings.screenStrategies.length, before + 1)
  const item = created.settings.screenStrategies.find((row) => row.id === 'strategy-test-ice')
  assert.equal(item.templateId, ICE_POINT_TEMPLATE_ID)
  assert.equal(item.usageNote, '阈值放宽后的观察')
  assert.equal(item.builtin, false)
  assert.equal(item.scanKind, 'ice')
  assert.equal(item.settings.common.rsiPeriod, 9)
  assert.equal(created.settings.screenStrategies.find((row) => row.id === 'default').settings.common.rsiPeriod, 9)
}

function testFixedEngineDoesNotDuplicate() {
  const base = mergeSignalSettings({})
  const before = base.screenStrategies.filter((item) => item.id === 'ext_xsmom_v1').length
  const selected = applyCreatePreset(base, {
    name: '再来一个动量',
    templateId: 'ext_xsmom_v1',
    usageNote: '不该写入',
  })
  assert.equal(selected.ok, true)
  assert.equal(selected.action, 'select_builtin')
  assert.equal(selected.message, '已是系统内置')
  assert.equal(selected.strategyId, 'ext_xsmom_v1')
  assert.equal(selected.settings.activeScreenStrategyId, 'ext_xsmom_v1')
  assert.equal(selected.settings.screenStrategies.filter((item) => item.id === 'ext_xsmom_v1').length, before)
  const planned = applyCreatePreset(base, { templateId: 'ext_meanrev_watch_v1' })
  assert.equal(planned.action, 'select_builtin')
  assert.equal(planned.settings.activeScreenStrategyId, 'ext_meanrev_watch_v1')
  assert.equal(planned.settings.screenStrategies.find((item) => item.id === 'ext_meanrev_watch_v1').engineStatus, 'planned')
}

function testIceTagIsSelectable() {
  assert.ok(SCREEN_SNAPSHOT_SIGNAL_TAGS.includes('冰'))
  assert.ok(SCREEN_SNAPSHOT_SIGNAL_TAGS.includes('强'))
  assert.ok(SCREEN_SNAPSHOT_SIGNAL_TAGS.includes('买'))
  // 筛选多选直接映射这份白名单；「冰」没有中文别名，选项就是「冰」。
  const screenLabels = SCREEN_SNAPSHOT_SIGNAL_TAGS.map((tag) => tag)
  assert.ok(screenLabels.includes('冰'))
  const iceSection = SIGNAL_PARAM_SECTIONS.find((item) => item.key === 'ice')
  assert.equal(iceSection.title, '冰')
  assert.equal(iceSection.storeSection, 'common')
  assert.ok(iceSection.fields.some((field) => field.key === 'iceThreshold'))
  assert.ok(iceSection.fields.some((field) => field.key === 'lookback'))
  const common = SIGNAL_PARAM_SECTIONS.find((item) => item.key === 'common')
  assert.equal(common.fields.some((field) => field.key === 'iceThreshold'), false)
}

function testRejectsBlankNameAndListsScope() {
  const rejected = applyCreatePreset(mergeSignalSettings({}), {
    name: '   ',
    templateId: ICE_POINT_TEMPLATE_ID,
  })
  assert.equal(rejected.ok, false)
  assert.match(rejected.message, /名称/)
  assert.match(SNAPSHOT_SCAN_OUT_OF_SCOPE_NOTE, /Dual Thrust/)
  assert.match(SNAPSHOT_SCAN_OUT_OF_SCOPE_NOTE, /Qlib Alpha158/)
  assert.equal(BUILTIN_SCREEN_STRATEGIES.length, builtinIds.length)
}

testIceTagIsSelectable()
testMergeKeepsDefaultKnobsAndAddsBuiltins()
testCreateIceCopiesKnobs()
testFixedEngineDoesNotDuplicate()
testRejectsBlankNameAndListsScope()
console.log('verify-signal-presets: ok')
