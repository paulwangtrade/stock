/**
 * @deprecated Prefer mapTSellDraftErrorMessage from tradePlansTSell.ts.
 * Kept for node test harness (tSellDraftErrors.test.mjs).
 */

export const T_SELL_DRAFT_ERROR_CODES = {
  insufficient_available: 'insufficient_available',
  cannot_sell: 'cannot_sell',
  invalid_stock: 'invalid_stock',
  invalid_quantity: 'invalid_quantity',
}

const USER_MESSAGES = {
  [T_SELL_DRAFT_ERROR_CODES.insufficient_available]: '可卖数量不足（含 T+1 锁定）',
  [T_SELL_DRAFT_ERROR_CODES.cannot_sell]: '该标的当前不可卖',
  [T_SELL_DRAFT_ERROR_CODES.invalid_stock]: '无此持仓或代码无效',
  [T_SELL_DRAFT_ERROR_CODES.invalid_quantity]: '数量须为正整数',
  INVALID_JSON: '请求格式无效',
  BAD_REQUEST: '请求无效',
}

/** Map backend error code to user-facing Chinese message. */
export function mapTSellDraftErrorMessage(code, httpStatus) {
  const key = String(code || '').trim()
  if (USER_MESSAGES[key]) return USER_MESSAGES[key]
  if (httpStatus === 404) return USER_MESSAGES[T_SELL_DRAFT_ERROR_CODES.invalid_stock]
  if (httpStatus === 409) return '该标的当前不可卖或数量冲突'
  if (httpStatus === 400) return '请求参数无效'
  return key ? `卖出草稿创建失败（${key}）` : '卖出草稿创建失败'
}
