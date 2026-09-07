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

export type UpcomingTradePlanWindow = {
  status?: string
  reason?: string
  reason_label?: string
  open_window_start?: string
  open_window_end?: string
  freeze_deadline?: string
}

export type UpcomingTradePlanMorning = {
  status?: string
  materialization_status?: string
  freeze_status?: string
  deadline_status?: string
  reason?: string
  reason_label?: string
}

export type UpcomingTradePlanAutomation = {
  mode?: string
  materialization?: string
  approval?: string
  freeze?: string
  materialization_reason?: string
  approval_reason?: string
  freeze_reason?: string
  materialize_time?: string
  approval_time?: string
  freeze_time?: string
  freeze_deadline?: string
}

export type UpcomingTradePlanItem = {
  id?: number
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
  /** Execution Preview（只读；AfterClose 时 limit/volume 可为 0） */
  ref_price: number
  limit_price: number
  target_volume: number
  entry_rule?: string
  intent_status?: string
  /** T-sell / exit_review sell intent reason (from trade_plan_items.reason). */
  reason?: string
  /** Execution write-back (when present on GET plan). */
  filled_volume?: number
  filled_price?: number
  filled_at?: string
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
  window?: UpcomingTradePlanWindow
  morning?: UpcomingTradePlanMorning
  automation?: UpcomingTradePlanAutomation
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
  const windowRaw = planRaw.window as Record<string, unknown> | undefined
  const morningRaw = planRaw.morning as Record<string, unknown> | undefined
  const automationRaw = planRaw.automation as Record<string, unknown> | undefined
  return {
    id: Number(planRaw.id) || 0,
    trade_date: String(planRaw.trade_date || ''),
    plan_version: Number(planRaw.plan_version) || 0,
    status: String(planRaw.status || ''),
    source_session: String(planRaw.source_session || planRaw.sourceSession || ''),
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
    window: windowRaw
      ? {
          status: windowRaw.status ? String(windowRaw.status) : '',
          reason: windowRaw.reason ? String(windowRaw.reason) : '',
          reason_label: windowRaw.reason_label ? String(windowRaw.reason_label) : '',
          open_window_start: windowRaw.open_window_start ? String(windowRaw.open_window_start) : '',
          open_window_end: windowRaw.open_window_end ? String(windowRaw.open_window_end) : '',
          freeze_deadline: windowRaw.freeze_deadline ? String(windowRaw.freeze_deadline) : '',
        }
      : undefined,
    morning: morningRaw
      ? {
          status: morningRaw.status ? String(morningRaw.status) : '',
          materialization_status: morningRaw.materialization_status
            ? String(morningRaw.materialization_status)
            : '',
          freeze_status: morningRaw.freeze_status ? String(morningRaw.freeze_status) : '',
          deadline_status: morningRaw.deadline_status ? String(morningRaw.deadline_status) : '',
          reason: morningRaw.reason ? String(morningRaw.reason) : '',
          reason_label: morningRaw.reason_label ? String(morningRaw.reason_label) : '',
        }
      : undefined,
    automation: automationRaw
      ? {
          mode: automationRaw.mode ? String(automationRaw.mode) : 'MANUAL',
          materialization: automationRaw.materialization ? String(automationRaw.materialization) : '',
          approval: automationRaw.approval ? String(automationRaw.approval) : '',
          freeze: automationRaw.freeze ? String(automationRaw.freeze) : '',
          materialization_reason: automationRaw.materialization_reason
            ? String(automationRaw.materialization_reason)
            : '',
          approval_reason: automationRaw.approval_reason ? String(automationRaw.approval_reason) : '',
          freeze_reason: automationRaw.freeze_reason ? String(automationRaw.freeze_reason) : '',
          materialize_time: automationRaw.materialize_time ? String(automationRaw.materialize_time) : '',
          approval_time: automationRaw.approval_time ? String(automationRaw.approval_time) : '',
          freeze_time: automationRaw.freeze_time ? String(automationRaw.freeze_time) : '',
          freeze_deadline: automationRaw.freeze_deadline ? String(automationRaw.freeze_deadline) : '',
        }
      : undefined,
    items: Array.isArray(planRaw.items)
      ? planRaw.items.map((it: Record<string, unknown>) => ({
          id: Number(it.id) || 0,
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
          ref_price: Number(it.ref_price) || 0,
          limit_price: Number(it.limit_price) || 0,
          target_volume: Number(it.target_volume) || 0,
          entry_rule: it.entry_rule ? String(it.entry_rule) : '',
          intent_status: it.intent_status ? String(it.intent_status) : '',
          reason: it.reason ? String(it.reason) : '',
          filled_volume: Number(it.filled_volume ?? it.filledVolume) || 0,
          filled_price: Number(it.filled_price ?? it.filledPrice) || 0,
          filled_at: it.filled_at ?? it.filledAt ? String(it.filled_at ?? it.filledAt) : '',
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

/** Same-day multi-origin candidate (Phase16.26-B1.2.1). */
export type SameDayCandidate = {
  id: number
  trade_date: string
  status: string
  source: string
  source_session: string
  plan_version: number
  side?: string
  is_frozen?: boolean
  created_at?: string
}

export type SameDayCandidatesResponse = {
  code: number
  ok: boolean
  trade_date: string
  count: number
  candidates: SameDayCandidate[]
  message?: string
}

/** GET /api/tradeplans/same-day-candidates?trade_date= — read-only list; does not change upcoming. */
export async function getSameDayCandidates(tradeDate?: string): Promise<SameDayCandidatesResponse> {
  const q =
    tradeDate != null && String(tradeDate).trim() !== ''
      ? `?trade_date=${encodeURIComponent(String(tradeDate).trim())}`
      : ''
  const res = await fetch(`/api/tradeplans/same-day-candidates${q}`)
  if (!res.ok) {
    throw new Error(`同日计划列表请求失败: HTTP ${res.status}`)
  }
  const body = (await res.json()) as Record<string, unknown>
  const rawList = Array.isArray(body.candidates) ? body.candidates : []
  const candidates: SameDayCandidate[] = rawList.map((row) => {
    const r = (row || {}) as Record<string, unknown>
    return {
      id: Math.trunc(Number(r.id) || 0),
      trade_date: String(r.trade_date || r.tradeDate || ''),
      status: String(r.status || ''),
      source: String(r.source || ''),
      source_session: String(r.source_session || r.sourceSession || ''),
      plan_version: Math.trunc(Number(r.plan_version ?? r.planVersion) || 0),
      side: r.side ? String(r.side) : undefined,
      is_frozen: Boolean(r.is_frozen ?? r.isFrozen),
      created_at: r.created_at ? String(r.created_at) : r.createdAt ? String(r.createdAt) : undefined,
    }
  })
  return {
    code: Number(body.code ?? -1),
    ok: !!body.ok,
    trade_date: String(body.trade_date || body.tradeDate || ''),
    count: Number(body.count ?? candidates.length) || candidates.length,
    candidates,
    message: body.message ? String(body.message) : undefined,
  }
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

/** GET /api/tradeplans/{id}/lifecycle — read-only display timeline (does not mutate DB status). */
export type TradePlanLifecycleView = {
  planId: number
  tradeDate: string
  dbStatus: string
  displayStatus: string
  planCreatedAt: string | null
  approvedAt: string | null
  freezeAt: string | null
  executionStartedAt: string | null
  firstFillAt: string | null
  dataSourceNote: string
}

export async function getTradePlanLifecycle(planId: number): Promise<{
  code: number
  ok: boolean
  lifecycle: TradePlanLifecycleView | null
  message?: string
}> {
  const id = Math.trunc(Number(planId) || 0)
  const res = await fetch(`/api/tradeplans/${id}/lifecycle`)
  if (!res.ok) throw new Error(`生命周期请求失败: HTTP ${res.status}`)
  const body = await res.json()
  const raw = body?.lifecycle || body?.Lifecycle || null
  const ts = (v: unknown) => (v == null || v === '' ? null : String(v))
  return {
    code: Number(body?.code ?? -1),
    ok: !!body?.ok,
    message: body?.message ? String(body.message) : undefined,
    lifecycle: raw
      ? {
          planId: Number(raw.plan_id ?? raw.planId) || id,
          tradeDate: String((raw.trade_date ?? raw.tradeDate) || ''),
          dbStatus: String((raw.db_status ?? raw.dbStatus) || ''),
          displayStatus: String((raw.display_status ?? raw.displayStatus) || 'DRAFT'),
          planCreatedAt: ts(raw.plan_created_at ?? raw.planCreatedAt),
          approvedAt: ts(raw.approved_at ?? raw.approvedAt),
          freezeAt: ts(raw.freeze_at ?? raw.freezeAt),
          executionStartedAt: ts(raw.execution_started_at ?? raw.executionStartedAt),
          firstFillAt: ts(raw.first_fill_at ?? raw.firstFillAt),
          dataSourceNote: String((raw.data_source_note ?? raw.dataSourceNote) || ''),
        }
      : null,
  }
}

/** GET /api/tradeplans/{id}/execution-readiness — E.5 账户语境执行准备观察（只读）. */
export type ExecutionReadinessConflict = {
  stockCode: string
  stockName: string
  side: string
  quantity: number
  message: string
}

export type ExecutionReadinessIssue = {
  code: string
  severity: string
  message: string
}

export type ExecutionReadinessWeight = {
  stockCode: string
  stockName: string
  requiredCash: number
  projectedWeight: number
  weightPct: number
}

export type TradePlanExecutionReadinessView = {
  status: string
  cashEnough: boolean
  availableCash: number
  requiredCash: number
  totalEquity: number
  concentration: string
  conflicts: ExecutionReadinessConflict[]
  issues: ExecutionReadinessIssue[]
  afterExecution: ExecutionReadinessWeight[]
}

export type TradePlanExecutionReadinessResponse = {
  code: number
  ok: boolean
  message?: string
} & TradePlanExecutionReadinessView

function mapExecutionReadinessConflict(raw: Record<string, unknown>): ExecutionReadinessConflict {
  return {
    stockCode: String(raw.stock_code ?? raw.stockCode ?? ''),
    stockName: String(raw.stock_name ?? raw.stockName ?? ''),
    side: String(raw.side ?? ''),
    quantity: Number(raw.quantity ?? 0) || 0,
    message: String(raw.message ?? ''),
  }
}

function mapExecutionReadinessIssue(raw: Record<string, unknown>): ExecutionReadinessIssue {
  return {
    code: String(raw.code ?? ''),
    severity: String(raw.severity ?? ''),
    message: String(raw.message ?? ''),
  }
}

function mapExecutionReadinessWeight(raw: Record<string, unknown>): ExecutionReadinessWeight {
  return {
    stockCode: String(raw.stock_code ?? raw.stockCode ?? ''),
    stockName: String(raw.stock_name ?? raw.stockName ?? ''),
    requiredCash: Number(raw.required_cash ?? raw.requiredCash ?? 0) || 0,
    projectedWeight: Number(raw.projected_weight ?? raw.projectedWeight ?? 0) || 0,
    weightPct: Number(raw.weight_pct ?? raw.weightPct ?? 0) || 0,
  }
}

/** GET /api/tradeplans/{id}/origin — G2.1 read-only plan provenance (Phase14-G2.2 UI). */
export type TradePlanOriginItem = {
  stock_code: string
  plan_id: string
  signal_time: string
  signal_price: string
  signal_tag: string
  source_reason: string
  selection_reason: string
  strategy_name: string
  score: string
}

export type TradePlanOriginResponse = {
  code: number
  ok: boolean
  plan_id?: number
  items: TradePlanOriginItem[]
  message?: string
}

function mapTradePlanOriginItem(raw: Record<string, unknown>): TradePlanOriginItem {
  return {
    stock_code: String(raw.stock_code ?? raw.stockCode ?? ''),
    plan_id: String(raw.plan_id ?? raw.planId ?? ''),
    signal_time: String(raw.signal_time ?? raw.signalTime ?? ''),
    signal_price: String(raw.signal_price ?? raw.signalPrice ?? ''),
    signal_tag: String(raw.signal_tag ?? raw.signalTag ?? ''),
    source_reason: String(raw.source_reason ?? raw.sourceReason ?? ''),
    selection_reason: String(raw.selection_reason ?? raw.selectionReason ?? ''),
    strategy_name: String(raw.strategy_name ?? raw.strategyName ?? ''),
    score: String(raw.score ?? ''),
  }
}

export async function getTradePlanOrigin(planId: number): Promise<TradePlanOriginResponse> {
  const id = Math.trunc(Number(planId) || 0)
  if (id <= 0) {
    throw new Error('plan_id is required')
  }
  const res = await fetch(`/api/tradeplans/${id}/origin`)
  if (!res.ok && res.status !== 404) {
    throw new Error(`计划来源请求失败: HTTP ${res.status}`)
  }
  const body = await res.json()
  const itemsRaw = Array.isArray(body?.items) ? body.items : []
  return {
    code: Number(body?.code ?? -1),
    ok: !!body?.ok,
    plan_id: body?.plan_id != null ? Number(body.plan_id) || undefined : undefined,
    items: itemsRaw.map((it: Record<string, unknown>) => mapTradePlanOriginItem(it || {})),
    message: body?.message ? String(body.message) : '',
  }
}

export async function getTradePlanExecutionReadiness(
  planId: number,
): Promise<TradePlanExecutionReadinessResponse> {
  const id = Math.trunc(Number(planId) || 0)
  const res = await fetch(`/api/tradeplans/${id}/execution-readiness`)
  if (!res.ok && res.status !== 404) {
    throw new Error(`执行准备观察请求失败: HTTP ${res.status}`)
  }
  const body = await res.json()
  const conflictsRaw = Array.isArray(body?.conflicts) ? body.conflicts : []
  const issuesRaw = Array.isArray(body?.issues) ? body.issues : []
  const afterRaw = Array.isArray(body?.after_execution ?? body?.afterExecution)
    ? body.after_execution ?? body.afterExecution
    : []
  return {
    code: Number(body?.code ?? -1),
    ok: !!body?.ok,
    message: body?.message ? String(body.message) : undefined,
    status: String(body?.status ?? ''),
    cashEnough: !!(body?.cash_enough ?? body?.cashEnough),
    availableCash: Number(body?.available_cash ?? body?.availableCash ?? 0) || 0,
    requiredCash: Number(body?.required_cash ?? body?.requiredCash ?? 0) || 0,
    totalEquity: Number(body?.total_equity ?? body?.totalEquity ?? 0) || 0,
    concentration: String(body?.concentration ?? 'NORMAL'),
    conflicts: conflictsRaw.map((c: Record<string, unknown>) => mapExecutionReadinessConflict(c || {})),
    issues: issuesRaw.map((i: Record<string, unknown>) => mapExecutionReadinessIssue(i || {})),
    afterExecution: afterRaw.map((w: Record<string, unknown>) => mapExecutionReadinessWeight(w || {})),
  }
}
