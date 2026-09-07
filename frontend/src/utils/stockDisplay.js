/**
 * Phase16.14 StockDisplayModel — unified read-only stock label contract.
 * Extends Phase12-D2-A identity parsing; displayText rules per implementation brief.
 *
 * displayText:
 *   - real name + code → `名称(code)` e.g. 阳光电源(sz300274)
 *   - no real name → code only (never code(code))
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

function looksLikeEastMoneyCode(raw) {
  return /^\d{6}\.(SH|SZ|BJ|HK)$/i.test(trimStr(raw))
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
      stock_code: toSinaCode(marketIn, symbolIn) || trimStr(source.stock_code || source.stockCode).toLowerCase(),
    }
  }

  const rawCode = trimStr(source.stock_code || source.stockCode || source.code)
  if (rawCode) {
    const parsed = fromEastMoney(rawCode)
    if (parsed) return parsed
  }

  return null
}

/**
 * Real security short name only. Never returns a code-shaped string.
 * @param {object} source
 * @param {string} code normalized internal code for equality check
 */
export function pickRealStockName(source, code = '') {
  if (!source || typeof source !== 'object') return ''
  const n = trimStr(source.name || source.stock_name || source.stockName)
  if (!n) return ''
  if (looksLikeInternalStockCode(n)) return ''
  if (looksLikeEastMoneyCode(n)) return ''
  if (n === '未知名称' || n === '—' || n === '暂无记录') return ''
  if (n.toLowerCase() === 'missing') return ''
  const c = trimStr(code)
  if (c && n.toLowerCase() === c.toLowerCase()) return ''
  return n
}

function buildDisplayText(name, code) {
  const n = trimStr(name)
  const c = trimStr(code)
  if (n && c) return `${n}(${c})`
  if (c) return c
  if (n) return n
  return ''
}

/**
 * Phase16.14 contract model.
 * @returns {{ code: string, name: string, displayText: string, klineKey: string }}
 */
export function toStockDisplayModel(source) {
  if (!source || typeof source !== 'object') {
    return { code: '', name: '', displayText: '', klineKey: '' }
  }

  const rawCode = trimStr(source.stock_code || source.stockCode || source.code)
  const parsed = parseIdentity(source)
  const code = trimStr(parsed?.stock_code || rawCode)
  const name = pickRealStockName(source, code)
  const klineKey = trimStr(parsed?.display_code || '')
  const displayText = buildDisplayText(name, code)

  return {
    code,
    name,
    displayText,
    klineKey,
  }
}

/**
 * Phase12-D2-A StockDisplay identity mapper (read-only) + Phase16.14 fields.
 * @param {object} source market+symbol+name or stock_code+stock_name
 * @returns {object|null} StockDisplay (legacy) with Phase16.14 aliases
 */
export function toStockDisplay(source) {
  if (!source || typeof source !== 'object') return null
  const parsed = parseIdentity(source)
  if (!parsed || !parsed.display_code || !parsed.symbol || !parsed.market) return null

  const model = toStockDisplayModel(source)
  const name = model.name
  const display_name = name || UNKNOWN_STOCK_NAME
  const titleName = name || parsed.display_code

  return {
    market: parsed.market,
    symbol: parsed.symbol,
    name,
    display_name,
    display_code: parsed.display_code,
    stock_code: parsed.stock_code,
    // Phase16.14 contract
    code: model.code,
    displayText: model.displayText,
    klineKey: model.klineKey,
    click_action: {
      type: CLICK_KLINE_MODAL,
      chart_code: parsed.display_code,
      title: `${titleName} ${parsed.display_code} — 日K`,
    },
  }
}

export function applyStockClickAction(model, target) {
  if (!model || !target) return false
  const action = model.click_action
  if (action && action.type && action.type !== CLICK_KLINE_MODAL) {
    return false
  }
  // Unified chart code: always prefer Phase16.14 klineKey.
  const chartCode = trimStr(model.klineKey || action?.chart_code)
  if (!chartCode) return false

  const displayText = trimStr(model.displayText)
  const name = trimStr(model.name || model.display_name || model.displayName)
  target.visible = true
  target.title = trimStr(action?.title) || `${displayText || chartCode} — 多周期K线`
  target.chartCode = chartCode
  target.stockName = name || displayText || ''
  return true
}

/**
 * Enrich a Phase16.14 model for StockLink / applyStockClickAction.
 * chart_code is always klineKey (never sina code).
 */
export function toStockKlineLinkModel(model) {
  if (!model || typeof model !== 'object') return null
  const klineKey = trimStr(model.klineKey)
  if (!klineKey) return null
  const displayText = trimStr(model.displayText) || klineKey
  return {
    ...model,
    display_name: trimStr(model.name) || displayText,
    display_code: klineKey,
    displayText,
    klineKey,
    click_action: {
      type: CLICK_KLINE_MODAL,
      chart_code: klineKey,
      title: `${displayText} — 多周期K线`,
    },
  }
}

/** Drawer / panel title: `{displayText} · {theme}` */
export function buildStockDisplayTitle(model, theme = '解释') {
  const topic = trimStr(theme) || '解释'
  const text = trimStr(model?.displayText)
  if (text) return `${text} · ${topic}`
  return topic
}
