/** Phase13-B3 Strategy Schema HTTP API（只读 + 手工生命周期写；无 AI / Intent / Promote） */

export type StrategySchemaListItem = {
  strategy_id: string
  name: string
  description?: string
  status: string
  current_revision?: string
  updated_at: string
}

export type StrategySchemaListResponse = {
  items: StrategySchemaListItem[]
  message?: string
}

export type StrategyRevisionSummary = {
  revision_id: string
  revision: string
  status: string
  revision_note?: string
  params_hash?: string
  revision_hash?: string
  source?: string
  updated_at: string
}

export type StrategySchemaDetail = {
  definition: {
    strategy_id: string
    schema_version: string
    name: string
    description?: string
    status: string
    current_revision?: string
    created_at: string
    updated_at: string
  }
  revisions: StrategyRevisionSummary[]
  current?: StrategyRevision
}

export type StrategyRevision = {
  revision_id: string
  strategy_id: string
  revision: string
  schema_version: string
  status: string
  name_override?: string
  revision_note?: string
  universe: Record<string, unknown>
  signals: Record<string, unknown>
  filters: Record<string, unknown>
  ranking: Record<string, unknown>
  risk_profile_ref: Record<string, unknown>
  parameters: { knobs?: Record<string, unknown>; params_hash?: string }
  revision_hash?: string
  source?: string
  parent_revision?: string
  created_at: string
  updated_at: string
  activated_at?: string
  retired_at?: string
}

export type StrategyRevisionListResponse = {
  strategy_id: string
  items: StrategyRevisionSummary[]
}

export type StrategySchemaWriteResult = {
  definition: StrategySchemaDetail['definition']
  revision: StrategyRevision
  message?: string
}

export type CreateDraftPayload = {
  strategy_id?: string
  slug?: string
  name: string
  description?: string
  parent_revision?: string
  revision_note?: string
  name_override?: string
  universe?: Record<string, unknown>
  signals?: Record<string, unknown>
  filters?: Record<string, unknown>
  ranking?: Record<string, unknown>
  risk_profile_ref?: Record<string, unknown>
  knobs?: Record<string, unknown>
}

export type UpdateDraftPayload = {
  revision_note?: string
  name_override?: string
  universe?: Record<string, unknown>
  signals?: Record<string, unknown>
  filters?: Record<string, unknown>
  ranking?: Record<string, unknown>
  risk_profile_ref?: Record<string, unknown>
  knobs?: Record<string, unknown>
  clear_knobs?: boolean
}

async function readError(res: Response, fallback: string): Promise<never> {
  const body = await res.json().catch(() => ({}))
  throw new Error(body?.message || body?.error || `${fallback}: HTTP ${res.status}`)
}

export async function listStrategySchemas(status?: string): Promise<StrategySchemaListResponse> {
  const q = status ? `?status=${encodeURIComponent(status)}` : ''
  const res = await fetch(`/api/strategy/schemas${q}`)
  if (!res.ok) await readError(res, 'Schema 列表失败')
  const body = await res.json()
  return {
    items: Array.isArray(body?.items) ? body.items : [],
    message: body?.message || '',
  }
}

export async function getStrategySchema(id: string): Promise<StrategySchemaDetail> {
  const res = await fetch(`/api/strategy/schemas/${encodeURIComponent(id)}`)
  if (!res.ok) await readError(res, 'Schema 详情失败')
  return await res.json()
}

export async function listStrategyRevisions(id: string): Promise<StrategyRevisionListResponse> {
  const res = await fetch(`/api/strategy/schemas/${encodeURIComponent(id)}/revisions`)
  if (!res.ok) await readError(res, 'Revision 列表失败')
  return await res.json()
}

export async function getStrategyRevision(id: string, version: string): Promise<StrategyRevision> {
  const res = await fetch(
    `/api/strategy/schemas/${encodeURIComponent(id)}/revisions/${encodeURIComponent(version)}`,
  )
  if (!res.ok) await readError(res, 'Revision 详情失败')
  return await res.json()
}

export async function createStrategySchemaDraft(
  payload: CreateDraftPayload,
): Promise<StrategySchemaWriteResult> {
  const res = await fetch('/api/strategy/schemas', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!res.ok) await readError(res, '创建 Draft 失败')
  return await res.json()
}

export async function updateStrategySchemaDraft(
  id: string,
  version: string,
  payload: UpdateDraftPayload,
): Promise<StrategySchemaWriteResult> {
  const res = await fetch(
    `/api/strategy/schemas/${encodeURIComponent(id)}/revisions/${encodeURIComponent(version)}`,
    {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    },
  )
  if (!res.ok) await readError(res, '更新 Draft 失败')
  return await res.json()
}

export async function submitStrategyRevision(
  id: string,
  version: string,
): Promise<StrategySchemaWriteResult> {
  const res = await fetch(
    `/api/strategy/schemas/${encodeURIComponent(id)}/revisions/${encodeURIComponent(version)}/submit`,
    { method: 'POST' },
  )
  if (!res.ok) await readError(res, '提交审阅失败')
  return await res.json()
}

export async function activateStrategyRevision(
  id: string,
  version: string,
): Promise<StrategySchemaWriteResult> {
  const res = await fetch(
    `/api/strategy/schemas/${encodeURIComponent(id)}/revisions/${encodeURIComponent(version)}/activate`,
    { method: 'POST' },
  )
  if (!res.ok) await readError(res, 'Activate 失败')
  return await res.json()
}

export function revStatusLabel(s: string): string {
  const map: Record<string, string> = {
    draft: '模板草稿',
    reviewing: '审阅中',
    active: '已发布',
    retired: '已退役',
    discarded: '已废弃',
  }
  return map[s] || s || '—'
}

export function isDraftEditable(status: string): boolean {
  return status === 'draft'
}

export function isRevisionFrozen(status: string): boolean {
  return status === 'reviewing' || status === 'active' || status === 'retired' || status === 'discarded'
}
