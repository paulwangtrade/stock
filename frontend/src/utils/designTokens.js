/**
 * Phase16.26-A — Design tokens (JS mirror of styles/designTokens.css).
 * Brand = blue; market up/profit = red; market down/loss = green (A-share).
 * Naive success/error remain system-status colors — never use for PnL/涨跌.
 */

/** Canonical hex values (keep in sync with designTokens.css). */
export const DESIGN_TOKENS = Object.freeze({
  brandPrimary: '#2080f0',
  brandPrimaryHover: '#4098fc',
  brandPrimaryPressed: '#1060c9',
  brandPrimarySuppl: '#4098fc',

  marketUp: '#ec0000',
  marketDown: '#00b578',
  marketFlat: '#8c8c8c',

  opportunity: '#f0a020',
  warning: '#f0a020',
  neutral: '#8c8c8c',

  /** System status only (Naive success) — NOT profit/up. */
  sysSuccess: '#18a058',
  sysSuccessHover: '#36ad6a',
  sysSuccessPressed: '#0c7a43',
  /** System danger only (Naive error) — NOT required for market up (use marketUp). */
  sysDanger: '#d03050',
  sysDangerHover: '#de576d',
  sysDangerPressed: '#ab1f3f',
  sysInfo: '#2080f0',
})

/**
 * Naive UI themeOverrides: primary = brand blue;
 * success = system OK only; do not map success → 盈利.
 */
export const naiveThemeOverrides = Object.freeze({
  common: {
    primaryColor: DESIGN_TOKENS.brandPrimary,
    primaryColorHover: DESIGN_TOKENS.brandPrimaryHover,
    primaryColorPressed: DESIGN_TOKENS.brandPrimaryPressed,
    primaryColorSuppl: DESIGN_TOKENS.brandPrimarySuppl,

    infoColor: DESIGN_TOKENS.sysInfo,
    infoColorHover: DESIGN_TOKENS.brandPrimaryHover,
    infoColorPressed: DESIGN_TOKENS.brandPrimaryPressed,
    infoColorSuppl: DESIGN_TOKENS.brandPrimarySuppl,

    successColor: DESIGN_TOKENS.sysSuccess,
    successColorHover: DESIGN_TOKENS.sysSuccessHover,
    successColorPressed: DESIGN_TOKENS.sysSuccessPressed,
    successColorSuppl: DESIGN_TOKENS.sysSuccessHover,

    warningColor: DESIGN_TOKENS.warning,
    warningColorHover: '#f7b94a',
    warningColorPressed: '#d9920b',
    warningColorSuppl: '#f7b94a',

    errorColor: DESIGN_TOKENS.sysDanger,
    errorColorHover: DESIGN_TOKENS.sysDangerHover,
    errorColorPressed: DESIGN_TOKENS.sysDangerPressed,
    errorColorSuppl: DESIGN_TOKENS.sysDangerHover,
  },
})

/** CSS custom property names (for style bindings). */
export const DESIGN_TOKEN_VARS = Object.freeze({
  brandPrimary: 'var(--color-brand-primary)',
  marketUp: 'var(--market-up)',
  marketDown: 'var(--market-down)',
  opportunity: 'var(--color-opportunity)',
  warning: 'var(--color-warning)',
  neutral: 'var(--color-neutral)',
})
