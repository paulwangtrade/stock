/**
 * QuantDecision Phase2-C：Decision Shadow Stability Report
 * 基于 Phase2-B0 semantic compare 的批量 shadow 稳定性报告。
 * 不改 js_legacy / UI / TradePlan / Execution。
 */

import { COMPARE_SLICES } from './quantDecisionCompare.js'
import { compareDecisionSemantic } from './quantDecisionSemantic.js'

/**
 * @typedef {{ id?: string, code?: string, baseline: object, candidate: object }} ShadowPair
 */

/**
 * 构建批量 Shadow Stability Report。
 * @param {ShadowPair[]} pairs
 * @param {object} [options]
 */
export function buildShadowStabilityReport(pairs = [], options = {}) {
  const items = Array.isArray(pairs) ? pairs : []
  const pairReports = []

  const actionCodeConflicts = new Map() // `${left}>${right}` -> count
  const actionCodeConflictPairs = []
  let actionCodeEqualCount = 0
  let actionCodeConflictCount = 0

  const mismatchBySlice = {}
  for (const slice of COMPARE_SLICES) {
    mismatchBySlice[slice] = {
      mismatchPairCount: 0,
      pathCounts: {},
    }
  }

  let semanticEqualCount = 0
  let semanticMismatchCount = 0
  let displayOnlyDiffCount = 0

  for (let i = 0; i < items.length; i++) {
    const row = items[i] || {}
    const id = row.id || row.code || `pair-${i + 1}`
    const code = row.code || row.baseline?.instrument?.stockCode || row.candidate?.instrument?.stockCode || ''
    const cmp = compareDecisionSemantic(row.baseline, row.candidate, options)

    if (cmp.semanticEqual) semanticEqualCount++
    else semanticMismatchCount++

    if (cmp.semanticEqual && (cmp.displayDiffs?.length || 0) > 0) {
      displayOnlyDiffCount++
    }

    if (cmp.actionCodeEqual) {
      actionCodeEqualCount++
    } else {
      actionCodeConflictCount++
      const key = `${cmp.leftActionCode || '∅'}→${cmp.rightActionCode || '∅'}`
      actionCodeConflicts.set(key, (actionCodeConflicts.get(key) || 0) + 1)
      actionCodeConflictPairs.push({
        id,
        code,
        leftCode: cmp.leftActionCode || '',
        rightCode: cmp.rightActionCode || '',
      })
    }

    for (const slice of COMPARE_SLICES) {
      const sliceResult = cmp.bySlice?.[slice]
      if (!sliceResult || sliceResult.equal) continue
      mismatchBySlice[slice].mismatchPairCount++
      for (const d of sliceResult.diffs || []) {
        const p = d.path || slice
        mismatchBySlice[slice].pathCounts[p] = (mismatchBySlice[slice].pathCounts[p] || 0) + 1
      }
    }

    pairReports.push({
      id,
      code,
      semanticEqual: cmp.semanticEqual,
      actionCodeEqual: cmp.actionCodeEqual,
      leftActionCode: cmp.leftActionCode,
      rightActionCode: cmp.rightActionCode,
      displayDiffCount: cmp.displayDiffs?.length || 0,
      semanticDiffCount: cmp.semanticDiffs?.length || 0,
      summary: cmp.summary,
      mismatchedSlices: COMPARE_SLICES.filter((s) => cmp.bySlice?.[s] && !cmp.bySlice[s].equal),
    })
  }

  const conflictStats = [...actionCodeConflicts.entries()]
    .map(([pair, count]) => {
      const [leftCode, rightCode] = pair.split('→')
      return { leftCode, rightCode, count }
    })
    .sort((a, b) => b.count - a.count || a.leftCode.localeCompare(b.leftCode))

  const total = items.length
  const semanticEqualRate = total ? semanticEqualCount / total : 1
  const actionCodeEqualRate = total ? actionCodeEqualCount / total : 1

  return {
    phase: 'Phase2-C',
    harness: 'quant-decision-shadow-stability',
    generatedAt: new Date().toISOString(),
    totalPairs: total,
    semanticEqualCount,
    semanticMismatchCount,
    displayOnlyDiffCount,
    semanticEqualRate,
    actionCode: {
      equalCount: actionCodeEqualCount,
      conflictCount: actionCodeConflictCount,
      equalRate: actionCodeEqualRate,
      conflictStats,
      conflictPairs: actionCodeConflictPairs,
    },
    mismatchBySlice,
    pairs: pairReports,
    summary: summarizeStability({
      total,
      semanticEqualCount,
      semanticMismatchCount,
      actionCodeConflictCount,
      displayOnlyDiffCount,
    }),
  }
}

function summarizeStability({
  total,
  semanticEqualCount,
  semanticMismatchCount,
  actionCodeConflictCount,
  displayOnlyDiffCount,
}) {
  if (total === 0) return 'EMPTY: no shadow pairs'
  if (semanticMismatchCount === 0 && actionCodeConflictCount === 0) {
    if (displayOnlyDiffCount > 0) {
      return `STABLE: ${semanticEqualCount}/${total} semantic OK; ${displayOnlyDiffCount} display-only`
    }
    return `STABLE: ${semanticEqualCount}/${total} semantic OK; 0 action.code conflicts`
  }
  return `UNSTABLE: semanticFail=${semanticMismatchCount}/${total}; actionCodeConflicts=${actionCodeConflictCount}`
}

/** 从 report 提取 Top N slice 失配路径 */
export function topSliceMismatchPaths(report, slice, limit = 10) {
  const row = report?.mismatchBySlice?.[slice]
  if (!row?.pathCounts) return []
  return Object.entries(row.pathCounts)
    .map(([path, count]) => ({ path, count }))
    .sort((a, b) => b.count - a.count || a.path.localeCompare(b.path))
    .slice(0, limit)
}
