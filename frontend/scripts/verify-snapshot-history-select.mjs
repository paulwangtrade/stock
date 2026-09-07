import assert from 'node:assert/strict'
import {
  pickBestUsableSnapshot,
  buildSnapshotStaleBannerText,
  snapshotAgeDays,
} from '../src/utils/snapshotHistorySelect.js'

const list = [
  { tradeDate: '2026-07-18', session: 'close', status: 'done', hitTotal: 0, createdAt: '2026-07-18T16:00:00+08:00' },
  { tradeDate: '2026-07-17', session: 'close', status: 'done', hitTotal: 42, createdAt: '2026-07-17T16:00:00+08:00' },
  { tradeDate: '2026-07-16', session: 'close', status: 'failed', hitTotal: 99, createdAt: '2026-07-19T10:00:00+08:00' },
  { tradeDate: '2026-07-15', session: 'close', status: 'done', hitTotal: 10, createdAt: '2026-07-15T16:00:00+08:00' },
]

const best = pickBestUsableSnapshot(list)
assert.equal(best?.tradeDate, '2026-07-17', '应跳过最新无命中，选最近有数据的 done')

const onlyEmpty = pickBestUsableSnapshot([
  { tradeDate: '2026-07-18', status: 'done', hitTotal: 0, createdAt: '2026-07-18T16:00:00Z' },
])
assert.equal(onlyEmpty?.tradeDate, '2026-07-18', '无命中时回退最近 done')

const now = new Date('2026-07-19T00:00:00+08:00').getTime()
assert.equal(snapshotAgeDays('2026-07-10T12:00:00+08:00', now), 8)
assert.equal(
  buildSnapshotStaleBannerText('2026-07-10T12:00:00+08:00', 7, now),
  '当前数据更新于 8 天前，建议重新扫描',
)
assert.equal(buildSnapshotStaleBannerText('2026-07-18T12:00:00+08:00', 7, now), '')

console.log('snapshotHistorySelect: ok')
