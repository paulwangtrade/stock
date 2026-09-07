/**
 * Unit tests: snapshot banner display (Phase14 UX-ISSUE-002 + Phase16.25).
 * Run: node frontend/src/utils/snapshotDisplay.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const modUrl = pathToFileURL(join(__dir, 'snapshotDisplay.js')).href
const {
  buildNonTradingDaySnapshotTip,
  buildSnapshotHistoryLabel,
  buildSnapshotMetaDisplay,
  formatCompactMonthDay,
  formatSnapshotCreatedAt,
  resolveMarketCutoffDate,
  snapshotSessionCloseLabel,
} = await import(modUrl)

{
  assert.equal(snapshotSessionCloseLabel('close'), '收盘')
  assert.equal(snapshotSessionCloseLabel('midday'), '午盘')
  assert.equal(snapshotSessionCloseLabel(''), '收盘')
}

{
  // Legacy dirty trade_date on weekend → cutoff Friday
  assert.equal(resolveMarketCutoffDate('2026-08-29'), '2026-08-28')
  assert.equal(formatCompactMonthDay('2026-08-29'), '8-29')
}

{
  assert.equal(resolveMarketCutoffDate('2026-08-30'), '2026-08-28')
}

{
  assert.equal(resolveMarketCutoffDate('2026-08-28'), '2026-08-28')
}

{
  const mondayScan = '2026-08-31'
  const createdAt = new Date('2026-08-31T08:00:00+08:00')
  assert.equal(resolveMarketCutoffDate(mondayScan, { createdAt }), '2026-08-28')
}

{
  const createdAt = new Date('2026-09-05T15:36:00+08:00')
  assert.equal(formatSnapshotCreatedAt(createdAt), '2026-09-05 15:36')
}

{
  const tip = buildNonTradingDaySnapshotTip({
    tradeDate: '2026-09-04',
    createdAt: new Date('2026-09-05T15:36:00+08:00'),
  })
  assert.match(tip, /非交易日/)
  assert.equal(
    buildNonTradingDaySnapshotTip({
      tradeDate: '2026-09-04',
      createdAt: new Date('2026-09-04T16:00:00+08:00'),
    }),
    '',
  )
}

{
  assert.equal(
    buildSnapshotHistoryLabel({
      tradeDate: '2026-09-04',
      session: 'close',
      strategyName: '默认',
      hitTotal: 47,
    }),
    '2026-09-04 收盘快照 · 默认 (47只)',
  )
}

{
  const d = buildSnapshotMetaDisplay({
    tradeDate: '2026-09-04',
    session: 'close',
    scannedTotal: 5123,
    hitTotal: 47,
    createdAt: new Date('2026-09-05T15:36:00+08:00'),
  })
  assert.ok(d)
  assert.equal(d.businessLine, '2026-09-04 收盘快照')
  assert.equal(d.marketLine, '业务交易日：2026-09-04 收盘快照')
  assert.equal(d.generatedLine, '生成于 2026-09-05 15:36')
  assert.match(d.nonTradingTip, /非交易日/)
  assert.match(d.statsLine, /5.?123/)
  assert.match(d.statsLine, /47/)
}

{
  const d = buildSnapshotMetaDisplay({
    tradeDate: '2026-08-28',
    session: 'midday',
    scannedTotal: 100,
    hitTotal: 5,
    createdAt: new Date('2026-08-28T12:00:00+08:00'),
  })
  assert.equal(d.marketLine, '业务交易日：2026-08-28 午盘快照')
  assert.equal(d.nonTradingTip, '')
}

{
  assert.equal(buildSnapshotMetaDisplay(null), null)
  assert.equal(buildSnapshotMetaDisplay({ tradeDate: '' }), null)
}

console.log('snapshotDisplay.test.mjs: all passed')
