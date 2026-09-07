/**
 * Phase8-0 Step3-A selftest: scan RiskAdvice shadow dual-write.
 * Run: node frontend/src/utils/riskintel/scanRiskAdviceShadow.selftest.mjs
 */

import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import {
  attachScanRiskAdviceShadow,
  buildScanRiskAdviceShadow,
} from './scanRiskAdviceShadow.js'
import { scanForbiddenAdviceKeys } from './riskAdvice.js'

function assert(cond, msg) {
  if (!cond) throw new Error(msg)
}

function deepFreeze(obj) {
  if (obj == null || typeof obj !== 'object') return obj
  Object.freeze(obj)
  for (const v of Object.values(obj)) {
    if (v && typeof v === 'object' && !Object.isFrozen(v)) deepFreeze(v)
  }
  return obj
}

function run() {
  const legacyHoldingAdvice = deepFreeze({
    action: 'reduce',
    actionLabel: '清仓防守',
    score: -40,
    suggestPct: 1,
    suggestPctDisplay: 100,
    marketLevel: 1,
    marketLevelName: '空仓防守',
    factors: [{ key: 'market', label: '市场防守', impact: -35, detail: 'level1' }],
    summaryLine: '市场防守',
  })

  const legacyRow = deepFreeze({
    code: 'sh600519',
    name: '贵州茅台',
    ok: true,
    tag: null,
    tagType: 'default',
    statusText: '持仓 · 清仓防守 · 市场防守',
    sellPositionPct: 1,
    addPositionPct: null,
    rushReducePct: null,
    sourceTag: '',
    costPrice: 1800,
    sellVolume: null,
    holdingAdvice: legacyHoldingAdvice,
    sortRank: 99,
    daysAgo: 0,
    effectiveSignalDayKey: '2026-07-28',
    buyPriceRange: null,
    latestStatus: '',
    hasRecentStrictBuy: false,
    hasRecentBreakout: false,
    signalBar: 10,
  })

  const legacySnapshot = JSON.stringify(legacyRow)

  const bundleInput = {
    stockCode: 'sh600519',
    marketModeKey: 'level1',
    effectiveMarketMode: { key: 'level1', level: 1, name: '空仓防守' },
    globalMarketMode: { key: 'level1', level: 1, name: '空仓防守' },
    sectorFlow: { name: '白酒', inflow: false, rank: 3 },
    volumeSummary: { volumeRatio: 1.4, priceUp: false },
    asOf: '2026-07-29T00:00:00.000Z',
  }

  // 1) Shadow exists with contract shape
  const shadow = buildScanRiskAdviceShadow(bundleInput)
  assert(shadow != null, 'shadow built')
  assert(shadow.riskContext?.marketState?.level === 1, 'bundle level')
  assert(shadow.riskAdvice?.level === 1, 'advice level')
  assert(typeof shadow.riskAdvice.category === 'string', 'category')
  assert(typeof shadow.riskAdvice.marketDriven === 'boolean', 'marketDriven')
  assert(Array.isArray(shadow.riskAdvice.reasons) && shadow.riskAdvice.reasons.length >= 1, 'reasons')
  assert(shadow.riskAdvice.marketDriven === true, 'level1 marketDriven')
  assert(
    shadow.riskAdvice.reasons.some((r) => r.marketDriven === true),
    'marketDriven reason',
  )

  // 2) Forbidden RiskAdvice fields (user + contract)
  const hits = scanForbiddenAdviceKeys(shadow.riskAdvice)
  assert(hits.length === 0, `forbidden on advice: ${hits.join(',')}`)
  for (const k of ['mustSell', 'sellRatio', 'positionAction', 'order', 'execution']) {
    assert(!(k in shadow.riskAdvice), `no ${k} on advice root`)
  }

  // 3) Legacy output unchanged after attach
  const dual = attachScanRiskAdviceShadow(legacyRow, bundleInput)
  assert(JSON.stringify(legacyRow) === legacySnapshot, 'legacy row object unchanged')
  assert(dual.holdingAdvice === legacyHoldingAdvice, 'holdingAdvice same reference')
  assert(dual.sellPositionPct === 1, 'sellPositionPct preserved')
  assert(dual.statusText === legacyRow.statusText, 'statusText preserved')
  assert(dual.tag === legacyRow.tag, 'tag preserved')
  assert(dual.riskAdvice != null, 'riskAdvice attached')
  assert(dual.riskContext != null, 'riskContext attached')
  assert(
    JSON.stringify({
      code: dual.code,
      tag: dual.tag,
      statusText: dual.statusText,
      sellPositionPct: dual.sellPositionPct,
      addPositionPct: dual.addPositionPct,
      rushReducePct: dual.rushReducePct,
      holdingAdvice: dual.holdingAdvice,
    }) ===
      JSON.stringify({
        code: legacyRow.code,
        tag: legacyRow.tag,
        statusText: legacyRow.statusText,
        sellPositionPct: legacyRow.sellPositionPct,
        addPositionPct: legacyRow.addPositionPct,
        rushReducePct: legacyRow.rushReducePct,
        holdingAdvice: legacyRow.holdingAdvice,
      }),
    'legacy output fields byte-equal',
  )

  // Polluted Context-like inputs must not leak into Advice
  const pollutedShadow = buildScanRiskAdviceShadow({
    ...bundleInput,
    mustSell: true,
    sellRatio: 1,
    positionAction: 'sell',
    order: { side: 'sell' },
    execution: { id: 'x' },
    holdingAdvice: legacyHoldingAdvice,
  })
  assert(pollutedShadow != null, 'polluted still builds')
  assert(scanForbiddenAdviceKeys(pollutedShadow.riskAdvice).length === 0, 'polluted advice clean')
  assert(!('mustSell' in pollutedShadow.riskAdvice), 'no mustSell leak')
  assert(!('order' in pollutedShadow.riskAdvice), 'no order leak')
  assert(!('execution' in pollutedShadow.riskAdvice), 'no execution leak')
  assert(!('riskAdvice' in pollutedShadow.riskContext), 'context is not advice')

  // 4) Execution boundary: no live-order / broker / approve imports; dual-write after adjust
  const __dirname = dirname(fileURLToPath(import.meta.url))
  const shadowSrc = readFileSync(join(__dirname, 'scanRiskAdviceShadow.js'), 'utf8')
  const scanSrc = readFileSync(join(__dirname, '../watchlistSignalScan.js'), 'utf8')
  assert(
    !/from\s+['"][^'"]*(execution|broker|approvegate|candidate)/i.test(shadowSrc),
    'no exec imports in shadow',
  )
  assert(
    !/from\s+['"][^'"]*(execution|broker|approvegate|candidate)/i.test(scanSrc),
    'no exec imports in scan',
  )
  assert(!/wailsjs\/go\/.*[Ee]xecution/.test(scanSrc), 'scan has no execution wails binding')
  assert(scanSrc.includes('applyHoldingPositionAdjustToSummary'), 'scan still calls holding adjust')
  assert(scanSrc.includes('buildScanRiskAdviceShadow'), 'scan wires shadow')
  const adjustIdx = scanSrc.indexOf('applyHoldingPositionAdjustToSummary(')
  const shadowCallIdx = scanSrc.indexOf('buildScanRiskAdviceShadow(')
  assert(adjustIdx >= 0 && shadowCallIdx > adjustIdx, 'shadow after holding adjust')

  // Failure path: null input �?null shadow; attach returns legacy-only copy
  const failed = buildScanRiskAdviceShadow(null)
  assert(failed.failReason === 'context_missing' && !failed.riskAdvice, 'context missing shadow')
  const legacyOnly = attachScanRiskAdviceShadow(legacyRow, null)
  assert(!('riskAdvice' in legacyOnly), 'no advice on fail')
  assert(legacyOnly.holdingAdvice === legacyHoldingAdvice, 'fail path preserves holdingAdvice ref')
  assert(JSON.stringify(legacyRow) === legacySnapshot, 'legacy still equal after fail attach')

  console.log('scanRiskAdviceShadow.selftest: PASS')
}

run()
