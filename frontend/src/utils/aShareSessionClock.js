/** A 股连续竞价时钟（无额外依赖；供 tradingSession / klineCache 复用）。 */

const CN_TZ = 'Asia/Shanghai'
const WEEKDAY_NUM = { Mon: 1, Tue: 2, Wed: 3, Thu: 4, Fri: 5, Sat: 6, Sun: 0 }

export function getChinaTimeParts(date = new Date()) {
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

/** A 股连续竞价时段（9:30–11:30、13:00–15:00） */
export function isAShareMarketOpenNow(date = new Date()) {
  const { weekday, hour, minute } = getChinaTimeParts(date)
  if (!WEEKDAY_NUM[weekday] || WEEKDAY_NUM[weekday] >= 6) return false
  const hm = Number(hour) * 60 + Number(minute)
  return (hm >= 9 * 60 + 30 && hm < 11 * 60 + 30) || (hm >= 13 * 60 && hm < 15 * 60)
}

/** Phase17-B.1: K 线 LIVE = 连续竞价中；否则 IDLE。 */
export function isKlineMarketLive(date = new Date()) {
  return isAShareMarketOpenNow(date)
}
