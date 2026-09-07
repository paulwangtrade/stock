/** 生产就绪 TradingDayStatus 只读 HTTP API（Phase6.5.4.2.1） */

export const OPS_TD_CODE_OK = 0
/** 与 TradePlans 空态码对齐的兼容处理（Ops 当前主路径用 0+data；保留解析） */
export const OPS_TD_CODE_NOT_FOUND = 40401
export const OPS_TD_CODE_BAD_TRADE_DATE = 40001

export type TradingDayAfterClose = {
  status: string
  last_run?: string
  pool_id?: number
  plan_id?: number
}

export type TradingDayPlan = {
  exists: boolean
  status?: string
  plan_version?: number
  source_session?: string
}

export type TradingDayRisk = {
  passed: boolean
  status: string
  reason?: string
}

export type TradingDayFreeze = {
  is_frozen: boolean
  freeze_at?: string
  freeze_by?: string
}

export type TradingDayMorning = {
  mode: string
}

export type TradingDayExecutionReady = {
  ready: boolean
  guard_status: string
  reason?: string
}

export type TradingDaySchema = {
  registry_version: number
  validation_status: string
}

export type TradingDayCron = {
  after_close_registered: boolean
  morning_registered: boolean
}

export type TradingDayStatus = {
  date: string
  after_close: TradingDayAfterClose
  plan: TradingDayPlan
  risk: TradingDayRisk
  freeze: TradingDayFreeze
  morning: TradingDayMorning
  execution_ready: TradingDayExecutionReady
  schema: TradingDaySchema
  cron: TradingDayCron
}

export type TradingDayStatusResponse = {
  code: number
  ok: boolean
  trade_date: string
  data: TradingDayStatus | null
  message?: string
}

function mapStatus(raw: Record<string, unknown> | null | undefined): TradingDayStatus | null {
  if (!raw || typeof raw !== 'object') return null
  const after = (raw.after_close || {}) as Record<string, unknown>
  const plan = (raw.plan || {}) as Record<string, unknown>
  const risk = (raw.risk || {}) as Record<string, unknown>
  const freeze = (raw.freeze || {}) as Record<string, unknown>
  const morning = (raw.morning || {}) as Record<string, unknown>
  const exec = (raw.execution_ready || {}) as Record<string, unknown>
  const schema = (raw.schema || {}) as Record<string, unknown>
  const cron = (raw.cron || {}) as Record<string, unknown>
  return {
    date: String(raw.date || ''),
    after_close: {
      status: String(after.status || ''),
      last_run: after.last_run ? String(after.last_run) : '',
      pool_id: Number(after.pool_id) || 0,
      plan_id: Number(after.plan_id) || 0,
    },
    plan: {
      exists: !!plan.exists,
      status: plan.status ? String(plan.status) : '',
      plan_version: Number(plan.plan_version) || 0,
      source_session: plan.source_session ? String(plan.source_session) : '',
    },
    risk: {
      passed: !!risk.passed,
      status: String(risk.status || ''),
      reason: risk.reason ? String(risk.reason) : '',
    },
    freeze: {
      is_frozen: !!freeze.is_frozen,
      freeze_at: freeze.freeze_at ? String(freeze.freeze_at) : '',
      freeze_by: freeze.freeze_by ? String(freeze.freeze_by) : '',
    },
    morning: {
      mode: String(morning.mode || ''),
    },
    execution_ready: {
      ready: !!exec.ready,
      guard_status: String(exec.guard_status || ''),
      reason: exec.reason ? String(exec.reason) : '',
    },
    schema: {
      registry_version: Number(schema.registry_version) || 0,
      validation_status: String(schema.validation_status || ''),
    },
    cron: {
      after_close_registered: !!cron.after_close_registered,
      morning_registered: !!cron.morning_registered,
    },
  }
}

/**
 * GET /api/ops/trading_day_status?trade_date=
 * - code=0：成功（data 可含 plan.exists=false）
 * - code=40401：兼容空态（非抛错，data=null）
 * - code=40001：坏日期
 */
export async function getTradingDayStatus(tradeDate?: string): Promise<TradingDayStatusResponse> {
  const q =
    tradeDate != null && String(tradeDate).trim() !== ''
      ? `?trade_date=${encodeURIComponent(String(tradeDate).trim())}`
      : ''
  const res = await fetch(`/api/ops/trading_day_status${q}`)
  if (!res.ok) {
    throw new Error(`生产就绪请求失败: HTTP ${res.status}`)
  }
  const body = await res.json()
  const code = Number(body?.code ?? -1)
  return {
    code,
    ok: !!body?.ok,
    trade_date: String(body?.trade_date || tradeDate || ''),
    data: mapStatus(body?.data),
    message: body?.message ? String(body.message) : '',
  }
}
