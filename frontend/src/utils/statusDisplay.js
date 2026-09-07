/**
 * Phase16.21-D — Beta-facing status / field labels (display only).
 * Never mutate API payloads or internal state machines; map for UI.
 */

/** @typedef {'default'|'info'|'success'|'warning'|'error'} NaiveTagType */
/** @typedef {{ label: string, type: NaiveTagType, tooltip: string, key: string }} StatusDisplay */

/**
 * @param {unknown} raw
 * @returns {string}
 */
function norm(raw) {
  return String(raw ?? '')
    .trim()
    .toLowerCase()
}

/**
 * @param {string} key
 * @param {string} label
 * @param {NaiveTagType} type
 * @param {string} tooltip
 * @returns {StatusDisplay}
 */
function pack(key, label, type, tooltip) {
  return { key, label, type, tooltip }
}

const RESEARCH = {
  new: pack('new', '新建', 'default', '新进入研究池，尚未标注'),
  watching: pack('watching', '观察中', 'info', '已加入观察，可持续跟踪'),
  reviewed: pack('reviewed', '已复盘', 'success', '已完成人工复盘记录'),
  discarded: pack('discarded', '已丢弃', 'warning', '已从当前研究关注中排除'),
}

const PLAN = {
  draft: pack('draft', '草稿', 'warning', '计划仍可编辑；不会自动买卖'),
  ready: pack('ready', '已准备', 'success', '策略条件已满足，等待执行流程'),
  frozen: pack('frozen', '已锁定', 'success', '计划已冻结，可供模拟执行读取'),
  approved: pack('approved', '已批准', 'info', '已人工批准，通常仍需冻结后才执行'),
  cancelled: pack('cancelled', '已取消', 'default', '计划已取消'),
  executed: pack('executed', '已执行', 'success', '模拟执行已完成'),
}

const PLAN_ITEM = {
  ...PLAN,
  filled: pack('filled', '已成交', 'success', '该项模拟成交已完成'),
  pending: pack('pending', '待处理', 'warning', '等待后续流程'),
  skipped: pack('skipped', '已跳过', 'default', '该项被跳过，不参与执行'),
  open: pack('open', '进行中', 'info', '该项仍在流程中'),
}

const INTENT = {
  priced: pack('priced', '已定价', 'info', '早盘已写入限价与数量参考'),
  ready: pack('ready', '已准备', 'success', '策略条件已满足，等待执行流程'),
  executed: pack('executed', '已执行', 'success', '意图已执行'),
  done: pack('done', '已完成', 'success', '意图流程已完成'),
  gap_skip: pack('gap_skip', '跳过缺口', 'warning', '因开盘缺口规则跳过'),
}

const EXECUTION_DISPLAY = {
  executed: pack('executed', '已执行', 'success', '模拟执行已完成'),
  ready: pack(
    'ready',
    '模拟执行准备',
    'info',
    '限价与数量已齐备，等待模拟执行流程。不会自动真实下单。',
  ),
  waiting_intent: pack(
    'waiting_intent',
    '待早盘准备',
    'warning',
    '盘后草稿阶段尚未生成执行限价/数量；早盘准备后才会出现参考限价。这不是系统故障。',
  ),
}

const EXPLANATION = {
  ok: pack('ok', '解释完整', 'success', '策略解释字段齐全（只读）'),
  degraded: pack(
    'degraded',
    '部分可恢复',
    'warning',
    '部分历史解释信息缺失，但核心策略数据仍可读取。这不是系统错误。',
  ),
  missing: pack(
    'missing',
    '暂无历史解释',
    'warning',
    '该计划创建时未保存完整策略解释，无法恢复历史决策依据。旧计划常见，属正常。',
  ),
  gated: pack('gated', '未授权', 'info', '当前权益未开通高级策略解释'),
  failed: pack('failed', '生成失败', 'error', '解释生成失败，请稍后重试'),
}

const READINESS = {
  ready: pack('ready', '模拟执行准备', 'success', '策略条件已满足，等待模拟执行流程'),
  not_ready: pack('not_ready', '未就绪', 'warning', '尚不满足执行准备条件'),
  pending: pack('pending', '进行中', 'info', '流程尚未完成'),
  pass: pack('pass', '通过', 'success', '检查已通过'),
  fail: pack('fail', '未通过', 'error', '检查未通过'),
  warning: pack('warning', '需注意', 'warning', '存在需关注项'),
}

const DOMAIN_MAP = {
  research: RESEARCH,
  plan: PLAN,
  plan_item: PLAN_ITEM,
  intent: INTENT,
  execution: EXECUTION_DISPLAY,
  explanation: EXPLANATION,
  readiness: READINESS,
  generic: { ...RESEARCH, ...PLAN_ITEM, ...INTENT, ...EXECUTION_DISPLAY, ...EXPLANATION, ...READINESS },
}

/**
 * Map any user-visible status enum to Chinese label + tag type + tooltip.
 * Unknown values fall back to original string (never invent business meaning).
 *
 * @param {unknown} status
 * @param {keyof typeof DOMAIN_MAP | string} [domain='generic']
 * @returns {StatusDisplay}
 */
export function formatStatus(status, domain = 'generic') {
  const key = norm(status)
  if (!key) {
    return pack('', '—', 'default', '')
  }
  const table = DOMAIN_MAP[domain] || DOMAIN_MAP.generic
  if (table[key]) return { ...table[key] }
  // Common aliases
  if (key === 'waiting_intent' || key === 'waiting-intent') {
    return { ...EXECUTION_DISPLAY.waiting_intent }
  }
  const raw = String(status).trim()
  return pack(key, raw, 'default', raw)
}

/** Draft-stage limit/volume placeholder (UI only; internal zero remains). */
export function pendingMorningPrepLabel() {
  return '待早盘准备'
}

export function pendingMorningPrepTooltip() {
  return '等待盘前生成执行方案（限价与数量）。盘后草稿阶段为 0 属正常，不是错误。'
}

/**
 * Shared field tooltips for score / signal / risk / prices.
 * @param {string} field
 * @returns {string}
 */
export function formatFieldTooltip(field) {
  switch (String(field || '').trim().toLowerCase()) {
    case 'score':
    case '评分':
      return '策略模型综合评分，不代表收益预测'
    case 'signal':
    case '信号':
    case 'signal_tag':
      return '策略信号标签（如强/趋），用于快速浏览，不代表买卖指令'
    case 'risk':
    case '风险':
      return '风控或相对分差提示，仅供观察；不代表账户真实风险评级'
    case 'ref':
    case 'ref_price':
    case '策略参考价':
      return '生成交易计划时记录的价格锚点'
    case 'limit':
    case 'limit_price':
    case '限价':
    case '限价（参考）':
      return '早盘准备后的参考限价，仅供观察，不会自动下单'
    case 'last':
    case 'last_price':
    case '最新行情价':
      return '最近一次行情数据，仅用于展示'
    case 'signal_price':
    case '信号触发价':
      return '信号触发时记录的价格，仅供对照'
    default:
      return ''
  }
}
