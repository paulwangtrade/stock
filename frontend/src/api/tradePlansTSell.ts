/**
 * POST /api/tradeplans/t-sell/draft — T-sell manual sell draft API client (Phase14-A-R1-A).
 * Frontend-only; does not mutate backend contracts.
 */

/** Known backend business error codes (tradeplans_t_sell.go). */
export const T_SELL_DRAFT_ERROR_CODES = {
  insufficient_available: 'insufficient_available',
  cannot_sell: 'cannot_sell',
  invalid_stock: 'invalid_stock',
  invalid_quantity: 'invalid_quantity',
} as const

const ERROR_MESSAGES: Record<string, string> = {
  [T_SELL_DRAFT_ERROR_CODES.insufficient_available]: '可卖数量不足（含 T+1 锁定）',
  [T_SELL_DRAFT_ERROR_CODES.cannot_sell]: '该标的当前不可卖',
  [T_SELL_DRAFT_ERROR_CODES.invalid_stock]: '无此持仓或代码无效',
  [T_SELL_DRAFT_ERROR_CODES.invalid_quantity]: '数量须为正整数',
  INVALID_JSON: '请求格式无效',
  BAD_REQUEST: '请求无效',
}

/** T-sell plans default source_session=t_sell; exit_review for Exit Review flow (Phase14-M1). */
export const T_SELL_SOURCE_SESSION = 't_sell'
export const EXIT_REVIEW_SOURCE_SESSION = 'exit_review'

/** Request body for POST /api/tradeplans/t-sell/draft. */
export type TSellDraftRequest = {
  stock_code: string
  quantity: number
  reason?: string
  /** Optional wire field; omitted → server default trade date. */
  trade_date?: string
  /** Optional wire field; default ui:portfolio-sell when omitted. */
  actor?: string
  /** Optional: t_sell (default) | exit_review (Phase14-M1). */
  source_session?: string
}

/** Legacy camelCase aliases accepted by createTSellDraft for existing UI callers. */
export type TSellDraftRequestCompat = TSellDraftRequest & {
  stockCode?: string
  tradeDate?: string
  sourceSession?: string
}

function normalizeRequest(payload: TSellDraftRequestCompat): TSellDraftRequest {
  const stock_code = String(payload.stock_code ?? payload.stockCode ?? '').trim()
  const trade_date = payload.trade_date ?? payload.tradeDate
  const source_session = payload.source_session ?? payload.sourceSession
  return {
    stock_code,
    quantity: payload.quantity,
    reason: payload.reason,
    trade_date: trade_date?.trim() || undefined,
    actor: payload.actor,
    source_session: source_session?.trim() || undefined,
  }
}

/** Success envelope normalized for UI. */
export type TSellDraftResponse = {
  ok: boolean
  plan_id: number
  trade_date: string
  source_session: string
  status?: string
  side?: string
  item_count?: number
  message?: string
  /** @deprecated Use plan_id — kept for existing UI callers. */
  planId?: number
  /** @deprecated Use trade_date — kept for existing UI callers. */
  tradeDate?: string
}

export class TSellDraftError extends Error {
  code: string
  httpStatus: number
  userMessage: string

  constructor(message: string, code: string, httpStatus: number, userMessage?: string) {
    super(message)
    this.name = 'TSellDraftError'
    this.code = code
    this.httpStatus = httpStatus
    this.userMessage = userMessage || message
  }
}

/** Map backend error code to user-facing Chinese message. */
export function mapTSellDraftErrorMessage(code: string, httpStatus?: number): string {
  const key = String(code || '').trim()
  if (ERROR_MESSAGES[key]) return ERROR_MESSAGES[key]
  if (httpStatus === 404) return ERROR_MESSAGES[T_SELL_DRAFT_ERROR_CODES.invalid_stock]
  if (httpStatus === 409) return '该标的当前不可卖或数量冲突'
  if (httpStatus === 400) return '请求参数无效'
  return key ? `卖出草稿创建失败（${key}）` : '卖出草稿创建失败'
}

function resolveUserMessage(httpStatus: number, code: string, serverMessage?: string): string {
  const mapped = mapTSellDraftErrorMessage(code, httpStatus)
  if (ERROR_MESSAGES[code]) return mapped
  if (serverMessage && httpStatus >= 400) {
    // Prefer mapped Chinese for known HTTP classes; keep server detail as fallback only for unknown codes.
    if (httpStatus === 404 || httpStatus === 409 || httpStatus === 400) return mapped
    return serverMessage
  }
  return mapped
}

function normalizeSuccessBody(body: Record<string, unknown>): TSellDraftResponse {
  const plan_id = Number(body?.plan_id ?? body?.planId) || 0
  const trade_date = String(body?.trade_date ?? body?.tradeDate ?? '')
  return {
    ok: !!body?.ok,
    plan_id,
    trade_date,
    source_session: String(body?.source_session ?? body?.sourceSession ?? T_SELL_SOURCE_SESSION),
    status: body?.status ? String(body.status) : undefined,
    side: body?.side ? String(body.side) : undefined,
    item_count:
      body?.item_count != null || body?.itemCount != null
        ? Number(body?.item_count ?? body?.itemCount) || 0
        : undefined,
    message: body?.message ? String(body.message) : undefined,
    planId: plan_id,
    tradeDate: trade_date,
  }
}

const DEFAULT_ACTOR = 'ui:portfolio-sell'

/**
 * Create a T-sell draft trade plan.
 * POST /api/tradeplans/t-sell/draft
 *
 * @throws {TSellDraftError} HTTP 400 / 404 / 409 with Chinese userMessage
 */
export async function createTSellDraft(payload: TSellDraftRequestCompat): Promise<TSellDraftResponse> {
  const req = normalizeRequest(payload)
  const stock_code = req.stock_code
  const quantity = Math.trunc(Number(req.quantity) || 0)
  const actor = String(req.actor || DEFAULT_ACTOR).trim()

  if (!stock_code) {
    const msg = mapTSellDraftErrorMessage(T_SELL_DRAFT_ERROR_CODES.invalid_stock, 404)
    throw new TSellDraftError(msg, T_SELL_DRAFT_ERROR_CODES.invalid_stock, 404, msg)
  }
  if (quantity < 1) {
    const msg = mapTSellDraftErrorMessage(T_SELL_DRAFT_ERROR_CODES.invalid_quantity, 400)
    throw new TSellDraftError(msg, T_SELL_DRAFT_ERROR_CODES.invalid_quantity, 400, msg)
  }

  const bodyPayload: Record<string, unknown> = {
    stock_code,
    quantity,
    actor,
    reason: String(req.reason || 'manual sell').trim(),
  }
  if (req.trade_date?.trim()) {
    bodyPayload.trade_date = req.trade_date.trim()
  }
  if (req.source_session?.trim()) {
    bodyPayload.source_session = req.source_session.trim()
  }

  const res = await fetch('/api/tradeplans/t-sell/draft', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(bodyPayload),
  })

  let body: Record<string, unknown> = {}
  try {
    body = (await res.json()) as Record<string, unknown>
  } catch {
    body = {}
  }

  if (!res.ok) {
    const code = String(body?.code || 'BAD_REQUEST')
    const serverMessage = body?.message ? String(body.message) : ''
    const userMessage = resolveUserMessage(res.status, code, serverMessage)
    throw new TSellDraftError(
      serverMessage || userMessage,
      code,
      res.status,
      userMessage,
    )
  }

  const out = normalizeSuccessBody(body)
  if (!out.ok || out.plan_id <= 0) {
    const code = String(body?.code || 'BAD_REQUEST')
    const userMessage = resolveUserMessage(res.status, code, out.message)
    throw new TSellDraftError(out.message || userMessage, code, res.status, userMessage)
  }
  return out
}
