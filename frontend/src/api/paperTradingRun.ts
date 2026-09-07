/**
 * POST /api/papertrading/run — manual Paper Trading execution (Phase14-A-R1-C/D).
 * Frontend-only; does not alter backend gateway contracts.
 */

export type PaperTradingRunRequest = {
  plan_id: number
  trigger: 'manual'
  actor: string
  trade_date?: string
}

export type PaperTradingRunResultPayload = {
  entry?: string
  status?: string
  message?: string
  filledCount?: number
  filled_count?: number
  ordersTotal?: number
  rejectCount?: number
  skippedItems?: number
  skippedAlready?: number
  decision?: string
  reason?: string
  priceMode?: string
  planId?: number
}

export type PaperTradingRunResponse = {
  ok: boolean
  code: number
  message?: string
  result?: PaperTradingRunResultPayload
  /** UI-only: normalized business error code when request failed or soft-skipped. */
  errorCode?: string
  /** True when HTTP ok but run was idempotent skip (already executed). */
  alreadyExecuted?: boolean
}

export class PaperTradingRunError extends Error {
  code: number
  httpStatus: number
  userMessage: string
  errorCode: string

  constructor(
    message: string,
    code: number,
    httpStatus: number,
    userMessage?: string,
    errorCode?: string,
  ) {
    super(message)
    this.name = 'PaperTradingRunError'
    this.code = code
    this.httpStatus = httpStatus
    this.userMessage = userMessage || message
    this.errorCode = errorCode || 'UNKNOWN'
  }
}

export const PAPER_RUN_ERROR = {
  MISSING_PLAN_ID: 'MISSING_PLAN_ID',
  PLAN_NOT_FROZEN: 'PLAN_NOT_FROZEN',
  PLAN_ALREADY_EXECUTED: 'PLAN_ALREADY_EXECUTED',
} as const

const ALREADY_EXECUTED_STATUSES = new Set([
  'skipped_already_run',
  'skipped_plan_lifecycle',
])

/** Extract business error token from backend message text. */
export function parsePaperRunErrorCode(message?: string, resultStatus?: string): string {
  const status = String(resultStatus || '').trim().toLowerCase()
  if (ALREADY_EXECUTED_STATUSES.has(status)) return PAPER_RUN_ERROR.PLAN_ALREADY_EXECUTED
  const m = String(message || '')
  if (/PLAN_NOT_FROZEN/i.test(m) || /skipped_no_frozen/i.test(m)) {
    return PAPER_RUN_ERROR.PLAN_NOT_FROZEN
  }
  if (/PLAN_ALREADY_EXECUTED/i.test(m) || /already completed/i.test(m) || /skipped_already/i.test(m)) {
    return PAPER_RUN_ERROR.PLAN_ALREADY_EXECUTED
  }
  if (/plan_id/i.test(m) && /invalid|missing|required/i.test(m)) {
    return PAPER_RUN_ERROR.MISSING_PLAN_ID
  }
  return ''
}

export function mapPaperRunUserMessage(errorCode: string, httpStatus?: number, serverMessage?: string): string {
  switch (errorCode) {
    case PAPER_RUN_ERROR.MISSING_PLAN_ID:
      return '缺少有效的 plan_id，无法执行卖出'
    case PAPER_RUN_ERROR.PLAN_NOT_FROZEN:
      return '计划尚未冻结，请先冻结后再执行卖出'
    case PAPER_RUN_ERROR.PLAN_ALREADY_EXECUTED:
      return '该卖出计划已执行过，可在下方查看成交记录'
    default:
      break
  }
  const m = String(serverMessage || '').trim()
  if (httpStatus === 409) {
    if (/frozen/i.test(m) || /PLAN_NOT_FROZEN/i.test(m)) return '计划尚未冻结，请先冻结后再执行卖出'
    if (/session/i.test(m)) return '当前不在可执行窗口，请稍后重试'
    return m || '模拟执行冲突（409）'
  }
  if (httpStatus === 400) return m || '请求参数无效'
  return m || '模拟执行失败'
}

function normalizeRunResult(raw: Record<string, unknown> | undefined): PaperTradingRunResultPayload | undefined {
  if (!raw || typeof raw !== 'object') return undefined
  return {
    ...raw,
    filledCount: Math.trunc(Number(raw.filledCount ?? raw.filled_count) || 0),
    status: raw.status ? String(raw.status) : undefined,
    message: raw.message ? String(raw.message) : undefined,
    planId: raw.planId != null ? Number(raw.planId) : undefined,
  }
}

const DEFAULT_ACTOR = 'ui:tradeplan-sell'

/**
 * Manual Paper Trading run for a frozen plan.
 * POST /api/papertrading/run
 */
export async function runPaperTradingManual(opts: {
  planId: number
  actor?: string
  tradeDate?: string
}): Promise<PaperTradingRunResponse> {
  const plan_id = Math.trunc(Number(opts.planId) || 0)
  const actor = String(opts.actor || DEFAULT_ACTOR).trim()
  if (plan_id <= 0) {
    throw new PaperTradingRunError(
      'plan_id invalid',
      400,
      400,
      mapPaperRunUserMessage(PAPER_RUN_ERROR.MISSING_PLAN_ID),
      PAPER_RUN_ERROR.MISSING_PLAN_ID,
    )
  }
  if (!actor) {
    throw new PaperTradingRunError('actor is required', 400, 400, '缺少操作者标识', 'BAD_REQUEST')
  }

  const body: PaperTradingRunRequest = {
    plan_id,
    trigger: 'manual',
    actor,
  }
  const tradeDate = String(opts.tradeDate || '').trim()
  if (tradeDate) body.trade_date = tradeDate

  const res = await fetch('/api/papertrading/run', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })

  let payload: Record<string, unknown> = {}
  try {
    payload = (await res.json()) as Record<string, unknown>
  } catch {
    payload = {}
  }

  const code = Math.trunc(Number(payload?.code) || res.status)
  const ok = !!payload?.ok
  const message = payload?.message ? String(payload.message) : ''
  const result = normalizeRunResult(payload?.result as Record<string, unknown> | undefined)
  const resultStatus = result?.status || message

  if (!res.ok || !ok) {
    const errorCode =
      parsePaperRunErrorCode(message, resultStatus) ||
      (res.status === 400 && plan_id <= 0 ? PAPER_RUN_ERROR.MISSING_PLAN_ID : '') ||
      'RUN_FAILED'
    const userMessage = mapPaperRunUserMessage(errorCode, res.status, message)
    throw new PaperTradingRunError(message || userMessage, code, res.status, userMessage, errorCode)
  }

  const errorCode = parsePaperRunErrorCode(result?.message || message, resultStatus)
  if (errorCode === PAPER_RUN_ERROR.PLAN_ALREADY_EXECUTED) {
    return {
      ok: true,
      code,
      message,
      result,
      errorCode,
      alreadyExecuted: true,
    }
  }

  return { ok, code, message, result }
}

/** Alias requested by Phase14-A-R1-D spec. */
export async function paperTradingRun(
  planId: number,
  opts?: { actor?: string; tradeDate?: string },
): Promise<PaperTradingRunResponse> {
  return runPaperTradingManual({
    planId,
    actor: opts?.actor,
    tradeDate: opts?.tradeDate,
  })
}
