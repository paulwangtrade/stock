/**
 * Unit tests: Position origin display (Phase17.5).
 * Run: node frontend/src/utils/positionOriginDisplay.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const mod = await import(pathToFileURL(join(__dir, 'positionOriginDisplay.js')).href)

const {
  resolvePositionOriginBucket,
  buildPositionOriginCard,
  buildPositionOriginCards,
  formatPlanSourceSession,
} = mod

assert.equal(resolvePositionOriginBucket({}, 'after_close'), 'strategy')
assert.equal(resolvePositionOriginBucket({}, 'watchlist'), 'watchlist')
assert.equal(resolvePositionOriginBucket({ strategy: 'follow' }, ''), 'manual')

const card = buildPositionOriginCard(
  {
    planId: 7,
    strategy: 'alpha-v1',
    signal: { tag: '冰点', time: '2026-09-01 14:30:00', price: '12.50', snapshotId: 9 },
    reason: { source: 'strategy_run', selection: 'top score' },
  },
  { sourceSession: 'after_close' },
)
assert.equal(card.sourceChipLabel, 'Strategy')
assert.equal(card.signalPrice, '12.50')
assert.equal(card.signalTime, '2026-09-01 14:30:00')
assert.ok(card.planSourceSession.includes('after_close'))
assert.ok(card.whySummary.includes('Strategy'))

assert.ok(formatPlanSourceSession('morning_rebuild').includes('早盘重建'))

const cards = buildPositionOriginCards(
  {
    trades: [{ planId: 7, fillPrice: 12.8, filledAt: '2026-09-02T09:35:00Z' }],
    origins: [
      {
        planId: 7,
        strategy: 'alpha-v1',
        signal: { tag: '冰点', time: '2026-09-01', price: '12.5', snapshotId: 9 },
        reason: { source: 'strategy_run' },
      },
    ],
  },
  { 7: 'after_close' },
)
assert.equal(cards.length, 1)
assert.equal(cards[0].bucket, 'strategy')
assert.equal(cards[0].planId, 7)

console.log('positionOriginDisplay.test.mjs: ok')
