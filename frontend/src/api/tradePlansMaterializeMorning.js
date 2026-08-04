/**
 * Phase10-A.2: Morning intent materialize UI helpers (pure).
 * No fetch / no auto Approve / Freeze.
 */

export const PRICING_STAGE_AFTER_CLOSE_INTENT = 'after_close_intent'
export const PRICING_STAGE_MORNING_MATERIALIZED = 'morning_materialized'

export const READINESS_STAGE_INTENT_DRAFT = 'S1_INTENT_DRAFT'
export const READINESS_STAGE_INTENT_MATERIALIZED = 'S2_INTENT_MATERIALIZED'

/**
 * Button visibility: Draft + after_close_intent + not frozen.
 * @param {{ status?: string, isFrozen?: boolean, pricingStage?: string }} opts
 */
export function canShowMorningMaterializeButton(opts = {}) {
  const status = String(opts.status || '').trim().toLowerCase()
  const stage = String(opts.pricingStage || '').trim()
  const frozen = !!opts.isFrozen
  return status === 'draft' && !frozen && stage === PRICING_STAGE_AFTER_CLOSE_INTENT
}

/**
 * Infer pricing_stage without changing upcoming API.
 * Prefer explicit stage; else map readiness lifecycle / intent blockers.
 *
 * @param {{
 *   explicitPricingStage?: string,
 *   lifecycleStage?: string,
 *   blockers?: Array<{ code?: string }>
 * }} opts
 * @returns {string}
 */
export function inferPricingStageForMaterializeUI(opts = {}) {
  const explicit = String(opts.explicitPricingStage || '').trim()
  if (explicit) return explicit

  const lifecycle = String(opts.lifecycleStage || '').trim()
  if (lifecycle === READINESS_STAGE_INTENT_MATERIALIZED) {
    return PRICING_STAGE_MORNING_MATERIALIZED
  }
  if (lifecycle === READINESS_STAGE_INTENT_DRAFT) {
    return PRICING_STAGE_AFTER_CLOSE_INTENT
  }

  const blockers = Array.isArray(opts.blockers) ? opts.blockers : []
  for (const b of blockers) {
    const code = String(b?.code || '').trim()
    if (
      code === 'INTENT_NOT_MATERIALIZED' ||
      code === 'LIMIT_PRICE_MISSING' ||
      code === 'TARGET_VOLUME_BELOW_LOT'
    ) {
      return PRICING_STAGE_AFTER_CLOSE_INTENT
    }
  }
  return ''
}

/** @param {number} planId */
export function buildMaterializeMorningRequestBody(planId) {
  return { plan_id: Math.trunc(Number(planId) || 0) }
}

/**
 * Normalize materialize-morning API JSON for UI.
 * @param {any} body
 */
export function normalizeMaterializeMorningResponse(body) {
  const blockersRaw = Array.isArray(body?.blockers) ? body.blockers : []
  const blockers = blockersRaw.map((b) => ({
    rule_code: String(b?.rule_code || ''),
    code: String(b?.code || ''),
    severity: String(b?.severity || ''),
    message: String(b?.message || ''),
    evidence:
      b?.evidence && typeof b.evidence === 'object' ? b.evidence : undefined,
  }))
  return {
    success: !!body?.success,
    plan_id: Number(body?.plan_id) || 0,
    materialized_items: Number(body?.materialized_items) || 0,
    readiness_ready: !!body?.readiness_ready,
    blockers,
    code: Number(body?.code ?? -1),
    message: body?.message ? String(body.message) : '',
    failed_step: body?.failed_step ? String(body.failed_step) : '',
    pricing_stage: body?.pricing_stage ? String(body.pricing_stage) : '',
  }
}

/**
 * @param {ReturnType<typeof normalizeMaterializeMorningResponse>} res
 * @returns {{ ok: boolean, title: string, detailLines: string[] }}
 */
export function formatMaterializeMorningDisplay(res) {
  if (!res || typeof res !== 'object') {
    return {
      ok: false,
      title: '早盘物化失败',
      detailLines: ['无响应'],
    }
  }
  if (res.success) {
    const lines = [
      `物化条目：${res.materialized_items}`,
      `Readiness：${res.readiness_ready ? 'Ready' : 'Blocked'}`,
    ]
    if (res.blockers.length) {
      lines.push(`阻断（${res.blockers.length}）：`)
      for (const b of res.blockers.slice(0, 8)) {
        const code = b.code || b.rule_code || '—'
        lines.push(`- [${code}] ${b.message || ''}`.trim())
      }
      if (res.blockers.length > 8) {
        lines.push(`…另有 ${res.blockers.length - 8} 项`)
      }
    } else {
      lines.push('阻断：无')
    }
    return { ok: true, title: '早盘物化成功', detailLines: lines }
  }
  const reason =
    String(res.message || '').trim() ||
    (res.failed_step ? `步骤失败：${res.failed_step}` : '') ||
    (res.code ? `业务码 ${res.code}` : '') ||
    '未知错误'
  const lines = [reason]
  if (res.failed_step) lines.push(`失败步骤：${res.failed_step}`)
  if (res.blockers.length) {
    lines.push(`阻断（${res.blockers.length}）：`)
    for (const b of res.blockers.slice(0, 5)) {
      const code = b.code || b.rule_code || '—'
      lines.push(`- [${code}] ${b.message || ''}`.trim())
    }
  }
  return { ok: false, title: '早盘物化失败', detailLines: lines }
}
