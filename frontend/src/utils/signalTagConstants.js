/** 股票筛选 / 信号快照：仅保留买点标签（不含冰） */

export const SCREEN_SNAPSHOT_SIGNAL_TAGS = ['强', '趋', '转', '突', '弹', '买']

export const SCREEN_SNAPSHOT_SIGNAL_TAG_SET = new Set(SCREEN_SNAPSHOT_SIGNAL_TAGS)

export function normalizeScreenSignalTag(tag) {
  return tag === '超' ? '趋' : tag
}

export function isScreenSnapshotSignalTag(tag) {
  return SCREEN_SNAPSHOT_SIGNAL_TAG_SET.has(normalizeScreenSignalTag(tag))
}
