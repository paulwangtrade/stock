/**
 * Phase11-K PositionState display helpers (read-only).
 * Labels come ONLY from backend `state` / `is_new_position` — never derive 新仓 from available_qty.
 */

export const POSITION_STATE = {
  S0: 'S0_NO_POSITION',
  S1: 'S1_NEW_LOCKED',
  S2: 'S2_AVAILABLE',
  S3: 'S3_PARTIAL_LOCKED',
  S4: 'S4_REDUCED_AVAILABLE',
}

/** User-facing label for PositionState.state code. */
export function positionStateLabel(state) {
  switch (String(state || '').trim()) {
    case POSITION_STATE.S0:
      return '无持仓'
    case POSITION_STATE.S1:
      return '新建仓(T+1锁定)'
    case POSITION_STATE.S2:
      return '正常持仓'
    case POSITION_STATE.S3:
      return '今日加仓锁定'
    case POSITION_STATE.S4:
      return '减仓后持有'
    default:
      return state ? String(state) : '—'
  }
}

/** Naive tag type for state (display only). */
export function positionStateTagType(state) {
  switch (String(state || '').trim()) {
    case POSITION_STATE.S1:
      return 'warning'
    case POSITION_STATE.S3:
      return 'info'
    case POSITION_STATE.S4:
      return 'success'
    case POSITION_STATE.S2:
      return 'success'
    case POSITION_STATE.S0:
      return 'default'
    default:
      return 'default'
  }
}

/**
 * Normalize one PositionState row from API (home bundle.positions or monitor/API list).
 * Does not invent is_new_position from quantities.
 * Display totalQty = availableQty + lockedQty (UI semantics; does not change backend calculator).
 */
export function mapPositionStateRow(raw) {
  if (!raw || typeof raw !== 'object') return null
  const state = String(raw.state || '').trim()
  const availableQty = Number(raw.available_qty ?? raw.availableQty ?? 0) || 0
  const lockedQty = Number(raw.locked_qty ?? raw.lockedQty ?? 0) || 0
  return {
    symbol: String(raw.symbol || raw.stock_code || raw.stockCode || '').trim(),
    positionStatus: state,
    positionStatusLabel: positionStateLabel(state),
    isNewPosition: !!raw.is_new_position,
    availableQty,
    lockedQty,
    /** UI 持仓数量：可卖 + T+1锁定，避免「可卖=持仓」误解 */
    totalQty: availableQty + lockedQty,
    canSell: raw.can_sell != null ? !!raw.can_sell : undefined,
    holdingDays: Number(raw.holding_days ?? raw.holdingDays ?? 0) || 0,
    riskTag: String(raw.risk_tag || raw.riskTag || ''),
    explanation: String(raw.explanation || ''),
  }
}

/** Display helper: total = available + locked (null-safe). */
export function displayTotalQty(availableQty, lockedQty) {
  if (availableQty == null && lockedQty == null) return null
  return (Number(availableQty) || 0) + (Number(lockedQty) || 0)
}

/** Bundle from GET home.position_states or GET /api/portfolio/position-state. */
export function mapPositionStateBundle(raw) {
  if (!raw || typeof raw !== 'object') {
    return { tradeDate: '', positions: [], dataSourceNote: '', disclaimer: '' }
  }
  const list = Array.isArray(raw.positions) ? raw.positions : Array.isArray(raw) ? raw : []
  return {
    tradeDate: String(raw.trade_date || raw.tradeDate || ''),
    positions: list.map(mapPositionStateRow).filter((p) => p && p.symbol),
    dataSourceNote: String(raw.data_source_note || raw.dataSourceNote || ''),
    disclaimer: String(raw.disclaimer || ''),
  }
}

/** Lowercase code → row map for join. */
export function indexPositionStatesBySymbol(positions) {
  const map = Object.create(null)
  for (const p of positions || []) {
    if (!p?.symbol) continue
    map[String(p.symbol).toLowerCase()] = p
  }
  return map
}

export function lookupPositionState(bySymbol, code) {
  if (!bySymbol || !code) return null
  return bySymbol[String(code).toLowerCase()] || null
}
