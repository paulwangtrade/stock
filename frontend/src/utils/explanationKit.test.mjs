/**
 * Unit tests: Explanation UI Kit helpers (Phase16-H0).
 * Run: node frontend/src/utils/explanationKit.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'explanationKit.js')).href
const {
  EXPLANATION_READONLY_LABEL,
  EXPLANATION_PIPELINE_CAPTION,
  buildExplanationDrawerTitle,
  normalizePipelineSteps,
  normalizeExplanationFields,
  isExplanationFieldWide,
  buildProvenanceOriginFields,
} = await import(modUrl)

assert.equal(EXPLANATION_READONLY_LABEL, '只读')
assert.ok(EXPLANATION_PIPELINE_CAPTION.includes('发现'))

assert.equal(
  buildExplanationDrawerTitle('腾亚精工', 'sz301125', '解释'),
  '腾亚精工(sz301125) · 解释',
)
assert.equal(buildExplanationDrawerTitle('', 'sz301125', '解释'), 'sz301125 · 解释')
assert.equal(buildExplanationDrawerTitle('腾亚精工', '', '解释'), '腾亚精工 · 解释')
assert.notEqual(buildExplanationDrawerTitle('', 'sz301125', '解释'), 'sz301125(sz301125) · 解释')

const steps = normalizePipelineSteps([
  { key: 'signal', label: '发现', active: true },
  { key: 'opportunity', label: '关注', active: false },
])
assert.equal(steps.length, 2)
assert.equal(steps[0].label, '发现')
assert.equal(steps[0].active, true)

const fields = normalizeExplanationFields([
  { label: '信号标签', value: '突', missing: false, wide: false },
  { label: '发现依据', value: '暂无记录', missing: true, wide: true, source: 'SignalSnapshot' },
])
assert.equal(fields[1].missing, true)
assert.equal(fields[1].source, 'SignalSnapshot')
assert.equal(isExplanationFieldWide(fields[1]), true)

const provFields = buildProvenanceOriginFields(
  {
    strategy: 'trend',
    strategyMissing: false,
    signalTag: '突',
    signalTagMissing: false,
    selectionReason: 'rank=1',
    selectionReasonMissing: false,
  },
  { signalPriceTooltip: 'tip' },
)
assert.equal(provFields.length, 7)
assert.equal(provFields.find((f) => f.key === 'selection_reason')?.value, 'rank=1')

const provFieldsNoSelection = buildProvenanceOriginFields({
  strategyMissing: true,
  selectionReasonMissing: true,
})
assert.equal(provFieldsNoSelection.find((f) => f.key === 'selection_reason'), undefined)

console.log('explanationKit.test.mjs: all passed')
