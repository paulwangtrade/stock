/**
 * 股票筛选 / 信号快照允许进入快照的标签（不含冰）。
 * XS_MOM_TOP / MA_TREND / BREAKOUT_N 只由对应 strategy_id 的观察扫描写入；
 * 默认冰点扫描仍只产出前面的买点标签。
 */
export const SCREEN_SNAPSHOT_SIGNAL_TAGS = ['强', '趋', '转', '突', '弹', '买', 'XS_MOM_TOP', 'MA_TREND', 'BREAKOUT_N']

export const SCREEN_SNAPSHOT_SIGNAL_TAG_SET = new Set(SCREEN_SNAPSHOT_SIGNAL_TAGS)

export function normalizeScreenSignalTag(tag) {
  return tag === '超' ? '趋' : tag
}

export function isScreenSnapshotSignalTag(tag) {
  return SCREEN_SNAPSHOT_SIGNAL_TAG_SET.has(normalizeScreenSignalTag(tag))
}
