/**
 * Phase13 Onboarding MVP — FirstLaunchState + content guards.
 * Run: node scripts/verify-first-launch.mjs
 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import {
  FIRST_LAUNCH_STORAGE_KEY,
  FIRST_LAUNCH_STATE_VERSION,
  defaultFirstLaunchState,
  readFirstLaunchState,
  writeFirstLaunchState,
  shouldShowFirstLaunch,
  markFirstLaunchCompleted,
  markFirstLaunchSkipped,
  resetFirstLaunchState,
  normalizeFirstLaunchState,
} from '../src/utils/firstLaunchState.js'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

function memoryStorage() {
  /** @type {Record<string, string>} */
  const map = {}
  return {
    getItem(k) {
      return Object.prototype.hasOwnProperty.call(map, k) ? map[k] : null
    },
    setItem(k, v) {
      map[k] = String(v)
    },
    removeItem(k) {
      delete map[k]
    },
  }
}

function testFirstLaunch() {
  const s = memoryStorage()
  assert.equal(shouldShowFirstLaunch(s), true, 'empty storage → show')
  const st = readFirstLaunchState(s)
  assert.deepEqual(st, defaultFirstLaunchState())
  assert.equal(st.version, 2)
  assert.equal(st.disclaimerAcceptedAt, null)
  assert.equal(st.completed, false)
  assert.equal(st.skipped, false)
}

function testSecondLaunchAfterComplete() {
  const s = memoryStorage()
  markFirstLaunchCompleted(5, s, { disclaimerAcceptedAt: '2026-08-23T01:00:00.000Z' })
  assert.equal(shouldShowFirstLaunch(s), false, 'completed → hide')
  const st = readFirstLaunchState(s)
  assert.equal(st.completed, true)
  assert.ok(st.completedAt)
  assert.equal(st.disclaimerAcceptedAt, '2026-08-23T01:00:00.000Z')
  assert.equal(shouldShowFirstLaunch(s), false)
}

function testCompleteWritesDisclaimer() {
  const s = memoryStorage()
  markFirstLaunchCompleted(5, s)
  const st = readFirstLaunchState(s)
  assert.equal(st.completed, true)
  assert.ok(st.disclaimerAcceptedAt, 'complete path must record disclaimerAcceptedAt')
}

function testSecondLaunchAfterSkip() {
  const s = memoryStorage()
  markFirstLaunchSkipped(2, s)
  assert.equal(shouldShowFirstLaunch(s), false, 'skipped → hide')
  const st = readFirstLaunchState(s)
  assert.equal(st.skipped, true)
  assert.equal(st.completed, false)
  assert.equal(st.disclaimerAcceptedAt, null, 'skip does not accept disclaimer')
}

function testCorruptSafe() {
  const s = memoryStorage()
  s.setItem(FIRST_LAUNCH_STORAGE_KEY, '{not-json')
  const st = readFirstLaunchState(s)
  assert.equal(st.completed, false)
  assert.equal(shouldShowFirstLaunch(s), true)
}

function testNormalizeMigratesV1() {
  const n = normalizeFirstLaunchState({
    version: 1,
    completed: 1,
    skipped: 0,
    lastStep: '3',
    completedAt: 'x',
  })
  assert.equal(n.version, FIRST_LAUNCH_STATE_VERSION)
  assert.equal(n.completed, true)
  assert.equal(n.lastStep, 3)
  assert.equal(n.completedAt, 'x')
  assert.equal(n.disclaimerAcceptedAt, null)
}

function testReset() {
  const s = memoryStorage()
  markFirstLaunchCompleted(1, s)
  resetFirstLaunchState(s)
  assert.equal(shouldShowFirstLaunch(s), true)
  assert.equal(s.getItem(FIRST_LAUNCH_STORAGE_KEY), null)
}

function testPersistRoundtrip() {
  const s = memoryStorage()
  writeFirstLaunchState(
    {
      version: 2,
      completed: false,
      skipped: false,
      completedAt: null,
      disclaimerAcceptedAt: null,
      lastStep: 4,
    },
    s,
  )
  const st = readFirstLaunchState(s)
  assert.equal(st.lastStep, 4)
  assert.equal(st.version, 2)
  assert.equal(shouldShowFirstLaunch(s), true)
}

function testOnboardingVueGuards() {
  const vuePath = path.join(__dirname, '../src/components/FirstLaunchOnboarding.vue')
  const text = fs.readFileSync(vuePath, 'utf8')
  assert.match(text, /step === 1/, 'step 1 present')
  assert.match(text, /step === 2/, 'step 2 present')
  assert.match(text, /step === 3/, 'step 3 present')
  assert.match(text, /step === 4/, 'step 4 present')
  assert.match(text, /风险免责声明/, 'step 5 risk copy')
  assert.match(text, /disclaimerAccepted/, 'disclaimer checkbox wiring')
  assert.match(text, /disclaimerAcceptedAt/, 'persists disclaimerAcceptedAt')
  assert.match(text, /TOTAL_STEPS = 5/, 'five steps')

  const forbidden = [
    '一键买入',
    '一键卖出',
    'Approve',
    'FreezeTradePlan',
    'OpenBuy',
    'CreatePlanWithItems',
    '修改策略',
    'SetActive',
  ]
  for (const bad of forbidden) {
    assert.equal(text.includes(bad), false, `onboarding must not contain: ${bad}`)
  }
  // No buy/sell primary CTAs
  assert.equal(/\b买入\b/.test(text), false, 'no 买入 CTA')
  assert.equal(/\b卖出\b/.test(text), false, 'no 卖出 CTA')
  // Navigation to trading ops must not use router.push for trade writes
  assert.equal(text.includes('router.push'), false, 'MVP: no route jump trading entries')
  assert.equal(text.includes('productCapabilities'), false, 'no tier/strategy upsell wiring')
}

testFirstLaunch()
testSecondLaunchAfterComplete()
testCompleteWritesDisclaimer()
testSecondLaunchAfterSkip()
testCorruptSafe()
testNormalizeMigratesV1()
testReset()
testPersistRoundtrip()
testOnboardingVueGuards()

console.log('verify-first-launch: PASS')
