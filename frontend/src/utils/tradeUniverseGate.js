/** 我的策略：定时扫描（enable）与模拟交易宇宙（feedsTradePlan）拆开。 */

export const FEEDS_TRADE_PLAN_HINT = '仅模拟盘 · 非实盘 · 非自动下单'

export const FEEDS_TRADE_PLAN_CONFIRM =
  '打开后，该策略的扫描命中可以进入模拟交易计划（Track-B 纸面）。仍需人工确认草稿，不会自动下单，也不会进入实盘。观察或信号不等于交易指令。确定仅对模拟盘打开？'

export const UNHOOKED_TRADE_UNIVERSE_MESSAGE =
  '交易宇宙未挂接任何策略，需人工打开 feedsTradePlan'

export function defaultStrategyGate() {
  return { enable: false, feedsTradePlan: false }
}

/** 定时开关不得推导出交易宇宙资格。 */
export function buildStrategyGatePayload({ enable, cronExpr, feedsTradePlan }) {
  const cron = String(cronExpr || '').trim()
  return {
    enable: !!enable && !!cron,
    feedsTradePlan: !!feedsTradePlan,
  }
}

export function needsFeedsTradePlanConfirm({ isNew, previousFeedsTradePlan, nextFeedsTradePlan }) {
  return !!nextFeedsTradePlan && (isNew || !previousFeedsTradePlan)
}

export function tradeUniverseBadge(feedsTradePlan) {
  return feedsTradePlan ? '可进模拟计划' : '观察'
}

export function presetAdmissionLabel(preset) {
  if (preset?.planAdmission === 'historical-observe' || preset?.id === 'ice_buy') {
    return '历史交易宇宙候选 · 新建默认观察'
  }
  return '默认观察，不进模拟计划'
}

export function presetFeedsTradePlanDefault(preset) {
  return preset?.feedsTradePlan === true
}
