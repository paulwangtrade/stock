/** Phase16.27-P2 — source chip mapping selftest (no network). */
import {
  pickLatestPlanIdFromTrades,
  provenanceSourceChipMeta,
  resolveProvenanceSourceBucket,
} from '../utils/portfolioSourceChip.js'

function assert(cond, msg) {
  if (!cond) throw new Error(msg || 'assert failed')
}

assert(resolveProvenanceSourceBucket('after_close') === 'strategy', 'strategy after_close')
assert(resolveProvenanceSourceBucket('watchlist') === 'watchlist', 'watchlist')
assert(resolveProvenanceSourceBucket('t_sell') === 'manual', 'manual t_sell')
assert(resolveProvenanceSourceBucket('') === 'unknown', 'empty → unknown')
assert(provenanceSourceChipMeta('strategy').label === 'Strategy', 'label Strategy')
assert(provenanceSourceChipMeta('watchlist').label === 'Watchlist', 'label Watchlist')
assert(provenanceSourceChipMeta('unknown').label === '未知来源', 'label unknown')

assert(
  pickLatestPlanIdFromTrades([
    { planId: 1, filledAt: '2026-09-01T10:00:00' },
    { planId: 9, filledAt: '2026-09-05T10:00:00' },
  ]) === 9,
  'latest plan id',
)

console.log('portfolioSourceChip.selftest: OK')
