import assert from 'node:assert/strict'
import {
  clearPerformanceMetrics,
  exportPerformanceMetricsJSON,
  getPerformanceMetrics,
  isPerfMetricsEnabled,
  measurePerformance,
  recordPerformanceMetric,
  setPerfMetricsEnabled,
} from '../src/services/performanceMetrics.js'

setPerfMetricsEnabled(true)
clearPerformanceMetrics()
assert.equal(isPerfMetricsEnabled(), true)

const finish = measurePerformance('test.metric', { a: 1 })
await new Promise((r) => setTimeout(r, 5))
finish()
finish() // idempotent
const list = getPerformanceMetrics('test.metric')
assert.equal(list.length, 1)
assert.ok(list[0].durationMs >= 0)

setPerfMetricsEnabled(false)
assert.equal(recordPerformanceMetric('hidden', 10), null)

setPerfMetricsEnabled(true)
clearPerformanceMetrics()
recordPerformanceMetric('export.me', 1.23)
const json = exportPerformanceMetricsJSON()
assert.ok(json.includes('export.me'))

console.log('performanceMetrics: ok')
