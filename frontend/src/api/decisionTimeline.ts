/**
 * GET /api/research/decision-timeline — read-only evidence trail.
 * Does not create trade plans or orders.
 */
import { DECISION_TIMELINE_DISCLAIMER } from '../utils/decisionTimelineDisplay.js'

export type DecisionTimelineEvent = {
  id: string
  kind: string
  lane: string
  occurred_on: string
  occurred_at?: string
  title: string
  detail: string
  tag?: string
  source: string
  kline_date?: string
}

export type DecisionTimelineSource = {
  source: string
  status: string
  message?: string
  count: number
}

export type DecisionTimeline = {
  stock_code: string
  stock_name?: string
  disclaimer: string
  observation_only: boolean
  events: DecisionTimelineEvent[]
  sources: DecisionTimelineSource[]
  generated_at: string
}

function str(v: unknown): string {
  if (v == null) return ''
  return String(v)
}

function mapEvent(raw: Record<string, unknown>): DecisionTimelineEvent {
  return {
    id: str(raw.id),
    kind: str(raw.kind),
    lane: str(raw.lane),
    occurred_on: str(raw.occurred_on || raw.occurredOn),
    occurred_at: str(raw.occurred_at || raw.occurredAt) || undefined,
    title: str(raw.title),
    detail: str(raw.detail),
    tag: str(raw.tag) || undefined,
    source: str(raw.source),
    kline_date: str(raw.kline_date || raw.klineDate) || undefined,
  }
}

function mapSource(raw: Record<string, unknown>): DecisionTimelineSource {
  return {
    source: str(raw.source),
    status: str(raw.status),
    message: str(raw.message) || undefined,
    count: Math.trunc(Number(raw.count) || 0),
  }
}

export async function fetchDecisionTimeline(stockCode: string): Promise<DecisionTimeline> {
  const code = String(stockCode || '').trim()
  if (!code) {
    throw new Error('请提供股票代码')
  }
  const params = new URLSearchParams({ stock_code: code })
  const res = await fetch(`/api/research/decision-timeline?${params.toString()}`)
  const body = (await res.json()) as Record<string, unknown>
  if (!res.ok || body?.ok === false) {
    throw new Error(str(body?.message) || `HTTP ${res.status}`)
  }
  const raw = (body.timeline && typeof body.timeline === 'object' ? body.timeline : {}) as Record<string, unknown>
  const eventsRaw = Array.isArray(raw.events) ? raw.events : []
  const sourcesRaw = Array.isArray(raw.sources) ? raw.sources : []
  return {
    stock_code: str(raw.stock_code || raw.stockCode || code),
    stock_name: str(raw.stock_name || raw.stockName) || undefined,
    disclaimer: str(raw.disclaimer) || DECISION_TIMELINE_DISCLAIMER,
    observation_only: raw.observation_only !== false && raw.observationOnly !== false,
    events: eventsRaw.map((e) => mapEvent(e as Record<string, unknown>)),
    sources: sourcesRaw.map((s) => mapSource(s as Record<string, unknown>)),
    generated_at: str(raw.generated_at || raw.generatedAt),
  }
}
