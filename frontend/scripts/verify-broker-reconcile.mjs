import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import {
  DIVERGENCE_KINDS,
  countDivergencesByKind,
  mapBrokerReconcileView,
} from '../src/api/brokerReconcileMap.js'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const root = path.resolve(__dirname, '..')

function read(rel) {
  return fs.readFileSync(path.join(root, rel), 'utf8')
}

// --- API client mapping tests ---
assert.deepEqual(DIVERGENCE_KINDS, [
  'missing_order',
  'status_mismatch',
  'filled_qty_mismatch',
  'avg_price_mismatch',
  'cancel_pending_timeout',
  'unknown_timeout',
])

const empty = mapBrokerReconcileView({
  status: 'READY',
  checked_at: '2026-07-26T12:00:00Z',
  total_orders: 0,
  divergence_count: 0,
  divergences: [],
})
assert.equal(empty.status, 'READY')
assert.equal(empty.total_orders, 0)
assert.equal(empty.divergence_count, 0)
assert.equal(empty.by_kind.missing_order, 0)
assert.equal(empty.by_kind.status_mismatch, 0)
assert.equal(empty.by_kind.unknown_timeout, 0)

const sample = mapBrokerReconcileView({
  status: 'ATTENTION',
  checked_at: '2026-07-26T12:00:00Z',
  total_orders: 2,
  divergence_count: 3,
  divergences: [
    { kind: 'missing_order', order_id: 'a', detail: 'gone', severity: 'attention' },
    { kind: 'status_mismatch', order_id: 'b', detail: 'working vs cancelled', severity: 'warn' },
    { kind: 'status_mismatch', order_id: 'c', detail: 'x', severity: 'warn' },
  ],
  observation_error: '',
})
assert.equal(sample.total_orders, 2)
assert.equal(sample.divergence_count, 3)
assert.equal(sample.by_kind.missing_order, 1)
assert.equal(sample.by_kind.status_mismatch, 2)
assert.equal(sample.by_kind.filled_qty_mismatch, 0)
assert.equal(sample.by_kind.cancel_pending_timeout, 0)

const counts = countDivergencesByKind([
  { kind: 'avg_price_mismatch' },
  { kind: 'unknown_timeout' },
  { kind: 'other_ignored' },
])
assert.equal(counts.avg_price_mismatch, 1)
assert.equal(counts.unknown_timeout, 1)
assert.equal(counts.missing_order, 0)
assert.equal(Object.prototype.hasOwnProperty.call(counts, 'other_ignored'), false)

assert.equal(mapBrokerReconcileView(null), null)
assert.equal(mapBrokerReconcileView(undefined), null)

const apiTs = read('src/api/brokerReconcile.ts')
assert.match(apiTs, /GET \/api\/execution\/broker-reconcile/)
assert.match(apiTs, /fetch\('\/api\/execution\/broker-reconcile'\)/)
assert.match(apiTs, /getBrokerReconcile/)
assert.doesNotMatch(apiTs, /function\s+(repair|retry|cancel|submit|execute)/i)
assert.doesNotMatch(apiTs, /fetch\('\/api\/.*\/(repair|retry|cancel|execute)/i)

// --- Page render contract (static template assertions) ---
const vue = read('src/components/BrokerReconcile.vue')
assert.match(vue, /总检查订单数/)
assert.match(vue, /divergence 总数/)
assert.match(vue, /checked_at/)
assert.match(vue, /Divergence 分类/)
for (const kind of DIVERGENCE_KINDS) {
  assert.ok(
    vue.includes('DIVERGENCE_KINDS') || vue.includes(kind),
    `page should cover kind ${kind}`,
  )
}
assert.match(vue, /DIVERGENCE_KINDS/)
assert.match(vue, /getBrokerReconcile/)
assert.match(vue, /只读/)
assert.match(vue, /刷新/)
assert.match(vue, /总检查订单数/)
assert.match(vue, /n-button[^>]*>刷新<\/n-button>/)
// Only refresh action — no write/trade buttons.
assert.equal((vue.match(/<n-button/g) || []).length, 1)
assert.doesNotMatch(vue, /@click=".*(repair|retry|cancel|submit|execute)/i)
assert.doesNotMatch(vue, />\s*(Repair|Retry|Cancel|下单|Approve|Execute)\s*</)

// --- Router + menu ---
const router = read('src/router/router.js')
assert.match(router, /path:\s*'\/broker-reconcile'/)
assert.match(router, /name:\s*'brokerReconcile'/)
assert.match(router, /BrokerReconcile\.vue/)

const app = read('src/App.vue')
assert.match(app, /Broker 对账/)
assert.match(app, /name:\s*'brokerReconcile'/)
assert.match(app, /brokerReconcile/)
assert.match(app, /GitCompareOutline/)
// menu near recovery readiness
const recoveryIdx = app.indexOf("name: 'recoveryReadiness'")
const brokerIdx = app.indexOf("name: 'brokerReconcile'")
assert.ok(recoveryIdx > 0 && brokerIdx > recoveryIdx, 'Broker 对账 menu should follow 恢复就绪')

console.log('verify-broker-reconcile: ok')
