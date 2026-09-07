/** Phase13-B1 Strategy Intent HTTP API（只读 + 手工生命周期写；无 AI / Promote） */

export type StrategySchemaRef = {
  unbound: boolean
  strategy_id?: string
  revision?: string
  params_hash?: string
  note?: string
}

export type StrategyIntent = {
  id: string
  schema_version: string
  candidate_id: string
  strategy_schema_ref: StrategySchemaRef
  schema_revision?: string
  intent_type: string
  conditions: Record<string, unknown>
  action: { verb: string; text?: string; side_hint?: string; entry_session?: string }
  risk_constraints: Record<string, unknown>
  status: string
  summary?: string
  explain_ref?: string
  promoted_pool_id: null
  trade_plan_id: null
  created_at: string
  updated_at: string
}

export type StrategyIntentListItem = {
  id: string
  candidate_id: string
  status: string
  intent_type: string
  summary?: string
  schema_revision?: string
  strategy_id?: string
  unbound: boolean
  updated_at: string
}

export type StrategyIntentListResponse = {
  items: StrategyIntentListItem[]
  message?: string
}

export type StrategyIntentWriteResult = {
  intent: StrategyIntent
  message?: string
}

export type CreateIntentPayload = {
  candidate_id: string
  explain_ref?: string
  strategy_schema_ref: StrategySchemaRef
  schema_revision?: string
  intent_type?: string
  summary?: string
  conditions?: Record<string, unknown>
  action: { verb: string; text?: string }
  risk_constraints?: Record<string, unknown>
}

export type UpdateIntentPayload = {
  summary?: string
  action?: { verb: string; text?: string }
  conditions?: Record<string, unknown>
  strategy_schema_ref?: StrategySchemaRef
  schema_revision?: string
  risk_constraints?: Record<string, unknown>
}

async function readError(res: Response, fallback: string): Promise<never> {
  const body = await res.json().catch(() => ({}))
  throw new Error(body?.message || body?.error || `${fallback}: HTTP ${res.status}`)
}

export async function listStrategyIntents(params?: {
  status?: string
  candidate_id?: string
}): Promise<StrategyIntentListResponse> {
  const q = new URLSearchParams()
  if (params?.status) q.set('status', params.status)
  if (params?.candidate_id) q.set('candidate_id', params.candidate_id)
  const suffix = q.toString() ? `?${q}` : ''
  const res = await fetch(`/api/strategy/intents${suffix}`)
  if (!res.ok) await readError(res, 'Intent 列表失败')
  const body = await res.json()
  return { items: Array.isArray(body?.items) ? body.items : [], message: body?.message || '' }
}

export async function getStrategyIntent(id: string): Promise<StrategyIntent> {
  const res = await fetch(`/api/strategy/intents/${encodeURIComponent(id)}`)
  if (!res.ok) await readError(res, 'Intent 详情失败')
  return await res.json()
}

export async function createStrategyIntentDraft(
  payload: CreateIntentPayload,
): Promise<StrategyIntentWriteResult> {
  const res = await fetch('/api/strategy/intents', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!res.ok) await readError(res, '创建 Intent 失败')
  return await res.json()
}

export async function updateStrategyIntentDraft(
  id: string,
  payload: UpdateIntentPayload,
): Promise<StrategyIntentWriteResult> {
  const res = await fetch(`/api/strategy/intents/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!res.ok) await readError(res, '更新 Intent 失败')
  return await res.json()
}

export async function submitStrategyIntent(id: string): Promise<StrategyIntentWriteResult> {
  const res = await fetch(`/api/strategy/intents/${encodeURIComponent(id)}/submit`, {
    method: 'POST',
  })
  if (!res.ok) await readError(res, '提交审阅失败')
  return await res.json()
}

export async function approveStrategyIntent(id: string): Promise<StrategyIntentWriteResult> {
  const res = await fetch(`/api/strategy/intents/${encodeURIComponent(id)}/approve`, {
    method: 'POST',
  })
  if (!res.ok) await readError(res, 'Approve 失败')
  return await res.json()
}

export function intentStatusLabel(s: string): string {
  const map: Record<string, string> = {
    draft: '意图草稿',
    reviewing: '审阅中',
    approved: '已批准',
    expired: '已过期',
    discarded: '已废弃',
  }
  return map[s] || s || '—'
}

export function isIntentDraftEditable(status: string): boolean {
  return status === 'draft'
}
