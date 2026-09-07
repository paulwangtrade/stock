/**
 * Phase16.23 — Beta friend-trial safety copy (display only).
 * No license / no API / no trading semantics change.
 */

export const BETA_SIM_MODE_TITLE = 'Beta 模拟研究模式'

/** Short banner for confirm dialogs. */
export const BETA_SIM_MODE_BANNER =
  '当前为 Beta 模拟研究模式。该操作不会真实下单，也不会连接券商。'

export const BETA_SIM_MODE_HINT =
  '确认后仅影响本地模拟研究数据；可随时取消。'

/**
 * Build multi-line dialog body (plain string).
 * @param {string[]} detailLines
 */
export function betaConfirmContent(detailLines = []) {
  const lines = [BETA_SIM_MODE_BANNER, BETA_SIM_MODE_HINT, '', ...detailLines.filter(Boolean)]
  return lines.join('\n')
}
