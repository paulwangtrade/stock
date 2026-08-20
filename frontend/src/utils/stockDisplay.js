/**
 * Phase12-D2-A StockDisplay identity mapper (read-only).
 * Reuses toEastMoneyCode. Does not resolve names from DB / SECURITY_SHORT_NAME.
 */
import { toEastMoneyCode } from './stockCode.js'

export const UNKNOWN_STOCK_NAME = '未知名称'
export const CLICK_KLINE_MODAL = 'kline_modal'

const MARKET_SET = new Set(['SH', 'SZ', 'BJ', 'HK'])

function trimStr(v) {
  if (v == null) return ''
  return String(v).trim()
}

/** Internal sina-style codes must never be used as a display name. */
export function looksLikeInternalStockCode(raw) {
  const s = trimStr(raw)
  if (!s) return false
  return /^(sh|sz|bj|hk)\d{6}$/i.test(s)
}

function normalizeMarket(raw) {
  const m = trimStr(raw).toUpperCase()
  if (m === 'SS') return 'SH'
  if (MARKET_SET.has(m)) return m
  return ''
}

function sixDigits(raw) {
  const s = trimStr(raw)
  if (/^\d{6}$/.test(s)) return s
  return ''
}

function toSinaCode(market, symbol) {
  const m = normalizeMarket(market)
  const sym = sixDigits(symbol)
  if (!m || !sym) return ''
  return `${m.toLowerCase()}${sym}`
}

function fromEastMoney(displayCode) {
  const em = toEastMoneyCode(displayCode)
  const m = em.match(/^(\d{6})\.(SH|SZ|BJ|HK)$/i)
  if (!m) return null
  const symbol = m[1]
  const market = m[2].toUpperCase()
  return {
    market,
    symbol,
    display_code: `${symbol}.${market}`,
    stock_code: toSinaCode(market, symbol),
  }
}

function parseIdentity(source) {
  const marketIn = normalizeMarket(source.market)
  const symbolIn = sixDigits(source.symbol)
  if (marketIn && symbolIn) {
    const display_code = `${symbolIn}.${marketIn}`
    return {
      market: marketIn,
      symbol: symbolIn,
      display_code,
      stock_code: toSinaCode(marketIn, symbolIn) || trimStr(source.stock_code).toLowerCase(),
    }
  }

  const rawCode = trimStr(source.stock_code || source.code)
  if (rawCode) {
    const parsed = fromEastMoney(rawCode)
    if (parsed) return parsed
  }

  return null
}

function pickName(source) {
  const n = trimStr(source.name || source.stock_name)
  if (!n) return ''
  if (looksLikeInternalStockCode(n)) return ''
  if (/^\d{6}\.(SH|SZ|BJ|HK)$/i.test(n)) return ''
  return n
}

/**
 * @param {object} source market+symbol+name or stock_code+stock_name
 * @returns {object|null} StockDisplayModel
 */
export function toStockDisplay(source) {
  if (!source || typeof source !== 'object') return null
  const parsed = parseIdentity(source)
  if (!parsed || !parsed.display_code || !parsed.symbol || !parsed.market) return null

  const name = pickName(source)
  const display_name = name || UNKNOWN_STOCK_NAME
  const titleName = name || parsed.display_code

  return {
    market: parsed.market,
    symbol: parsed.symbol,
    name,
    display_name,
    display_code: parsed.display_code,
    stock_code: parsed.stock_code,
    click_action: {
      type: CLICK_KLINE_MODAL,
      chart_code: parsed.display_code,
      title: `${titleName} ${parsed.display_code} — 日K`,
    },
  }
}

export function applyStockClickAction(model, target) {
  const action = model?.click_action
  if (!action || action.type !== CLICK_KLINE_MODAL || !action.chart_code || !target) {
    return false
  }
  target.visible = true
  target.title = action.title
  target.chartCode = action.chart_code
  target.stockName = model.display_name || ''
  return true
}
