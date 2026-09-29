/**
 * 我的策略 · 对比观察包：默认不启用，不带定时，条件落在东财自然语言或技术面勾选。
 * Run: node scripts/verify-comparison-observation-presets.mjs
 */
import assert from 'node:assert/strict'
import {
  COMPARISON_OBSERVATION_PRESETS,
  STRATEGY_PRESETS,
  buildComparisonObservationPayload,
  hasActiveTechnicalIndicator,
} from '../src/utils/technicalIndicators.js'

const ALLOWED_QUERY = new Set(['eastmoney_nl', 'technical'])

function testPackShape() {
  assert.ok(
    COMPARISON_OBSERVATION_PRESETS.length >= 4 && COMPARISON_OBSERVATION_PRESETS.length <= 6,
    `expected 4–6 presets, got ${COMPARISON_OBSERVATION_PRESETS.length}`,
  )
  const ids = new Set()
  const names = new Set()
  for (const preset of COMPARISON_OBSERVATION_PRESETS) {
    assert.equal(preset.observationOnly, true, preset.id)
    assert.equal(preset.enable, false, `${preset.id} must ship enable:false`)
    assert.equal(preset.cronExpr, '', `${preset.id} must not ship a cron`)
    assert.ok(ALLOWED_QUERY.has(preset.queryType), preset.id)
    assert.ok(preset.id.startsWith('cmp_'), preset.id)
    assert.ok(String(preset.name).trim(), preset.id)
    assert.ok(!ids.has(preset.id), preset.id)
    assert.ok(!names.has(preset.name), preset.name)
    ids.add(preset.id)
    names.add(preset.name)
    assert.match(preset.description, /对比观察/)
    assert.match(preset.description, /不启用定时/)
    assert.ok(preset.description.length <= 500, `${preset.id} description too long`)
    const payload = buildComparisonObservationPayload(preset)
    assert.equal(payload.enable, false)
    assert.equal(payload.cronExpr, '')
    assert.equal(payload.id, 0)
    assert.equal(payload.name, preset.name)
    assert.equal(payload.keyword, '')
    assert.equal(payload.industry, '')
    assert.ok(payload.pageSize >= 10 && payload.pageSize <= 500)
    if (preset.queryType === 'eastmoney_nl') {
      assert.equal(payload.queryJson, '')
      assert.ok(payload.queryText.includes('；'), preset.id)
      assert.ok(!payload.queryText.includes('"') && !payload.queryText.includes('\\'), preset.id)
      assert.equal(payload.queryText, preset.queryText)
    } else {
      assert.equal(payload.queryText, '')
      const technical = JSON.parse(payload.queryJson)
      assert.equal(hasActiveTechnicalIndicator(technical), true, preset.id)
    }
  }
}

function testBuilderForcesDisable() {
  const poisoned = {
    ...COMPARISON_OBSERVATION_PRESETS[0],
    enable: true,
    cronExpr: '0 35 9 * * 1-5',
  }
  const payload = buildComparisonObservationPayload(poisoned)
  assert.equal(payload.enable, false)
  assert.equal(payload.cronExpr, '')
}

function testDoesNotCloneIceBuyEnable() {
  const iceBuy = STRATEGY_PRESETS.find((p) => p.id === 'ice_buy')
  assert.ok(iceBuy)
  assert.equal(iceBuy.enable, true)
  const iceNames = new Set(STRATEGY_PRESETS.map((p) => p.name))
  for (const preset of COMPARISON_OBSERVATION_PRESETS) {
    assert.equal(iceNames.has(preset.name), false, preset.name)
  }
}

function testTechnicalPacksUseDistinctFlags() {
  const packs = COMPARISON_OBSERVATION_PRESETS.filter((p) => p.queryType === 'technical')
  assert.ok(packs.length >= 2)
  const signatures = packs.map((preset) => {
    const technical = JSON.parse(buildComparisonObservationPayload(preset).queryJson)
    return Object.entries(technical)
      .filter(([, value]) => value === true || (typeof value === 'number' && value > 0))
      .map(([key]) => key)
      .sort()
      .join(',')
  })
  assert.equal(new Set(signatures).size, signatures.length)
}

testPackShape()
testBuilderForcesDisable()
testDoesNotCloneIceBuyEnable()
testTechnicalPacksUseDistinctFlags()
console.log('verify-comparison-observation-presets: ok')
