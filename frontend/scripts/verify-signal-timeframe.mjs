/**
 * Phase17.4 — Signal timeframe + signalPresets compat smoke tests
 * Run: node frontend/scripts/verify-signal-timeframe.mjs
 */
import assert from 'node:assert/strict'
import {
  ICE_SIGNAL_DESCRIPTOR,
  isIceSignalVisibleOnKlt,
  isSignalCompatibleWithChartTimeframe,
  kltToSignalTimeframe,
} from '../src/utils/signalTimeframe.js'

// --- timeframe matching ---
assert.equal(kltToSignalTimeframe('101'), 'daily')
assert.equal(kltToSignalTimeframe('102'), 'weekly')
assert.equal(kltToSignalTimeframe('103'), 'monthly')
assert.equal(kltToSignalTimeframe('5'), 'intraday')

assert.equal(
  isSignalCompatibleWithChartTimeframe(ICE_SIGNAL_DESCRIPTOR, 'daily'),
  true,
  'daily signal + daily chart → visible',
)
assert.equal(
  isSignalCompatibleWithChartTimeframe(ICE_SIGNAL_DESCRIPTOR, 'weekly'),
  false,
  'daily signal + weekly chart → hidden',
)
assert.equal(isIceSignalVisibleOnKlt('101'), true)
assert.equal(isIceSignalVisibleOnKlt('102'), false)
assert.equal(isIceSignalVisibleOnKlt('103'), false)
assert.equal(isIceSignalVisibleOnKlt('30'), false)

/**
 * Mirrors resolveRawPresetList / resolveRawActivePresetId in signalSettings.js
 * (avoid importing signalSettings — ESM needs .js on all transitive imports).
 */
function resolveRawPresetList(raw) {
  if (!raw || typeof raw !== 'object') return []
  if (Array.isArray(raw.signalPresets) && raw.signalPresets.length) return raw.signalPresets
  if (Array.isArray(raw.screenStrategies) && raw.screenStrategies.length) return raw.screenStrategies
  return []
}

function resolveRawActivePresetId(raw, fallback) {
  if (!raw || typeof raw !== 'object') return fallback
  const fromNew = String(raw.activeSignalPresetId || '').trim()
  if (fromNew) return fromNew
  const fromOld = String(raw.activeScreenStrategyId || '').trim()
  if (fromOld) return fromOld
  return fallback
}

function mirrorAliases(base) {
  return {
    ...base,
    signalPresets: base.screenStrategies,
    activeSignalPresetId: base.activeScreenStrategyId,
  }
}

function mergeLite(raw) {
  const list = resolveRawPresetList(raw).map((item, index) => ({
    id: String(item?.id || '').trim() || `strategy-${index + 1}`,
    name: String(item?.name || '').trim() || `参数预设 ${index + 1}`,
    settings: item?.settings || {},
  }))
  const active = resolveRawActivePresetId(raw, list[0]?.id || 'default')
  const screenStrategies = list.length
    ? list
    : [{ id: 'default', name: '默认参数预设', settings: {} }]
  const activeScreenStrategyId = screenStrategies.some((i) => i.id === active)
    ? active
    : screenStrategies[0].id
  return mirrorAliases({ screenStrategies, activeScreenStrategyId })
}

// legacy: only screenStrategies
const legacy = mergeLite({
  activeScreenStrategyId: 'legacy-a',
  screenStrategies: [{ id: 'legacy-a', name: '旧预设', settings: { iceThreshold: 28 } }],
})
assert.equal(legacy.activeScreenStrategyId, 'legacy-a')
assert.equal(legacy.signalPresets[0].id, 'legacy-a')
assert.equal(legacy.activeSignalPresetId, 'legacy-a')

// prefer signalPresets when present
const presetsFirst = mergeLite({
  activeSignalPresetId: 'p1',
  signalPresets: [{ id: 'p1', name: '新预设', settings: { iceThreshold: 25 } }],
  screenStrategies: [{ id: 'old', name: '应被忽略', settings: { iceThreshold: 40 } }],
})
assert.equal(presetsFirst.activeScreenStrategyId, 'p1')
assert.equal(presetsFirst.screenStrategies[0].settings.iceThreshold, 25)

// dual-write shape
const serialized = JSON.stringify(legacy)
const parsed = JSON.parse(serialized)
assert.ok(Array.isArray(parsed.screenStrategies))
assert.ok(Array.isArray(parsed.signalPresets))
assert.equal(parsed.signalPresets[0].id, parsed.screenStrategies[0].id)

console.log('verify-signal-timeframe: PASS')
