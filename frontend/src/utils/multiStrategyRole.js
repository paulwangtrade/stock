/**
 * 多策略对照的角色标签。只标注设计意图。
 * 不写 TradePlan，不按角色加权，也不改日 K 扫描引擎。
 *
 * strategy_id 以对照目录 WIRED_SCAN_ENGINES 与设置页内置预设为准。
 * 买点扫描是 scan_*_v1；趋势 / 动量 / 突破 / 回踩 / 均值回归 / 截面动量
 * 是 ext_* 预设（对照行 id 为 preset:<id>）。未知 id → other。
 */

export const STRATEGY_ROLES = {
  entry: { id: 'entry', label: '入场', tagType: 'success' },
  confirm: { id: 'confirm', label: '趋势确认', tagType: 'info' },
  reduce: { id: 'reduce', label: '减仓防守', tagType: 'warning' },
  exit: { id: 'exit', label: '止盈清仓', tagType: 'error' },
  other: { id: 'other', label: '其他观察', tagType: 'default' },
}

export const STRATEGY_ROLE_FOOTER =
  '角色仅表示设计意图，命中不等于交易指令，不按角色自动打分。'

/**
 * 精确 strategy_id。
 * 对照里的参数预设写成 preset:<id>，裸 id 与前缀形式都登记。
 */
export const STRATEGY_ROLE_BY_ID = {
  scan_strong_v1: 'entry',
  scan_trend_v1: 'entry',
  scan_reversal_v1: 'entry',
  scan_breakout_v1: 'entry',
  scan_rebound_v1: 'entry',
  scan_ice_buy_v1: 'entry',
  default: 'entry',
  'preset:default': 'entry',

  ext_xsmom_v1: 'confirm',
  'preset:ext_xsmom_v1': 'confirm',
  ext_ma_trend_v1: 'confirm',
  'preset:ext_ma_trend_v1': 'confirm',
  ext_vol_mom_v1: 'confirm',
  'preset:ext_vol_mom_v1': 'confirm',
  ext_breakout_v1: 'confirm',
  'preset:ext_breakout_v1': 'confirm',
  ext_ma_pullback_v1: 'confirm',
  'preset:ext_ma_pullback_v1': 'confirm',
  ext_meanrev_watch_v1: 'confirm',
  'preset:ext_meanrev_watch_v1': 'confirm',

  scan_reduce_v1: 'reduce',
  scan_take_profit_v1: 'exit',
  scan_ice_v1: 'other',
}

/**
 * 预设的 scanKind / templateId。
 * 同一引擎新建的预设没有独立 strategy_id，角色仍与该引擎一致。
 */
export const STRATEGY_ROLE_BY_ENGINE = {
  ice: 'entry',
  ice_point: 'entry',
  xsmom: 'confirm',
  ext_xsmom_v1: 'confirm',
  ma_trend: 'confirm',
  ext_ma_trend_v1: 'confirm',
  breakout: 'confirm',
  ext_breakout_v1: 'confirm',
  vol_mom: 'confirm',
  ext_vol_mom_v1: 'confirm',
  ma_pullback: 'confirm',
  ext_ma_pullback_v1: 'confirm',
  meanrev: 'confirm',
  ext_meanrev_watch_v1: 'confirm',
}

function roleById(id) {
  const key = STRATEGY_ROLE_BY_ID[id]
  return key ? STRATEGY_ROLES[key] : null
}

/**
 * @param {string} strategyId
 * @param {{ kind?: string, scanKind?: string, templateId?: string }} [meta]
 */
export function resolveStrategyRole(strategyId, meta) {
  const id = String(strategyId ?? '').trim()
  const direct = roleById(id)
  if (direct) return direct

  const scanKind = String(meta?.scanKind || '').trim()
  const templateId = String(meta?.templateId || '').trim()
  const engineKey = STRATEGY_ROLE_BY_ENGINE[scanKind] || STRATEGY_ROLE_BY_ENGINE[templateId]
  if (engineKey) return STRATEGY_ROLES[engineKey]
  if (scanKind || templateId) return STRATEGY_ROLES.other

  const kind = String(meta?.kind || '').trim()
  // 当前对照里未标注引擎的参数预设，都是默认买点参数引擎的副本。
  if (kind === 'scan_preset' || id.startsWith('preset:')) return STRATEGY_ROLES.entry
  return STRATEGY_ROLES.other
}
