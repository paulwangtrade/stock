/**
 * Paper-sim Exit observation display (Track-B).
 * Reads the server projection. Unknown / missing class fails closed to 数据不足.
 * Does not create sell drafts.
 */
import { canShowSellButton } from './portfolioSellEntry.js'

export const EXIT_OBSERVE_CLASS = {
  HOLD: 'HOLD_OBSERVE',
  REDUCE: 'REDUCE_OBSERVE',
  FLATTEN: 'FLATTEN_OBSERVE',
  INSUFFICIENT: 'DATA_INSUFFICIENT',
}

export const EXIT_OBSERVATION_DISCLAIMER =
  '非交易指令 · 不进入实盘 · 需人工确认才可走模拟卖出'

const KNOWN = new Set(Object.values(EXIT_OBSERVE_CLASS))

function defaultLabel(klass) {
  switch (klass) {
    case EXIT_OBSERVE_CLASS.HOLD:
      return '持有观察'
    case EXIT_OBSERVE_CLASS.REDUCE:
      return '减仓观察'
    case EXIT_OBSERVE_CLASS.FLATTEN:
      return '清仓观察'
    default:
      return '数据不足'
  }
}

function insufficientObservation(reason) {
  return {
    pending: false,
    class: EXIT_OBSERVE_CLASS.INSUFFICIENT,
    label: '数据不足',
    reason: reason || '退出观察缺失或类别未知，失败关闭为数据不足',
    sellIntentAllowed: false,
    disclaimer: EXIT_OBSERVATION_DISCLAIMER,
  }
}

function pendingObservation() {
  return {
    pending: true,
    class: '',
    label: '评价中',
    reason: '退出观察计算中',
    sellIntentAllowed: false,
    disclaimer: EXIT_OBSERVATION_DISCLAIMER,
  }
}

function flagTrue(raw, snake, camel) {
  if (!raw || typeof raw !== 'object') return false
  if (raw[snake] === true || raw[camel] === true) return true
  return false
}

/**
 * @param {object|null|undefined} raw server observation
 * @param {'idle'|'pending'|'loading'|'ready'|'error'} phase
 */
export function readExitObservation(raw, phase) {
  const p = String(phase || 'ready')
  if (p === 'idle' || p === 'pending' || p === 'loading') return pendingObservation()
  const klass = String(raw?.class || '').trim()
  if (!KNOWN.has(klass)) {
    return insufficientObservation(
      p === 'error'
        ? '退出观察请求失败，失败关闭为数据不足'
        : '退出观察缺失或类别未知，失败关闭为数据不足',
    )
  }
  const writes =
    flagTrue(raw, 'persist_sell_plans', 'persistSellPlans') ||
    flagTrue(raw, 'writes_trade_plan', 'writesTradePlan')
  const sellClass =
    klass === EXIT_OBSERVE_CLASS.REDUCE || klass === EXIT_OBSERVE_CLASS.FLATTEN
  const explicitAllow = raw?.sellIntentAllowed === true || raw?.sell_intent_allowed === true
  const sellAllowed = sellClass && !writes && explicitAllow
  return {
    pending: false,
    class: klass,
    label: String(raw?.label || '').trim() || defaultLabel(klass),
    reason: String(raw?.reason || '').trim() || '—',
    sellIntentAllowed: sellAllowed,
    disclaimer: EXIT_OBSERVATION_DISCLAIMER,
  }
}

export function exitObservationTagType(klass) {
  switch (klass) {
    case EXIT_OBSERVE_CLASS.HOLD:
      return 'success'
    case EXIT_OBSERVE_CLASS.REDUCE:
      return 'warning'
    case EXIT_OBSERVE_CLASS.FLATTEN:
      return 'error'
    case EXIT_OBSERVE_CLASS.INSUFFICIENT:
      return 'default'
    default:
      return 'default'
  }
}

/** Optional sim-sell intent button. Never true for 持有观察 / 数据不足 / pending. */
export function canOfferExitSellIntent(obs, row) {
  if (!obs || obs.pending || !obs.sellIntentAllowed) return false
  if (obs.class !== EXIT_OBSERVE_CLASS.REDUCE && obs.class !== EXIT_OBSERVE_CLASS.FLATTEN) {
    return false
  }
  return canShowSellButton(row)
}

export function exitSellIntentConfirmCopy(obs) {
  const label = obs?.label || '退出观察'
  const reason = obs?.reason || '—'
  return [
    EXIT_OBSERVATION_DISCLAIMER,
    '确认后仅打开模拟卖出草稿，需再填写数量并提交。',
    '不会自动成交，也不会仅因退出观察标签写入交易计划。',
    `${label}：${reason}`,
  ].join('\n')
}
