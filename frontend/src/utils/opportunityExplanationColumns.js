/**
 * Phase16.28 — Opportunity table short explanation columns (display-only).
 * Reads OpportunityProjection already loaded for decision badges; no new API.
 */
import { provenanceSourceChipMeta } from './portfolioSourceChip.js'

function str(v) {
  if (v == null) return ''
  return String(v).trim()
}

/** @returns {'strategy'|'watchlist'|'manual'|'unknown'} */
export function resolveOpportunitySourceBucket(projection) {
  if (!projection || typeof projection !== 'object') return 'unknown'
  const pool = str(projection.opportunity?.pool_source).toLowerCase()
  const meta = str(projection.metadata?.source_type).toLowerCase()
  if (pool === 'follow') return 'manual'
  if (pool === 'strategy_run' || pool === 'mixed') return 'strategy'
  if (meta === 'candidate_pool') return 'strategy'
  if (meta === 'research_candidate') return 'strategy'
  if (meta === 'signal_snapshot') return 'strategy'
  // Watchlist plans are rare on stock-screen list; keep bucket for future / watched flows.
  if (pool === 'watchlist' || meta === 'watchlist') return 'watchlist'
  return 'unknown'
}

export function opportunitySourceChipMeta(projection) {
  return provenanceSourceChipMeta(resolveOpportunitySourceBucket(projection))
}

/** Strategy display name from projection (short). */
export function resolveOpportunityStrategyLabel(projection) {
  if (!projection || typeof projection !== 'object') return ''
  const name = str(projection.opportunity?.strategy_name)
  if (name) return name
  const src = str(projection.opportunity?.strategy_source)
  if (!src) return ''
  // Drop @version suffix for table density when present.
  const at = src.indexOf('@')
  return at > 0 ? src.slice(0, at) : src
}

/** One-line selection / discovery reason; never invents long prose. */
export function resolveOpportunityReasonSummary(projection, maxLen = 28) {
  if (!projection || typeof projection !== 'object') return ''
  const sig = projection.signal || {}
  let text = str(sig.trigger_reason)
  if (!text) text = str(sig.signal_tag)
  if (!text) text = str(projection.research?.explain_summary)
  if (!text) return ''
  if (text.length <= maxLen) return text
  return `${text.slice(0, Math.max(1, maxLen - 1))}…`
}

export function formatOpportunityTableCell(text, empty = '—') {
  const s = str(text)
  return s || empty
}

/**
 * Display-only fallback when batch ProjectList misses a snapshot hit
 * (ProjectList is pool-ordered; drawer ProjectOne still resolves signal_snapshot).
 * Keeps table chips/reason aligned with drawer without new API.
 */
export function buildSignalSnapshotDisplayProjection(signalSummary, strategyName = '') {
  if (!signalSummary || typeof signalSummary !== 'object') return null
  const tag = str(signalSummary.tag)
  const reason = str(signalSummary.statusText) || str(signalSummary.trigger_reason) || tag
  if (!tag && !reason) return null
  const name = str(strategyName)
  return {
    signal: {
      present: true,
      signal_tag: tag,
      trigger_reason: reason,
    },
    opportunity: name ? { strategy_name: name } : {},
    metadata: { source_type: 'signal_snapshot' },
  }
}
