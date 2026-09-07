/**
 * QuantDecision Phase1-D：Dual Run Comparator
 * 为未来 Go Producer 接入准备字段级对比（不改 Action / UI / 交易链路）。
 *
 * 比较范围：Signal / EntryZone / Gate / Risk / Size / Action
 * 忽略：id、producer、时间字段（asOf 等）、_legacyHint
 */

export const PRODUCER_JS_LEGACY = 'js_legacy'
export const PRODUCER_GO_ENGINE = 'go_engine'
export const PRODUCER_FUTURE = 'future_producer'

/** 默认忽略路径（精确或前缀） */
export const DEFAULT_IGNORE_PATHS = [
  'id',
  'asOf',
  'tradeDate',
  'meta.producer',
  'meta.asOf',
  'meta.generatedAt',
  'meta.createdAt',
  'meta.updatedAt',
  '_legacyHint',
]

/** Dual-run 语义切片 */
export const COMPARE_SLICES = ['signal', 'entryZone', 'gate', 'risk', 'size', 'action']

const FLOAT_EPS = 1e-9

function isPlainObject(v) {
  return v != null && typeof v === 'object' && !Array.isArray(v)
}

function approxEqual(a, b, { looseEmpty = false } = {}) {
  if (looseEmpty && isLooseEmptyPair(a, b)) return true
  if (typeof a === 'number' && typeof b === 'number') {
    if (Number.isNaN(a) && Number.isNaN(b)) return true
    return Math.abs(a - b) <= FLOAT_EPS
  }
  return Object.is(a, b)
}

/** 跨 producer omitempty：null/undefined/''/0/false 视为空占位 */
function isLooseEmpty(v) {
  return v == null || v === '' || v === 0 || v === false
}

function isLooseEmptyPair(a, b) {
  return isLooseEmpty(a) && isLooseEmpty(b)
}

function shouldIgnorePath(path, ignorePaths) {
  if (!path) return false
  return ignorePaths.some((p) => path === p || path.startsWith(`${p}.`))
}

/**
 * 规范化决策切片：只保留比较范围，剥离忽略字段。
 */
export function normalizeDecisionForCompare(decision, {
  slices = COMPARE_SLICES,
  ignorePaths = DEFAULT_IGNORE_PATHS,
} = {}) {
  if (!decision || typeof decision !== 'object') return null
  const out = {}
  for (const key of slices) {
    if (Object.prototype.hasOwnProperty.call(decision, key)) {
      out[key] = decision[key] == null ? null : cloneComparable(decision[key])
    } else {
      out[key] = null
    }
  }
  // Action：语义核字段（lines/tooltip 为展示投影，仍纳入以便文案漂移可检；可用 options 收窄）
  return stripIgnored(out, '', ignorePaths)
}

function cloneComparable(v) {
  if (v == null) return v
  if (Array.isArray(v)) return v.map(cloneComparable)
  if (isPlainObject(v)) {
    const o = {}
    for (const [k, val] of Object.entries(v)) {
      o[k] = cloneComparable(val)
    }
    return o
  }
  return v
}

function stripIgnored(node, path, ignorePaths) {
  if (shouldIgnorePath(path, ignorePaths)) return undefined
  if (Array.isArray(node)) {
    return node.map((item, i) => stripIgnored(item, path ? `${path}[${i}]` : `[${i}]`, ignorePaths))
  }
  if (!isPlainObject(node)) return node
  const out = {}
  for (const [k, v] of Object.entries(node)) {
    const childPath = path ? `${path}.${k}` : k
    if (shouldIgnorePath(childPath, ignorePaths)) continue
    const next = stripIgnored(v, childPath, ignorePaths)
    if (next !== undefined) out[k] = next
  }
  return out
}

/**
 * 深度字段级 diff。
 * @returns {{ path: string, left: any, right: any }[]}
 */
export function diffValues(left, right, basePath = '', diffs = [], opts = {}) {
  if (approxEqual(left, right, opts)) return diffs

  const lObj = isPlainObject(left)
  const rObj = isPlainObject(right)
  const lArr = Array.isArray(left)
  const rArr = Array.isArray(right)

  if (lArr && rArr) {
    const n = Math.max(left.length, right.length)
    for (let i = 0; i < n; i++) {
      const p = `${basePath}[${i}]`
      if (i >= left.length) {
        if (!(opts.looseEmpty && isLooseEmpty(right[i]))) {
          diffs.push({ path: p, left: undefined, right: right[i] })
        }
      } else if (i >= right.length) {
        if (!(opts.looseEmpty && isLooseEmpty(left[i]))) {
          diffs.push({ path: p, left: left[i], right: undefined })
        }
      } else {
        diffValues(left[i], right[i], p, diffs, opts)
      }
    }
    return diffs
  }

  if (lObj && rObj) {
    const keys = new Set([...Object.keys(left), ...Object.keys(right)])
    for (const k of keys) {
      const p = basePath ? `${basePath}.${k}` : k
      if (!(k in left)) {
        if (!(opts.looseEmpty && isLooseEmpty(right[k]))) {
          diffs.push({ path: p, left: undefined, right: right[k] })
        }
      } else if (!(k in right)) {
        if (!(opts.looseEmpty && isLooseEmpty(left[k]))) {
          diffs.push({ path: p, left: left[k], right: undefined })
        }
      } else {
        diffValues(left[k], right[k], p, diffs, opts)
      }
    }
    return diffs
  }

  diffs.push({ path: basePath || '(root)', left, right })
  return diffs
}

/**
 * 比较两个 QuantDecision（字段级）。
 * @param {object} left - 通常 js_legacy
 * @param {object} right - 通常 future / go_engine
 * @returns {{
 *   equal: boolean,
 *   diffs: Array<{ path: string, left: any, right: any, slice?: string }>,
 *   bySlice: Record<string, { equal: boolean, diffs: Array }>,
 *   leftProducer: string,
 *   rightProducer: string,
 *   summary: string,
 * }}
 */
export function compareQuantDecisions(left, right, options = {}) {
  const slices = options.slices || COMPARE_SLICES
  const ignorePaths = options.ignorePaths || DEFAULT_IGNORE_PATHS
  const looseEmpty = options.looseEmpty === true
  const leftProducer = left?.meta?.producer || options.leftProducer || PRODUCER_JS_LEGACY
  const rightProducer = right?.meta?.producer || options.rightProducer || PRODUCER_FUTURE

  const normL = normalizeDecisionForCompare(left, { slices, ignorePaths })
  const normR = normalizeDecisionForCompare(right, { slices, ignorePaths })
  const diffOpts = { looseEmpty }

  const bySlice = {}
  const allDiffs = []

  for (const slice of slices) {
    const sliceDiffs = diffValues(normL?.[slice] ?? null, normR?.[slice] ?? null, slice, [], diffOpts)
    bySlice[slice] = {
      equal: sliceDiffs.length === 0,
      diffs: sliceDiffs,
    }
    for (const d of sliceDiffs) {
      allDiffs.push({ ...d, slice })
    }
  }

  const equal = allDiffs.length === 0
  return {
    equal,
    diffs: allDiffs,
    bySlice,
    leftProducer,
    rightProducer,
    summary: equal
      ? `OK ${leftProducer} ≡ ${rightProducer}`
      : `DIFF ${leftProducer} vs ${rightProducer}: ${allDiffs.length} field(s)`,
  }
}

/**
 * Dual Run Harness：baseline(js_legacy) vs candidate(future producer)。
 * 不执行交易、不改 UI；仅产出对比报告。
 */
export function runDualDecisionCompare({
  baseline,
  candidate,
  baselineProducer = PRODUCER_JS_LEGACY,
  candidateProducer = PRODUCER_GO_ENGINE,
  options = {},
} = {}) {
  const result = compareQuantDecisions(baseline, candidate, {
    ...options,
    leftProducer: baseline?.meta?.producer || baselineProducer,
    rightProducer: candidate?.meta?.producer || candidateProducer,
  })
  return {
    ...result,
    harness: options.harness || 'quant-decision-dual-run',
    phase: options.phase || 'Phase1-D',
    comparedAt: new Date().toISOString(),
    baselineId: baseline?.id || null,
    candidateId: candidate?.id || null,
  }
}

/** 浅拷贝并覆盖 Action（制造差异用，不碰 derive） */
export function mutateDecisionAction(decision, actionPatch, { producer = PRODUCER_FUTURE } = {}) {
  if (!decision) return null
  return {
    ...decision,
    action: {
      ...(decision.action || {}),
      ...actionPatch,
    },
    meta: {
      ...(decision.meta || {}),
      producer,
    },
  }
}
