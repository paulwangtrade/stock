/**
 * 机会页信号筛选随策略收窄。
 * 冰点预设使用买点词表；观察扫描只用自己的标签（截面动量只有 XS_MOM_TOP）。
 * 未接入的计划策略没有词表，不能再拿「趋/强」去筛出空结果。
 */

export const ICE_SCREEN_SIGNAL_TAGS = ['强', '趋', '转', '突', '弹', '买', '冰']

const TAGS_BY_STRATEGY_ID = {
  default: ICE_SCREEN_SIGNAL_TAGS,
  ext_xsmom_v1: ['XS_MOM_TOP'],
  ext_ma_trend_v1: ['MA_TREND'],
  ext_breakout_v1: ['BREAKOUT_N'],
  ext_vol_mom_v1: [],
  ext_ma_pullback_v1: [],
  ext_meanrev_watch_v1: [],
}

const TAGS_BY_SCAN_KIND = {
  ice: ICE_SCREEN_SIGNAL_TAGS,
  xsmom: ['XS_MOM_TOP'],
  ma_trend: ['MA_TREND'],
  breakout: ['BREAKOUT_N'],
  vol_mom: [],
  ma_pullback: [],
  meanrev: [],
}

function copyTags(tags) {
  return Array.isArray(tags) ? tags.slice() : []
}

/**
 * @param {{ id?: string, scanKind?: string } | null | undefined} strategy
 * @returns {string[]}
 */
export function signalTagsForScreenStrategy(strategy) {
  const id = String(strategy?.id || '').trim()
  const scanKind = String(strategy?.scanKind || '').trim()
  if (!strategy || !id || id === 'default') return copyTags(ICE_SCREEN_SIGNAL_TAGS)
  if (Object.prototype.hasOwnProperty.call(TAGS_BY_STRATEGY_ID, id)) {
    return copyTags(TAGS_BY_STRATEGY_ID[id])
  }
  if (scanKind && Object.prototype.hasOwnProperty.call(TAGS_BY_SCAN_KIND, scanKind)) {
    return copyTags(TAGS_BY_SCAN_KIND[scanKind])
  }
  return copyTags(ICE_SCREEN_SIGNAL_TAGS)
}

/**
 * 单标签策略直接勾上该标签；多标签只保留仍在词表内的已选。
 * @param {string[] | null | undefined} previous
 * @param {string[] | null | undefined} allowed
 * @returns {string[]}
 */
export function reconcileScreenSignalTagSelection(previous, allowed) {
  const allow = copyTags(allowed).filter(Boolean)
  if (allow.length === 1) return [allow[0]]
  const allowedSet = new Set(allow)
  return copyTags(previous).filter((tag) => allowedSet.has(tag))
}
