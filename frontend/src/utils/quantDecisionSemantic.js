/**
 * QuantDecision Phase2-B0：语义归一化层
 * - 不改 js_legacy / UI / TradePlan / Execution
 * - Action.Code 为语义最高优先级
 * - Action.Label 仅进入 displayDiff（不影响 semanticEqual）
 */

import {
  COMPARE_SLICES,
  DEFAULT_IGNORE_PATHS,
  PRODUCER_JS_LEGACY,
  PRODUCER_GO_ENGINE,
  diffValues,
} from './quantDecisionCompare.js'

/** 语义比较时从 Action 剥离的展示字段 */
export const ACTION_DISPLAY_FIELDS = ['label', 'type', 'lines', 'tooltip', 'decisionId', 'actionSource']

/** 其它展示/扩展字段（不进语义核） */
export const SEMANTIC_EXTRA_IGNORE = [
  ...DEFAULT_IGNORE_PATHS,
  'action.label',
  'action.type',
  'action.lines',
  'action.tooltip',
  'action.decisionId',
  'action.actionSource',
  'entryZone.instantText',
  'entryZone.extended',
  'entryZone.tag',
  'entryZone.rangeHigh',
  'entryZone.note',
  'signal.summary',
  'regime.source',
  'regime.name',
  'risk.message',
  'size.reason',
]

function isPlainObject(v) {
  return v != null && typeof v === 'object' && !Array.isArray(v)
}

function clone(v) {
  if (v == null) return v
  if (Array.isArray(v)) return v.map(clone)
  if (isPlainObject(v)) {
    const o = {}
    for (const [k, val] of Object.entries(v)) o[k] = clone(val)
    return o
  }
  return v
}

function scrubEmpty(v) {
  if (v == null || v === '') return null
  if (Array.isArray(v)) return v.map(scrubEmpty)
  if (isPlainObject(v)) {
    const o = {}
    for (const [k, val] of Object.entries(v)) {
      const next = scrubEmpty(val)
      if (next === null || next === undefined) continue
      if (isPlainObject(next) && Object.keys(next).length === 0) continue
      o[k] = next
    }
    return o
  }
  return v
}

/**
 * 语义归一化：保留 Signal/Zone/Gate/Risk/Size/Action 核字段。
 * Action 仅保留 code / allowDraft / side（Code 最高优先级）。
 */
export function normalizeDecisionSemantic(decision) {
  if (!decision || typeof decision !== 'object') return null
  const raw = {}
  for (const slice of COMPARE_SLICES) {
    raw[slice] = decision[slice] == null ? null : clone(decision[slice])
  }

  if (raw.action && isPlainObject(raw.action)) {
    const a = raw.action
    raw.action = {
      code: a.code || '',
      allowDraft: !!a.allowDraft,
      side: a.side || 'none',
    }
  } else {
    raw.action = { code: '', allowDraft: false, side: 'none' }
  }

  // Gate items：只保留 id/passed/required（label 视为展示）
  if (raw.gate?.items && Array.isArray(raw.gate.items)) {
    raw.gate = {
      score: raw.gate.score ?? 0,
      ready: !!raw.gate.ready,
      requiredPassed: !!raw.gate.requiredPassed,
      readyThreshold: raw.gate.readyThreshold ?? 0.85,
      items: raw.gate.items.map((it) => ({
        id: it?.id || '',
        passed: !!it?.passed,
        required: !!it?.required,
      })),
    }
  }

  if (raw.signal && isPlainObject(raw.signal)) {
    const s = raw.signal
    raw.signal = scrubEmpty({
      tag: s.tag || '',
      tagKind: s.tagKind || 'none',
      daysAgo: s.daysAgo ?? 0,
      score: s.score ?? 0,
      ratioPct: s.ratioPct,
      sourceTag: s.sourceTag || null,
      barIndex: s.barIndex ?? null,
      dayKey: s.dayKey || null,
    })
  }

  if (raw.entryZone && isPlainObject(raw.entryZone)) {
    const z = raw.entryZone
    raw.entryZone = scrubEmpty({
      low: z.low,
      high: z.high,
      instantPrice: z.instantPrice,
      mode: z.mode || 'unknown',
      deferMode: z.deferMode || 'same',
      daysAgo: z.daysAgo ?? 0,
      text: z.text || null,
    })
  }

  if (raw.risk && isPlainObject(raw.risk)) {
    raw.risk = {
      passed: !!raw.risk.passed,
      code: raw.risk.code || '',
    }
  }

  if (raw.size && isPlainObject(raw.size)) {
    const s = raw.size
    raw.size = scrubEmpty({
      ok: !!s.ok,
      entryPrice: s.entryPrice,
      stopPrice: s.stopPrice,
      riskPerShare: s.riskPerShare,
      confidence: s.confidence,
      targetShares: s.targetShares,
      addShares: s.addShares,
      targetAmount: s.targetAmount,
      positionPct: s.positionPct,
      bindingConstraint: s.bindingConstraint || null,
    })
  }

  return scrubEmpty(raw)
}

/**
 * 仅提取 Action.Label 展示差异（不影响 semanticEqual）。
 */
export function collectActionDisplayDiffs(left, right) {
  const ll = left?.action?.label ?? ''
  const rl = right?.action?.label ?? ''
  if (ll === rl) return []
  return [{
    path: 'action.label',
    kind: 'display',
    left: ll,
    right: rl,
  }]
}

/**
 * 语义相等比较。
 * @returns {{
 *   semanticEqual: boolean,
 *   actionCodeEqual: boolean,
 *   displayDiffs: Array,
 *   semanticDiffs: Array,
 *   bySlice: object,
 *   leftActionCode: string,
 *   rightActionCode: string,
 *   summary: string,
 * }}
 */
export function compareDecisionSemantic(left, right, options = {}) {
  const leftProducer = left?.meta?.producer || options.leftProducer || PRODUCER_JS_LEGACY
  const rightProducer = right?.meta?.producer || options.rightProducer || PRODUCER_GO_ENGINE

  const normL = normalizeDecisionSemantic(left)
  const normR = normalizeDecisionSemantic(right)

  const leftCode = normL?.action?.code || ''
  const rightCode = normR?.action?.code || ''
  const actionCodeEqual = leftCode === rightCode

  const bySlice = {}
  const semanticDiffs = []

  for (const slice of COMPARE_SLICES) {
    const diffs = diffValues(
      normL?.[slice] ?? null,
      normR?.[slice] ?? null,
      slice,
      [],
      { looseEmpty: true },
    )
    bySlice[slice] = { equal: diffs.length === 0, diffs }
    for (const d of diffs) semanticDiffs.push({ ...d, slice, kind: 'semantic' })
  }

  // Action.Code 最高优先级：若 code 不同，强制语义不等，并置顶提示
  if (!actionCodeEqual) {
    const hasCodeDiff = semanticDiffs.some((d) => d.path === 'action.code')
    if (!hasCodeDiff) {
      semanticDiffs.unshift({
        path: 'action.code',
        slice: 'action',
        kind: 'semantic',
        priority: 'highest',
        left: leftCode,
        right: rightCode,
      })
    } else {
      for (const d of semanticDiffs) {
        if (d.path === 'action.code') d.priority = 'highest'
      }
    }
  }

  const displayDiffs = collectActionDisplayDiffs(left, right)
  const semanticEqual = actionCodeEqual && semanticDiffs.length === 0

  let summary
  if (semanticEqual && displayDiffs.length === 0) {
    summary = `SEMANTIC_OK ${leftProducer} ≡ ${rightProducer}`
  } else if (semanticEqual && displayDiffs.length > 0) {
    summary = `SEMANTIC_OK_DISPLAY_DIFF ${leftProducer} ≈ ${rightProducer} (label)`
  } else if (!actionCodeEqual) {
    summary = `SEMANTIC_FAIL action.code ${leftCode || '∅'} ≠ ${rightCode || '∅'}`
  } else {
    summary = `SEMANTIC_FAIL ${semanticDiffs.length} field(s)`
  }

  return {
    semanticEqual,
    actionCodeEqual,
    displayDiffs,
    semanticDiffs,
    bySlice,
    leftActionCode: leftCode,
    rightActionCode: rightCode,
    leftProducer,
    rightProducer,
    summary,
    ignorePaths: SEMANTIC_EXTRA_IGNORE,
  }
}

export function assertSemanticEqual(left, right) {
  const r = compareDecisionSemantic(left, right)
  if (!r.semanticEqual) {
    const err = new Error(r.summary)
    err.report = r
    throw err
  }
  return r
}
