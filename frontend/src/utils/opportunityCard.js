/**
 * Phase12-D2-B OpportunityCard adapter (read-only).
 * Source: decision_summary.opportunity_attention. Never treat attention.stock_code as the candidate.
 */
import { toStockDisplay, looksLikeInternalStockCode, UNKNOWN_STOCK_NAME } from './stockDisplay.js'

function trimStr(v) {
  if (v == null) return ''
  return String(v).trim()
}

function finiteScore(v) {
  const n = Number(v)
  return Number.isFinite(n) ? n : null
}

function normalizeCode(raw) {
  return trimStr(raw).toLowerCase()
}

function pickHintName(raw) {
  const n = trimStr(raw)
  if (!n) return ''
  if (looksLikeInternalStockCode(n)) return ''
  if (/^\d{6}\.(SH|SZ|BJ|HK)$/i.test(n)) return ''
  return n
}

function addNameHint(map, code, name) {
  const key = normalizeCode(code)
  const n = pickHintName(name)
  if (!key || !n || map[key]) return
  map[key] = n
}

/** Side-join names from the same home payload. Incomplete by design; never invent names. */
export function collectOpportunityNameHints(raw) {
  const map = Object.create(null)
  if (!raw || typeof raw !== 'object') return map

  const focus = raw.daily_summary?.position_attention
  if (Array.isArray(focus)) {
    for (const it of focus) {
      addNameHint(map, it?.stock_code || it?.stockCode, it?.stock_name || it?.stockName)
    }
  }

  const items = raw.daily_attention?.items
  if (Array.isArray(items)) {
    for (const it of items) {
      addNameHint(map, it?.stock_code || it?.stockCode, it?.stock_name || it?.stockName)
    }
  }
  return map
}

export function opportunityCardUserLabel(userAction) {
  const s = String(userAction || '').toUpperCase()
  if (s === 'REVIEW') return '建议研究'
  return '关注'
}

function toSide(code, score, nameByCode, explicitName = '') {
  const key = normalizeCode(code)
  if (!key) return null
  const name = pickHintName(explicitName) || pickHintName(nameByCode?.[key])
  const display = toStockDisplay({ stock_code: key, stock_name: name })
  if (!display) return null
  return {
    code: display.stock_code || key,
    name: display.name || '',
    score,
    display,
  }
}

/**
 * @param {object|null} opp opportunity_attention JSON
 * @param {Record<string, string>} [nameByCode]
 * @param {number} [limit] Phase16.19-B3 home Top N (default 10)
 */
export function toOpportunityCards(opp, nameByCode = {}, limit = 10) {
  if (!opp || typeof opp !== 'object') return []
  const candidateScore = finiteScore(opp.candidate_score ?? opp.candidateScore)
  const candidateCode = trimStr(opp.candidate_code || opp.candidateCode)
  if (!candidateCode || candidateScore == null) return []

  const highlights = Array.isArray(opp.highlights) ? opp.highlights : []
  const max = Math.max(0, Number(limit) || 0)
  const label = opportunityCardUserLabel(opp.user_action || opp.userAction)
  const candidateExplicitName = pickHintName(opp.candidate_name || opp.candidateName)
  const out = []

  for (const h of highlights) {
    if (out.length >= max) break
    const holdingScore = finiteScore(h?.holding_score ?? h?.holdingScore)
    const holdingCode = trimStr(h?.holding_code || h?.holdingCode)
    if (!holdingCode || holdingScore == null) continue

    const candidate_stock = toSide(candidateCode, candidateScore, nameByCode, candidateExplicitName)
    const holding_stock = toSide(holdingCode, holdingScore, nameByCode)
    if (!candidate_stock || !holding_stock) continue

    out.push({
      kind: 'opportunity_compare',
      candidate_stock,
      holding_stock,
      candidate_score: candidateScore,
      holding_score: holdingScore,
      score_gap: candidateScore - holdingScore,
      score_kind: 'investment_quality',
      user_label: label,
    })
  }
  return out
}

export { UNKNOWN_STOCK_NAME }
