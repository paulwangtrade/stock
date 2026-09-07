/**
 * Phase8-0 RiskIntel object types (JSDoc).
 * Contract: PHASE8_0_RISKINTEL_OBJECT_CONTRACT.md
 *
 * These typedefs are the Step0 skeleton only — no builders, no scan wiring.
 */

/**
 * Market discipline snapshot inside RiskContextBundle.
 * @typedef {Object} RiskMarketState
 * @property {number|null} level Effective market level 1–5; null if unknown (lower confidence)
 * @property {string} [levelName] Display name (e.g. 空仓防守) — not an order instruction
 * @property {string} [modeKey] Trading level / market mode key
 * @property {string} [marketCode] Market/index identity
 * @property {number|null} [globalLevel]
 * @property {number|null} [segmentLevel]
 * @property {string} [segmentKey]
 */

/**
 * Provenance for RiskContextBundle assembly.
 * @typedef {Object} RiskContextSource
 * @property {string} producer e.g. PRODUCER_WATCHLIST_SCAN
 * @property {string} [stockCode] Required when subject is a single stock
 * @property {string[]} [inputs] e.g. index_kline, sector_rank, daily_bars, follow_position
 * @property {string} [snapshotRef]
 */

/**
 * Context-only bundle (not Constraint / Action).
 * Minimal required: marketState, source, asOf, policyVersion, confidence.
 *
 * @typedef {Object} RiskContextBundle
 * @property {RiskMarketState} marketState
 * @property {RiskContextSource} source
 * @property {string} asOf ISO datetime of observation
 * @property {string} policyVersion e.g. POLICY_VERSION
 * @property {number} confidence Context completeness 0–1 (not order confidence)
 * @property {Object} [sector] Optional sector context extension
 * @property {Object} [stock] Optional stock context extension
 * @property {Object} [position] Optional position context extension
 * @property {'stock'|'portfolio'} [subjectScope]
 */

/**
 * Single explainability reason on RiskAdvice.
 * @typedef {Object} RiskAdviceReason
 * @property {string} code Stable machine-readable code (e.g. MARKET_LEVEL_1)
 * @property {string} message Human-readable explanation
 * @property {boolean} marketDriven Whether this reason is market-driven
 */

/**
 * Advice category (ADVICE_CATEGORY values).
 * @typedef {'market_discipline'|'position_assist'|'hold_observe'|'none'} RiskAdviceCategory
 */

/**
 * Explainable advice (not PlanFilter result, not Execution intent).
 * Minimal required: level, reasons, category, marketDriven.
 *
 * Forbidden on this object: mustSell, sellRatio, order, execution, etc.
 *
 * @typedef {Object} RiskAdvice
 * @property {number|null} level Defense / discipline level (aligns with market semantics; not an order tier)
 * @property {RiskAdviceReason[]} reasons At least one entry required at engine time
 * @property {RiskAdviceCategory} category
 * @property {boolean} marketDriven Whole-advice market-driven flag; must stay consistent with reasons
 * @property {'reduce'|'hold'|'add'|'none'} [suggestedAction] Optional soft enum
 * @property {{min: number, max: number}} [suggestedPctRange] Optional weak range — not sellRatio
 * @property {number} [confidence]
 * @property {string} [asOf]
 * @property {string} [policyVersion]
 * @property {{stockCode?: string, portfolioScope?: string}} [subject]
 */

export {}
