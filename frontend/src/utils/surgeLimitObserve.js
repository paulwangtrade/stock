/**
 * 大涨 / 涨停降权（只观察）。
 * 入场、趋势确认命中时，若当日或近几日已经大涨 / 涨停，标成「动量末端观察」并降低展示顺序。
 * 不写 TradePlan，不生成委托，也不把命中改成买点。
 *
 * 涨跌停幅度：主板 10%、科创/创业板 20%、北交所 30%、名称含 ST 为 5%。
 * 板块或名称未知时失败关闭：可能已经接近最严的 5% 幅度，就不把命中当成干净确认。
 */
import { resolveStrategyRole } from './multiStrategyRole.js'
import { resolveStockMarketSegment } from './stockMarketSegment.js'

/** 与 signalBuyGuide.isStStockRow 同一规则：名称里出现 ST。这里不引用该文件，避免把 Vue 设置仓库拉进纯函数。 */
function nameLooksSt(name) {
  return String(name || '').toUpperCase().includes('ST')
}

export const SURGE_OBSERVE_LABEL = '动量末端观察'
export const SURGE_OBSERVE_COPY = '不是买卖指令；大涨后确认≠安全买点'

/** 只降权这两类角色的「命中」。减仓 / 止盈 / 其他保持原样。 */
export const SURGE_DEMOTE_ROLE_IDS = new Set(['entry', 'confirm'])

/** 机会列表里算作入场向命中的信号字。 */
export const SURGE_ENTRY_HIT_TAGS = new Set(['强', '趋', '转', '突', '弹', '买'])

const LIMIT_BY_SEGMENT = {
  shMain: 0.1,
  szMain: 0.1,
  star: 0.2,
  chinext: 0.2,
  beijing: 0.3,
}

/** 距名义涨停 0.3 个百分点内，或收盘价距涨停价 1 分以内，视为已近涨停。 */
const LIMIT_TOUCH_GAP = 0.003
/** 达到涨跌停幅度的七成，视为大涨。 */
const LARGE_BAND_RATIO = 0.7
/** 含当日在内的近端交易日。 */
const RECENT_SESSIONS = 3
/** 近端累计涨幅达到 1.5 倍涨跌停幅度，视为已经大涨。 */
const RECENT_CUM_BANDS = 1.5
/** 规则未知时用最严的 ST 5% 带做失败关闭，避免把未知标的当成主板。 */
const STRICT_UNKNOWN_LIMIT = 0.05

/**
 * @param {{ code?: string, name?: string }} input
 * @returns {{ known: boolean, limitPct?: number, kind?: string, segment?: string, reason?: string }}
 */
export function resolveAshareLimitBand({ code, name } = {}) {
  const segment = resolveStockMarketSegment(code)
  if (!segment || segment.key === 'unknown') {
    return { known: false, reason: 'unknown_board' }
  }
  const nameText = String(name ?? '').trim()
  if (!nameText) {
    return { known: false, reason: 'unknown_name', segment: segment.key }
  }
  if (nameLooksSt(nameText)) {
    return { known: true, limitPct: 0.05, kind: 'st', segment: segment.key }
  }
  const limitPct = LIMIT_BY_SEGMENT[segment.key]
  if (!Number.isFinite(limitPct)) {
    return { known: false, reason: 'unknown_board', segment: segment.key }
  }
  return { known: true, limitPct, kind: segment.key, segment: segment.key }
}

function positivePrice(value) {
  const n = Number(value)
  return Number.isFinite(n) && n > 0 ? n : null
}

function sessionMoves(closes) {
  const list = Array.isArray(closes) ? closes : []
  const moves = []
  for (let i = 1; i < list.length; i++) {
    const prev = positivePrice(list[i - 1])
    const cur = Number(list[i])
    if (!prev || !Number.isFinite(cur)) {
      moves.push(null)
      continue
    }
    moves.push({ prev, cur, ret: (cur - prev) / prev })
  }
  return moves
}

function cumulativeReturn(closes, sessions) {
  const list = Array.isArray(closes) ? closes : []
  if (list.length < 2) return null
  const end = list.length - 1
  const start = Math.max(0, end - sessions)
  const prev = positivePrice(list[start])
  const cur = Number(list[end])
  if (!prev || !Number.isFinite(cur)) return null
  return (cur - prev) / prev
}

function hitLimitUp(move, limitPct) {
  if (!move || !Number.isFinite(limitPct) || limitPct <= 0) return false
  const limitPrice = Math.round(move.prev * (1 + limitPct) * 100) / 100
  if (move.cur + 1e-8 >= limitPrice - 0.01) return true
  return move.ret >= limitPct - LIMIT_TOUCH_GAP
}

function hitLarge(move, limitPct) {
  return !!move && move.ret >= limitPct * LARGE_BAND_RATIO
}

function recentHot(moves, cum, limitPct) {
  const recent = moves.slice(-RECENT_SESSIONS)
  if (recent.some((move) => hitLimitUp(move, limitPct) || hitLarge(move, limitPct))) return true
  return cum != null && cum >= limitPct * RECENT_CUM_BANDS
}

/**
 * 用日 K 收盘判断当日 / 近端是否已经大涨或涨停。
 * state: clear | limit_up | large_move | unknown
 * unknown 且 failClosed：调用方不得把入场/确认命中当成干净买点。
 */
export function assessPriceSurge({ closes, code, name } = {}) {
  const moves = sessionMoves(closes)
  const today = moves.length ? moves[moves.length - 1] : null
  const band = resolveAshareLimitBand({ code, name })
  const cum = cumulativeReturn(closes, RECENT_SESSIONS)
  if (!today) {
    return {
      state: 'unknown',
      failClosed: true,
      reason: 'missing_prior_close',
      todayPct: null,
      recentPct: cum,
      band,
    }
  }
  if (!band.known) {
    const hot = recentHot(moves, cum, STRICT_UNKNOWN_LIMIT)
    return {
      state: 'unknown',
      failClosed: hot,
      reason: band.reason || 'unknown_band',
      todayPct: today.ret,
      recentPct: cum,
      band,
    }
  }
  const limitPct = band.limitPct
  const recent = moves.slice(-RECENT_SESSIONS)
  const todayLimit = hitLimitUp(today, limitPct)
  const recentLimit = recent.some((move) => hitLimitUp(move, limitPct))
  const todayLarge = hitLarge(today, limitPct)
  const recentLarge = recent.some((move) => hitLarge(move, limitPct))
  const cumLarge = cum != null && cum >= limitPct * RECENT_CUM_BANDS
  if (todayLimit || recentLimit) {
    return {
      state: 'limit_up',
      failClosed: false,
      reason: todayLimit ? 'same_day_limit' : 'recent_limit',
      todayPct: today.ret,
      recentPct: cum,
      band,
    }
  }
  if (todayLarge || recentLarge || cumLarge) {
    return {
      state: 'large_move',
      failClosed: false,
      reason: todayLarge ? 'same_day_large' : recentLarge ? 'recent_large' : 'recent_cumulative',
      todayPct: today.ret,
      recentPct: cum,
      band,
    }
  }
  return {
    state: 'clear',
    failClosed: false,
    reason: 'within_band',
    todayPct: today.ret,
    recentPct: cum,
    band,
  }
}

export function parseChangeRatePoints(value) {
  if (value == null || value === '') return null
  const n = Number(String(value).replace(/,/g, ''))
  return Number.isFinite(n) ? n : null
}

/** 行情涨跌幅是百分数（9.98 表示 9.98%）。有昨收和现价时优先用价格。 */
export function assessQuotedMove({ preClose, last, changeRate, code, name } = {}) {
  const prev = positivePrice(preClose)
  const cur = positivePrice(last)
  if (prev && cur) {
    return assessPriceSurge({ closes: [prev, cur], code, name })
  }
  const points = parseChangeRatePoints(changeRate)
  if (points == null) {
    return assessPriceSurge({ closes: [], code, name })
  }
  const base = 100
  return assessPriceSurge({ closes: [base, base * (1 + points / 100)], code, name })
}

function surgeNote(surge) {
  if (!surge || surge.state === 'unknown') return '涨跌停幅度未知，命中不作买点，仅作动量末端观察'
  if (surge.state === 'limit_up') return '已近涨停或近日涨停，仅作动量末端观察'
  return '已有较大涨幅，仅作动量末端观察'
}

export function surgeShouldDemote(roleId, verdict, surge) {
  if (!SURGE_DEMOTE_ROLE_IDS.has(roleId) || verdict !== '命中' || !surge) return false
  if (surge.state === 'limit_up' || surge.state === 'large_move') return true
  return surge.state === 'unknown' && surge.failClosed === true
}

/**
 * 给对照行加观察标注。不改结论原文，不增加分数或委托字段。
 */
export function annotateSurgeObservation(row, { roleId, surge } = {}) {
  const demote = surgeShouldDemote(roleId, row?.verdict, surge)
  const displayPriority = demote ? 2 : row?.verdict === '命中' ? 0 : 1
  if (!demote) {
    return { ...row, surgeObserve: null, displayPriority }
  }
  const note = surgeNote(surge)
  const reason = String(row?.reason || '').trim()
  return {
    ...row,
    reason: reason ? `${reason} · ${note}` : note,
    surgeObserve: {
      label: SURGE_OBSERVE_LABEL,
      copy: SURGE_OBSERVE_COPY,
      state: surge.state,
      reason: surge.reason,
      demote: true,
    },
    displayPriority,
  }
}

export function sortCompareRowsByPriority(rows) {
  return [...(rows || [])]
    .map((row, index) => ({ row, index }))
    .sort((a, b) => {
      const pa = Number.isFinite(Number(a.row?.displayPriority)) ? Number(a.row.displayPriority) : 1
      const pb = Number.isFinite(Number(b.row?.displayPriority)) ? Number(b.row.displayPriority) : 1
      if (pa !== pb) return pa - pb
      return a.index - b.index
    })
    .map((item) => item.row)
}

/**
 * 同一只股票的对照行共用一次涨幅判断，再按角色降权排序。
 * roleOf(strategyId) 返回 multiStrategyRole 的 id。
 */
export function decorateCompareRows(rows, { closes, code, name, roleOf } = {}) {
  const surge = assessPriceSurge({ closes, code, name })
  const decorated = (rows || []).map((row) => {
    const roleId = typeof roleOf === 'function' ? roleOf(row?.strategyId) : 'other'
    return annotateSurgeObservation(row, { roleId, surge })
  })
  return sortCompareRowsByPriority(decorated)
}

/**
 * 机会列表：入场/确认策略的入场向信号，在当日大涨或涨停后给出同一枚观察章。
 * 返回 null 表示不降权。
 */
export function resolveOpportunitySurgeBadge({
  strategyId,
  strategyMeta,
  tag,
  code,
  name,
  preClose,
  last,
  changeRate,
} = {}) {
  const signalTag = String(tag ?? '').trim()
  if (!SURGE_ENTRY_HIT_TAGS.has(signalTag)) return null
  const role = resolveStrategyRole(strategyId, strategyMeta || { kind: 'scan_preset' })
  if (!SURGE_DEMOTE_ROLE_IDS.has(role.id)) return null
  const surge = assessQuotedMove({ preClose, last, changeRate, code, name })
  const annotated = annotateSurgeObservation({ verdict: '命中', reason: '' }, { roleId: role.id, surge })
  return annotated.surgeObserve
}
