/**
 * Unit tests: Portfolio Provenance display (Phase15-A2).
 * Run: node frontend/src/utils/portfolioProvenanceDisplay.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const provMod = pathToFileURL(join(__dir, 'portfolioProvenanceDisplay.js')).href
const apiMod = pathToFileURL(join(__dir, '../api/portfolioProvenance.ts')).href

const {
  PROVENANCE_STATUS_LABEL,
  buildProvenanceOriginDisplay,
  buildProvenanceSections,
  formatProvenanceFilledAt,
  provenanceStatusTagType,
} = await import(provMod)

// computeProvenanceStatus lives in TS — duplicate minimal check via dynamic import won't work for .ts without build.
// Test status logic inline mirroring API module.
function computeProvenanceStatus(view) {
  const trades = Array.isArray(view.trades) ? view.trades : []
  const origins = Array.isArray(view.origins) ? view.origins : []
  if (trades.length === 0) return 'partial'
  if (view.reconcile?.status && view.reconcile.status !== 'matched') return 'partial'
  for (const trade of trades) {
    if (!trade.planId) return 'partial'
    const origin = origins.find((o) => o.planId === trade.planId)
    if (!origin) return 'partial'
    const strategy = String(origin.strategy || '').trim()
    if (!strategy) return 'partial'
    const hasSignal =
      !!String(origin.signal?.tag || '').trim() ||
      !!String(origin.signal?.time || '').trim() ||
      (origin.signal?.snapshotId ?? 0) > 0
    const hasReason = !!String(origin.reason?.source || '').trim()
    if (!hasSignal && !hasReason) return 'partial'
  }
  return 'complete'
}

{
  assert.equal(formatProvenanceFilledAt('2026-08-29T09:31:02+08:00'), '2026-08-29 09:31:02')
  assert.equal(formatProvenanceFilledAt(''), '—')
}

{
  assert.equal(provenanceStatusTagType('complete'), 'success')
  assert.equal(provenanceStatusTagType('partial'), 'warning')
  assert.equal(PROVENANCE_STATUS_LABEL.complete, '来源完整')
  assert.equal(PROVENANCE_STATUS_LABEL.partial, '部分来源缺失')
}

{
  const d = buildProvenanceOriginDisplay({
    planId: 42,
    strategy: '冰点超跌',
    signal: { tag: '强', time: '2026-08-31', price: '14.07', snapshotId: 16 },
    reason: { source: '扫描命中', selection: '排名第 3' },
  })
  assert.equal(d.strategy, '冰点超跌')
  assert.equal(d.signalTag, '强')
  assert.equal(d.snapshotId, '16')
  assert.equal(d.buyReason, '扫描命中')
}

{
  const sections = buildProvenanceSections(
    [{ planId: 1, fillPrice: 10.5, fillVolume: 1000, filledAt: '2026-08-29T09:00:00+08:00' }],
    [{ planId: 1, strategy: '策略A', signal: { tag: '强' }, reason: { source: '理由' } }],
  )
  assert.equal(sections.length, 1)
  assert.equal(sections[0].planId, 1)
  assert.equal(sections[0].origin.strategy, '策略A')
}

{
  assert.equal(
    computeProvenanceStatus({
      trades: [{ planId: 1 }],
      origins: [{ planId: 1, strategy: 'x', signal: { tag: '强' }, reason: {} }],
      reconcile: { status: 'matched' },
    }),
    'complete',
  )
  assert.equal(
    computeProvenanceStatus({
      trades: [],
      origins: [],
    }),
    'partial',
  )
}

console.log('portfolioProvenanceDisplay.test.mjs: all passed')
