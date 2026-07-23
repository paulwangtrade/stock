/** 明日交易计划只读 HTTP API（Phase6.5.3） */

export const TRADE_PLAN_CODE_OK = 0
export const TRADE_PLAN_CODE_NO_UPCOMING = 40401

export type UpcomingTradePlanRisk = {
  passed: boolean
  reasons: string[]
}

export type UpcomingTradePlanFreeze = {
  is_frozen: boolean
  freeze_at?: string
  freeze_by?: string
  freeze_reason?: string
  approved_at?: string
  approved_by?: string
}

export type UpcomingTradePlanItem = {
  stock_code: string
  stock_name: string
  side: string
  priority: number
  target_amount: number
  status: string
  score: number
  risk_code?: string
  risk_message?: string
  strategy_name?: string
}

export type UpcomingTradePlan = {
  id: number
  trade_date: string
  plan_version: number
  status: string
  source_session: string
  pool_id: number
  risk: UpcomingTradePlanRisk
  freeze: UpcomingTradePlanFreeze
  items: UpcomingTradePlanItem[]
}

export type UpcomingTradePlanResponse = {
  code: number
  ok: boolean
  trade_date: string
  plan: UpcomingTradePlan | null
  message?: string
}

/**
 * GET /api/tradeplans/upcoming?trade_date=
 * - code=0：成功（plan 可能仍需由调用方判断）
 * - code=40401：无 upcoming（空态，非抛错）
 */
export async function getUpcomingTradePlan(tradeDate?: string): Promise<UpcomingTradePlanResponse> {
  const q =
    tradeDate != null && String(tradeDate).trim() !== ''
      ? `?trade_date=${encodeURIComponent(String(tradeDate).trim())}`
      : ''
  const res = await fetch(`/api/tradeplans/upcoming${q}`)
  if (!res.ok) {
    throw new Error(`交易计划请求失败: HTTP ${res.status}`)
  }
  const body = await res.json()
  const code = Number(body?.code ?? -1)
  const planRaw = body?.plan
  const plan =
    planRaw && typeof planRaw === 'object'
      ? {
          id: Number(planRaw.id) || 0,
          trade_date: String(planRaw.trade_date || ''),
          plan_version: Number(planRaw.plan_version) || 0,
          status: String(planRaw.status || ''),
          source_session: String(planRaw.source_session || ''),
          pool_id: Number(planRaw.pool_id) || 0,
          risk: {
            passed: !!planRaw.risk?.passed,
            reasons: Array.isArray(planRaw.risk?.reasons) ? planRaw.risk.reasons.map(String) : [],
          },
          freeze: {
            is_frozen: !!planRaw.freeze?.is_frozen,
            freeze_at: planRaw.freeze?.freeze_at ? String(planRaw.freeze.freeze_at) : '',
            freeze_by: planRaw.freeze?.freeze_by ? String(planRaw.freeze.freeze_by) : '',
            freeze_reason: planRaw.freeze?.freeze_reason ? String(planRaw.freeze.freeze_reason) : '',
            approved_at: planRaw.freeze?.approved_at ? String(planRaw.freeze.approved_at) : '',
            approved_by: planRaw.freeze?.approved_by ? String(planRaw.freeze.approved_by) : '',
          },
          items: Array.isArray(planRaw.items)
            ? planRaw.items.map((it: Record<string, unknown>) => ({
                stock_code: String(it.stock_code || ''),
                stock_name: String(it.stock_name || ''),
                side: String(it.side || ''),
                priority: Number(it.priority) || 0,
                target_amount: Number(it.target_amount) || 0,
                status: String(it.status || ''),
                score: Number(it.score) || 0,
                risk_code: it.risk_code ? String(it.risk_code) : '',
                risk_message: it.risk_message ? String(it.risk_message) : '',
                strategy_name: it.strategy_name ? String(it.strategy_name) : '',
              }))
            : [],
        }
      : null

  return {
    code,
    ok: !!body?.ok,
    trade_date: String(body?.trade_date || tradeDate || ''),
    plan,
    message: body?.message ? String(body.message) : '',
  }
}
