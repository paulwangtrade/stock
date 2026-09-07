/**
 * Phase11-K — GET /api/portfolio/position-state (read-only).
 * Display mapping lives in positionStateDisplay.js (no frontend 新仓推断).
 */
import {
  indexPositionStatesBySymbol,
  mapPositionStateBundle,
} from '../utils/positionStateDisplay.js'

export type PositionStateRow = {
  symbol: string
  positionStatus: string
  positionStatusLabel: string
  isNewPosition: boolean
  totalQty: number
  availableQty: number
  lockedQty: number
  canSell?: boolean
  holdingDays: number
  riskTag: string
  explanation: string
}

export type PositionStateBundle = {
  tradeDate: string
  positions: PositionStateRow[]
  dataSourceNote: string
  disclaimer: string
}

/** GET /api/portfolio/position-state */
export async function getPortfolioPositionState(tradeDate?: string): Promise<PositionStateBundle> {
  const q = tradeDate ? `?trade_date=${encodeURIComponent(tradeDate)}` : ''
  const res = await fetch(`/api/portfolio/position-state${q}`)
  if (!res.ok) {
    throw new Error(`position-state HTTP ${res.status}`)
  }
  const body = await res.json()
  if (!body?.ok) {
    throw new Error(body?.message || 'position-state failed')
  }
  return mapPositionStateBundle(body.position_states) as PositionStateBundle
}

export { indexPositionStatesBySymbol, mapPositionStateBundle }
