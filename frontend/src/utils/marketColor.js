/**
 * Phase16.26-A — A-share market / PnL color helpers (display only).
 * Up / profit → red; down / loss → green. Do NOT use Naive success for profit.
 */
import { DESIGN_TOKENS, DESIGN_TOKEN_VARS } from './designTokens.js'

/**
 * @param {unknown} value
 * @returns {number|null}
 */
export function parseMarketNumber(value) {
  if (value === null || value === undefined || value === '') return null
  if (typeof value === 'number') return Number.isFinite(value) ? value : null
  const s = String(value).trim().replace(/%/g, '').replace(/,/g, '')
  if (!s || s === '—' || s === '-') return null
  const n = Number(s)
  return Number.isFinite(n) ? n : null
}

/**
 * @param {unknown} value
 * @returns {'up'|'down'|'flat'|'empty'}
 */
export function marketDirection(value) {
  const n = parseMarketNumber(value)
  if (n === null) return 'empty'
  if (n > 0) return 'up'
  if (n < 0) return 'down'
  return 'flat'
}

/**
 * Resolve a concrete color (hex). Prefer for canvas / charts.
 * @param {unknown} value signed change, pnl, or return ratio
 * @param {{ flat?: string, empty?: string }} [opts]
 * @returns {string|undefined}
 */
export function marketColor(value, opts = {}) {
  const dir = marketDirection(value)
  if (dir === 'empty') return opts.empty
  if (dir === 'flat') return opts.flat ?? DESIGN_TOKENS.marketFlat
  if (dir === 'up') return DESIGN_TOKENS.marketUp
  return DESIGN_TOKENS.marketDown
}

/**
 * CSS var() form for Vue style bindings (follows designTokens.css).
 * @param {unknown} value
 * @param {{ flat?: string, empty?: string }} [opts]
 * @returns {string|undefined}
 */
export function marketColorCssVar(value, opts = {}) {
  const dir = marketDirection(value)
  if (dir === 'empty') return opts.empty
  if (dir === 'flat') return opts.flat ?? DESIGN_TOKEN_VARS.neutral
  if (dir === 'up') return DESIGN_TOKEN_VARS.marketUp
  return DESIGN_TOKEN_VARS.marketDown
}

/**
 * Inline style object: { color }.
 * @param {unknown} value
 * @param {{ flat?: string, empty?: string, useCssVar?: boolean }} [opts]
 * @returns {{ color?: string }}
 */
export function marketColorStyle(value, opts = {}) {
  const useVar = opts.useCssVar !== false
  const color = useVar ? marketColorCssVar(value, opts) : marketColor(value, opts)
  return color ? { color } : {}
}

/**
 * Utility class name for optional CSS hooks in designTokens.css.
 * @param {unknown} value
 * @returns {string}
 */
export function marketToneClass(value) {
  const dir = marketDirection(value)
  if (dir === 'up') return 'gs-market-up'
  if (dir === 'down') return 'gs-market-down'
  if (dir === 'flat') return 'gs-market-flat'
  return 'gs-market-empty'
}

/**
 * Alias for PnL / 收益率 / 涨幅 — same signed semantics as marketColor.
 * @param {unknown} value
 * @param {{ flat?: string, empty?: string, useCssVar?: boolean }} [opts]
 */
export function pnlColor(value, opts = {}) {
  return opts.useCssVar === false ? marketColor(value, opts) : marketColorCssVar(value, opts)
}

/**
 * Alias for change-rate / 涨跌幅 display color.
 * @param {unknown} value
 * @param {{ flat?: string, empty?: string, useCssVar?: boolean }} [opts]
 */
export function changeRateColor(value, opts = {}) {
  return pnlColor(value, opts)
}
