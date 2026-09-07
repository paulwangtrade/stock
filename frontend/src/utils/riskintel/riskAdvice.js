/**
 * buildRiskAdvice — Phase8-0 Step2
 * RiskContextBundle → RiskAdvice (explainable; not Execution / PlanFilter).
 */

import { ADVICE_CATEGORY, POLICY_VERSION } from './constants.js'

/**
 * Forbidden property keys on RiskAdvice output tree.
 * Includes user Step2 list + legacy action leaks.
 */
export const ADVICE_OUTPUT_FORBIDDEN_KEYS = Object.freeze([
  'mustSell',
  'sellRatio',
  'sellPositionPct',
  'addPositionPct',
  'rushReducePct',
  'positionAction',
  'targetPosition',
  'order',
  'orders',
  'orderSide',
  'orderQty',
  'execution',
  'executionId',
  'executionPayload',
  'broker',
  'brokerRequest',
  'brokerOrderId',
  'reduce',
  'increase',
  'close',
  'liquidate',
  'forceLiquidate',
  'forceClear',
  'holdingAdvice',
  'action',
  'actionLabel',
  'suggestPct',
  'suggestPctDisplay',
  'mustBuy',
  'buyRatio',
])

/**
 * @param {unknown} value
 * @param {string[]} [acc]
 * @returns {string[]}
 */
export function scanForbiddenAdviceKeys(value, acc = []) {
  if (value == null || typeof value !== 'object') return acc
  if (Array.isArray(value)) {
    for (const item of value) scanForbiddenAdviceKeys(item, acc)
    return acc
  }
  for (const [k, v] of Object.entries(value)) {
    if (ADVICE_OUTPUT_FORBIDDEN_KEYS.includes(k) && !acc.includes(k)) acc.push(k)
    scanForbiddenAdviceKeys(v, acc)
  }
  return acc
}

/**
 * @param {unknown} level
 * @returns {number|null}
 */
function normalizeLevel(level) {
  if (level == null || level === '') return null
  const n = Number(level)
  if (!Number.isFinite(n) || n < 1 || n > 5) return null
  return n
}

/**
 * @param {string} code
 * @param {string} message
 * @param {boolean} marketDriven
 */
function reason(code, message, marketDriven) {
  return { code, message, marketDriven: !!marketDriven }
}

/**
 * @param {object|null|undefined} bundle
 */
function assertBundle(bundle) {
  if (!bundle || typeof bundle !== 'object') {
    throw new TypeError('buildRiskAdvice: contextBundle (RiskContextBundle) is required')
  }
  if (!bundle.marketState || typeof bundle.marketState !== 'object') {
    throw new TypeError('buildRiskAdvice: contextBundle.marketState is required')
  }
}

/**
 * @param {object} bundle
 * @returns {{ code: string, message: string, marketDriven: boolean }[]}
 */
function buildReasons(bundle) {
  const level = normalizeLevel(bundle.marketState?.level)
  const levelName = String(bundle.marketState?.levelName || '').trim()
  const reasons = []

  if (level === 1) {
    reasons.push(
      reason(
        'MARKET_LEVEL_1',
        levelName
          ? `市场纪律：当前为${level}级（${levelName}），以降敞口纪律为主，非个股强制卖出指令`
          : '市场纪律：当前为1级空仓防守档，以降敞口纪律为主，非个股强制卖出指令',
        true,
      ),
    )
  } else if (level === 2) {
    reasons.push(
      reason(
        'MARKET_LEVEL_2',
        levelName
          ? `市场纪律：当前为${level}级（${levelName}），宜谨慎控制仓位，非下单指令`
          : '市场纪律：当前为2级战略撤退档，宜谨慎控制仓位，非下单指令',
        true,
      ),
    )
  } else if (level === 3) {
    reasons.push(
      reason(
        'MARKET_LEVEL_3',
        levelName
          ? `市场纪律：当前为${level}级（${levelName}），以观望与控制新开仓为主`
          : '市场纪律：当前为3级中性观望档，以观望与控制新开仓为主',
        true,
      ),
    )
  } else if (level === 4 || level === 5) {
    reasons.push(
      reason(
        `MARKET_LEVEL_${level}`,
        levelName
          ? `市场纪律：当前为${level}级（${levelName}），偏进攻纪律档，仍非自动下单`
          : `市场纪律：当前为${level}级偏进攻纪律档，仍非自动下单`,
        true,
      ),
    )
  } else {
    reasons.push(
      reason(
        'CONTEXT_SPARSE',
        '市场级别不足或未知，暂不给出可执行意味的持仓结论',
        false,
      ),
    )
  }

  const sector = bundle.sector
  if (sector && typeof sector === 'object') {
    if (sector.inflow === true) {
      reasons.push(
        reason(
          'SECTOR_INFLOW',
          sector.name
            ? `板块资金偏流入（${sector.name}），仅作辅助观察`
            : '板块资金偏流入，仅作辅助观察',
          false,
        ),
      )
    } else if (sector.inflow === false) {
      reasons.push(
        reason(
          'SECTOR_OUTFLOW',
          sector.name
            ? `板块资金偏流出（${sector.name}），仅作辅助观察`
            : '板块资金偏流出，仅作辅助观察',
          false,
        ),
      )
    }
  }

  const stock = bundle.stock
  if (stock && typeof stock === 'object' && stock.volumeRatio != null) {
    const vr = Number(stock.volumeRatio)
    if (Number.isFinite(vr)) {
      if (stock.priceUp === true && vr >= 1.25) {
        reasons.push(
          reason('VOLUME_EXPAND_UP', `个股量比约 ${vr.toFixed(2)} 且价涨，仅作量价观察`, false),
        )
      } else if (stock.priceUp === false && vr >= 1.25) {
        reasons.push(
          reason('VOLUME_EXPAND_DOWN', `个股量比约 ${vr.toFixed(2)} 且价跌，仅作量价观察`, false),
        )
      }
    }
  }

  // Guaranteed non-empty (including low-context / none category cases).
  if (reasons.length === 0) {
    reasons.push(reason('CONTEXT_SPARSE', '上下文不足，暂无额外解释项', false))
  }

  return reasons
}

/**
 * Sector/stock assist reasons only — CONTEXT_SPARSE is not "assist".
 * @param {{ code: string, marketDriven: boolean }[]} reasons
 */
function hasSectorOrStockAssist(reasons) {
  return reasons.some(
    (r) =>
      !r.marketDriven &&
      (r.code === 'SECTOR_INFLOW' ||
        r.code === 'SECTOR_OUTFLOW' ||
        r.code === 'VOLUME_EXPAND_UP' ||
        r.code === 'VOLUME_EXPAND_DOWN'),
  )
}

/**
 * @param {number|null} level
 * @param {{ code: string, message: string, marketDriven: boolean }[]} reasons
 * @param {object} bundle
 * @returns {string}
 */
function resolveCategory(level, reasons, bundle) {
  const confidence = Number(bundle.confidence)
  const hasAssist = hasSectorOrStockAssist(reasons)
  const sparse = level == null

  // Design priority: level==null and no sector/stock signal → none
  if (sparse && !hasAssist) {
    return ADVICE_CATEGORY.NONE
  }
  if (Number.isFinite(confidence) && confidence < 0.35 && sparse && !hasAssist) {
    return ADVICE_CATEGORY.NONE
  }
  if (level === 1 || level === 2) {
    return ADVICE_CATEGORY.MARKET_DISCIPLINE
  }
  if ((level === 4 || level === 5) && hasAssist) {
    return ADVICE_CATEGORY.POSITION_ASSIST
  }
  if (hasAssist && level === 3) {
    return ADVICE_CATEGORY.POSITION_ASSIST
  }
  if (level === 3 || level === 4 || level === 5) {
    return ADVICE_CATEGORY.HOLD_OBSERVE
  }
  // sparse + sector/stock assist only
  return hasAssist ? ADVICE_CATEGORY.POSITION_ASSIST : ADVICE_CATEGORY.NONE
}

/**
 * Build RiskAdvice from a RiskContextBundle only.
 *
 * @param {object} contextBundle RiskContextBundle from buildRiskContextBundle
 * @param {object} [options] Reserved; must not carry order/execution/sellRatio semantics
 * @returns {object} RiskAdvice
 */
export function buildRiskAdvice(contextBundle, options = {}) {
  assertBundle(contextBundle)
  // options reserved for future policy flags; ignore action-like keys if present
  void options

  const level = normalizeLevel(contextBundle.marketState?.level)
  const reasons = buildReasons(contextBundle)

  // Hard rule: level 1/2 must have marketDriven reason
  if ((level === 1 || level === 2) && !reasons.some((r) => r.marketDriven === true)) {
    reasons.unshift(
      reason(
        level === 1 ? 'MARKET_LEVEL_1' : 'MARKET_LEVEL_2',
        '市场纪律：防守档必须附带可解释的市场驱动原因',
        true,
      ),
    )
  }

  const rootMarketDriven = reasons.some((r) => r.marketDriven === true)
  const category = resolveCategory(level, reasons, contextBundle)

  /** @type {object} */
  const advice = {
    level,
    category,
    marketDriven: rootMarketDriven,
    reasons: reasons.map((r) => ({
      code: String(r.code),
      message: String(r.message),
      marketDriven: !!r.marketDriven,
    })),
  }

  // Safe optional audit fields (not forbidden)
  if (contextBundle.asOf) advice.asOf = contextBundle.asOf
  advice.policyVersion = contextBundle.policyVersion || POLICY_VERSION
  if (typeof contextBundle.confidence === 'number') {
    advice.confidence = contextBundle.confidence
  }
  const stockCode = contextBundle.source?.stockCode
  if (stockCode) {
    advice.subject = { stockCode: String(stockCode) }
  }

  return advice
}
