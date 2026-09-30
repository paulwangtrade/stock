/**
 * 实盘镜像观察（external_mirror）录入口径。
 * 仅观察：不进 paper_sim 账本、不进交易计划、不下单。
 */

export const EXTERNAL_MIRROR_SOURCE = 'external_mirror'

/** 镜像页固定文案。 */
export const EXTERNAL_MIRROR_DISCLAIMER = '仅观察 · 不进模拟账本 · 不进交易计划 · 不下单'

export const PAPER_SIM_MIRROR_HINT = '真实持股请到「实盘镜像（观察）」录入，不要当作模拟仓。'

export const LIVE_BROKER_PLACEHOLDER =
  '实盘券商未启用。Beta 默认关闭 Track-A，不对接券商下单。LLM 与信号不能从本账户交易。'

export const COPY_TO_PAPER_STUB = '复制到模拟账户演练（后续）'

/** 账户/组合三页签。legacy paper_accounts 不是第三账户。 */
export const ACCOUNT_BOOK_TABS = [
  { id: 'paper_sim', label: '模拟量化' },
  { id: 'external_mirror', label: '实盘镜像（观察）' },
  { id: 'live_broker', label: '实盘券商（未启用）' },
]

export function todayISODate(now = new Date()) {
  const d = now instanceof Date ? now : new Date()
  const z = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${z(d.getMonth() + 1)}-${z(d.getDate())}`
}

/**
 * Client-side fail-closed check. Server remains authoritative.
 * @returns {{ ok: boolean, reason: string, message: string }}
 */
export function validateMirrorDraft(input) {
  const code = String(input?.stockCode ?? input?.stock_code ?? '').trim()
  const qty = Number(input?.quantity)
  const cost = Number(input?.costPrice ?? input?.cost_price)
  const entry = String(input?.entryDate ?? input?.entry_date ?? '').trim()
  const source = String(input?.source ?? EXTERNAL_MIRROR_SOURCE).trim().toLowerCase()
  if (!code) {
    return { ok: false, reason: 'missing_code', message: '请填写代码' }
  }
  if (!Number.isFinite(qty) || qty <= 0 || !Number.isInteger(qty)) {
    return { ok: false, reason: 'invalid_quantity', message: '数量必须为大于 0 的整数' }
  }
  if (!Number.isFinite(cost) || cost <= 0) {
    return { ok: false, reason: 'invalid_cost', message: '成本价必须大于 0' }
  }
  if (entry && !/^\d{4}-\d{2}-\d{2}$/.test(entry)) {
    return { ok: false, reason: 'invalid_entry_date', message: '录入日格式应为 YYYY-MM-DD' }
  }
  if (source !== EXTERNAL_MIRROR_SOURCE) {
    return { ok: false, reason: 'invalid_source', message: '来源必须为实盘镜像，不能写入模拟账本' }
  }
  if (input?.feedsTradePlan === true || input?.tradable === true) {
    return { ok: false, reason: 'observation_only', message: '镜像持仓仅观察，不能进入交易' }
  }
  return { ok: true, reason: '', message: '' }
}

export function isExternalMirrorRow(row) {
  if (!row || typeof row !== 'object') return false
  const src = String(row.source ?? row.accountType ?? row.account_type ?? '')
    .trim()
    .toLowerCase()
  return src === EXTERNAL_MIRROR_SOURCE || row.tab === 'external_mirror'
}
