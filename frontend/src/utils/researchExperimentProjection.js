/**
 * Research Experiment projection.
 *
 * Records a human hypothesis against a Research Finding reference
 * (and optionally report / backtest identity echoes). Does not score,
 * tune, run backtests, or create a strategy.
 *
 * Schema: experiment.v1
 */

export const RESEARCH_EXPERIMENT_SCHEMA = 'experiment.v1'
export const RESEARCH_FINDING_SCHEMA = 'research_finding.v1'
const REPORT_SCHEMA = 'research_report_projection.v1'
const RESULT_SCHEMA = 'backtest_result.v1'
const RECORDED_STATUS = 'recorded'

/** In-memory session list. Not durable. Not an API. */
const experimentStore = []

function asObj(value) {
  return value && typeof value === 'object' && !Array.isArray(value) ? value : null
}

function identityString(value) {
  return typeof value === 'string' && value.trim() ? value.trim() : null
}

function fnv1aHex(str) {
  let h = 0x811c9dc5
  const s = String(str)
  for (let i = 0; i < s.length; i += 1) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 0x01000193)
  }
  return (h >>> 0).toString(16).padStart(8, '0')
}

function stableJson(value) {
  if (value === null || typeof value !== 'object') return JSON.stringify(value)
  if (Array.isArray(value)) return `[${value.map((item) => stableJson(item)).join(',')}]`
  const keys = Object.keys(value).sort()
  return `{${keys.map((key) => `${JSON.stringify(key)}:${stableJson(value[key])}`).join(',')}}`
}

/**
 * Accept a finding_id string or a research_finding.v1 object. Never copies statistics.
 * @param {unknown} value
 * @returns {string|null}
 */
export function readFindingIdRef(value) {
  const direct = identityString(value)
  if (direct) return direct
  const obj = asObj(value)
  if (!obj) return null
  const fromIdentity = identityString(obj.identity?.finding_id)
  if (fromIdentity) return fromIdentity
  return identityString(obj.finding_id)
}

function emptyExperiment() {
  return {
    schema_version: RESEARCH_EXPERIMENT_SCHEMA,
    available: false,
    experiment_id: null,
    finding_id: null,
    hypothesis: null,
    referenced_strategy_identity: null,
    referenced_dataset_identity: null,
    referenced_backtest_result: null,
    status: null,
  }
}

function readReportEchoes(report) {
  const backtest = asObj(asObj(report)?.sections)?.backtest
  const bag = asObj(backtest)
  return {
    referenced_strategy_identity: identityString(bag?.strategy_identity),
    referenced_dataset_identity: identityString(bag?.dataset_identity),
    referenced_backtest_result: {
      schema_version: RESULT_SCHEMA,
      evaluator_identity: identityString(bag?.evaluator_identity),
    },
  }
}

/**
 * @param {object} [input]
 * @param {string} [input.hypothesis] caller sentence. Not inferred.
 * @param {string|object} [input.finding_id] or finding / researchFinding object — id only.
 * @param {object} [input.researchReport] optional identity echo.
 * @param {object} [input.backtestResult] optional schema gate when report path used.
 * @returns {object} experiment.v1
 */
export function projectResearchExperiment(input) {
  const bag = asObj(input)
  if (!bag) return emptyExperiment()

  const hypothesis = identityString(bag.hypothesis)
  if (!hypothesis) return emptyExperiment()

  const findingId = readFindingIdRef(
    bag.finding_id ?? bag.findingId ?? bag.finding ?? bag.researchFinding ?? bag.research_finding,
  )

  const report = asObj(bag.researchReport) || asObj(bag.research_report)
  const result = asObj(bag.backtestResult) || asObj(bag.backtest_result)
  const reportOk =
    !!report &&
    !!result &&
    identityString(report.schema_version) === REPORT_SCHEMA &&
    identityString(result.schema_version) === RESULT_SCHEMA

  // MVP primary: hypothesis + finding_id. Legacy: hypothesis + report + backtest.
  if (!findingId && !reportOk) return emptyExperiment()

  const echoes = reportOk
    ? readReportEchoes(report)
    : {
        referenced_strategy_identity: null,
        referenced_dataset_identity: null,
        referenced_backtest_result: null,
      }

  const experimentId = `rexp:${fnv1aHex(
    stableJson({
      finding_id: findingId,
      hypothesis,
      referenced_strategy_identity: echoes.referenced_strategy_identity,
      referenced_dataset_identity: echoes.referenced_dataset_identity,
      referenced_backtest_result: echoes.referenced_backtest_result,
    }),
  )}`

  return {
    schema_version: RESEARCH_EXPERIMENT_SCHEMA,
    available: true,
    experiment_id: experimentId,
    finding_id: findingId,
    hypothesis,
    referenced_strategy_identity: echoes.referenced_strategy_identity,
    referenced_dataset_identity: echoes.referenced_dataset_identity,
    referenced_backtest_result: echoes.referenced_backtest_result,
    status: RECORDED_STATUS,
  }
}

/**
 * Create (or replace same experiment_id) in the session store.
 * @param {object} [input]
 * @returns {object} experiment.v1
 */
export function createResearchExperiment(input) {
  const card = projectResearchExperiment(input)
  if (!card.available || !card.experiment_id) return card
  const idx = experimentStore.findIndex((row) => row.experiment_id === card.experiment_id)
  if (idx >= 0) experimentStore[idx] = card
  else experimentStore.push(card)
  return { ...card }
}

/**
 * @returns {object[]} recorded cards, sorted by experiment_id
 */
export function listResearchExperiments() {
  return experimentStore
    .slice()
    .sort((a, b) => {
      const ea = identityString(a?.experiment_id) || ''
      const eb = identityString(b?.experiment_id) || ''
      if (ea < eb) return -1
      if (ea > eb) return 1
      return 0
    })
    .map((row) => ({ ...row }))
}

/**
 * @param {string} experimentId
 * @returns {object|null}
 */
export function getResearchExperiment(experimentId) {
  const id = identityString(experimentId)
  if (!id) return null
  const found = experimentStore.find((row) => row.experiment_id === id)
  return found ? { ...found } : null
}

/** Test / session reset only. Not a product delete API. */
export function clearResearchExperimentStore() {
  experimentStore.length = 0
}
