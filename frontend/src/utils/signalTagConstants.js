/**
 * 股票筛选 / 信号快照允许进入快照的标签。
 * 「冰」由默认冰点扫描在 RSI 处于冰点区、且当日没有更高优先级标签时写入。
 * XS_MOM_TOP / MA_TREND / BREAKOUT_N 只由对应 strategy_id 的观察扫描写入。
 */
export const SCREEN_SNAPSHOT_SIGNAL_TAGS = ['强', '趋', '转', '突', '弹', '买', '冰', 'XS_MOM_TOP', 'MA_TREND', 'BREAKOUT_N']

export const SCREEN_SNAPSHOT_SIGNAL_TAG_SET = new Set(SCREEN_SNAPSHOT_SIGNAL_TAGS)

export function normalizeScreenSignalTag(tag) {
  return tag === '超' ? '趋' : tag
}

export function isScreenSnapshotSignalTag(tag) {
  return SCREEN_SNAPSHOT_SIGNAL_TAG_SET.has(normalizeScreenSignalTag(tag))
}
