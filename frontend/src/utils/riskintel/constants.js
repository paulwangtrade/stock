/**
 * Phase8-0 Risk Intelligence constants.
 * Domain: Watchlist Context/Advice only — not backend PlanFilter / ApproveGate / Execution.
 */

/** @type {string} */
export const POLICY_VERSION = 'riskintel-policy-v1'

/** @type {string} */
export const PRODUCER_WATCHLIST_SCAN = 'watchlist_signal_scan'

/**
 * RiskAdvice.category minimal enum (Object Contract §2.2).
 * @enum {string}
 */
export const ADVICE_CATEGORY = Object.freeze({
  MARKET_DISCIPLINE: 'market_discipline',
  POSITION_ASSIST: 'position_assist',
  HOLD_OBSERVE: 'hold_observe',
  NONE: 'none',
})

/**
 * Forbidden field names on RiskContextBundle / RiskAdvice (Object Contract §3).
 * Listed for documentation and future validators — not Action/Execution payloads.
 */
export const FORBIDDEN_FIELDS = Object.freeze([
  'mustSell',
  'forceLiquidate',
  'forceClear',
  'sellRatio',
  'buyRatio',
  'order',
  'orders',
  'orderSide',
  'orderQty',
  'execution',
  'executionId',
  'executionPayload',
  'submitOrder',
  'brokerRequest',
  'brokerOrderId',
  'fillId',
  'fillQty',
])
