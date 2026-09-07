/**
 * Component contract tests: Explanation UI Kit (Phase16-H0).
 * Run: node frontend/src/components/explanation/explanationComponents.test.mjs
 */
import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const componentsDir = __dir

const COMPONENT_FILES = [
  'ExplanationHeader.vue',
  'ExplanationPipeline.vue',
  'ExplanationFieldGrid.vue',
  'ExplanationEmpty.vue',
]

for (const file of COMPONENT_FILES) {
  assert.ok(existsSync(join(componentsDir, file)), `missing component ${file}`)
}

const kitUrl = pathToFileURL(join(__dir, '../../utils/explanationKit.js')).href
const emptyUrl = pathToFileURL(join(__dir, '../../utils/tradePlanOriginDisplay.js')).href
const { EXPLANATION_READONLY_LABEL, buildExplanationDrawerTitle } = await import(kitUrl)
const { ORIGIN_EMPTY } = await import(emptyUrl)

assert.equal(EXPLANATION_READONLY_LABEL, '只读')
assert.equal(ORIGIN_EMPTY, '暂无记录')
assert.equal(
  buildExplanationDrawerTitle('浦发银行', 'sh600000', '解释'),
  '浦发银行（sh600000）· 解释',
)

const projectionDrawer = join(componentsDir, '../OpportunityProjectionDrawer.vue')
const provenanceDrawer = join(componentsDir, '../PortfolioProvenanceDrawer.vue')
assert.ok(existsSync(projectionDrawer))
assert.ok(existsSync(provenanceDrawer))

import { readFileSync } from 'node:fs'
const projSrc = readFileSync(projectionDrawer, 'utf8')
const provSrc = readFileSync(provenanceDrawer, 'utf8')

for (const token of [
  'ExplanationHeader',
  'ExplanationPipeline',
  'ExplanationFieldGrid',
  'buildExplanationDrawerTitle',
]) {
  assert.ok(projSrc.includes(token), `OpportunityProjectionDrawer should import ${token}`)
}

for (const token of [
  'ExplanationHeader',
  'ExplanationFieldGrid',
  'ExplanationEmpty',
  'buildExplanationDrawerTitle',
  'buildProvenanceOriginFields',
]) {
  assert.ok(provSrc.includes(token), `PortfolioProvenanceDrawer should import ${token}`)
}

console.log('explanationComponents.test.mjs: all passed')
