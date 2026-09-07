/**
 * Exit Review display helpers (Phase14-D2).
 * Pure functions for list sort/filter, action visibility, and risk-factor checklist.
 */

export const EXIT_EVAL_STATE = {
  NORMAL: 'NORMAL',
  WATCH: 'WATCH',
  REVIEW_REQUIRED: 'REVIEW_REQUIRED',
}

export const EXIT_REASON = {
  TIME_REVIEW: 'TIME_REVIEW',
  LOSS_REVIEW: 'LOSS_REVIEW',
  PLAN_REVIEW: 'PLAN_REVIEW',
}

export const EXIT_EVAL_FILTER = {
  ALL: 'all',
  WATCH_PLUS: 'watch_plus',
  REVIEW_REQUIRED: 'review_required',
}

export const EXIT_REVIEW_DECISION = {
  HOLD: 'HOLD',
  WATCH: 'WATCH',
  CREATE_SELL_PLAN: 'CREATE_SELL_PLAN',
}

export function exitReviewDecisionLabel(decision) {
  switch (String(decision || '').trim()) {
    case EXIT_REVIEW_DECISION.HOLD:
      return '继续持有'
    case EXIT_REVIEW_DECISION.WATCH:
      return '加入观察'
    case EXIT_REVIEW_DECISION.CREATE_SELL_PLAN:
      return '生成卖出计划'
    default:
      return String(decision || '').trim() || '—'
  }
}

export function formatOutcomeReviewTime(iso) {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return String(iso)
  return d.toLocaleString('zh-CN', { hour12: false })
}

/** Whether row has a persisted latest outcome (for list badge). */
export function hasLatestExitOutcome(row) {
  return !!String(row?.latestOutcome?.id || row?.latest_outcome?.id || '').trim()
}

export function latestExitOutcomeSummary(row) {
  const o = row?.latestOutcome || row?.latest_outcome
  if (!o?.decision) return ''
  const when = formatOutcomeReviewTime(o.reviewTime || o.review_time)
  return `已于 ${when} 记录：${exitReviewDecisionLabel(o.decision)}`
}

/** Rank for sort: higher = more urgent. */
export function exitEvalStateRank(state) {
  switch (String(state || '').trim()) {
    case EXIT_EVAL_STATE.REVIEW_REQUIRED:
      return 3
    case EXIT_EVAL_STATE.WATCH:
      return 2
    case EXIT_EVAL_STATE.NORMAL:
      return 1
    default:
      return 0
  }
}

export function exitEvalRowState(row) {
  return String(row?.evaluation?.state || EXIT_EVAL_STATE.NORMAL).trim()
}

/** D2: show list action only for WATCH / REVIEW_REQUIRED. */
export function shouldShowExitReviewAction(row) {
  const st = exitEvalRowState(row)
  return st === EXIT_EVAL_STATE.WATCH || st === EXIT_EVAL_STATE.REVIEW_REQUIRED
}

/** Naive UI button props for 「查看复评」. */
export function exitReviewActionButtonProps(state) {
  const st = String(state || '').trim()
  if (st === EXIT_EVAL_STATE.REVIEW_REQUIRED) {
    return { type: 'primary', quaternary: true }
  }
  if (st === EXIT_EVAL_STATE.WATCH) {
    return { type: 'default', quaternary: true }
  }
  return { type: 'default', text: true }
}

export function exitReasonLabel(code) {
  switch (String(code || '').trim()) {
    case EXIT_REASON.TIME_REVIEW:
      return '持有时间关注'
    case EXIT_REASON.LOSS_REVIEW:
      return '亏损关注'
    case EXIT_REASON.PLAN_REVIEW:
      return '计划变化关注'
    default:
      return String(code || '').trim() || '—'
  }
}

export function sortExitEvalHoldings(holdings) {
  const list = Array.isArray(holdings) ? [...holdings] : []
  list.sort((a, b) => {
    const dr = exitEvalStateRank(exitEvalRowState(b)) - exitEvalStateRank(exitEvalRowState(a))
    if (dr !== 0) return dr
    return String(a?.stockCode || '').localeCompare(String(b?.stockCode || ''))
  })
  return list
}

export function filterExitEvalByMode(holdings, mode) {
  const list = Array.isArray(holdings) ? holdings : []
  const m = String(mode || EXIT_EVAL_FILTER.ALL).trim()
  if (m === EXIT_EVAL_FILTER.REVIEW_REQUIRED) {
    return list.filter((row) => exitEvalRowState(row) === EXIT_EVAL_STATE.REVIEW_REQUIRED)
  }
  if (m === EXIT_EVAL_FILTER.WATCH_PLUS) {
    return list.filter((row) => {
      const st = exitEvalRowState(row)
      return st === EXIT_EVAL_STATE.WATCH || st === EXIT_EVAL_STATE.REVIEW_REQUIRED
    })
  }
  return list
}

/** Join holding evaluation row by stock code (case-insensitive). */
export function findHoldingEvalRow(holdingsEval, stockCode) {
  const code = String(stockCode || '').trim().toLowerCase()
  if (!code || !holdingsEval?.holdings) return null
  return (
    holdingsEval.holdings.find((h) => String(h?.stockCode || '').trim().toLowerCase() === code) ||
    null
  )
}

function hasReason(reasonCodes, code) {
  const set = new Set((reasonCodes || []).map((c) => String(c || '').trim()))
  return set.has(code)
}

function fmtPctRatio(ratio) {
  if (ratio == null || ratio === '' || !Number.isFinite(Number(ratio))) return null
  return `${(Number(ratio) * 100).toFixed(1)}%`
}

/**
 * Risk-factor checklist for ExitReviewDrawer §5.
 * @returns {Array<{ code, label, triggered, fact, detail }>}
 */
export function buildExitReviewRiskFactors(input) {
  const holdingDays = Number(input?.holdingDays) || 0
  const unrealizedReturn =
    input?.unrealizedReturn != null && Number.isFinite(Number(input.unrealizedReturn))
      ? Number(input.unrealizedReturn)
      : null
  const reasonCodes = Array.isArray(input?.reasonCodes) ? input.reasonCodes : []
  const state = String(input?.state || '').trim()
  const policy = input?.policy || {}
  const maxDays = Number(policy.maxHoldingDays) || 20
  const lossWatch = Number(policy.lossWatchThreshold)
  const lossReview = Number(policy.lossReviewThreshold)
  const lossWatchPct = Number.isFinite(lossWatch) ? lossWatch : -0.05
  const lossReviewPct = Number.isFinite(lossReview) ? lossReview : -0.1

  const planStatuses = []
  for (const lot of input?.lots || []) {
    const ps = String(lot?.context?.plan?.planStatus || '').trim()
    if (ps) planStatuses.push(ps)
  }

  const factors = []

  const timeTriggered = hasReason(reasonCodes, EXIT_REASON.TIME_REVIEW) || holdingDays > maxDays
  factors.push({
    code: EXIT_REASON.TIME_REVIEW,
    label: '持有时间',
    triggered: timeTriggered,
    fact: `已持有 ${holdingDays} 天`,
    detail: timeTriggered
      ? `超过策略观察周期 ${maxDays} 天`
      : `未超过观察周期 ${maxDays} 天`,
  })

  let lossTriggered = hasReason(reasonCodes, EXIT_REASON.LOSS_REVIEW)
  let lossFact = '现价缺失，无法计算浮动亏损'
  let lossDetail = '缺少有效现价，暂不推断亏损因素'
  if (unrealizedReturn != null) {
    lossFact = `当前收益 ${fmtPctRatio(unrealizedReturn)}`
    if (unrealizedReturn <= lossReviewPct) {
      lossTriggered = true
      lossDetail = `低于复评线 ${fmtPctRatio(lossReviewPct)}`
    } else if (unrealizedReturn <= lossWatchPct) {
      lossTriggered = true
      lossDetail = `低于关注线 ${fmtPctRatio(lossWatchPct)}`
    } else {
      lossDetail = `高于关注线 ${fmtPctRatio(lossWatchPct)}`
    }
  }
  factors.push({
    code: EXIT_REASON.LOSS_REVIEW,
    label: '浮动亏损',
    triggered: lossTriggered,
    fact: lossFact,
    detail: lossDetail,
  })

  const planTriggered = hasReason(reasonCodes, EXIT_REASON.PLAN_REVIEW)
  const planStatusText = planStatuses.length ? planStatuses.join('、') : '—'
  factors.push({
    code: EXIT_REASON.PLAN_REVIEW,
    label: '计划生命周期',
    triggered: planTriggered,
    fact: `关联计划状态：${planStatusText}`,
    detail: planTriggered
      ? '计划状态异常，需重新核对买入逻辑是否仍成立'
      : '关联计划状态正常',
  })

  const triggeredCount = factors.filter((f) => f.triggered).length
  return {
    factors,
    triggeredCount,
    state,
    summaryLine:
      triggeredCount > 0
        ? `共 ${triggeredCount} 项因素触线，建议结合买入依据综合判断。`
        : state === EXIT_EVAL_STATE.NORMAL
          ? '暂无复评信号。'
          : '建议复核持仓假设（非卖出指令）。',
  }
}
