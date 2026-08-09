/** Phase13-D: product capability UI wiring (FeatureGate / Explain / Risk / Usage). */

export const PRODUCT_TIER_KEY = 'go-stock.productTier'

export type ProductTier = 'free' | 'pro'

export type FeatureGateDecision = {
  feature: string
  allowed: boolean
  tier: string
  reason: string
}

export type FeatureGateResponse = {
  code: number
  ok: boolean
  tier: string
  features: FeatureGateDecision[]
  message?: string
}

export type ProductUsageResponse = {
  code: number
  ok: boolean
  event?: Record<string, unknown>
  message?: string
}

export function readProductTier(): ProductTier {
  try {
    const v = String(localStorage.getItem(PRODUCT_TIER_KEY) || '').trim().toLowerCase()
    if (v === 'pro') return 'pro'
  } catch {
    /* ignore */
  }
  return 'free'
}

export function writeProductTier(tier: ProductTier) {
  try {
    localStorage.setItem(PRODUCT_TIER_KEY, tier === 'pro' ? 'pro' : 'free')
  } catch {
    /* ignore */
  }
}

function tierQuery(tier?: ProductTier) {
  const t = tier || readProductTier()
  return `tier=${encodeURIComponent(t)}`
}

/** GET /api/product/feature-gate?tier= */
export async function getProductFeatureGate(tier?: ProductTier): Promise<FeatureGateResponse> {
  const res = await fetch(`/api/product/feature-gate?${tierQuery(tier)}`)
  return res.json()
}

export function gateAllowed(resp: FeatureGateResponse | null | undefined, feature: string): boolean {
  const list = resp?.features || []
  const hit = list.find((f) => f.feature === feature)
  return !!hit?.allowed
}

/** GET /api/product/strategy-explanation */
export async function getStrategyExplanation(params: {
  planId?: number
  planItemId?: number
  snapshotId?: string
  includeExit?: boolean
  tier?: ProductTier
}) {
  const q = new URLSearchParams()
  q.set('tier', params.tier || readProductTier())
  if (params.planId) q.set('plan_id', String(params.planId))
  if (params.planItemId) q.set('plan_item_id', String(params.planItemId))
  if (params.snapshotId) q.set('snapshot_id', params.snapshotId)
  if (params.includeExit) q.set('include_exit', '1')
  const res = await fetch(`/api/product/strategy-explanation?${q.toString()}`)
  return res.json()
}

/** GET /api/product/risk-report */
export async function getAdvancedRiskReport(params?: { tradeDate?: string; tier?: ProductTier }) {
  const q = new URLSearchParams()
  q.set('tier', params?.tier || readProductTier())
  if (params?.tradeDate) q.set('trade_date', params.tradeDate)
  const res = await fetch(`/api/product/risk-report?${q.toString()}`)
  return res.json()
}

/** GET /api/product/assistant-context */
export async function getAssistantContext(params?: {
  scene?: string
  tradeDate?: string
  tier?: ProductTier
}) {
  const q = new URLSearchParams()
  q.set('tier', params?.tier || readProductTier())
  if (params?.scene) q.set('scene', params.scene)
  if (params?.tradeDate) q.set('trade_date', params.tradeDate)
  const res = await fetch(`/api/product/assistant-context?${q.toString()}`)
  return res.json()
}

/** POST /api/product/usage — Shell UI opened/viewed telemetry */
export async function recordProductUsage(input: {
  feature: string
  event: 'opened' | 'viewed' | string
  usageKey?: string
  scene?: string
  tier?: ProductTier
  metadata?: Record<string, string>
}): Promise<ProductUsageResponse> {
  const res = await fetch('/api/product/usage', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      tier: input.tier || readProductTier(),
      feature: input.feature,
      event: input.event,
      usage_key: input.usageKey,
      scene: input.scene,
      metadata: input.metadata,
    }),
  })
  return res.json()
}

export type UsageSummaryRow = {
  user_id?: string
  feature: string
  event_type: string
  count: number
  last_used: string
  time_range?: { from?: string; to?: string }
}

/** GET /api/product/usage/summary — User Feature Summary */
export async function getUsageSummary(params?: {
  userId?: string
  tier?: ProductTier
  from?: string
  to?: string
}) {
  const q = new URLSearchParams()
  if (params?.userId) q.set('user_id', params.userId)
  q.set('tier', params?.tier || readProductTier())
  if (params?.from) q.set('from', params.from)
  if (params?.to) q.set('to', params.to)
  const res = await fetch(`/api/product/usage/summary?${q.toString()}`)
  return res.json()
}

/** GET /api/product/usage/count — Feature Usage Count */
export async function getFeatureUsageCount(params: {
  feature: string
  eventType?: string
  userId?: string
  from?: string
  to?: string
}) {
  const q = new URLSearchParams()
  q.set('feature', params.feature)
  if (params.eventType) q.set('event_type', params.eventType)
  if (params.userId) q.set('user_id', params.userId)
  if (params.from) q.set('from', params.from)
  if (params.to) q.set('to', params.to)
  const res = await fetch(`/api/product/usage/count?${q.toString()}`)
  return res.json()
}

/** GET /api/product/usage/analytics — commercial rollup */
export async function getUsageAnalytics(params?: { from?: string; to?: string; userId?: string }) {
  const q = new URLSearchParams()
  if (params?.from) q.set('from', params.from)
  if (params?.to) q.set('to', params.to)
  if (params?.userId) q.set('user_id', params.userId)
  const qs = q.toString()
  const res = await fetch(`/api/product/usage/analytics${qs ? `?${qs}` : ''}`)
  return res.json()
}

/** GET /api/product/plans — Commercial Demo Plan Overview */
export async function getProductPlans(tier?: ProductTier) {
  const res = await fetch(`/api/product/plans?${tierQuery(tier)}`)
  return res.json()
}

/** GET /api/product/feature-comparison */
export async function getFeatureComparison(tier?: ProductTier) {
  const res = await fetch(`/api/product/feature-comparison?${tierQuery(tier)}`)
  return res.json()
}

/** POST /api/product/upgrade-demo — no payment; acknowledges demo tier switch */
export async function postUpgradeDemo(target: 'free' | 'pro' | 'enterprise' | string) {
  const res = await fetch('/api/product/upgrade-demo', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ target }),
  })
  return res.json()
}
