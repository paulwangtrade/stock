/**
 * Phase17.5 — Position origin / “为什么买入” display helpers (UI only).
 * Reuses Portfolio Provenance DTO + optional TradePlan.source_session.
 * Does not change provenance API or trading.
 */
import {
  ORIGIN_EMPTY,
  ORIGIN_SIGNAL_PRICE_FOOTER,
  formatOriginField,
  formatOriginSignalPrice,
  isOriginFieldMissing,
  resolveOriginSourceBucket,
} from './tradePlanOriginDisplay.js'
import {
  provenanceSourceChipMeta,
  resolveProvenanceSourceBucket,
} from './portfolioSourceChip.js'
import {
  buildProvenanceOriginDisplay,
  buildProvenanceSections,
} from './portfolioProvenanceDisplay.js'

export { ORIGIN_SIGNAL_PRICE_FOOTER }

const SESSION_LABEL = {
  after_close: '盘后生成',
  morning_rebuild: '早盘重建',
  cash_rescale: '仓位缩放',
  watchlist: '跟踪名单',
  t_sell: '人工卖出计划',
  exit_review: '退出复核',
}

/**
 * Resolve Strategy / Watchlist / Manual for one origin + optional plan session.
 * @param {object} origin raw ProvenanceOriginRow or display row
 * @param {string} [sourceSession] trade_plans.source_session
 */
export function resolvePositionOriginBucket(origin, sourceSession) {
  const session = String(sourceSession || '').trim()
  if (session) {
    const fromSession = resolveProvenanceSourceBucket(session)
    if (fromSession !== 'unknown') return fromSession
  }
  const o = origin && typeof origin === 'object' ? origin : {}
  const reason = o.reason && typeof o.reason === 'object' ? o.reason : {}
  return resolveOriginSourceBucket({
    strategyName: o.strategy,
    sourceReason: reason.source ?? o.buyReason,
    sourceSession: session,
  })
}

export function positionOriginSourceChip(origin, sourceSession) {
  return provenanceSourceChipMeta(resolvePositionOriginBucket(origin, sourceSession))
}

/** Human label for plan source_session (display only). */
export function formatPlanSourceSession(sourceSession) {
  const s = String(sourceSession || '').trim()
  if (!s) return ORIGIN_EMPTY
  const key = s.toLowerCase()
  if (SESSION_LABEL[key]) return `${SESSION_LABEL[key]}（${s}）`
  return s
}

/**
 * Build one “为什么买入” card from provenance origin + optional session.
 * @param {object} origin ProvenanceOriginRow
 * @param {{ sourceSession?: string, trade?: object }} [opts]
 */
export function buildPositionOriginCard(origin, opts = {}) {
  const session = String(opts.sourceSession || '').trim()
  const display = buildProvenanceOriginDisplay(origin)
  const bucket = resolvePositionOriginBucket(origin, session)
  const chip = provenanceSourceChipMeta(bucket)
  const whyParts = []
  if (chip.label && bucket !== 'unknown') whyParts.push(`来源渠道：${chip.label}`)
  if (!display.strategyMissing) whyParts.push(`策略：${display.strategy}`)
  if (!display.signalTagMissing) whyParts.push(`信号：${display.signalTag}`)
  if (!display.buyReasonMissing) whyParts.push(display.buyReason)
  else if (!display.selectionReasonMissing) whyParts.push(display.selectionReason)

  return {
    planId: display.planId,
    bucket,
    sourceChipLabel: chip.label,
    sourceChipType: chip.type,
    strategy: display.strategy,
    strategyMissing: display.strategyMissing,
    signalTag: display.signalTag,
    signalTagMissing: display.signalTagMissing,
    signalPrice: display.signalPrice,
    signalPriceMissing: display.signalPriceMissing,
    signalTime: display.signalTime,
    signalTimeMissing: display.signalTimeMissing,
    snapshotId: display.snapshotId,
    snapshotIdMissing: display.snapshotIdMissing,
    planSourceSession: formatPlanSourceSession(session),
    planSourceSessionMissing: !session,
    planSourceSessionRaw: session || '',
    buyReason: display.buyReason,
    buyReasonMissing: display.buyReasonMissing,
    selectionReason: display.selectionReason,
    selectionReasonMissing: display.selectionReasonMissing,
    whySummary: whyParts.length ? whyParts.join(' · ') : '暂无足够来源信息解释买入原因',
    trade: opts.trade || null,
  }
}

/**
 * Cards ordered by trades (with origin), then orphan origins.
 * @param {object|null} provenance PortfolioProvenanceView
 * @param {Record<number, string>} [sessionByPlanId] planId → source_session
 */
export function buildPositionOriginCards(provenance, sessionByPlanId = {}) {
  const sessions =
    sessionByPlanId && typeof sessionByPlanId === 'object' ? sessionByPlanId : {}
  const trades = Array.isArray(provenance?.trades) ? provenance.trades : []
  const origins = Array.isArray(provenance?.origins) ? provenance.origins : []
  const originByPlan = Object.create(null)
  for (const o of origins) {
    const pid = Math.trunc(Number(o?.planId) || 0)
    if (pid > 0) originByPlan[pid] = o
  }

  const cards = []
  const seen = new Set()
  for (const trade of trades) {
    const planId = Math.trunc(Number(trade?.planId) || 0)
    if (planId > 0) seen.add(planId)
    const sections = buildProvenanceSections([trade], origins)
    const tradeDisp = sections[0]?.trade || null
    cards.push(
      buildPositionOriginCard(originByPlan[planId] || { planId }, {
        sourceSession: sessions[planId] || '',
        trade: tradeDisp,
      }),
    )
  }

  for (const o of origins) {
    const planId = Math.trunc(Number(o?.planId) || 0)
    if (planId > 0 && seen.has(planId)) continue
    cards.push(
      buildPositionOriginCard(o, {
        sourceSession: sessions[planId] || '',
      }),
    )
  }
  return cards
}

/** Primary card for drawer hero (newest trade first already in sections order). */
export function pickPrimaryOriginCard(cards) {
  const list = Array.isArray(cards) ? cards : []
  return list[0] || null
}

export function formatOriginSignalPriceSafe(value) {
  return formatOriginSignalPrice(value)
}

export function formatOriginFieldSafe(value) {
  return formatOriginField(value)
}

export function isOriginMissing(value) {
  return isOriginFieldMissing(value)
}
