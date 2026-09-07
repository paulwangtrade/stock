/** Phase13 Research Candidate Pool + Explain HTTP API（与 Trade Candidate 隔离） */

export const RESEARCH_STATUSES = ['new', 'watching', 'reviewed', 'discarded'] as const

export type ResearchStatus = (typeof RESEARCH_STATUSES)[number]

export type ResearchCandidate = {
  id: string
  trade_date: string
  stock_code: string
  stock_name: string
  status: string
  source: string
  source_ref?: string
  signal_tag?: string
  signal_score?: number
  signal_snapshot_id?: number
  score?: number | null
  rank?: number | null
  tags?: string[]
  note?: string
  updated_at?: string | null
  promoted_pool_id?: number | null
  promoted_at?: string | null
  explain_ref?: string | null
  explain_summary?: string
  direction?: string
  price?: string
  reason?: string
}

export type ResearchCandidateListResponse = {
  trade_date: string
  items: ResearchCandidate[]
  as_of: string
  message?: string
  threshold?: number
  snapshot_id?: number
}

export type ResearchCandidateDetailResponse = {
  candidate: ResearchCandidate
  explanation: {
    available: boolean
    summary: string
    missing_reason?: string
    explain_ref?: string
  }
  links: {
    trade_pool_id: number | null
    trade_plan_id: number | null
  }
}

export type ResearchCandidateUpdatePayload = {
  status?: string
  note?: string
  tags?: string[]
}

export type ResearchExplain = {
  id: string
  schema_version: string
  candidate_id: string
  trade_date?: string
  stock_code?: string
  explain_type: string
  available: boolean
  missing_reason?: string
  summary: string
  evidence: {
    as_of?: string
    signal_snapshot_id?: number
    source?: string
    source_ref?: string
    signal_tag?: string
    signal_score?: number
    direction?: string
    price?: string
    status_text?: string
    score_steps?: string[]
    evidence_hash?: string
  }
  research_reason: { kind: string; text: string }
  risk_note: { severity: string; text: string } | null
  strategy_intent_ref: string | null
  created_at: string
  updated_at: string
}

export type ResearchExplainUpdatePayload = {
  summary?: string
  research_reason?: { kind: string; text: string }
  risk_note?: { severity: string; text: string } | null
  clear_risk_note?: boolean
}

export async function listResearchCandidates(opts?: {
  tradeDate?: string
  status?: string
  source?: string
  minScore?: number
}): Promise<ResearchCandidateListResponse> {
  const q = new URLSearchParams()
  if (opts?.tradeDate) q.set('trade_date', opts.tradeDate)
  if (opts?.status) q.set('status', opts.status)
  if (opts?.source) q.set('source', opts.source)
  if (opts?.minScore != null && opts.minScore > 0) q.set('min_score', String(opts.minScore))
  const qs = q.toString()
  const res = await fetch(`/api/research/candidates${qs ? `?${qs}` : ''}`)
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body?.message || body?.error || `研究候选请求失败: HTTP ${res.status}`)
  }
  const body = await res.json()
  return {
    trade_date: body?.trade_date || '',
    items: Array.isArray(body?.items) ? body.items : [],
    as_of: body?.as_of || '',
    message: body?.message || '',
    threshold: Number(body?.threshold) || 0,
    snapshot_id: Number(body?.snapshot_id) || 0,
  }
}

export async function getResearchCandidate(id: string): Promise<ResearchCandidateDetailResponse> {
  const res = await fetch(`/api/research/candidates/${encodeURIComponent(id)}`)
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body?.message || body?.error || `研究候选详情失败: HTTP ${res.status}`)
  }
  return await res.json()
}

export async function updateResearchCandidate(
  id: string,
  patch: ResearchCandidateUpdatePayload,
): Promise<ResearchCandidateDetailResponse> {
  const res = await fetch(`/api/research/candidates/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body?.message || body?.error || `更新研究候选失败: HTTP ${res.status}`)
  }
  return await res.json()
}

export async function getResearchExplain(candidateId: string): Promise<ResearchExplain> {
  const res = await fetch(`/api/research/candidates/${encodeURIComponent(candidateId)}/explain`)
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body?.message || body?.error || `研究解释请求失败: HTTP ${res.status}`)
  }
  return await res.json()
}

export async function updateResearchExplain(
  candidateId: string,
  patch: ResearchExplainUpdatePayload,
): Promise<ResearchExplain> {
  const res = await fetch(`/api/research/candidates/${encodeURIComponent(candidateId)}/explain`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body?.message || body?.error || `更新研究解释失败: HTTP ${res.status}`)
  }
  return await res.json()
}
