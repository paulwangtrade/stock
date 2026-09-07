import assert from 'node:assert/strict'
import { formatWatchlistPerfReport, estimateAfterSWR, estimateBeforeMs } from '../src/utils/watchlistPerfEstimate.js'

const before = estimateBeforeMs()
const swr = estimateAfterSWR()
assert.ok(swr.timeToFirstListMs < before.timeToFirstListMs)
assert.ok(swr.savedFirstPaintMs >= 300)
assert.ok(swr.savedFirstPaintMs <= 800)

const report = formatWatchlistPerfReport()
assert.equal(report.items.length, 5)
for (const item of report.items) {
  assert.ok(item.afterMs < item.beforeMs || item.deltaMs > 0, item.name)
}

console.log('verify-watchlist-perf-estimate: ok')
console.log(JSON.stringify(report.items.map((i) => ({
  name: i.name,
  before: i.beforeMs,
  after: i.afterMs,
  saved: i.deltaMs,
  note: i.note,
})), null, 2))
