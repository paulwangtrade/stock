/**
 * QuantDecision Phase1-C：观测冻结（只观测，不改 Action 语义 / UI 文案）
 *
 * - DecisionID 可追踪
 * - actionSource 计数（含 legacy_fallback）
 * - 同 code + 同 asOf 的 Action 冲突检测
 */

export const OBS_CONSUMER_CARD = 'card'
export const OBS_CONSUMER_ALERT = 'alert'
export const OBS_CONSUMER_KLINE = 'kline'

export const OBS_SOURCE_DECISION = 'decision'
export const OBS_SOURCE_LEGACY = 'legacy_fallback'

const MAX_TRACES = 500

/** @type {{
 *   enabled: boolean,
 *   sourceCounts: Record<string, number>,
 *   byConsumer: Record<string, Record<string, number>>,
 *   traces: Array<object>,
 *   conflicts: Array<object>,
 * }} */
let state = createEmptyState()

function createEmptyState() {
  return {
    enabled: true,
    sourceCounts: {
      [OBS_SOURCE_DECISION]: 0,
      [OBS_SOURCE_LEGACY]: 0,
    },
    byConsumer: {
      [OBS_CONSUMER_CARD]: { [OBS_SOURCE_DECISION]: 0, [OBS_SOURCE_LEGACY]: 0 },
      [OBS_CONSUMER_ALERT]: { [OBS_SOURCE_DECISION]: 0, [OBS_SOURCE_LEGACY]: 0 },
      [OBS_CONSUMER_KLINE]: { [OBS_SOURCE_DECISION]: 0, [OBS_SOURCE_LEGACY]: 0 },
    },
    traces: [],
    conflicts: [],
  }
}

export function resetQuantDecisionObservability() {
  state = createEmptyState()
}

export function setQuantDecisionObservabilityEnabled(on) {
  state.enabled = !!on
}

export function normalizeObsCode(code) {
  return String(code || '').trim().toLowerCase()
}

/**
 * 稳定 DecisionID（同 code/asOf/action 可复现，便于跨消费者回溯）
 */
export function buildDecisionId({
  code = '',
  asOf = '',
  actionCode = '',
  actionLabel = '',
  producer = 'js_legacy',
  schemaVersion = 1,
} = {}) {
  const c = normalizeObsCode(code) || '_'
  const a = String(asOf || '').trim() || '_'
  const ac = String(actionCode || '').trim() || '_'
  const al = String(actionLabel || '').trim() || '_'
  return `qd${schemaVersion}:${producer}:${c}:${a}:${ac}:${al}`
}

/** 若缺 id 则写入（原地），不改 action */
export function ensureDecisionId(decision) {
  if (!decision || typeof decision !== 'object') return decision
  if (decision.id) return decision
  const code = decision.instrument?.stockCode || decision.meta?.stockCode || ''
  decision.id = buildDecisionId({
    code,
    asOf: decision.asOf || '',
    actionCode: decision.action?.code || '',
    actionLabel: decision.action?.label || '',
    producer: decision.meta?.producer || 'js_legacy',
    schemaVersion: decision.meta?.schemaVersion ?? 1,
  })
  return decision
}

function conflictKey(code, asOf) {
  return `${normalizeObsCode(code)}|${String(asOf || '')}`
}

/**
 * 记录一次消费者 Action 观测；同 code+asOf 若 label/code 不一致则记冲突。
 * @returns {{ decisionId: string|null, conflict: object|null }}
 */
export function recordActionObservation({
  consumer,
  code = '',
  asOf = '',
  decisionId = null,
  actionLabel = '',
  actionCode = '',
  actionSource = OBS_SOURCE_LEGACY,
} = {}) {
  if (!state.enabled) {
    return { decisionId: decisionId || null, conflict: null }
  }

  const src = actionSource === OBS_SOURCE_DECISION ? OBS_SOURCE_DECISION : OBS_SOURCE_LEGACY
  state.sourceCounts[src] = (state.sourceCounts[src] || 0) + 1
  if (!state.byConsumer[consumer]) {
    state.byConsumer[consumer] = { [OBS_SOURCE_DECISION]: 0, [OBS_SOURCE_LEGACY]: 0 }
  }
  state.byConsumer[consumer][src] = (state.byConsumer[consumer][src] || 0) + 1

  const trace = {
    at: Date.now(),
    consumer,
    code: normalizeObsCode(code),
    asOf: String(asOf || ''),
    decisionId: decisionId || null,
    actionLabel: String(actionLabel || ''),
    actionCode: String(actionCode || ''),
    actionSource: src,
  }
  state.traces.push(trace)
  if (state.traces.length > MAX_TRACES) {
    state.traces.splice(0, state.traces.length - MAX_TRACES)
  }

  const conflict = detectConflictForTrace(trace)
  return { decisionId: trace.decisionId, conflict }
}

function detectConflictForTrace(trace) {
  if (!trace.code || !trace.asOf) return null
  const key = conflictKey(trace.code, trace.asOf)
  const peers = state.traces.filter(
    (t) => t !== trace && conflictKey(t.code, t.asOf) === key && t.actionLabel,
  )
  if (!peers.length || !trace.actionLabel) return null

  const labels = new Set([trace.actionLabel, ...peers.map((p) => p.actionLabel)].filter(Boolean))
  const codes = new Set([trace.actionCode, ...peers.map((p) => p.actionCode)].filter(Boolean))
  if (labels.size <= 1 && codes.size <= 1) return null

  const conflict = {
    key,
    code: trace.code,
    asOf: trace.asOf,
    actionLabels: [...labels],
    actionCodes: [...codes],
    decisionIds: [...new Set([trace.decisionId, ...peers.map((p) => p.decisionId)].filter(Boolean))],
    consumers: [...new Set([trace.consumer, ...peers.map((p) => p.consumer)])],
    at: Date.now(),
  }
  // 去重：同 key 已有相同 labels 则不重复推
  const dup = state.conflicts.some(
    (c) => c.key === conflict.key
      && c.actionLabels.slice().sort().join('|') === conflict.actionLabels.slice().sort().join('|'),
  )
  if (!dup) state.conflicts.push(conflict)
  return conflict
}

export function getActionSourceCounts() {
  return {
    ...state.sourceCounts,
    byConsumer: JSON.parse(JSON.stringify(state.byConsumer)),
    legacyFallback: state.sourceCounts[OBS_SOURCE_LEGACY] || 0,
  }
}

export function getActionConflicts() {
  return state.conflicts.slice()
}

export function getActionTraces(limit = 100) {
  return state.traces.slice(-limit)
}

/** 默认验收：无冲突 */
export function assertNoActionConflicts() {
  return state.conflicts.length === 0
}

export function getQuantDecisionObservabilitySnapshot() {
  return {
    enabled: state.enabled,
    sourceCounts: { ...state.sourceCounts },
    byConsumer: JSON.parse(JSON.stringify(state.byConsumer)),
    legacyFallbackCount: state.sourceCounts[OBS_SOURCE_LEGACY] || 0,
    conflictCount: state.conflicts.length,
    conflicts: state.conflicts.slice(),
    traceCount: state.traces.length,
  }
}
