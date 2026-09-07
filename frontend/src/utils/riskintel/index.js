/**
 * Phase8-0 Risk Intelligence (Watchlist Context / Advice).
 * Not backend PlanFilter, ApproveGate, Execution, or Broker.
 */

export {
  POLICY_VERSION,
  PRODUCER_WATCHLIST_SCAN,
  ADVICE_CATEGORY,
  FORBIDDEN_FIELDS,
} from './constants.js'

export {
  buildRiskContextBundle,
  scanForbiddenBundleKeys,
  BUNDLE_OUTPUT_FORBIDDEN_KEYS,
} from './riskContextBundle.js'

export {
  buildRiskAdvice,
  scanForbiddenAdviceKeys,
  ADVICE_OUTPUT_FORBIDDEN_KEYS,
} from './riskAdvice.js'

export {
  buildScanRiskAdviceShadow,
  attachScanRiskAdviceShadow,
} from './scanRiskAdviceShadow.js'

export {
  resetScanRiskAdviceShadowMetrics,
  setScanRiskAdviceShadowMetricsEnabled,
  recordScanRiskAdviceShadowGenerated,
  recordScanRiskAdviceShadowFailed,
  getScanRiskAdviceShadowMetrics,
  exportScanRiskAdviceShadowMetricsJSON,
} from './scanRiskAdviceShadowMetrics.js'

export {
  compareLegacyRiskAdvice,
  observeLegacyRiskAdvicePair,
  normalizeAdviceLevel,
  resetScanRiskAdviceShadowEvaluation,
  setScanRiskAdviceShadowEvaluationEnabled,
  getScanRiskAdviceShadowEvaluation,
  getTopDisagreementReasons,
  getShadowEvaluationSummary,
  formatShadowEvaluationReport,
  getShadowMigrationReadiness,
  getScanRiskAdviceShadowObservation,
  exportScanRiskAdviceShadowObservationJSON,
  resetScanRiskAdviceShadowObservation,
  setScanRiskAdviceShadowObservationEnabled,
} from './scanRiskAdviceShadowEvaluation.js'

export {
  OBSERVATION_CATEGORY,
  OBSERVATION_REASON,
  classifyLegacyRiskAdviceObservation,
} from './scanRiskAdviceShadowObservation.js'

export {
  buildRiskAdviceProjection,
  compareLegacyHoldingToProjection,
  mapRiskAdviceToProjectedAction,
  resolveCurrentHoldingState,
  RISK_ADVICE_PROJECTION_SOURCE,
  PROJECTED_ACTION,
} from './riskAdviceProjection.js'

export {
  resetRiskAdviceProjectionMetrics,
  setRiskAdviceProjectionMetricsEnabled,
  getRiskAdviceProjectionMetrics,
  exportRiskAdviceProjectionMetricsJSON,
} from './riskAdviceProjectionMetrics.js'

export {
  getHoldingDecision,
  resolveAuthorityHoldingDecision,
  resolveHoldingDecisionDiff,
  classifyHoldingDecisionComparison,
  simulateRiskAdviceSource,
  mapProjectionToSimulatedDecision,
  getHoldingMigrationReadiness,
  getHoldingMigrationStability,
  getHoldingDecisionPreferredSource,
  setHoldingDecisionAuthoritySource,
  enableHoldingDecisionControlledSwitch,
  rollbackHoldingDecisionToLegacy,
  isHoldingDecisionControlledSwitchEnabled,
  getControlledSwitchObservations,
  resetControlledSwitchObservations,
  recordControlledSwitchObservation,
  getControlledSwitchMetrics,
  getControlledSwitchStatus,
  CONTROLLED_SWITCH_FALLBACK_DEGRADED,
  CONTROLLED_SWITCH_FALLBACK_ROLLBACK,
  CONTROLLED_SWITCH_DIFF_DEGRADED,
  CONTROLLED_SWITCH_DIFF_ROLLBACK,
  HOLDING_DECISION_SOURCE_LEGACY,
  HOLDING_DECISION_SOURCE_PROJECTION,
  HOLDING_DECISION_COMPARISON,
  HOLDING_MIGRATION_MIN_SAMPLES,
  ROLLING_WINDOW_SIZE,
} from './holdingDecisionAdapter.js'

export {
  resetHoldingDecisionAdapterMetrics,
  setHoldingDecisionAdapterMetricsEnabled,
  getHoldingDecisionAdapterMetrics,
  getShadowMigrationMetrics,
  getHoldingDecisionRecentOutcomes,
  exportHoldingDecisionAdapterMetricsJSON,
} from './holdingDecisionAdapterMetrics.js'

export {
  getHoldingMigrationGateSnapshot,
  getHoldingMigrationTrend,
  formatHoldingMigrationGateReport,
  getHoldingMigrationGateHistory,
  resetHoldingMigrationGateHistory,
  HOLDING_MIGRATION_GATE_HISTORY_SIZE,
} from './holdingMigrationGate.js'

export {
  evaluateHealthyWindowEligibility,
  getPhase9C3ObservationBundle,
  exportPhase9C3ObservationBundleJSON,
  buildPhase9C3ObservationCheckpoint,
  getLastPhase9C3ObservationCheckpoint,
  rememberPhase9C3ObservationCheckpoint,
  PHASE9_C3_OBSERVATION_CHECKPOINT_EVENT,
} from './phase9C3ObservationBundle.js'

export {
  beijingTradingDateFromISO,
  buildRuntimeObservationArtifact,
  buildPhase9C3ObservationSnapshotDocument,
  renderPhase9C3ObservationEntryMarkdown,
  persistPhase9C3ObservationSnapshot,
  PHASE9_C3_PERSISTENCE_WRITER,
  PHASE9_C3_SNAPSHOT_KIND,
  PHASE9_C3_SNAPSHOT_SCHEMA_VERSION,
  RUNTIME_OBSERVATION_ARTIFACT_REQUIRED_KEYS,
} from './phase9C3ObservationPersistence.js'

// Type typedefs live in ./types.js (JSDoc).
