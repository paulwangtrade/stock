/**
 * Phase9-C.3 Observation Persistence selftest (no Go write).
 * Run: node --import ./frontend/scripts/loaders/register-js-ext.mjs frontend/src/utils/riskintel/phase9C3ObservationPersistence.selftest.mjs
 */
import assert from 'node:assert/strict'
import {
  beijingTradingDateFromISO,
  buildRuntimeObservationArtifact,
  buildPhase9C3ObservationSnapshotDocument,
  renderPhase9C3ObservationEntryMarkdown,
  resetPhase9C3PersistenceSessionForTests,
  PHASE9_C3_SNAPSHOT_KIND,
  PHASE9_C3_PERSISTENCE_WRITER,
  PHASE9_C3_SNAPSHOT_SCHEMA_VERSION,
  RUNTIME_OBSERVATION_ARTIFACT_REQUIRED_KEYS,
} from './phase9C3ObservationPersistence.js'

resetPhase9C3PersistenceSessionForTests()

{
  const d = beijingTradingDateFromISO('2026-08-04T16:00:00.000Z') // UTC 16:00 = Beijing 00:00 next day
  assert.equal(d, '2026-08-05')
}

{
  const bundle = {
    capturedAt: '2026-08-04T02:00:00.000Z',
    status: {
      source: 'riskAdvice_projection',
      status: 'HEALTHY',
      totalDecisions: 3,
      projectionUsageRate: 1,
      fallbackRate: 0,
      unavailableRate: 0,
      diffRate: 1,
    },
    metrics: {
      totalDecisions: 3,
      preferredSource: 'riskAdvice_projection',
      projectionUsageRate: 1,
      fallbackRate: 0,
      decisionDiffRate: 1,
      projectionSelectedCount: 3,
      legacyFallbackCount: 0,
    },
    gate: {
      sampleCount: 3,
      recommendation: 'OBSERVE',
      topDiffReasons: ['TARGET_DIFF'],
      topDiffSymbols: ['sh603629', 'sz000021', 'sz300604'],
      authoritySource: 'riskAdvice_projection',
    },
    trend: {},
    shadowObservation: {
      summary: {
        totalComparisons: 3,
        matched: 0,
        mismatched: 3,
        failed: 0,
        matchRate: 0,
        mismatchRate: 1,
        failedRate: 0,
        topMismatchReasons: [{ reason: 'TARGET_DIFF', count: 3 }],
      },
      evaluation: {
        matchCount: 0,
        mismatchCount: 3,
        failedCount: 0,
        observationCategoryFrequency: { MATCH: 0, MISMATCH: 3, FAILED: 0 },
      },
      readiness: {
        majorMismatchReasons: [{ reason: 'TARGET_DIFF', count: 3 }],
        migrationRecommendation: 'OBSERVE_LONGER',
      },
      holdingDecisionAdapter: {
        total: 3,
        sourceFrequency: { legacy: 0, riskAdvice_projection: 3 },
        switchMetrics: {
          projectionSelectedCount: 3,
          legacyFallbackCount: 0,
          projectionUsageRate: 1,
          fallbackRate: 0,
        },
        shadowMigrationMetrics: {
          comparisonFrequency: { TARGET_DIFF: 3, EXACT_MATCH: 0 },
          actionFrequency: { hold: 3 },
          levelFrequency: { '3': 3 },
          highestFrequencyDiffTypes: [{ key: 'TARGET_DIFF', count: 3 }],
          highestFrequencyDiffSymbols: [
            { key: 'sh603629', count: 1 },
            { key: 'sz000021', count: 1 },
            { key: 'sz300604', count: 1 },
          ],
        },
      },
      projection: { generated: 3, failed: 0 },
    },
    healthyWindowHint: {
      label: 'INSUFFICIENT SAMPLE',
      countsTowardHealthyWindow: false,
      reasons: ['totalDecisions=3 < 30'],
    },
    meta: { trigger: 'scan_success', sameProcessScan: true, scannedCount: 10 },
  }

  const artifact = buildRuntimeObservationArtifact(bundle, {
    processSessionId: 'c3-test-session',
    decisionId: 'c3-obs-test-0001',
  })
  for (const key of RUNTIME_OBSERVATION_ARTIFACT_REQUIRED_KEYS) {
    assert.ok(key in artifact, `artifact missing ${key}`)
    assert.ok(artifact[key] != null, `artifact.${key} null`)
  }
  assert.equal(artifact.timestamp, '2026-08-04T02:00:00.000Z')
  assert.equal(artifact.decisionId, 'c3-obs-test-0001')
  assert.equal(artifact.selectedSource, 'riskAdvice_projection')
  assert.equal(artifact.legacyResult.kind, 'aggregate_from_observation')
  assert.equal(artifact.projectionResult.kind, 'aggregate_from_observation')
  assert.ok(typeof artifact.confidence.score === 'number')
  assert.ok(artifact.mismatchClassification.primary.some((r) => r.reason === 'TARGET_DIFF'))
  assert.equal(artifact.metricsSummary.totalDecisions, 3)

  resetPhase9C3PersistenceSessionForTests()
  const snap = buildPhase9C3ObservationSnapshotDocument(bundle, {
    processSessionId: 'c3-test-session',
  })
  assert.equal(snap.kind, PHASE9_C3_SNAPSHOT_KIND)
  assert.equal(snap.schemaVersion, PHASE9_C3_SNAPSHOT_SCHEMA_VERSION)
  assert.equal(snap.tradingDate, '2026-08-04')
  assert.equal(snap.processSessionId, 'c3-test-session')
  assert.equal(snap.trigger, 'scan_success')
  assert.equal(snap.meta.persistence.writer, PHASE9_C3_PERSISTENCE_WRITER)
  assert.equal(snap.meta.persistence.sourceOfTruth, 'json')
  assert.equal(snap.status.totalDecisions, 3)
  for (const key of RUNTIME_OBSERVATION_ARTIFACT_REQUIRED_KEYS) {
    assert.ok(key in snap, `snapshot root missing ${key}`)
    assert.ok(snap.runtimeObservationArtifact && key in snap.runtimeObservationArtifact)
  }

  const md = renderPhase9C3ObservationEntryMarkdown(snap)
  assert.ok(md.includes('source: auto'))
  assert.ok(md.includes('jsonSourceOfTruth'))
  assert.ok(md.includes('decisionId:'))
  assert.ok(md.includes('Runtime Observation Artifact'))
  assert.ok(md.includes('TARGET_DIFF'))
  assert.ok(md.includes('NOT COUNTED'))
  assert.ok(md.includes('INSUFFICIENT SAMPLE'))
  assert.ok(md.includes('selectedSource:'))
  assert.ok(md.includes('confidence:'))
}

console.log('phase9C3ObservationPersistence.selftest: OK')
