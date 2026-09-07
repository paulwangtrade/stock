/**
 * Phase16.14 StockDisplay adapters — map page DTOs → StockDisplayModel.
 * Read-only; no API / trading side effects.
 */
import { toStockDisplayModel, buildStockDisplayTitle } from './stockDisplay.js'

function firstNonEmpty(...vals) {
  for (const v of vals) {
    const s = v == null ? '' : String(v).trim()
    if (s) return s
  }
  return ''
}

/** TradePlan item / blocked / preview (snake_case). */
export function adaptTradePlanItem(item, nameHint = '') {
  if (!item || typeof item !== 'object') return toStockDisplayModel(null)
  return toStockDisplayModel({
    stock_code: item.stock_code ?? item.stockCode,
    stock_name: firstNonEmpty(item.stock_name, item.stockName, nameHint),
  })
}

/** TradePlan camelCase rows (sell fill / readiness). */
export function adaptTradePlanCamel(row, nameHint = '') {
  if (!row || typeof row !== 'object') return toStockDisplayModel(null)
  return toStockDisplayModel({
    stock_code: row.stockCode ?? row.stock_code,
    stock_name: firstNonEmpty(row.stockName, row.stock_name, nameHint),
  })
}

/** Origin item — often code-only. */
export function adaptTradePlanOrigin(item, nameHint = '') {
  if (!item || typeof item !== 'object') return toStockDisplayModel(null)
  const code = item.stock_code ?? item.stockCode
  if (String(code || '').trim() === '暂无记录') {
    return toStockDisplayModel(null)
  }
  return toStockDisplayModel({
    stock_code: code,
    stock_name: firstNonEmpty(item.stock_name, item.stockName, nameHint),
  })
}

/** Opportunity list row + optional projection (also accepts EastMoney SECUCODE rows). */
export function adaptOpportunity(row, projection) {
  const proj = projection && typeof projection === 'object' ? projection : null
  const r = row && typeof row === 'object' ? row : null
  return toStockDisplayModel({
    stock_code: firstNonEmpty(
      proj?.stock_code,
      proj?.stockCode,
      r?.stockCode,
      r?.stock_code,
      r?.SECUCODE,
      r?.secucode,
    ),
    stock_name: firstNonEmpty(
      proj?.stock_name,
      proj?.stockName,
      r?.stockName,
      r?.stock_name,
      r?.SECURITY_NAME_ABBR,
      r?.SECURITY_NAME,
      r?.name,
    ),
  })
}

/** Portfolio position row. */
export function adaptPortfolioPosition(row) {
  if (!row || typeof row !== 'object') return toStockDisplayModel(null)
  return toStockDisplayModel({
    stock_code: row.stockCode ?? row.stock_code ?? row.code,
    stock_name: row.stockName ?? row.stock_name ?? row.name,
  })
}

/** Provenance API view + optional table row. */
export function adaptProvenance(view, row) {
  const v = view && typeof view === 'object' ? view : null
  const r = row && typeof row === 'object' ? row : null
  return toStockDisplayModel({
    stock_code: firstNonEmpty(v?.stockCode, v?.stock_code, r?.stockCode, r?.stock_code),
    stock_name: firstNonEmpty(v?.stockName, v?.stock_name, r?.stockName, r?.stock_name),
  })
}

/** Research candidate row (snake_case API). Phase16.18-B1 K-line closure. */
export function adaptResearchCandidate(row) {
  if (!row || typeof row !== 'object') return toStockDisplayModel(null)
  return toStockDisplayModel({
    stock_code: row.stock_code ?? row.stockCode ?? row.code,
    stock_name: firstNonEmpty(row.stock_name, row.stockName, row.name),
  })
}

/** @deprecated Prefer adaptResearchCandidate — alias for StockLink mapping. */
export function researchCandidateToStockLink(row) {
  return adaptResearchCandidate(row)
}

/** Outcome row. */
export function adaptOutcome(item) {
  if (!item || typeof item !== 'object') return toStockDisplayModel(null)
  return toStockDisplayModel({
    stock_code: item.stock_code ?? item.stockCode,
    stock_name: item.stock_name ?? item.stockName,
  })
}

export { buildStockDisplayTitle, toStockDisplayModel }
export { applyStockClickAction, toStockKlineLinkModel } from './stockDisplay.js'
