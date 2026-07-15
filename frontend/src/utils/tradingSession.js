import { dayKeyToNum, normalizeDayKey } from './icePointSignals'

const CN_TZ = 'Asia/Shanghai'
const WEEKDAY_NUM = { Mon: 1, Tue: 2, Wed: 3, Thu: 4, Fri: 5, Sat: 6, Sun: 0 }

/** 上海时区日历今天 YYYY-MM-DD */
export function getChinaTodayKey(date = new Date()) {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: CN_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(date)
}

function getChinaTimeParts(date = new Date()) {
  const fmt = new Intl.DateTimeFormat('en-US', {
    timeZone: CN_TZ,
    weekday: 'short',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
  const parts = {}
  for (const p of fmt.formatToParts(date)) {
    if (p.type !== 'literal') parts[p.type] = p.value
  }
  return parts
}

/** A 股交易日 9:30 前（含周末则 false） */
export function isBeforeAShareMarketOpen(date = new Date()) {
  const { weekday, hour, minute } = getChinaTimeParts(date)
  if (!WEEKDAY_NUM[weekday] || WEEKDAY_NUM[weekday] >= 6) return false
  const hm = Number(hour) * 60 + Number(minute)
  return hm < 9 * 60 + 30
}

/** A 股连续竞价时段（9:30–11:30、13:00–15:00） */
export function isAShareMarketOpenNow(date = new Date()) {
  const { weekday, hour, minute } = getChinaTimeParts(date)
  if (!WEEKDAY_NUM[weekday] || WEEKDAY_NUM[weekday] >= 6) return false
  const hm = Number(hour) * 60 + Number(minute)
  return (hm >= 9 * 60 + 30 && hm < 11 * 60 + 30) || (hm >= 13 * 60 && hm < 15 * 60)
}

/**
 * 信号计算用的「最后一根有效 K 线」下标：
 * - 数据最后一根不是日历今天 → 用最后一根（未开盘时通常为昨收）
 * - 最后一根是今天且当前在 9:30 前 → 用上一根（避免未完成日 K）
 * - 否则 → 用最后一根（盘中/收盘后均为今日）
 */
export function resolveSignalLastBarIndex(dayKeys) {
  if (!dayKeys?.length) return -1
  const last = dayKeys.length - 1
  const todayKey = getChinaTodayKey()
  const lastKey = normalizeDayKey(dayKeys[last])
  if (lastKey !== todayKey) return last
  if (isBeforeAShareMarketOpen() && last > 0) return last - 1
  return last
}

export function resolveEffectiveSignalDayKey(dayKeys) {
  const idx = resolveSignalLastBarIndex(dayKeys)
  if (idx < 0) return ''
  return normalizeDayKey(dayKeys[idx])
}

/**
 * 信号回放：指定截止日 asOfDay 时，取 <= 该日的最后一根 K 线下标（与快照扫描一致）
 */
export function resolveSignalLastBarIndexForReplay(dayKeys, asOfDay = '') {
  const asOf = normalizeDayKey(String(asOfDay || '').trim())
  if (asOf && dayKeys?.length) {
    const target = dayKeyToNum(asOf)
    if (target) {
      let idx = -1
      for (let i = 0; i < dayKeys.length; i++) {
        const n = dayKeyToNum(normalizeDayKey(dayKeys[i]))
        if (n > 0 && n <= target) idx = i
      }
      if (idx >= 0) return idx
    }
  }
  return resolveSignalLastBarIndex(dayKeys)
}
