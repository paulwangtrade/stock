/**
 * Shared ExitWatch display. Observation is not a sell order.
 * Unknown class and external_mirror both fail closed for sell intent.
 */

export const EXIT_WATCH_DISCLAIMER = '观察≠卖出指令'

export const EXIT_WATCH_CLASS = {
  HOLD: 'HOLD_OBSERVE',
  REDUCE: 'REDUCE_OBSERVE',
  FLATTEN: 'FLATTEN_OBSERVE',
  INSUFFICIENT: 'DATA_INSUFFICIENT',
}

export const EXIT_WATCH_SOURCE = {
  PAPER: 'paper_sim',
  MIRROR: 'external_mirror',
}

const KNOWN = new Set(Object.values(EXIT_WATCH_CLASS))

const REASON_LABEL = {
  TIME: '持有时间',
  LOSS: '浮亏',
  SIGNAL: '信号变化',
  PLAN: '计划异常',
  STALE: '行情过期',
  NO_SOURCE: '来源缺失',
  T1_LOCKED: '当日买入锁定',
}

function str(v) {
  if (v == null) return ''
  return String(v).trim()
}

function defaultLabel(klass) {
  switch (klass) {
    case EXIT_WATCH_CLASS.HOLD:
      return '持有观察'
    case EXIT_WATCH_CLASS.REDUCE:
      return '减仓观察'
    case EXIT_WATCH_CLASS.FLATTEN:
      return '清仓观察'
    default:
      return '数据不足'
  }
}

function insufficient(reason) {
  return {
    source: '',
    positionId: '',
    stockCode: '',
    stockName: '',
    class: EXIT_WATCH_CLASS.INSUFFICIENT,
    label: '数据不足',
    reasonCodes: [],
    reasonText: reason || '退出观察缺失或类别未知，失败关闭为数据不足',
    rule: 'INCOMPLETE',
    dedupKey: '',
    bar: '',
    policyRef: '',
    asOf: '',
    summary: reason || '退出观察缺失或类别未知，失败关闭为数据不足',
    sellIntentAllowed: false,
    manualSellDraftRef: '',
    disclaimer: EXIT_WATCH_DISCLAIMER,
    notAnOrder: true,
  }
}

/**
 * @param {object|null|undefined} raw
 * @param {'pending'|'loading'|'ready'|'error'} [phase]
 */
export function readExitWatchItem(raw, phase) {
  const p = str(phase || 'ready')
  if (p === 'pending' || p === 'loading') {
    return {
      ...insufficient('退出观察计算中'),
      label: '计算中',
      class: '',
      pending: true,
    }
  }
  if (p === 'error' || !raw || typeof raw !== 'object') {
    return insufficient(p === 'error' ? '退出观察读取失败，已失败关闭为数据不足' : '退出观察缺失，失败关闭为数据不足')
  }
  const klass = str(raw.class)
  if (!KNOWN.has(klass)) {
    return insufficient('退出观察类别未知，失败关闭为数据不足')
  }
  const source = str(raw.source).toLowerCase()
  const reasonCodes = Array.isArray(raw.reason_codes)
    ? raw.reason_codes.map((c) => str(c)).filter(Boolean)
    : Array.isArray(raw.reasonCodes)
      ? raw.reasonCodes.map((c) => str(c)).filter(Boolean)
      : []
  const exitClass = klass === EXIT_WATCH_CLASS.REDUCE || klass === EXIT_WATCH_CLASS.FLATTEN
  const serverSell = raw.sell_intent_allowed === true || raw.sellIntentAllowed === true
  const sellIntentAllowed = source === EXIT_WATCH_SOURCE.PAPER && exitClass && serverSell && klass !== EXIT_WATCH_CLASS.INSUFFICIENT
  return {
    source,
    positionId: str(raw.position_id ?? raw.positionId),
    stockCode: str(raw.stock_code ?? raw.stockCode),
    stockName: str(raw.stock_name ?? raw.stockName),
    class: klass,
    label: str(raw.label) || defaultLabel(klass),
    reasonCodes,
    reasonText: formatWatchReasons(reasonCodes),
    rule: str(raw.rule),
    dedupKey: str(raw.dedup_key ?? raw.dedupKey),
    bar: str(raw.bar),
    policyRef: str(raw.policy_ref ?? raw.policyRef),
    asOf: str(raw.as_of ?? raw.asOf),
    summary: str(raw.summary),
    sellIntentAllowed,
    manualSellDraftRef: sellIntentAllowed ? str(raw.manual_sell_draft_ref ?? raw.manualSellDraftRef) : '',
    disclaimer: EXIT_WATCH_DISCLAIMER,
    notAnOrder: true,
    pending: false,
  }
}

export function formatWatchReasons(codes) {
  const list = (Array.isArray(codes) ? codes : [])
    .map((c) => REASON_LABEL[str(c)] || '')
    .filter(Boolean)
  return list.join('、')
}

export function exitWatchTagType(klass) {
  switch (klass) {
    case EXIT_WATCH_CLASS.HOLD:
      return 'success'
    case EXIT_WATCH_CLASS.REDUCE:
      return 'warning'
    case EXIT_WATCH_CLASS.FLATTEN:
      return 'error'
    default:
      return 'default'
  }
}

/** Mirror never offers a sell draft. Paper only after a reduce/flatten observation. */
export function canOfferExitWatchSell(item) {
  if (!item || item.pending) return false
  if (item.source !== EXIT_WATCH_SOURCE.PAPER) return false
  if (item.class !== EXIT_WATCH_CLASS.REDUCE && item.class !== EXIT_WATCH_CLASS.FLATTEN) return false
  return item.sellIntentAllowed === true
}

export function filterExitWatchBySource(items, source) {
  const want = str(source).toLowerCase()
  return (Array.isArray(items) ? items : []).filter((raw) => str(raw?.source).toLowerCase() === want)
}
