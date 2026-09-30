/**
 * 多策略对照角色标签：静态映射，未知 id 归入其他观察。
 * Run: node frontend/src/utils/multiStrategyRole.test.mjs
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
  STRATEGY_ROLES,
  STRATEGY_ROLE_BY_ID,
  STRATEGY_ROLE_FOOTER,
  resolveStrategyRole,
} = await import(pathToFileURL(join(__dir, 'multiStrategyRole.js')).href)
const { WIRED_SCAN_ENGINES, buildStrategyCatalog } = await import(
  pathToFileURL(join(__dir, 'multiStrategyCompare.js')).href
)

const SCAN_ROLES = {
  scan_strong_v1: 'entry',
  scan_trend_v1: 'entry',
  scan_reversal_v1: 'entry',
  scan_breakout_v1: 'entry',
  scan_rebound_v1: 'entry',
  scan_ice_buy_v1: 'entry',
  scan_reduce_v1: 'reduce',
  scan_take_profit_v1: 'exit',
  scan_ice_v1: 'other',
}

{
  const wired = WIRED_SCAN_ENGINES.map((item) => item.strategyId).sort()
  assert.deepEqual(wired, Object.keys(SCAN_ROLES).sort())
  for (const [id, roleId] of Object.entries(SCAN_ROLES)) {
    const role = resolveStrategyRole(id)
    assert.equal(role.id, roleId, id)
    assert.equal(role.label, STRATEGY_ROLES[roleId].label)
    assert.equal(STRATEGY_ROLE_BY_ID[id], roleId)
  }
}

{
  assert.equal(resolveStrategyRole('scan_trend_v1').id, 'entry')
  assert.equal(resolveStrategyRole('ext_ma_trend_v1').id, 'confirm')
  assert.equal(resolveStrategyRole('preset:ext_ma_trend_v1').id, 'confirm')
  assert.equal(resolveStrategyRole('scan_breakout_v1').id, 'entry')
  assert.equal(resolveStrategyRole('ext_breakout_v1').id, 'confirm')
}

{
  const confirmIds = [
    'ext_xsmom_v1',
    'preset:ext_xsmom_v1',
    'ext_ma_trend_v1',
    'ext_vol_mom_v1',
    'ext_breakout_v1',
    'ext_ma_pullback_v1',
    'ext_meanrev_watch_v1',
  ]
  for (const id of confirmIds) {
    assert.equal(resolveStrategyRole(id).id, 'confirm', id)
    assert.equal(resolveStrategyRole(id).label, '趋势确认')
  }
}

{
  const catalog = buildStrategyCatalog({ settings: null, stockStrategies: [] })
  const preset = catalog.find((item) => item.strategyId === 'preset:default')
  assert.ok(preset)
  const role = resolveStrategyRole(preset.strategyId, preset)
  assert.equal(role.id, 'entry')
  assert.equal(role.label, '入场')
  assert.equal(role.tagType, 'success')
}

{
  const left = resolveStrategyRole('preset:user-ice-a', {
    kind: 'scan_preset',
    scanKind: 'ice',
    templateId: 'ice_point',
  })
  const right = resolveStrategyRole('preset:user-ice-b', {
    kind: 'scan_preset',
    scanKind: 'ice',
    templateId: 'ice_point',
  })
  assert.equal(left.id, 'entry')
  assert.equal(right.id, left.id)
  const unlabeled = resolveStrategyRole('preset:custom-knobs', { kind: 'scan_preset' })
  assert.equal(unlabeled.id, 'entry')
  const trend = resolveStrategyRole('preset:user-trend', {
    kind: 'scan_preset',
    scanKind: 'ma_trend',
    templateId: 'ext_ma_trend_v1',
  })
  assert.equal(trend.id, 'confirm')
  assert.equal(trend.label, '趋势确认')
}

{
  assert.equal(resolveStrategyRole('scan_not_in_repo').id, 'other')
  assert.equal(resolveStrategyRole('scan_not_in_repo').label, '其他观察')
  assert.equal(resolveStrategyRole('pack:ice_buy', { kind: 'unwired' }).id, 'other')
  assert.equal(resolveStrategyRole('ss:7', { kind: 'unwired' }).id, 'other')
  assert.equal(resolveStrategyRole('').id, 'other')
  assert.equal(resolveStrategyRole('preset:future', { kind: 'scan_preset', scanKind: 'dual_thrust' }).id, 'other')
}

{
  const labels = Object.values(STRATEGY_ROLES).map((item) => item.label)
  assert.deepEqual(labels, ['入场', '趋势确认', '减仓防守', '止盈清仓', '其他观察'])
  for (const role of Object.values(STRATEGY_ROLES)) {
    assert.match(role.label, /\p{Script=Han}/u)
    assert.ok(role.tagType)
    assert.equal(Object.hasOwn(role, 'icon'), false)
  }
  assert.match(STRATEGY_ROLE_FOOTER, /设计意图/)
  assert.match(STRATEGY_ROLE_FOOTER, /命中不等于交易指令/)
  assert.match(STRATEGY_ROLE_FOOTER, /不按角色自动打分/)
}

{
  const vue = readFileSync(join(__dir, '../components/MultiStrategyComparePanel.vue'), 'utf8')
  assert.ok(vue.includes('STRATEGY_ROLE_FOOTER'))
  assert.ok(vue.includes('resolveStrategyRole'))
  assert.ok(vue.includes('NTag'))
  assert.ok(vue.includes('<n-tag'))
  assert.equal(vue.includes('TradePlan'), false)
  assert.equal(vue.includes('signalScore'), false)
}

console.log('multiStrategyRole.test.mjs ok')
