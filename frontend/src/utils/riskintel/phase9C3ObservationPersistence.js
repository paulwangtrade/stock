/**
 * Phase9-C.3 Observation Persistence — JSON snapshot truth + derived Markdown.
 * Observation-only. Never mutates adapter / riskAdvice / legacy / execution.
 *
 * Runtime artifact fields are derived from existing observation snapshot /
 * shadow metrics (no scan/adapter hooks).
 */

import { PersistPhase9C3ObservationSnapshot } from '../../../wailsjs/go/main/App'

export const PHASE9_C3_PERSISTENCE_WRITER = 'c3_json_md_v1'
export const PHASE9_C3_SNAPSHOT_KIND = 'phase9_c3_observation_snapshot'
/** v2: explicit runtimeObservationArtifact contract fields */
export const PHASE9_C3_SNAPSHOT_SCHEMA_VERSION = 2

/** @type {string} */
let processSessionId = ''
/** @type {number} */
let snapshotSeq = 0

function ensureProcessSessionId() {
  if (!processSessionId) {
    const rand = (typeof crypto !== 'undefined' && crypto.randomUUID)
      ? crypto.randomUUID()
      : `r${Date.now().toString(36)}`
    processSessionId = `c3-${rand}`
  }
  return processSessionId
}

/** Reset session id (tests only). */
export function resetPhase9C3PersistenceSessionForTests() {
  processSessionId = ''
  snapshotSeq = 0
}

/**
 * Beijing calendar trading date YYYY-MM-DD.
 * @param {string} [iso]
 */
export function beijingTradingDateFromISO(iso) {
  const d = iso ? new Date(iso) : new Date()
  if (Number.isNaN(d.getTime())) {
    return new Intl.DateTimeFormat('en-CA', {
      timeZone: 'Asia/Shanghai',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    }).format(new Date())
  }
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(d)
}

function num(v, fallback = 0) {
  const n = Number(v)
  return Number.isFinite(n) ? n : fallback
}

function pct(rate) {
  const n = num(rate, 0)
  return `${(n <= 1 && n >= 0 ? n * 100 : n).toFixed(2)}%`
}

function clamp01(v) {
  const n = num(v, 0)
  if (n < 0) return 0
  if (n > 1) return 1
  return n
}

function topReasonList(raw, limit = 5) {
  if (!Array.isArray(raw) || raw.length === 0) return []
  return raw.slice(0, limit).map((row) => {
    if (row == null) return null
    if (typeof row === 'string') return { reason: row, count: null }
    if (typeof row === 'object') {
      return {
        reason: String(row.reason || row.key || row.code || ''),
        count: row.count != null ? num(row.count, null) : null,
      }
    }
    return { reason: String(row), count: null }
  }).filter((r) => r && r.reason)
}

/**
 * Build required runtime observation artifact from existing snapshot/metrics.
 * Per-decision legacy/projection payloads are not stored by adapter aggregates;
 * fields are filled from observation summaries (explicitly tagged).
 *
 * @param {object} bundle
 * @param {{ processSessionId?: string, tradingDate?: string, decisionId?: string }} [options]
 */
export function buildRuntimeObservationArtifact(bundle = {}, options = {}) {
  const capturedAt = bundle.capturedAt || new Date().toISOString()
  const tradingDate = options.tradingDate || beijingTradingDateFromISO(capturedAt)
  const sessionId = options.processSessionId || ensureProcessSessionId()
  snapshotSeq += 1
  const decisionId = options.decisionId
    || `c3-obs-${tradingDate}-${sessionId.slice(0, 8)}-${String(snapshotSeq).padStart(4, '0')}`

  const status = bundle.status || {}
  const metrics = bundle.metrics || {}
  const gate = bundle.gate || {}
  const hint = bundle.healthyWindowHint || {}
  const shadow = bundle.shadowObservation || {}
  const summary = shadow.summary || {}
  const evaluation = shadow.evaluation || {}
  const readiness = shadow.readiness || {}
  const adapter = shadow.holdingDecisionAdapter || {}
  const switchMetrics = adapter.switchMetrics || metrics || {}
  const shadowMig = adapter.shadowMigrationMetrics || {}
  const projectionMetrics = shadow.projection || {}

  const selectedSource = String(
    status.source
    || metrics.preferredSource
    || gate.authoritySource
    || gate.sourceLocked
    || '',
  )

  const mismatchClassification = {
    primary: topReasonList(
      readiness.majorMismatchReasons
      || summary.topMismatchReasons
      || evaluation.topObservationReasons
      || gate.topDiffReasons,
      5,
    ),
    observationCategoryFrequency: evaluation.observationCategoryFrequency
      || summary.observationCategoryFrequency
      || null,
    comparisonFrequency: shadowMig.comparisonFrequency || null,
    topDiffSymbols: Array.isArray(gate.topDiffSymbols)
      ? gate.topDiffSymbols
      : (Array.isArray(shadowMig.highestFrequencyDiffSymbols)
        ? shadowMig.highestFrequencyDiffSymbols.map((r) => r.key || r)
        : []),
    mismatchRate: num(summary.mismatchRate ?? status.diffRate ?? metrics.decisionDiffRate, 0),
    failedRate: num(summary.failedRate ?? readiness.failedRate, 0),
  }

  const legacyResult = {
    kind: 'aggregate_from_observation',
    note: 'Full per-decision legacy payloads are not retained in adapter metrics; aggregate only.',
    present: num(adapter.total ?? status.totalDecisions ?? metrics.totalDecisions, 0) > 0,
    sourceFrequency: adapter.sourceFrequency || null,
    actionFrequency: shadowMig.actionFrequency || null,
    levelFrequency: shadowMig.levelFrequency || null,
    legacyFallbackCount: num(
      switchMetrics.legacyFallbackCount ?? metrics.legacyFallbackCount,
      0,
    ),
  }

  const projectionResult = {
    kind: 'aggregate_from_observation',
    note: 'Full per-decision projection payloads are not retained in adapter metrics; aggregate only.',
    present: num(switchMetrics.projectionSelectedCount ?? metrics.projectionSelectedCount, 0) > 0
      || selectedSource.includes('projection'),
    projectionSelectedCount: num(
      switchMetrics.projectionSelectedCount ?? metrics.projectionSelectedCount,
      0,
    ),
    projectionUsageRate: num(
      status.projectionUsageRate ?? metrics.projectionUsageRate ?? switchMetrics.projectionUsageRate,
      0,
    ),
    projectionMetrics: projectionMetrics && typeof projectionMetrics === 'object'
      ? projectionMetrics
      : null,
    unavailableReasons: switchMetrics.projectionUnavailableReasons
      || shadowMig.projectionUnavailableReasons
      || metrics.projectionUnavailableReasons
      || null,
  }

  const matchRate = num(
    summary.matchRate
    ?? readiness.effectiveMatchRate
    ?? gate.readiness?.alignedMatchRate,
    0,
  )
  const usage = num(projectionResult.projectionUsageRate, 0)
  const unavail = num(
    status.unavailableRate
    ?? gate.projectionUnavailableRate
    ?? gate.readiness?.projectionUnavailableRate,
    0,
  )
  const confidenceScore = clamp01(
    matchRate * 0.45
    + usage * 0.35
    + (hint.countsTowardHealthyWindow === true ? 0.15 : 0)
    + (1 - clamp01(unavail)) * 0.05,
  )

  const confidence = {
    score: Number(confidenceScore.toFixed(4)),
    label: confidenceScore >= 0.75 ? 'high' : confidenceScore >= 0.45 ? 'medium' : 'low',
    basis: {
      matchRate,
      projectionUsageRate: usage,
      unavailableRate: unavail,
      healthyWindowLabel: hint.label || null,
      countsTowardHealthyWindow: hint.countsTowardHealthyWindow === true,
    },
  }

  const metricsSummary = {
    totalDecisions: num(status.totalDecisions ?? metrics.totalDecisions ?? adapter.total, 0),
    totalComparisons: num(summary.totalComparisons, 0),
    matched: num(summary.matched ?? evaluation.matchCount, 0),
    mismatched: num(summary.mismatched ?? evaluation.mismatchCount, 0),
    failed: num(summary.failed ?? evaluation.failedCount, 0),
    projectionUsageRate: usage,
    fallbackRate: num(status.fallbackRate ?? metrics.fallbackRate ?? switchMetrics.fallbackRate, 0),
    unavailableRate: unavail,
    diffRate: num(status.diffRate ?? metrics.decisionDiffRate ?? summary.mismatchRate, 0),
    gateRecommendation: gate.recommendation || readiness.migrationRecommendation || null,
    controlledSwitchStatus: status.status || null,
  }

  /** Best-effort sample rows from top diff types/symbols (no per-decision bodies in aggregates). */
  const decisionSamples = []
  const diffTypes = Array.isArray(shadowMig.highestFrequencyDiffTypes)
    ? shadowMig.highestFrequencyDiffTypes
    : mismatchClassification.primary
  const symbols = mismatchClassification.topDiffSymbols
  const sampleN = Math.min(5, Math.max(diffTypes.length, symbols.length))
  for (let i = 0; i < sampleN; i += 1) {
    const dtype = diffTypes[i] || null
    const reason = dtype && (dtype.reason || dtype.key) ? String(dtype.reason || dtype.key) : (mismatchClassification.primary[0]?.reason || 'UNKNOWN')
    const sym = typeof symbols[i] === 'string' ? symbols[i] : (symbols[i]?.key || null)
    decisionSamples.push({
      decisionId: `${decisionId}-s${i + 1}`,
      timestamp: capturedAt,
      symbol: sym,
      selectedSource,
      mismatchClassification: reason,
      legacyResult: { kind: 'unavailable_in_aggregate', symbol: sym },
      projectionResult: { kind: 'unavailable_in_aggregate', symbol: sym },
      confidence: {
        score: reason === 'EXACT_MATCH' ? 1 : confidence.score,
        label: reason === 'EXACT_MATCH' ? 'high' : confidence.label,
      },
    })
  }

  return {
    timestamp: capturedAt,
    decisionId,
    tradingDate,
    processSessionId: sessionId,
    legacyResult,
    projectionResult,
    selectedSource,
    mismatchClassification,
    confidence,
    metricsSummary,
    decisionSamples,
    provenance: {
      source: 'phase9_c3_observation_checkpoint',
      payloadKind: 'aggregate_observation',
      sameProcessScan: bundle.meta?.sameProcessScan === true,
      trigger: bundle.meta?.trigger || 'scan_success',
    },
  }
}

/**
 * Build durable snapshot document (JSON source of truth).
 * @param {object} bundle from getPhase9C3ObservationBundle / checkpoint
 * @param {{ processSessionId?: string, tradingDate?: string, decisionId?: string }} [options]
 */
export function buildPhase9C3ObservationSnapshotDocument(bundle = {}, options = {}) {
  const capturedAt = bundle.capturedAt || new Date().toISOString()
  const tradingDate = options.tradingDate || beijingTradingDateFromISO(capturedAt)
  const sessionId = options.processSessionId || ensureProcessSessionId()
  const metaIn = bundle.meta && typeof bundle.meta === 'object' ? { ...bundle.meta } : {}
  const trigger = metaIn.trigger || 'scan_success'

  const runtimeObservationArtifact = buildRuntimeObservationArtifact(bundle, {
    processSessionId: sessionId,
    tradingDate,
    decisionId: options.decisionId,
  })

  return {
    schemaVersion: PHASE9_C3_SNAPSHOT_SCHEMA_VERSION,
    kind: PHASE9_C3_SNAPSHOT_KIND,
    tradingDate,
    processSessionId: sessionId,
    trigger,
    capturedAt,
    /** Required runtime observation artifact (minimal contract). */
    runtimeObservationArtifact,
    /** Convenience mirrors of artifact root fields */
    timestamp: runtimeObservationArtifact.timestamp,
    decisionId: runtimeObservationArtifact.decisionId,
    legacyResult: runtimeObservationArtifact.legacyResult,
    projectionResult: runtimeObservationArtifact.projectionResult,
    selectedSource: runtimeObservationArtifact.selectedSource,
    mismatchClassification: runtimeObservationArtifact.mismatchClassification,
    confidence: runtimeObservationArtifact.confidence,
    metricsSummary: runtimeObservationArtifact.metricsSummary,
    status: bundle.status ?? null,
    metrics: bundle.metrics ?? null,
    gate: bundle.gate ?? null,
    trend: bundle.trend ?? null,
    shadowObservation: bundle.shadowObservation ?? null,
    healthyWindowHint: bundle.healthyWindowHint ?? null,
    meta: {
      observationOnly: true,
      sameProcessScan: metaIn.sameProcessScan === true || trigger === 'scan_success',
      doesNotChangeGateState: true,
      doesNotTriggerMigration: true,
      doesNotAffectExecution: true,
      ...metaIn,
      persistence: {
        writer: PHASE9_C3_PERSISTENCE_WRITER,
        sourceOfTruth: 'json',
        markdownDerived: true,
        schemaVersion: PHASE9_C3_SNAPSHOT_SCHEMA_VERSION,
        paths: [],
      },
    },
  }
}

/**
 * Derive Runbook-compatible Markdown ENTRY from snapshot JSON document.
 * @param {object} snapshot
 */
export function renderPhase9C3ObservationEntryMarkdown(snapshot = {}) {
  const artifact = snapshot.runtimeObservationArtifact || {}
  const status = snapshot.status || {}
  const metrics = snapshot.metrics || {}
  const hint = snapshot.healthyWindowHint || {}
  const shadow = snapshot.shadowObservation || {}
  const summary = shadow.summary || {}
  const evaluation = shadow.evaluation || {}
  const readiness = shadow.readiness || {}

  const ms = artifact.metricsSummary || {}
  const totalDecisions = num(ms.totalDecisions ?? status.totalDecisions ?? metrics.totalDecisions, 0)
  const projectionUsage = artifact.projectionResult?.projectionUsageRate
    ?? status.projectionUsageRate ?? metrics.projectionUsageRate ?? 0
  const fallbackRate = ms.fallbackRate ?? status.fallbackRate ?? metrics.fallbackRate ?? 0
  const unavailableRate = ms.unavailableRate ?? status.unavailableRate ?? 0
  const diffRate = ms.diffRate ?? status.diffRate ?? metrics.decisionDiffRate ?? 0

  const comparisons = num(ms.totalComparisons ?? summary.totalComparisons, 0)
  const mismatched = num(ms.mismatched ?? summary.mismatched ?? evaluation.mismatchCount, 0)
  const failed = num(ms.failed ?? summary.failed ?? evaluation.failedCount, 0)
  const primary = artifact.mismatchClassification?.primary || []
  const topReasons = primary.length
    ? primary.map((r) => (r.count != null ? `${r.reason}×${r.count}` : r.reason)).join(', ')
    : (Array.isArray(readiness.topDiffReasons) ? readiness.topDiffReasons.join(', ') : '(none)')
  const topSymbols = Array.isArray(artifact.mismatchClassification?.topDiffSymbols)
    && artifact.mismatchClassification.topDiffSymbols.length
    ? artifact.mismatchClassification.topDiffSymbols.map((s) => (typeof s === 'string' ? s : s.key)).join(', ')
    : '(none)'

  const healthyLabel = hint.label || 'UNKNOWN'
  const counted = hint.countsTowardHealthyWindow === true ? 'COUNTED' : 'NOT COUNTED'
  const conf = artifact.confidence || {}

  const lines = [
    '# Phase9-C.3 Observation Entry',
    '',
    '> source: auto',
    `> schemaVersion: ${snapshot.schemaVersion ?? PHASE9_C3_SNAPSHOT_SCHEMA_VERSION}`,
    `> kind: ${snapshot.kind || PHASE9_C3_SNAPSHOT_KIND}`,
    `> writer: ${PHASE9_C3_PERSISTENCE_WRITER}`,
    `> tradingDate: ${snapshot.tradingDate || artifact.tradingDate || ''}`,
    `> timestamp: ${artifact.timestamp || snapshot.capturedAt || ''}`,
    `> decisionId: ${artifact.decisionId || snapshot.decisionId || ''}`,
    `> processSessionId: ${snapshot.processSessionId || artifact.processSessionId || ''}`,
    `> trigger: ${snapshot.trigger || ''}`,
    `> selectedSource: ${artifact.selectedSource || ''}`,
    `> confidence: ${conf.score ?? ''} (${conf.label || ''})`,
    `> sameProcessScan: ${snapshot.meta?.sameProcessScan === true ? 'yes' : 'no'}`,
    '> jsonSourceOfTruth: latest.json (this Markdown is derived)',
    '',
    '## Runtime Observation Artifact',
    '',
    `| timestamp | ${artifact.timestamp || ''} |`,
    `| decisionId | ${artifact.decisionId || ''} |`,
    `| selectedSource | ${artifact.selectedSource || ''} |`,
    `| confidence | ${conf.score ?? ''} / ${conf.label || ''} |`,
    `| legacyResult.kind | ${artifact.legacyResult?.kind || ''} |`,
    `| projectionResult.kind | ${artifact.projectionResult?.kind || ''} |`,
    `| mismatch primary | ${topReasons} |`,
    '',
    '## Authority',
    `| preferred source | ${artifact.selectedSource || status.source || metrics.preferredSource || ''} |`,
    `| controlledSwitch status | ${status.status ?? ''} |`,
    `| activeSince | ${status.activeSince ?? ''} |`,
    '',
    '## Metrics Summary',
    '',
    `Samples:`,
    `${totalDecisions}`,
    '',
    `Controlled Switch:`,
    `${artifact.selectedSource || status.source || ''}`,
    '',
    `Projection Usage:`,
    `${pct(projectionUsage)}`,
    '',
    `Fallback:`,
    `${pct(fallbackRate)}`,
    '',
    `Unavailable:`,
    `${pct(unavailableRate)}`,
    '',
    `Diff Rate:`,
    `${pct(diffRate)}`,
    '',
    '## Shadow / Mismatch Classification',
    '',
    `comparisons=${comparisons}`,
    `mismatch=${mismatched}`,
    `failed=${failed}`,
    '',
    `Mismatch Reason:`,
    `${topReasons}`,
    '',
    `Symbols:`,
    `${topSymbols}`,
    '',
    '## Healthy Window',
    '',
    `Healthy Window:`,
    `${counted}`,
    '',
    `Label:`,
    `${healthyLabel}`,
    '',
    `Reason:`,
    `${Array.isArray(hint.reasons) ? hint.reasons.join('; ') : (hint.reasons || '')}`,
    '',
  ]
  return `${lines.join('\n')}\n`
}

/**
 * Persist checkpoint bundle: JSON truth + derived Markdown via Go writer.
 * Failures are warnings only — never throw to callers that ignore the return.
 *
 * @param {object} bundle
 * @param {{ tradingDate?: string, processSessionId?: string, decisionId?: string }} [options]
 * @returns {Promise<{ ok: boolean, error?: string, paths?: string[], baseDir?: string, snapshot?: object }>}
 */
export async function persistPhase9C3ObservationSnapshot(bundle, options = {}) {
  try {
    const snapshot = buildPhase9C3ObservationSnapshotDocument(bundle, options)
    const tradingDate = snapshot.tradingDate
    const json = `${JSON.stringify(snapshot, null, 2)}\n`
    const markdown = renderPhase9C3ObservationEntryMarkdown(snapshot)

    if (typeof PersistPhase9C3ObservationSnapshot !== 'function') {
      const msg = 'PersistPhase9C3ObservationSnapshot binding unavailable'
      console.warn('[phase9-c3-persistence]', msg)
      return { ok: false, error: msg, snapshot }
    }

    const result = await PersistPhase9C3ObservationSnapshot(tradingDate, json, markdown)
    const ok = result && result.ok === true
    if (!ok) {
      const err = (result && result.error) || 'persist returned not ok'
      console.warn('[phase9-c3-persistence]', err)
      return {
        ok: false,
        error: String(err),
        paths: result?.paths,
        baseDir: result?.baseDir,
        snapshot,
      }
    }

    if (snapshot.meta && snapshot.meta.persistence) {
      snapshot.meta.persistence.paths = Array.isArray(result.paths) ? result.paths : []
    }

    return {
      ok: true,
      paths: result.paths,
      baseDir: result.baseDir,
      snapshot,
    }
  } catch (e) {
    const msg = e && e.message ? e.message : String(e)
    console.warn('[phase9-c3-persistence] write failed:', msg)
    return { ok: false, error: msg }
  }
}

/** Required artifact keys for smoke / selftest. */
export const RUNTIME_OBSERVATION_ARTIFACT_REQUIRED_KEYS = Object.freeze([
  'timestamp',
  'decisionId',
  'legacyResult',
  'projectionResult',
  'selectedSource',
  'mismatchClassification',
  'confidence',
  'metricsSummary',
])
