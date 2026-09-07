/**
 * Component contract tests: Outcome Tab (Phase16-D4).
 * Run: node frontend/src/components/explanation/outcomeTab.test.mjs
 */
import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = dirname(fileURLToPath(import.meta.url))
const componentsRoot = join(root, '..')

assert.ok(existsSync(join(componentsRoot, 'OutcomeTabContent.vue')))
assert.ok(existsSync(join(componentsRoot, 'PortfolioProvenanceDrawer.vue')))

const drawerSrc = readFileSync(join(componentsRoot, 'PortfolioProvenanceDrawer.vue'), 'utf8')
const outcomeSrc = readFileSync(join(componentsRoot, 'OutcomeTabContent.vue'), 'utf8')

for (const token of [
  'NTabs',
  'NTabPane',
  'tab="来源"',
  'tab="结果"',
  'OutcomeTabContent',
  'fetchOpportunityOutcome',
  'loadOutcomes',
  'outcomeLoaded',
]) {
  assert.ok(drawerSrc.includes(token), `PortfolioProvenanceDrawer missing ${token}`)
}

for (const token of [
  'ExplanationFieldGrid',
  'buildOutcomeTabView',
  'OUTCOME_DISCLAIMER',
  'n-radio-group',
  'goToPlan',
]) {
  assert.ok(outcomeSrc.includes(token), `OutcomeTabContent missing ${token}`)
}

console.log('outcomeTab.test.mjs: all passed')
