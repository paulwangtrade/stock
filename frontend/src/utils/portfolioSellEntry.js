/**
 * Paper sim portfolio sell entry helpers (Phase14-A-R1-B).
 * Only for paper_sim snapshot / quant holdings — never self-holdings tab.
 */

/** Normalize available sellable qty from snapshot or quant row shapes. */
export function maxSellQuantity(row) {
  if (!row || typeof row !== 'object') return 0
  const raw =
    row.availableQty ??
    row.available_qty ??
    row.sellable ??
    row.Sellable ??
    0
  const n = Math.trunc(Number(raw) || 0)
  return n > 0 ? n : 0
}

/**
 * True when row belongs to paper_sim read model (Portfolio snapshot / quant tab).
 * Rows without explicit account markers are treated as paper_sim (snapshot default).
 */
export function isPaperSimPosition(row) {
  if (!row || typeof row !== 'object') return false
  if (row.isSelfHolding === true || row.tab === 'self') return false
  const src = String(
    row.source ?? row.accountType ?? row.account_type ?? row.account ?? '',
  )
    .trim()
    .toLowerCase()
  if (!src) return true
  return src === 'paper_sim' || src === 'paper' || src.startsWith('paper_sim')
}

/**
 * Whether to show the sell button for a paper_sim position row.
 * Requires paper_sim context, positive available qty, positive total qty,
 * and canSell !== false when present.
 */
export function canShowSellButton(row) {
  if (!isPaperSimPosition(row)) return false
  const maxQty = maxSellQuantity(row)
  if (maxQty <= 0) return false
  const totalRaw =
    row.totalQty ??
    row.total_qty ??
    row.volume ??
    row.Volume ??
    0
  const totalQty = Math.trunc(Number(totalRaw) || 0)
  if (totalQty <= 0) return false
  if (row.canSell === false || row.can_sell === false) return false
  return true
}

/** Client-side guard: quantity must be integer in (0, maxQty]. Returns null if invalid. */
export function validateSellQuantity(quantity, maxQty) {
  const max = Math.trunc(Number(maxQty) || 0)
  const qty = Math.trunc(Number(quantity) || 0)
  if (max <= 0 || qty < 1 || qty > max) return null
  return qty
}

/** Stock code from portfolio snapshot or quant holdings row. */
export function sellRowStockCode(row) {
  if (!row) return ''
  return String(row.stockCode ?? row.stock_code ?? row.code ?? '').trim()
}

/** Stock name from portfolio snapshot or quant holdings row. */
export function sellRowStockName(row) {
  if (!row) return ''
  return String(row.stockName ?? row.stock_name ?? row.name ?? '').trim()
}
