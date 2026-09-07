/**
 * Exit Review → T-sell draft bridge (Phase14-M1).
 * Builds SellDraftDialog row + default reason from drawer context.
 */
import { exitReasonLabel } from './exitReviewDisplay.js'

export const EXIT_REVIEW_SELL_ACTOR = 'ui:exit-review-sell'

/** Row shape compatible with portfolioSellEntry helpers + SellDraftDialog. */
export function buildExitReviewSellDraftRow({
  stockCode,
  stockName,
  availableQty,
  positionState,
  canSell,
}) {
  return {
    stockCode: String(stockCode || '').trim(),
    stockName: String(stockName || '').trim(),
    availableQty: Math.max(0, Math.trunc(Number(availableQty) || 0)),
    positionState: String(positionState || '').trim(),
    canSell: canSell !== false,
    source: 'paper_sim',
  }
}

/**
 * Default sell draft reason from exit evaluation snapshot.
 * Format: exit_review:{state};{codes};{labels};note={user}
 */
export function buildExitReviewSellReason({
  exitState,
  reasonCodes = [],
  userNote = '',
} = {}) {
  const state = String(exitState || 'NORMAL').trim()
  const codes = (Array.isArray(reasonCodes) ? reasonCodes : [])
    .map((c) => String(c || '').trim())
    .filter(Boolean)
  const labels = codes.map(exitReasonLabel).filter((l) => l && l !== '—')
  const parts = [`exit_review:${state}`]
  if (codes.length) parts.push(codes.join(','))
  if (labels.length) parts.push(labels.join('、'))
  const note = String(userNote || '').trim()
  if (note) parts.push(`note=${note}`)
  return parts.join(';')
}
