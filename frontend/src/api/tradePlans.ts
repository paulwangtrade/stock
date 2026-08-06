/** 明日交易计划只读 HTTP API（Phase6.5.3 + 6.5.6.14.2 Readiness） */

import {
  buildMaterializeMorningRequestBody,
  normalizeMaterializeMorningResponse,
} from './tradePlansMaterializeMorning.js'

export const TRADE_PLAN_CODE_OK = 0
export const TRADE_PLAN_CODE_NO_UPCOMING = 40401
/** Readiness / upcoming 无计划业务码（与后端 TradePlanCodeNoPlan 一致） */
export const TRADE_PLAN_CODE_NO_PLAN = 40401

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
  generated_at?: string
  pool_id: number
  risk: UpcomingTradePlanRisk
  freeze: UpcomingTradePlanFreeze
  items: UpcomingTradePlanItem[]
}

export type UpcomingTradePlanResponse = {
  code: number
  ok: boolean
  trade_date: string
  next_trading_day?: string
  plan_id?: number
  plan: UpcomingTradePlan | null
  message?: string
}

export type ReadinessFinding = {
  rule_code: string
  code: string
  severity: string
  message: string
  evidence?: Record<string, unknown>
}

export type ExecutionIntentReadinessView = {
  plan_id: number
  trade_date: string
  lifecycle_stage: string
  ready: boolean
  blockers: ReadinessFinding[]
  warnings: ReadinessFinding[]
}

export type TradePlanReadinessResponse = {
  code: number
  ok: boolean
  trade_date: string
  plan_id?: number
  readiness: ExecutionIntentReadinessView | null
  message?: string
}

export type GenerateNextTradePlanResponse = {
  code: number
  ok: boolean
  trigger: string
  actor: string
  source_date: string
  trade_date: string
  candidate_pool_id: number
  plan_id: number
  plan_version: number
  risk_passed: boolean
  failed_step?: string
  message?: string
}

export type TradePlanApproveResponse = {
  code: number
  ok: boolean
  result_code: string
  plan_id?: number
  status?: string
  approved_at?: string
  approved_by?: string
  approved_source?: string
  already_approved?: boolean
  risk_passed?: boolean
  readiness_ready?: boolean
  blockers?: ReadinessFinding[]
  message?: string
}

export type TradePlanFreezeResponse = {
  code: number
  ok: boolean
  result_code: string
  plan_id?: number
  status?: string
  is_frozen?: boolean
  freeze_at?: string
  freeze_by?: string
  freeze_reason?: string
  freeze_source?: string
  approved_at?: string
  approved_by?: string
  already_frozen?: boolean
  risk_passed?: boolean
  readiness_ready?: boolean
  blockers?: ReadinessFinding[]
  message?: string
}

export type TradePlanMaterializeMorningResponse = {
  success: boolean
  plan_id: number
  materialized_items: number
  readiness_ready: boolean
  blockers: ReadinessFinding[]
  code: number
  message?: string
  failed_step?: string
  pricing_stage?: string
}

const UI_ACTOR = 'ui:trade-plan-upcoming'
const UI_SOURCE = 'ui'

/** POST /api/tradeplans/approve：审批 Draft（不改 status，不冻结）. */
export async function approveTradePlan(opts: {
  planId: number
  actor?: string
  source?: string
}): Promise<TradePlanApproveResponse> {
  const res = await fetch('/api/tradeplans/approve', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      plan_id: Math.trunc(Number(opts.planId) || 0),
      actor: String(opts.actor || UI_ACTOR).trim(),
      source: String(opts.source || UI_SOURCE).trim(),
    }),
  })
  if (!res.ok) {
    throw new Error(`批准交易计划失败: HTTP ${res.status}`)
  }
  const body = await res.json()
  return {
    code: Number(body?.code ?? -1),
    ok: !!body?.ok,
    result_code: String(body?.result_code || ''),
    plan_id: body?.plan_id != null ? Number(body.plan_id) || undefined : undefined,
    status: body?.status ? String(body.status) : '',
    approved_at: body?.approved_at ? String(body.approved_at) : '',
    approved_by: body?.approved_by ? String(body.approved_by) : '',
    approved_source: body?.approved_source ? String(body.approved_source) : '',
    already_approved: !!body?.already_approved,
    risk_passed: body?.risk_passed != null ? !!body.risk_passed : undefined,
    readiness_ready: body?.readiness_ready != null ? !!body.readiness_ready : undefined,
    blockers: Array.isArray(body?.blockers)
      ? body.blockers.map((f: Record<string, unknown>) => mapReadinessFinding(f || {}))
      : undefined,
    message: body?.message ? String(body.message) : '',
  }
}

/** POST /api/tradeplans/freeze：冻结已审批 Draft → ready（不触发执行）. */
export async function freezeTradePlan(opts: {
  planId: number
  actor?: string
  reason?: string
  source?: string
}): Promise<TradePlanFreezeResponse> {
  const res = await fetch('/api/tradeplans/freeze', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      plan_id: Math.trunc(Number(opts.planId) || 0),
      actor: String(opts.actor || UI_ACTOR).trim(),
      reason: String(opts.reason || 'ui freeze').trim(),
      source: String(opts.source || UI_SOURCE).trim(),
    }),
  })
  if (!res.ok) {
    throw new Error(`冻结交易计划失败: HTTP ${res.status}`)
  }
  const body = await res.json()
  return {
    code: Number(body?.code ?? -1),
    ok: !!body?.ok,
    result_code: String(body?.result_code || ''),
    plan_id: body?.plan_id != null ? Number(body.plan_id) || undefined : undefined,
    status: body?.status ? String(body.status) : '',
    is_frozen: !!body?.is_frozen,
    freeze_at: body?.freeze_at ? String(body.freeze_at) : '',
    freeze_by: body?.freeze_by ? String(body.freeze_by) : '',
    freeze_reason: body?.freeze_reason ? String(body.freeze_reason) : '',
    freeze_source: body?.freeze_source ? String(body.freeze_source) : '',
    approved_at: body?.approved_at ? String(body.approved_at) : '',
    approved_by: body?.approved_by ? String(body.approved_by) : '',
    already_frozen: !!body?.already_frozen,
    risk_passed: body?.risk_passed != null ? !!body.risk_passed : undefined,
    readiness_ready: body?.readiness_ready != null ? !!body.readiness_ready : undefined,
    blockers: Array.isArray(body?.blockers)
      ? body.blockers.map((f: Record<string, unknown>) => mapReadinessFinding(f || {}))
      : undefined,
    message: body?.message ? String(body.message) : '',
  }
}

/**
 * After generate-next: choose trade_date query for upcoming refresh.
 * When planId is present, prefer the generated trade_date so the new plan is visible
 * (upcoming still uses unchanged server semantics for that date).
 * Without planId, keep the caller's input / default upcoming fallback.
 */
export {
  resolveUpcomingQueryTradeDateAfterGenerate,
  isPreferredGeneratedPlan,
} from './tradePlansPostGenerate.js'

export {
  canShowMorningMaterializeButton,
  inferPricingStageForMaterializeUI,
  formatMaterializeMorningDisplay,
  buildMaterializeMorningRequestBody,
  PRICING_STAGE_AFTER_CLOSE_INTENT,
  PRICING_STAGE_MORNING_MATERIALIZED,
} from './tradePlansMaterializeMorning.js'

function normalizeUpcomingPlanFromBody(planRaw: Record<string, unknown> | null | undefined): UpcomingTradePlan | null {
  if (!planRaw || typeof planRaw !== 'object') return null
  const risk = planRaw.risk as Record<string, unknown> | undefined
  const freeze = planRaw.freeze as Record<string, unknown> | undefined
  return {
    id: Number(planRaw.id) || 0,
    trade_date: String(planRaw.trade_date || ''),
    plan_version: Number(planRaw.plan_version) || 0,
    status: String(planRaw.status || ''),
    source_session: String(planRaw.source_session || ''),
    generated_at: planRaw.generated_at ? String(planRaw.generated_at) : '',
    pool_id: Number(planRaw.pool_id) || 0,
    risk: {
      passed: !!risk?.passed,
      reasons: Array.isArray(risk?.reasons) ? risk.reasons.map(String) : [],
    },
    freeze: {
      is_frozen: !!freeze?.is_frozen,
      freeze_at: freeze?.freeze_at ? String(freeze.freeze_at) : '',
      freeze_by: freeze?.freeze_by ? String(freeze.freeze_by) : '',
      freeze_reason: freeze?.freeze_reason ? String(freeze.freeze_reason) : '',
      approved_at: freeze?.approved_at ? String(freeze.approved_at) : '',
      approved_by: freeze?.approved_by ? String(freeze.approved_by) : '',
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
}

function normalizeUpcomingTradePlanResponse(
  body: Record<string, unknown>,
  fallbackTradeDate?: string,
): UpcomingTradePlanResponse {
  const planRaw = body?.plan
  const plan =
    planRaw && typeof planRaw === 'object'
      ? normalizeUpcomingPlanFromBody(planRaw as Record<string, unknown>)
      : null
  return {
    code: Number(body?.code ?? -1),
    ok: !!body?.ok,
    trade_date: String(body?.trade_date || fallbackTradeDate || ''),
    next_trading_day: body?.next_trading_day ? String(body.next_trading_day) : '',
    plan_id: body?.plan_id != null ? Number(body.plan_id) || undefined : undefined,
    plan,
    message: body?.message ? String(body.message) : '',
  }
}

/** POST /api/tradeplans/materialize-morning：Draft Intent → 早盘物化 → Readiness 重检（不 Approve/Freeze）. */
export async function materializeMorningTradePlan(opts: {
  planId: number
}): Promise<TradePlanMaterializeMorningResponse> {
  const res = await fetch('/api/tradeplans/materialize-morning', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(buildMaterializeMorningRequestBody(opts.planId)),
  })
  if (!res.ok) {
    throw new Error(`早盘物化请求失败: HTTP ${res.status}`)
  }
  const body = await res.json()
  return normalizeMaterializeMorningResponse(body)
}

/** POST /api/tradeplans/generate-next：手动触发 Candidate → Draft → Risk。 */
export async function generateNextTradePlan(opts: {
  actor: string
  sourceDate?: string
  tradeDate?: string
}): Promise<GenerateNextTradePlanResponse> {
  const res = await fetch('/api/tradeplans/generate-next', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      actor: String(opts.actor || '').trim(),
      ...(opts.sourceDate && String(opts.sourceDate).trim()
        ? { source_date: String(opts.sourceDate).trim() }
        : {}),
      ...(opts.tradeDate && String(opts.tradeDate).trim()
        ? { trade_date: String(opts.tradeDate).trim() }
        : {}),
    }),
  })
  if (!res.ok) {
    throw new Error(`生成明日交易计划失败: HTTP ${res.status}`)
  }
  const body = await res.json()
  return {
    code: Number(body?.code ?? -1),
    ok: !!body?.ok,
    trigger: String(body?.trigger || ''),
    actor: String(body?.actor || ''),
    source_date: String(body?.source_date || ''),
    trade_date: String(body?.trade_date || ''),
    candidate_pool_id: Number(body?.candidate_pool_id) || 0,
    plan_id: Number(body?.plan_id) || 0,
    plan_version: Number(body?.plan_version) || 0,
    risk_passed: !!body?.risk_passed,
    failed_step: body?.failed_step ? String(body.failed_step) : '',
    message: body?.message ? String(body.message) : '',
  }
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
  return normalizeUpcomingTradePlanResponse(body, tradeDate)
}

/** GET /api/tradeplans/plan?plan_id=：按 id 精确加载计划（不走 upcoming Frozen 优先）。 */
export async function getTradePlanById(planId: number): Promise<UpcomingTradePlanResponse> {
  const id = Math.trunc(Number(planId) || 0)
  if (id <= 0) {
    throw new Error('plan_id is required')
  }
  const res = await fetch(`/api/tradeplans/plan?plan_id=${encodeURIComponent(String(id))}`)
  if (!res.ok) {
    throw new Error(`交易计划请求失败: HTTP ${res.status}`)
  }
  const body = await res.json()
  return normalizeUpcomingTradePlanResponse(body)
}

function mapReadinessFinding(raw: Record<string, unknown>): ReadinessFinding {
  return {
    rule_code: String(raw.rule_code || ''),
    code: String(raw.code || ''),
    severity: String(raw.severity || ''),
    message: String(raw.message || ''),
    evidence:
      raw.evidence && typeof raw.evidence === 'object'
        ? (raw.evidence as Record<string, unknown>)
        : undefined,
  }
}

/**
 * GET /api/tradeplans/readiness?plan_id=
 * - code=0：成功（ready 另看 readiness.ready）
 * - code=40401：无计划（空态，非抛错）
 */
export async function getTradePlanReadiness(opts?: {
  planId?: number
  tradeDate?: string
}): Promise<TradePlanReadinessResponse> {
  const params = new URLSearchParams()
  if (opts?.planId != null && Number(opts.planId) > 0) {
    params.set('plan_id', String(Math.trunc(Number(opts.planId))))
  }
  if (opts?.tradeDate != null && String(opts.tradeDate).trim() !== '') {
    params.set('trade_date', String(opts.tradeDate).trim())
  }
  const q = params.toString() ? `?${params.toString()}` : ''
  const res = await fetch(`/api/tradeplans/readiness${q}`)
  if (!res.ok) {
    throw new Error(`Readiness 请求失败: HTTP ${res.status}`)
  }
  const body = await res.json()
  const code = Number(body?.code ?? -1)
  const raw = body?.readiness
  let readiness: ExecutionIntentReadinessView | null = null
  if (raw && typeof raw === 'object') {
    const blockers = Array.isArray(raw.blockers)
      ? raw.blockers.map((f: Record<string, unknown>) => mapReadinessFinding(f || {}))
      : []
    const warnings = Array.isArray(raw.warnings)
      ? raw.warnings.map((f: Record<string, unknown>) => mapReadinessFinding(f || {}))
      : []
    // Hard rule: ready == blockers empty (do not trust any QualityGate Passed).
    readiness = {
      plan_id: Number(raw.plan_id) || 0,
      trade_date: String(raw.trade_date || ''),
      lifecycle_stage: String(raw.lifecycle_stage || ''),
      ready: blockers.length === 0,
      blockers,
      warnings,
    }
  }
  return {
    code,
    ok: !!body?.ok,
    trade_date: String(body?.trade_date || opts?.tradeDate || ''),
    plan_id: body?.plan_id != null ? Number(body.plan_id) || undefined : undefined,
    readiness,
    message: body?.message ? String(body.message) : '',
  }
}
