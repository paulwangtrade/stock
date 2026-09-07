/** Recovery Readiness 只读 HTTP API（Phase6.5.8.5.3） */

export type RecoveryCheckpointSample = {
  scope: string
  last_audit_id: number
  last_event_id: string
  version: number
  intact: boolean
  lag_events: number
  error?: string
}

export type RecoveryCheckpointStatus = {
  scope_count: number
  intact_count: number
  corrupt_count: number
  contract_mismatch_count: number
  missing_baseline: boolean
  max_lag_events: number
  samples: RecoveryCheckpointSample[]
  status_hint: string
}

export type RecoveryAuditContinuity = {
  max_audit_id: number
  event_count: number
  high_watermark: number
  window_checked: boolean
  gap_count: number
  anchor_mismatch: number
  status_hint: string
}

export type RecoveryReplayVerification = {
  mode: string
  sample_size: number
  passed_count: number
  failed_count: number
  skipped_count: number
  last_error?: string
  status_hint: string
}

export type RecoveryDivergenceItem = {
  kind: string
  severity: string
  blocking: boolean
  scope?: string
  event_id?: string
  event_type?: string
  detail?: string
}

export type RecoveryDivergenceSummary = {
  total: number
  by_kind: Record<string, number>
  by_severity: Record<string, number>
  blocking_count: number
  top: RecoveryDivergenceItem[]
}

export type RecoveryReadinessView = {
  status: string
  checkpoint_status: RecoveryCheckpointStatus
  audit_continuity: RecoveryAuditContinuity
  replay_verification: RecoveryReplayVerification
  divergence_summary: RecoveryDivergenceSummary
}

function asRecord(v: unknown): Record<string, unknown> {
  return v && typeof v === 'object' ? (v as Record<string, unknown>) : {}
}

function mapSample(raw: unknown): RecoveryCheckpointSample {
  const s = asRecord(raw)
  return {
    scope: String(s.scope || ''),
    last_audit_id: Number(s.last_audit_id) || 0,
    last_event_id: String(s.last_event_id || ''),
    version: Number(s.version) || 0,
    intact: !!s.intact,
    lag_events: Number(s.lag_events) || 0,
    error: s.error ? String(s.error) : '',
  }
}

function mapDivergenceItem(raw: unknown): RecoveryDivergenceItem {
  const d = asRecord(raw)
  return {
    kind: String(d.kind || ''),
    severity: String(d.severity || ''),
    blocking: !!d.blocking,
    scope: d.scope ? String(d.scope) : '',
    event_id: d.event_id ? String(d.event_id) : '',
    event_type: d.event_type ? String(d.event_type) : '',
    detail: d.detail ? String(d.detail) : '',
  }
}

function mapNumberMap(raw: unknown): Record<string, number> {
  const src = asRecord(raw)
  const out: Record<string, number> = {}
  for (const [k, v] of Object.entries(src)) {
    out[k] = Number(v) || 0
  }
  return out
}

function mapView(raw: Record<string, unknown> | null | undefined): RecoveryReadinessView | null {
  if (!raw || typeof raw !== 'object') return null
  const cp = asRecord(raw.checkpoint_status)
  const cont = asRecord(raw.audit_continuity)
  const replay = asRecord(raw.replay_verification)
  const div = asRecord(raw.divergence_summary)
  const samplesRaw = Array.isArray(cp.samples) ? cp.samples : []
  const topRaw = Array.isArray(div.top) ? div.top : []
  return {
    status: String(raw.status || ''),
    checkpoint_status: {
      scope_count: Number(cp.scope_count) || 0,
      intact_count: Number(cp.intact_count) || 0,
      corrupt_count: Number(cp.corrupt_count) || 0,
      contract_mismatch_count: Number(cp.contract_mismatch_count) || 0,
      missing_baseline: !!cp.missing_baseline,
      max_lag_events: Number(cp.max_lag_events) || 0,
      samples: samplesRaw.map(mapSample),
      status_hint: String(cp.status_hint || ''),
    },
    audit_continuity: {
      max_audit_id: Number(cont.max_audit_id) || 0,
      event_count: Number(cont.event_count) || 0,
      high_watermark: Number(cont.high_watermark) || 0,
      window_checked: !!cont.window_checked,
      gap_count: Number(cont.gap_count) || 0,
      anchor_mismatch: Number(cont.anchor_mismatch) || 0,
      status_hint: String(cont.status_hint || ''),
    },
    replay_verification: {
      mode: String(replay.mode || ''),
      sample_size: Number(replay.sample_size) || 0,
      passed_count: Number(replay.passed_count) || 0,
      failed_count: Number(replay.failed_count) || 0,
      skipped_count: Number(replay.skipped_count) || 0,
      last_error: replay.last_error ? String(replay.last_error) : '',
      status_hint: String(replay.status_hint || ''),
    },
    divergence_summary: {
      total: Number(div.total) || 0,
      by_kind: mapNumberMap(div.by_kind),
      by_severity: mapNumberMap(div.by_severity),
      blocking_count: Number(div.blocking_count) || 0,
      top: topRaw.map(mapDivergenceItem),
    },
  }
}

/**
 * GET /api/recovery/readiness
 * 直接返回 RecoveryReadinessView（无 code/ok 信封）。
 */
export async function getRecoveryReadiness(): Promise<RecoveryReadinessView> {
  const res = await fetch('/api/recovery/readiness')
  if (!res.ok) {
    throw new Error(`Recovery Readiness 请求失败: HTTP ${res.status}`)
  }
  const body = await res.json()
  const view = mapView(body)
  if (!view || !view.status) {
    throw new Error('Recovery Readiness 响应无效')
  }
  return view
}
