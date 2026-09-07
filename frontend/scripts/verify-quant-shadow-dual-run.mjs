/**
 * QuantDecision Phase2-A：js_legacy vs go_engine Shadow Dual Run
 * 1) 先跑：go test ./backend/decision/shadow/ （写出 go_engine JSON）
 * 2) 再跑：
 *    node --import ./scripts/loaders/register-js-ext.mjs scripts/verify-quant-shadow-dual-run.mjs
 */
import assert from 'node:assert/strict'
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { assembleWatchlistDecision } from '../src/utils/quantDecisionAssemble.js'
import {
  runDualDecisionCompare,
  PRODUCER_JS_LEGACY,
  PRODUCER_GO_ENGINE,
  DEFAULT_IGNORE_PATHS,
} from '../src/utils/quantDecisionCompare.js'

const __dir = dirname(fileURLToPath(import.meta.url))
const goJSONPath = join(__dir, '../../backend/decision/shadow/testdata/go_engine_buy_ready.json')
const reportDir = join(__dir, '../../backend/decision/shadow/testdata')
const reportPath = join(reportDir, 'shadow_dual_run_report.json')

/** Go Schema v1 Action 无 type/lines/tooltip；EntryZone 无 JS 扩展字段 */
const SHADOW_IGNORE = [
  ...DEFAULT_IGNORE_PATHS,
  'action.type',
  'action.lines',
  'action.tooltip',
  'action.decisionId',
  'action.actionSource',
  'entryZone.instantText',
  'entryZone.extended',
  'entryZone.tag',
  'entryZone.rangeHigh',
  'regime.source', // js: watchlist_js / go: shadow_go
  'risk.message',  // placeholder 文案可不同
]

const zoneNear = {
  text: '10.00~10.20',
  instantText: '10.10',
  instantPrice: 10.1,
  mode: 'near',
  deferMode: 'same',
  daysAgo: 0,
  note: 'near',
  low: 10,
  high: 10.2,
  extended: false,
}

const jsLegacy = assembleWatchlistDecision({
  entry: {
    code: 'sz000001',
    name: 'PingAnBank',
    tag: '强',
    daysAgo: 0,
    buyPriceRange: zoneNear,
    statusText: 'today-strong',
  },
  checklist: {
    ready: true,
    score: 0.875,
    requiredPassed: true,
    readyThreshold: 0.85,
    items: [{ id: 'buy_signal', label: 'buy_signal', passed: true }],
  },
  plan: {
    ok: true,
    suggestedShares: 1000,
    suggestedAddShares: 1000,
    positionPct: 12.5,
    stopPrice: 9.85,
    reason: 'buy-1000',
  },
  existingVolume: 0,
  marketModeKey: 'level3',
  purpose: 'watchlist',
  tradeDate: '2026-07-21',
  asOf: '2026-07-21T00:00:00.000Z',
})

let goEngine
try {
  goEngine = JSON.parse(readFileSync(goJSONPath, 'utf8'))
} catch (e) {
  console.error('Missing go_engine JSON. Run first:\n  go test ./backend/decision/shadow/')
  process.exit(1)
}

assert.equal(jsLegacy.meta.producer, PRODUCER_JS_LEGACY)
assert.equal(goEngine.meta.producer, PRODUCER_GO_ENGINE)

const report = runDualDecisionCompare({
  baseline: jsLegacy,
  candidate: goEngine,
  baselineProducer: PRODUCER_JS_LEGACY,
  candidateProducer: PRODUCER_GO_ENGINE,
  options: {
    ignorePaths: SHADOW_IGNORE,
    looseEmpty: true,
    harness: 'quant-decision-shadow-dual-run',
    phase: 'Phase2-A',
  },
})

mkdirSync(reportDir, { recursive: true })
writeFileSync(reportPath, JSON.stringify(report, null, 2), 'utf8')

console.log('Phase2-A Shadow Dual Run')
console.log('  js_legacy action:', jsLegacy.action.code, jsLegacy.action.label)
console.log('  go_engine action:', goEngine.action.code, goEngine.action.label)
console.log('  report:', report.summary)
console.log('  bySlice:', Object.fromEntries(
  Object.entries(report.bySlice).map(([k, v]) => [k, v.equal ? 'OK' : `DIFF(${v.diffs.length})`]),
))
console.log('  wrote:', reportPath)

if (!report.equal) {
  console.error('FIELD DIFFS:')
  for (const d of report.diffs) {
    console.error(`  [${d.slice}] ${d.path}:`, JSON.stringify(d.left), '→', JSON.stringify(d.right))
  }
  process.exit(1)
}

// 人工 Action 差异可定位
const mutated = {
  ...goEngine,
  action: { ...goEngine.action, code: 'WAIT_PULLBACK', label: '等回踩', allowDraft: false, side: 'none' },
}
const bad = runDualDecisionCompare({
  baseline: jsLegacy,
  candidate: mutated,
  options: { ignorePaths: SHADOW_IGNORE, looseEmpty: true },
})
assert.equal(bad.equal, false)
assert.equal(bad.bySlice.action.equal, false)
assert.ok(bad.diffs.some((d) => d.path === 'action.code' || d.path === 'action.label'))
console.log('Action diff probe: OK (locatable)')
console.log('quant-shadow-dual-run Phase2-A: passed')
