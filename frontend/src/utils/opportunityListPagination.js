/**
 * Phase16.26-B — opportunity / snapshot list pagination policy (pure helpers).
 * Avoid dual pagination: never pre-slice displayData while Naive local-paginates.
 */

/** Show all rows without pager when result count is at or below this. */
export const OPPORTUNITY_LIST_PAGINATE_THRESHOLD = 50

/**
 * @param {unknown} count
 * @param {number} [threshold]
 * @returns {boolean}
 */
export function shouldPaginateOpportunityList(count, threshold = OPPORTUNITY_LIST_PAGINATE_THRESHOLD) {
  const n = Number(count)
  if (!Number.isFinite(n) || n <= 0) return false
  return n > threshold
}

/**
 * @param {unknown} count
 * @returns {string}
 */
export function formatOpportunityListCountLabel(count) {
  const n = Number(count)
  if (!Number.isFinite(n) || n < 0) return '共 0 只股票'
  return `共 ${n} 只股票`
}

/**
 * Slice rows for side effects (quotes) only — not for table :data when using local pagination.
 * @param {Array} rows
 * @param {{ page: number, pageSize: number, paginate: boolean }} opts
 */
export function sliceOpportunityPage(rows, { page, pageSize, paginate }) {
  const list = Array.isArray(rows) ? rows : []
  if (!paginate) return list
  const p = Math.max(1, Number(page) || 1)
  const size = Math.max(1, Number(pageSize) || 20)
  const start = (p - 1) * size
  return list.slice(start, start + size)
}
