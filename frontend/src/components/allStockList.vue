<script setup>
import {h, onBeforeMount, onMounted, onBeforeUnmount, ref, reactive, computed, watch} from 'vue'
import {
  GetAllStockInfoList,
  GetAllStocks,
  GetConfig,
  GetSponsorInfo,
  GetAllIndustries,
  GetHotIndustryPlates,
  GetIndustryRank,
  Follow,
  GetFollowList,
  GetStockEastMoneyKLine,
  GetLatestSignalScanSnapshotByStrategy,
  ParseSignalScanSnapshotPayload,
  StartSignalScanSnapshot,
  GetLatestSignalScanTask,
  GetSignalScanTask,
  ListSignalScanSnapshots,
  IsSignalScanRunning,
  GetStockRealTimePrice,
} from "../../wailsjs/go/main/App";
import { followWithDateGroup, formatFollowGroupMessage } from "../utils/followDateGroup"
import {NButton, NInput, NTag, NText, NTooltip, NProgress, useMessage, useNotification, NDataTable, NSpace, NPagination, NFlex, NSelect, NIcon, NModal, NCard, NTable, NSpin, NAlert} from "naive-ui";
import StockKlineModal from "./StockKlineModal.vue"
import StockLink from "./StockLink.vue"
import OpportunityProjectionDrawer from "./OpportunityProjectionDrawer.vue"
import InvestmentNarrativePanel from './InvestmentNarrativePanel.vue'
import { resolveStrategyRowCode, resolveStrategyRowName, toEastMoneyCode, toFollowCodeFromRow } from "../utils/stockCode"
import { applyStockClickAction, toStockDisplayModel } from "../utils/stockDisplayAdapters.js"
import {
  formatPriceWithContext,
  priceColumnTitle,
  PRICE_KIND,
} from '../utils/priceDisplay.js'
import { scanRowsLastBarSignals } from "../utils/watchlistSignalScan"
import { calcTrendCompositeScore, getTrendScoreStyle } from "../utils/trendBreakoutScore"
import { passesSignalTagFilter, passesReboundScreenFilter, getSignalTagColor, buildScreenSignalFilterOptions, formatSignalTagLabel, normalizeScreenSignalTag, SCREEN_SNAPSHOT_SIGNAL_TAG_SET } from "../utils/signalBuyGuide"
import {
  getActiveScreenStrategyId,
  getReboundScreenMaxRsi,
  getScreenStrategies,
  getScreenStrategySettingsById,
  parseSignalParams,
  serializeSignalParams,
} from "../utils/signalSettings"
import {format} from "date-fns";
import {EventsOn, EventsOff} from "../../wailsjs/runtime";
import {
  calcReturnSinceSignal,
  enrichHitToRow,
  enrichHitToSummary,
  formatPctWithSign,
  isDerivedSignalPriceStatus,
  isMissingSignalPriceStatus,
  OPPORTUNITY_PRICE_FOOTER,
  resolveLatestPrice,
  resolveLiveChangeRate,
  resolveSignalFields,
  resolveSnapshotChange,
  signalPriceTooltipForStatus,
  vsSignalReturnMissingHint,
} from '../utils/opportunityListMetrics.js'
import {
  buildNonTradingDaySnapshotTip,
  buildSnapshotHistoryLabel,
  buildSnapshotMetaDisplay,
} from '../utils/snapshotDisplay.js'
import {
  formatOpportunityListCountLabel,
  OPPORTUNITY_LIST_PAGINATE_THRESHOLD,
  shouldPaginateOpportunityList,
  sliceOpportunityPage,
} from '../utils/opportunityListPagination.js'
import { fetchOpportunityList, postOpportunityAction, OPPORTUNITY_ACTION } from '../api/opportunities.ts'
import { fetchOpportunityProjections } from '../api/opportunityProjection.ts'
import {
  buildDecisionBadgeView,
  buildProjectionMap,
  OPPORTUNITY_DECISION_FOOTER,
  resolveProjectionBatchLimit,
} from '../utils/opportunityDecisionBadgeDisplay.js'
import {
  buildSignalSnapshotDisplayProjection,
  formatOpportunityTableCell,
  opportunitySourceChipMeta,
  resolveOpportunityReasonSummary,
  resolveOpportunityStrategyLabel,
} from '../utils/opportunityExplanationColumns.js'
import {
  buildIgnoreActionPayload,
  buildWatchActionPayload,
  indexOpportunityEntriesBySecucode,
  isOpportunityWatched,
  resolveOpportunityEntryForRow,
} from '../utils/opportunityUserAction.js'
import {
  isSignalScanTaskActiveStatus,
  resolveSignalScanTaskPollOutcome,
  resolveStockScreenTableLoading,
} from '../utils/signalScanTaskLoading.js'
import {
  collectQuoteCodesFromRows,
  mergeQuoteIntoCache,
  parseLiveQuoteResponse,
  planQuoteRefresh,
  shouldRefreshSnapshotQuotes,
} from '../utils/opportunityQuoteRefresh.js'

const notify = useNotification()
const message = useMessage()

const editorDataRef = reactive({
  darkTheme: false
})

onBeforeMount(() => {
  GetConfig().then(result => {
    if (result.darkTheme) {
      editorDataRef.darkTheme = true
    }
    applyScreenStrategiesFromConfig(result?.signalParams)
    loadSnapshotHistoryOptions()
    refreshSnapshotBanner()
    if (shouldRunLivePageSignalScan()) {
      scanPageSignals(dataRef.value)
    }
  })

  GetSponsorInfo().then((res) => {
    // console.log(res)
    vipLevel.value = res.vipLevel;
    vipStartTime.value = res.vipStartTime;
    vipEndTime.value = res.vipEndTime;
    //鍒ゆ柇鏃堕棿鏄惁鍒版湡
    if (res.vipLevel) {
      if (res.vipEndTime < format(new Date(), 'yyyy-MM-dd HH:mm:ss')) {
        //notify.warning({content: 'VIP宸插埌鏈?})
        expired.value = true;
      }
    }else{
      //notify.success({content: '鏈紑閫歏IP'})
    }
    isValidVip.value = !(vipLevel.value === "" || Number(vipLevel.value) <= 0);

  })

})

onMounted(() => {
  loadIndustryOptions()
  loadFollowedStockCodes()
  refreshStocks(1)
  EventsOn('allStockListRefresh', handleExternalRefresh)
  EventsOn('signalScanProgress', onBackendSignalScanProgress)
  EventsOn('signalScanDone', onBackendSignalScanDone)
  refreshScanTaskView().then((task) => {
    const st = String(task?.status || '')
    if (isSignalScanTaskActiveStatus(st)) {
      backendScanLoading.value = true
      signalScanLoading.value = true
      signalScanStatus.value = '后台扫描进行中...'
      startScanTaskPoll()
    }
  })
})

onBeforeUnmount(() => {
  EventsOff('allStockListRefresh')
  EventsOff('signalScanProgress')
  EventsOff('signalScanDone')
  stopScanTaskPoll()
})

const dataRef = ref([])
const loadingRef = ref(false)
/** Phase14-H1: table spinner follows user fetch only, not background/page signal scan. */
const tableLoading = computed(() => resolveStockScreenTableLoading(loadingRef.value))
const dataRefreshKey = ref(0)
const signalByCode = ref(new Map())
/** Phase14-P0: invalidates in-flight live scanPageSignals when snapshot search applies. */
const signalScanGeneration = ref(0)
const signalScanLoading = ref(false)
const signalScanStatus = ref('')
const signalScanProgress = ref({ phase: '', done: 0, total: 0 })
/** Phase6.7-G: async snapshot task view */
const scanTaskView = ref(null)
let scanTaskPollTimer = null
const SIGNAL_SCAN_CONCURRENCY = 24
const SIGNAL_PAGE_SCAN_CONCURRENCY = 12
const signalFilteredRows = ref([])
const liveQuoteByCode = ref(new Map())
/** Codes with an active GetStockRealTimePrice request (Phase14-H2.1 dedupe). */
const quoteFetchInFlight = new Set()
const showOpportunityColumns = computed(
  () => signalDataSource.value === 'snapshot' || hasSignalFilter.value,
)
const snapshotSession = ref('close')
const snapshotTradeDate = ref('')
const selectedSnapshotHistoryValue = ref(null)
const snapshotMeta = ref(null)
const signalDataSource = ref('')
const lastSnapshotPayload = ref(null)
const backendScanLoading = ref(false)
const snapshotHistoryOptions = ref([])
const selectableSnapshotHistoryOptions = computed(() => snapshotHistoryOptions.value.filter((item) => item?.value))
const followedStockCodes = ref(new Set())
const opportunityEntryBySecucode = ref(Object.create(null))
const decisionProjectionByCode = ref(new Map())
const decisionProjectionLoading = ref(false)
const decisionProjectionError = ref(false)
let decisionProjectionLoadToken = 0
let decisionProjectionDebounceTimer = null
const opportunityWatchSaving = ref(false)
const starBacktestVisible = ref(false)
const starBacktestLoading = ref(false)
const starBacktestError = ref('')
const starBacktestRows = ref([])
const starBacktestSummary = ref([])
const screenStrategySettings = ref(null)
const screenStrategyOptions = ref([{ label: '默认策略', value: 'default' }])
const selectedScreenStrategyId = ref('default')
const marketSegmentOptions = [
  { label: '全部市场', value: '' },
  { label: '沪市', value: 'sh' },
  { label: '深市', value: 'sz' },
  { label: '科创', value: 'star' },
  { label: '创业', value: 'gem' },
  { label: '北交', value: 'bj' },
]
const filterIndustry = ref('')
const filterMarketSegment = ref('')
const filterSignalTags = ref([])
const filterCollapseExpanded = ref([])
const industryOptions = ref([{ label: '全部行业', value: '' }])
const signalFilterOptions = buildScreenSignalFilterOptions()
const hasSignalFilter = computed(() => filterSignalTags.value.length > 0)
const hasReboundFilter = computed(() => filterSignalTags.value.includes('弹'))
const selectedScreenStrategy = computed(() => {
  const settings = screenStrategySettings.value
  if (!settings) return null
  return getScreenStrategies(settings).find((item) => item.id === selectedScreenStrategyId.value) || getScreenStrategies(settings)[0]
})
const selectedScreenStrategyParams = computed(() => {
  const settings = screenStrategySettings.value
  if (!settings) return null
  return getScreenStrategySettingsById(settings, selectedScreenStrategyId.value)
})
const reboundScreenMaxRsi = computed(() => getReboundScreenMaxRsi(selectedScreenStrategyParams.value))
const signalFilterPassOptions = computed(() => ({
  maxRsi: reboundScreenMaxRsi.value,
}))

/** snapshot = 盘后快照 hits；live_scan / live = 页内 K 线扫描（默认浏览） */
function isSnapshotSignalSource() {
  return signalDataSource.value === 'snapshot'
}

/** 机会表数据源：快照 hits 或信号筛选结果（非 live 分页 dataRef） */
const usesOpportunityRowSource = computed(
  () => hasSignalFilter.value || isSnapshotSignalSource(),
)

function shouldRunLivePageSignalScan() {
  if (isSnapshotSignalSource()) return false
  if (hasSignalFilter.value) return false
  return true
}

function applyScreenStrategiesFromConfig(raw) {
  const settings = parseSignalParams(raw)
  screenStrategySettings.value = settings
  const strategies = getScreenStrategies(settings)
  screenStrategyOptions.value = strategies.map((item) => ({ label: item.name, value: item.id }))
  selectedScreenStrategyId.value = getActiveScreenStrategyId(settings)
}

const scanProgressPercent = computed(() => {
  const { done, total } = signalScanProgress.value
  if (!total) return 0
  return Math.min(100, Math.round((done / total) * 100))
})

const snapshotMetaDisplay = computed(() => {
  if (!snapshotMeta.value) return null
  return buildSnapshotMetaDisplay(snapshotMeta.value)
})

const scanTaskStatusLabel = computed(() => {
  const st = String(scanTaskView.value?.status || '').toLowerCase()
  if (st === 'pending') return '未运行(排队)'
  if (st === 'running') return '运行中'
  if (st === 'completed') return '已完成'
  if (st === 'failed') return '失败'
  return '未运行'
})

const scanTaskStatusType = computed(() => {
  const st = String(scanTaskView.value?.status || '').toLowerCase()
  if (st === 'running' || st === 'pending') return 'warning'
  if (st === 'completed') return 'success'
  if (st === 'failed') return 'error'
  return 'default'
})

const scanTaskSummaryText = computed(() => {
  const task = scanTaskView.value
  if (!task) return ''
  const parts = []
  if (task.startTime) parts.push(`开始 ${task.startTime}`)
  if (task.durationMs > 0) parts.push(`耗时 ${(Number(task.durationMs) / 1000).toFixed(0)}s`)
  if (task.hitTotal != null && task.status === 'completed') parts.push(`命中 ${task.hitTotal}`)
  if (task.message) parts.push(task.message)
  return parts.join(' · ')
})

const starBacktestNoDataHint = computed(() => {
  if (starBacktestLoading.value || !starBacktestRows.value.length) return ''
  const hasAnyReturn = starBacktestRows.value.some(
    (r) => Number.isFinite(r.ret1) || Number.isFinite(r.ret3) || Number.isFinite(r.ret5),
  )
  if (hasAnyReturn) return ''
  const snapDay = starBacktestRows.value[0]?.tradeDate || snapshotMeta.value?.tradeDate || ''
  const allWaiting = starBacktestRows.value.every((r) => (r.forwardDays ?? 0) <= 0)
  if (allWaiting) {
    return `快照日 ${snapDay} 仍是当前最新 K 线，尚无后续交易日，1/3/5 日收益无法计算。请等几个交易日后再回测，或选择更早的历史快照。`
  }
  return '部分股票后续 K 线不足，汇总仅统计已有数据的周期。'
})

function resolveRowStockCode(row) {
  const code = String(row?.SECURITY_CODE || row?.SECURITYCODE || '').trim()
  if (code) return code
  const secucode = String(row?.SECUCODE || '').trim()
  const m = secucode.match(/\d{6}/)
  return m ? m[0] : ''
}

function resolveRowExchange(row) {
  const secucode = String(row?.SECUCODE || '').trim().toUpperCase()
  if (secucode.endsWith('.SH') || secucode.startsWith('SH')) return 'sh'
  if (secucode.endsWith('.SZ') || secucode.startsWith('SZ')) return 'sz'
  if (secucode.endsWith('.BJ') || secucode.startsWith('BJ')) return 'bj'
  const market = String(row?.MARKET || '').trim().toUpperCase()
  if (market.includes('SH') || market.includes('沪') || market.includes('SSE')) return 'sh'
  if (market.includes('SZ') || market.includes('深') || market.includes('SZSE')) return 'sz'
  if (market.includes('BJ') || market.includes('北') || market.includes('BSE')) return 'bj'
  const code = resolveRowStockCode(row)
  if (code.startsWith('6')) return 'sh'
  if (code.startsWith('4') || code.startsWith('8')) return 'bj'
  if (code) return 'sz'
  return ''
}

function rowMatchesMarketSegment(row) {
  const segment = filterMarketSegment.value
  if (!segment) return true
  const code = resolveRowStockCode(row)
  if (segment === 'star') return code.startsWith('688') || code.startsWith('689')
  if (segment === 'gem') return code.startsWith('300') || code.startsWith('301')
  return resolveRowExchange(row) === segment
}

function filterRowsByMarketSegment(rows) {
  if (!filterMarketSegment.value) return rows || []
  return (rows || []).filter(rowMatchesMarketSegment)
}

const snapshotTopPickCodes = computed(() => {
  if (!hasSignalFilter.value || signalDataSource.value !== 'snapshot' || !signalFilteredRows.value?.length) {
    return new Set()
  }
  const toFiniteNumber = (value, defaultValue = 0) => {
    if (value == null || value === '') return defaultValue
    const n = Number(String(value).replace(/,/g, ''))
    return Number.isFinite(n) ? n : defaultValue
  }
  const preferredTags = new Set(['强', '趋', '转', '突'])
  const sorted = sortRowsByTrendScore(signalFilteredRows.value).filter((row) => {
    const summary = signalByCode.value.get(row.SECUCODE)
    if (!preferredTags.has(summary?.tag)) return false
    const score = calcTrendCompositeScore(row, summary, technicalIndicatorReactive)
    if (score.total < 75) return false
    const changeRate = toFiniteNumber(row.CHANGE_RATE, 0)
    if (changeRate > 5) return false
    const turnover = toFiniteNumber(row.TURNOVERRATE, 0)
    if (turnover > 15) return false
    return true
  })
  return new Set(sorted.slice(0, 3).map((row) => row.SECUCODE).filter(Boolean))
})

function isSnapshotTopPick(row) {
  return !!row?.SECUCODE && snapshotTopPickCodes.value.has(row.SECUCODE)
}

function pctText(value) {
  if (value == null || !Number.isFinite(Number(value))) return '--'
  const n = Number(value)
  return `${n >= 0 ? '+' : ''}${n.toFixed(2)}%`
}

function fmtBacktestMetric(value, suffix = '%') {
  if (value == null || !Number.isFinite(Number(value))) return '--'
  return `${Number(value).toFixed(2)}${suffix}`
}

function normalizeSnapshotDay(day) {
  const s = String(day || '').trim().replace(/\//g, '-')
  const m = s.match(/^(\d{4})-(\d{2})-(\d{2})/)
  return m ? `${m[1]}-${m[2]}-${m[3]}` : s.slice(0, 10)
}

function dayNum(day) {
  const s = normalizeSnapshotDay(day)
  const m = s.match(/^(\d{4})-(\d{2})-(\d{2})$/)
  if (m) return Number(`${m[1]}${m[2]}${m[3]}`)
  const compact = String(day || '').replace(/\D/g, '')
  if (/^\d{8}$/.test(compact)) return Number(compact)
  return 0
}

function countForwardBars(bars, baseIdx) {
  if (baseIdx < 0 || !bars?.length) return 0
  return Math.max(0, bars.length - 1 - baseIdx)
}

function describeStarBacktestStatus(bars, baseIdx) {
  const forward = countForwardBars(bars, baseIdx)
  if (forward <= 0) return '快照日为最新K线，尚无后续交易日'
  if (forward >= 5) return '完成'
  return `部分完成（后续 ${forward} 个交易日）`
}

function sortKlineRows(rows) {
  return [...(rows || [])].sort((a, b) => dayNum(klineDay(a)) - dayNum(klineDay(b)))
}

function klineDay(row) {
  return row?.day ?? row?.Day ?? row?.date ?? row?.Date ?? row?.time ?? row?.Time ?? ''
}

function klineNumber(row, keys) {
  for (const key of keys) {
    const raw = row?.[key]
    if (raw == null || raw === '') continue
    const n = Number(String(raw).replace(/,/g, ''))
    if (Number.isFinite(n) && n > 0) return n
  }
  return null
}

function closeValue(row) {
  return klineNumber(row, ['close', 'Close', '收盘价', '收盘'])
}

function lowValue(row) {
  return klineNumber(row, ['low', 'Low', '最低价', '最低']) ?? closeValue(row)
}

function calcForwardReturn(bars, baseIdx, days) {
  const base = closeValue(bars[baseIdx])
  const endIdx = Math.min(baseIdx + days, bars.length - 1)
  const end = endIdx > baseIdx ? closeValue(bars[endIdx]) : null
  if (!base || !end) return null
  return ((end - base) / base) * 100
}

function calcForwardDrawdown(bars, baseIdx, days) {
  const base = closeValue(bars[baseIdx])
  if (!base) return null
  const endIdx = Math.min(baseIdx + days, bars.length - 1)
  if (endIdx <= baseIdx) return null
  let minLow = base
  for (let i = baseIdx + 1; i <= endIdx; i++) {
    const low = lowValue(bars[i])
    if (low != null) minLow = Math.min(minLow, low)
  }
  return ((minLow - base) / base) * 100
}

function findSnapshotBarIndex(bars, tradeDate) {
  const target = dayNum(tradeDate)
  if (!target) return -1
  let idx = -1
  for (let i = 0; i < bars.length; i++) {
    const n = dayNum(klineDay(bars[i]))
    if (n > 0 && n <= target) idx = i
    if (n > target) break
  }
  return idx
}

function summarizeStarBacktest(rows) {
  const periods = [1, 3, 5]
  return periods.map((days) => {
    const vals = rows.map((r) => r[`ret${days}`]).filter((v) => Number.isFinite(v))
    const dds = rows.map((r) => r[`dd${days}`]).filter((v) => Number.isFinite(v))
    const win = vals.filter((v) => v > 0).length
    const avg = vals.length ? vals.reduce((a, b) => a + b, 0) / vals.length : null
    const maxDd = dds.length ? Math.min(...dds) : null
    return {
      days,
      sample: vals.length,
      winRate: vals.length ? (win / vals.length) * 100 : null,
      avgReturn: avg,
      maxDrawdown: maxDd,
    }
  })
}

async function runStarBacktest() {
  if (!snapshotMeta.value || signalDataSource.value !== 'snapshot') {
    message.warning('请先加载一个信号快照')
    return
  }
  const picks = signalFilteredRows.value.filter((row) => isSnapshotTopPick(row)).slice(0, 3)
  if (!picks.length) {
    message.warning('当前快照没有符合规则的星标股票')
    return
  }
  starBacktestVisible.value = true
  starBacktestLoading.value = true
  starBacktestError.value = ''
  starBacktestRows.value = []
  starBacktestSummary.value = []
  try {
    const tradeDate = normalizeSnapshotDay(snapshotMeta.value.tradeDate)
    const snapDayNum = dayNum(tradeDate)
    if (snapDayNum >= dayNum(new Date().toISOString().slice(0, 10))) {
      message.info('快照日期较近，可能尚无足够后续交易日，可先选更早的历史快照验证')
    }
    const rows = picks.map((row) => {
      const code = resolveStrategyRowCode(row) || toEastMoneyCode(row.SECUCODE)
      const name = resolveStrategyRowName(row) || row.SECURITY_NAME_ABBR || code
      return {
        code,
        name,
        tag: signalByCode.value.get(row.SECUCODE)?.tag || '',
        tradeDate,
        baseDate: '',
        baseClose: null,
        ret1: null,
        ret3: null,
        ret5: null,
        dd1: null,
        dd3: null,
        dd5: null,
        forwardDays: 0,
        status: '正在拉取K线',
      }
    })
    starBacktestRows.value = [...rows]
    for (const row of picks) {
      const code = resolveStrategyRowCode(row) || toEastMoneyCode(row.SECUCODE)
      const name = resolveStrategyRowName(row) || row.SECURITY_NAME_ABBR || code
      const result = rows.find((item) => item.code === code) || rows[0]
      try {
        const bars = sortKlineRows(await GetStockEastMoneyKLine(code, name, '101', 260))
        const baseIdx = findSnapshotBarIndex(bars, tradeDate)
        const baseClose = baseIdx >= 0 ? closeValue(bars[baseIdx]) : null
        result.baseDate = baseIdx >= 0 ? normalizeSnapshotDay(klineDay(bars[baseIdx])) : ''
        result.baseClose = baseClose
        if (baseIdx < 0 || !baseClose) {
          result.status = bars.length ? '未找到快照日K' : 'K线为空'
        } else {
          for (const days of [1, 3, 5]) {
            result[`ret${days}`] = calcForwardReturn(bars, baseIdx, days)
            result[`dd${days}`] = calcForwardDrawdown(bars, baseIdx, days)
          }
          result.forwardDays = countForwardBars(bars, baseIdx)
          result.status = describeStarBacktestStatus(bars, baseIdx)
        }
      } catch (err) {
        result.status = `K线拉取失败：${err?.message || err}`
      }
      starBacktestRows.value = [...rows]
    }
    starBacktestSummary.value = summarizeStarBacktest(rows)
  } catch (err) {
    starBacktestError.value = err?.message || String(err)
  } finally {
    starBacktestLoading.value = false
  }
}

function hitToSummary(hit) {
  return enrichHitToSummary({
    ...hit,
    tag: normalizeScreenSignalTag(hit.tag),
  })
}

function hitToRow(hit, snapTradeDate = '') {
  return enrichHitToRow(hit, snapTradeDate || snapshotTradeDate.value || snapshotMeta.value?.tradeDate || '')
}

function getRowStockCodeForQuote(row) {
  return resolveStrategyRowCode(row) || row?.SECUCODE || ''
}

function getLiveQuote(row) {
  const code = getRowStockCodeForQuote(row)
  return code ? liveQuoteByCode.value.get(code) : null
}

function renderPctCell(rate, { muted = false } = {}) {
  if (rate == null || !Number.isFinite(Number(rate))) {
    return h(NText, { depth: 3 }, { default: () => '—' })
  }
  const n = Number(rate)
  const type = muted ? undefined : (n >= 0 ? 'error' : 'success')
  return h(NText, { type }, { default: () => formatPctWithSign(n) })
}

function renderSnapshotPctCell(row) {
  if (signalDataSource.value !== 'snapshot') {
    return h(NText, { depth: 3 }, { default: () => '—' })
  }
  const snap = resolveSnapshotChange(row, snapshotMeta.value?.tradeDate)
  if (snap.rate == null) {
    return h(NText, { depth: 3 }, { default: () => '—' })
  }
  return h('div', { class: 'opp-pct-cell' }, [
    renderPctCell(snap.rate),
    snap.date
      ? h(NText, { depth: 3, class: 'opp-pct-cell__date' }, { default: () => `(${snap.date})` })
      : null,
  ])
}

async function refreshLiveQuotesForRows(rows) {
  const codes = collectQuoteCodesFromRows(rows, getRowStockCodeForQuote)
  const { toFetch } = planQuoteRefresh(codes, liveQuoteByCode.value, quoteFetchInFlight)
  if (!toFetch.length) return

  const concurrency = 8
  let next = liveQuoteByCode.value
  let updated = false
  for (let i = 0; i < toFetch.length; i += concurrency) {
    const batch = toFetch.slice(i, i + concurrency)
    await Promise.all(
      batch.map(async (code) => {
        if (quoteFetchInFlight.has(code)) return
        quoteFetchInFlight.add(code)
        try {
          const quote = await GetStockRealTimePrice(code)
          const entry = parseLiveQuoteResponse(quote)
          if (entry) {
            next = mergeQuoteIntoCache(next, code, entry)
            updated = true
          }
        } catch {
          /* ignore single quote failure */
        } finally {
          quoteFetchInFlight.delete(code)
        }
      }),
    )
  }
  if (updated) {
    liveQuoteByCode.value = next
    dataRefreshKey.value++
  }
}

function applySnapshotPayload(payload, snap) {
  signalScanGeneration.value++
  const prevSnapId = snapshotMeta.value?.id
  lastSnapshotPayload.value = payload || null
  const mapObj = {}
  for (const hit of payload?.items || []) {
    if (!hit?.SECUCODE || !SCREEN_SNAPSHOT_SIGNAL_TAG_SET.has(normalizeScreenSignalTag(hit.tag))) continue
    mapObj[hit.SECUCODE] = hitToSummary(hit)
  }
  signalByCode.value = new Map(Object.entries(mapObj))
  const tags = filterSignalTags.value
  const industry = filterIndustry.value || ''
  let rows = (payload?.items || [])
    .filter((hit) => hit?.SECUCODE && SCREEN_SNAPSHOT_SIGNAL_TAG_SET.has(normalizeScreenSignalTag(hit.tag)))
    .map((hit) => hitToRow(hit, snap?.tradeDate || payload?.tradeDate || ''))
  rows = filterRowsByMarketSegment(rows)
  if (industry) {
    rows = rows.filter((r) => r.INDUSTRY === industry)
  }
  signalFilteredRows.value = tags.length
    ? rows.filter((row) =>
        passesSignalTagFilter(row, mapObj[row.SECUCODE], tags, signalFilterPassOptions.value),
      )
    : rows
  paginationReactive.page = 1
  paginationReactive.itemCount = signalFilteredRows.value.length
  paginationReactive.pageCount = Math.max(1, Math.ceil(signalFilteredRows.value.length / paginationReactive.pageSize))
  snapshotMeta.value = snap
  if (snap?.id != null && snap.id !== prevSnapId) {
    liveQuoteByCode.value = new Map()
  }
  signalDataSource.value = 'snapshot'
  dataRefreshKey.value++
  loadOpportunityUserActions()
  scheduleDecisionProjectionReload()
}
function scheduleDecisionProjectionReload() {
  if (decisionProjectionDebounceTimer) {
    clearTimeout(decisionProjectionDebounceTimer)
  }
  decisionProjectionDebounceTimer = setTimeout(() => {
    decisionProjectionDebounceTimer = null
    loadDecisionProjectionsForTable()
  }, 300)
}
async function loadDecisionProjectionsForTable() {
  if (!showOpportunityColumns.value) {
    decisionProjectionByCode.value = new Map()
    decisionProjectionLoading.value = false
    decisionProjectionError.value = false
    return
  }
  const tradeDate = snapshotMeta.value?.tradeDate || snapshotTradeDate.value
  if (!tradeDate) {
    decisionProjectionByCode.value = new Map()
    decisionProjectionLoading.value = false
    decisionProjectionError.value = false
    return
  }
  const token = ++decisionProjectionLoadToken
  decisionProjectionLoading.value = true
  decisionProjectionError.value = false
  try {
    const limit = resolveProjectionBatchLimit(
      signalFilteredRows.value.length,
      paginationReactive.pageSize,
    )
    const strategyId = selectedScreenStrategyId.value
    const result = await fetchOpportunityProjections({
      tradeDate,
      limit,
      strategyId: strategyId && strategyId !== 'default' ? strategyId : undefined,
    })
    if (token !== decisionProjectionLoadToken) return
    decisionProjectionByCode.value = buildProjectionMap(result.items)
    decisionProjectionError.value = false
    dataRefreshKey.value++
  } catch {
    if (token !== decisionProjectionLoadToken) return
    decisionProjectionByCode.value = new Map()
    decisionProjectionError.value = true
    dataRefreshKey.value++
  } finally {
    if (token === decisionProjectionLoadToken) {
      decisionProjectionLoading.value = false
    }
  }
}
async function loadOpportunityUserActions() {
  if (!showOpportunityColumns.value || !snapshotMeta.value?.id) {
    opportunityEntryBySecucode.value = Object.create(null)
    return
  }
  try {
    const pool = await fetchOpportunityList({
      tradeDate: snapshotMeta.value.tradeDate,
      session: snapshotMeta.value.session || snapshotSession.value,
      strategyId: selectedScreenStrategyId.value,
      includeUserAction: true,
    })
    opportunityEntryBySecucode.value = indexOpportunityEntriesBySecucode(pool.entries)
    dataRefreshKey.value++
  } catch {
    opportunityEntryBySecucode.value = Object.create(null)
  }
}

async function watchOpportunityRow(row) {
  if (opportunityWatchSaving.value) return
  const entry = resolveOpportunityEntryForRow(row, opportunityEntryBySecucode.value)
  const watched = isOpportunityWatched(entry)
  const payload = watched
    ? buildIgnoreActionPayload({
        snap: snapshotMeta.value,
        row,
        signalSummary: signalByCode.value.get(row.SECUCODE),
      })
    : buildWatchActionPayload({
        snap: snapshotMeta.value,
        row,
        signalSummary: signalByCode.value.get(row.SECUCODE),
      })
  if (!payload.scanBatchKey) {
    message.warning(watched ? '当前无扫描批次，无法取消跟踪' : '当前无扫描批次，无法跟踪')
    return
  }
  opportunityWatchSaving.value = true
  try {
    await postOpportunityAction({
      scanBatchKey: payload.scanBatchKey,
      secucode: payload.secucode,
      signalTime: payload.signalTime,
      signalTag: payload.signalTag,
      action: watched ? OPPORTUNITY_ACTION.IGNORE : OPPORTUNITY_ACTION.WATCH,
    })
    await loadOpportunityUserActions()
    message.success(watched ? '已取消跟踪' : '已加入跟踪')
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    opportunityWatchSaving.value = false
  }
}
async function refreshSnapshotBanner() {
  try {
    if (!selectedSnapshotHistoryValue.value) {
      snapshotMeta.value = null
      return
    }
    const snap = await GetLatestSignalScanSnapshotByStrategy(snapshotTradeDate.value, snapshotSession.value, selectedScreenStrategyId.value)
    if (snap?.id) {
      snapshotMeta.value = snap
    }
  } catch {
    /* ignore */
  }
}

async function tryLoadSignalSnapshot() {
  try {
    if (!selectedSnapshotHistoryValue.value || !snapshotTradeDate.value) return false
    const snap = await GetLatestSignalScanSnapshotByStrategy(snapshotTradeDate.value, snapshotSession.value, selectedScreenStrategyId.value)
    if (!snap?.id) return false
    const payload = await ParseSignalScanSnapshotPayload(snap)
    if (!payload?.items?.length && !payload?.scannedTotal) return false
    applySnapshotPayload(payload, snap)
    return true
  } catch {
    return false
  }
}

async function loadSnapshotHistoryOptions() {
  try {
    const res = await ListSignalScanSnapshots({ page: 1, pageSize: 30, session: snapshotSession.value, strategyId: selectedScreenStrategyId.value })
    const opts = (res?.data || []).map((s) => {
      const label = buildSnapshotHistoryLabel(s)
      return { label, value: `${s.tradeDate}|${s.session}` }
    })
    snapshotHistoryOptions.value = opts
  } catch {
    snapshotHistoryOptions.value = []
  }
}

function onSnapshotHistoryChange(val) {
  selectedSnapshotHistoryValue.value = val || null
  if (!val) {
    snapshotTradeDate.value = ''
    snapshotSession.value = 'close'
    snapshotMeta.value = null
    lastSnapshotPayload.value = null
    signalDataSource.value = ''
    if (hasSignalFilter.value) refreshStocks()
    return
  }
  const [d, session] = String(val).split('|')
  snapshotTradeDate.value = d || ''
  snapshotSession.value = session || 'close'
  if (hasSignalFilter.value) refreshStocks()
}

function onScreenStrategyChange() {
  snapshotTradeDate.value = ''
  selectedSnapshotHistoryValue.value = null
  snapshotMeta.value = null
  signalDataSource.value = ''
  signalByCode.value = new Map()
  signalFilteredRows.value = []
  paginationReactive.page = 1
  loadSnapshotHistoryOptions()
  refreshSnapshotBanner()
  if (hasSignalFilter.value) {
    refreshStocks(1)
  } else {
    signalDataSource.value = 'live_scan'
    signalScanGeneration.value++
    scanPageSignals(dataRef.value)
  }
}

function normalizeFollowCode(code) {
  return String(code || '').trim().toLowerCase()
}

function getFollowStockCode(row) {
  return normalizeFollowCode(toFollowCodeFromRow(row))
}

function isRowFollowed(row) {
  const code = getFollowStockCode(row)
  return !!code && followedStockCodes.value.has(code)
}

async function loadFollowedStockCodes() {
  try {
    const list = await GetFollowList(0)
    const next = new Set()
    for (const item of list || []) {
      const code = item?.stockCode || item?.StockCode || item?.code || item?.Code
      if (code) next.add(normalizeFollowCode(code))
    }
    followedStockCodes.value = next
  } catch {
    followedStockCodes.value = new Set()
  }
}

function onBackendSignalScanProgress(p) {
  if (!p) return
  backendScanLoading.value = true
  signalScanLoading.value = true
  signalScanProgress.value = { phase: p.phase || 'scan', done: p.done || 0, total: p.total || 0 }
  const phaseLabel = p.phase === 'fetch' ? '拉取名单' : p.phase === 'compute' ? '计算信号' : '扫描'
  signalScanStatus.value = `${phaseLabel} ${p.done || 0}/${p.total || 0}...`
  if (scanTaskView.value) {
    scanTaskView.value = {
      ...scanTaskView.value,
      status: 'running',
      phase: p.phase || scanTaskView.value.phase,
      done: p.done || 0,
      total: p.total || 0,
    }
  }
}

function stopScanTaskPoll() {
  if (scanTaskPollTimer) {
    clearInterval(scanTaskPollTimer)
    scanTaskPollTimer = null
  }
}

async function refreshScanTaskView() {
  try {
    const id = scanTaskView.value?.taskId
    const task = id ? await GetSignalScanTask(id) : await GetLatestSignalScanTask()
    if (task) scanTaskView.value = task
    return task
  } catch {
    return null
  }
}

function applySignalScanTaskPollOutcome(taskStatus) {
  const outcome = resolveSignalScanTaskPollOutcome(taskStatus)
  if (!outcome.shouldStopPoll) return false
  stopScanTaskPoll()
  if (outcome.releaseBackendLoading) {
    backendScanLoading.value = false
  }
  if (outcome.releaseSignalScanLoading) {
    signalScanLoading.value = false
  }
  if (outcome.clearScanStatus) {
    signalScanStatus.value = ''
  }
  return true
}

function startScanTaskPoll() {
  stopScanTaskPoll()
  scanTaskPollTimer = setInterval(async () => {
    const task = await refreshScanTaskView()
    applySignalScanTaskPollOutcome(task?.status)
  }, 2000)
}

async function onBackendSignalScanDone(ev) {
  backendScanLoading.value = false
  signalScanLoading.value = false
  signalScanStatus.value = ''
  stopScanTaskPoll()
  await refreshScanTaskView()
  await loadSnapshotHistoryOptions()
  if (ev?.ok === false || scanTaskView.value?.status === 'failed') {
    message.error(ev?.error || scanTaskView.value?.error || '快照扫描失败')
    return
  }
  const tradeDate = ev?.tradeDate || scanTaskView.value?.tradeDate
  const session = ev?.session || scanTaskView.value?.session || snapshotSession.value
  if (tradeDate) {
    selectedSnapshotHistoryValue.value = `${tradeDate}|${session}`
    snapshotTradeDate.value = tradeDate
    snapshotSession.value = session
    await tryLoadSignalSnapshot()
    message.success(`快照完成：命中 ${ev?.hitTotal ?? scanTaskView.value?.hitTotal ?? 0} 只有信号`)
    const tip = buildNonTradingDaySnapshotTip({
      tradeDate,
      createdAt: snapshotMeta.value?.createdAt || new Date(),
    })
    if (tip) message.info(tip)
  } else if (signalDataSource.value === 'snapshot' || hasSignalFilter.value) {
    message.success('全市场信号快照已更新')
  }
}

async function runBackendSnapshotScan() {
  if (await IsSignalScanRunning()) {
    message.warning('后台扫描进行中，请稍候')
    await refreshScanTaskView()
    startScanTaskPoll()
    return
  }
  backendScanLoading.value = true
  signalScanLoading.value = true
  signalScanStatus.value = '已提交后台全市场扫描...'
  try {
    const paramsJson = selectedScreenStrategyParams.value ? serializeSignalParams(selectedScreenStrategyParams.value) : ''
    const task = await StartSignalScanSnapshot(
      snapshotSession.value,
      paramsJson,
      selectedScreenStrategyId.value,
      selectedScreenStrategy.value?.name || '',
    )
    scanTaskView.value = task
    message.success('快照任务已创建，正在后台执行（可继续浏览；完成后自动刷新）')
    startScanTaskPoll()
  } catch (err) {
    backendScanLoading.value = false
    signalScanLoading.value = false
    signalScanStatus.value = ''
    await refreshScanTaskView()
    message.error('无法启动快照任务: ' + (err?.message || err))
  }
}

/** Phase6.7-G: reuse snapshot only; never auto full-market rescan. */
async function ensureSnapshotForSignalFilter() {
  if (signalDataSource.value === 'snapshot' && lastSnapshotPayload.value) {
    applySnapshotPayload(lastSnapshotPayload.value, snapshotMeta.value)
    return true
  }
  if (selectedSnapshotHistoryValue.value) {
    const loaded = await tryLoadSignalSnapshot()
    if (loaded) return true
  }
  try {
    const snap = await GetLatestSignalScanSnapshotByStrategy(
      '',
      snapshotSession.value || 'close',
      selectedScreenStrategyId.value || 'default',
    )
    if (snap?.id) {
      selectedSnapshotHistoryValue.value = `${snap.tradeDate}|${snap.session || snapshotSession.value}`
      snapshotTradeDate.value = snap.tradeDate || ''
      snapshotSession.value = snap.session || snapshotSession.value
      const payload = await ParseSignalScanSnapshotPayload(snap)
      if (payload?.items?.length || payload?.scannedTotal) {
        applySnapshotPayload(payload, snap)
        await loadSnapshotHistoryOptions()
        return true
      }
    }
  } catch {
    /* ignore */
  }
  message.warning('需要先生成快照后再按信号筛选（已禁止自动全市场重扫）')
  return false
}

async function loadLiveSignalScan() {
  snapshotMeta.value = null
  snapshotTradeDate.value = ''
  selectedSnapshotHistoryValue.value = null
  lastSnapshotPayload.value = null
  signalDataSource.value = 'live'
  signalFilteredRows.value = []
  signalByCode.value = new Map()
  liveQuoteByCode.value = new Map()
  quoteFetchInFlight.clear()
  if (hasSignalFilter.value) {
    message.info('已退出快照视图。按信号筛选请选择历史快照或先「生成快照」。')
  }
  loadingRef.value = true
  try {
    const pageSize = paginationReactive.pageSize
    const res = await GetAllStocks(1, pageSize, paginationReactive.keyword, filterIndustry.value || '', '', '', technicalIndicatorReactive)
    if (res?.result) {
      const rows = filterRowsByMarketSegment(Array.isArray(res.result.data) ? res.result.data : [])
      dataRef.value = rows
      dataRefreshKey.value++
      paginationReactive.page = 1
      paginationReactive.pageCount = Math.max(1, Math.ceil((res.result.count || rows.length) / pageSize))
      paginationReactive.itemCount = res.result.count ?? rows.length
      if (rows.length && shouldRunLivePageSignalScan()) {
        signalScanGeneration.value++
        scanPageSignals(rows)
      }
    }
  } catch (err) {
    message.error('获取股票数据失败: ' + (err?.message || err))
  } finally {
    loadingRef.value = false
  }
}

const vipLevel = ref('')
const vipStartTime=ref("");
const vipEndTime=ref("");
const expired=ref(false)
const isValidVip=ref(false) // 鏄惁鏄細鍛?

function buildCodeNameSignalTrendColumns() {
  return [
  {
    title: '股票代码',
    key: 'SECUCODE',
    width: 100,
    render(row) {
      return h(NText, { type: "info" }, { default: () => row.SECUCODE })
    }
  },
  {
    title: '股票名称',
    key: 'SECURITY_NAME_ABBR',
    width: 160,
    fixed: 'left',
    render(row) {
      const topPick = isSnapshotTopPick(row)
      const model = toStockDisplayModel({
        stock_code: toFollowCodeFromRow(row),
        stock_name: resolveStrategyRowName(row),
      })
      const label = `${topPick ? '★ ' : ''}${model.displayText || resolveStrategyRowName(row) || '—'}`
      return h(
        NButton,
        {
          text: true,
          type: 'success',
          style: 'font-weight: 600',
          onClick: () => showKline(row),
        },
        { default: () => label },
      )
    }
  },
  {
    title: '信号',
    key: 'signal',
    width: 72,
    fixed: 'left',
    render(row) {
      const s = signalByCode.value.get(row.SECUCODE)
      if (signalScanLoading.value && !s) {
        return h(NText, { depth: 3 }, { default: () => '...' })
      }
      if (!s?.ok) {
        return h(NText, { depth: 3 }, { default: () => '无' })
      }
      if (s.tag) {
        const label = formatSignalTagLabel(s.tag, s.sellPositionPct)
        const tagEl = h(
          NTag,
          {
            size: 'small',
            bordered: true,
            color: getSignalTagColor(s.tag),
            style: 'cursor: pointer',
            onClick: () => showKline(row, s.tag),
          },
          { default: () => label },
        )
        if (s.statusText) {
          return h(NTooltip, { trigger: 'hover' }, {
            trigger: () => tagEl,
            default: () => s.statusText,
          })
        }
        return tagEl
      }
      return h(NText, { depth: 3 }, { default: () => '无' })
    }
  },
  {
    title: '趋势分',
    key: 'trendScore',
    width: 56,
    fixed: 'left',
    render(row) {
      if (signalScanLoading.value && !signalByCode.value.get(row.SECUCODE)) {
        return h(NText, { depth: 3 }, { default: () => '...' })
      }
      const s = signalByCode.value.get(row.SECUCODE)
      const score = calcTrendCompositeScore(row, s, technicalIndicatorReactive)
      const text = h(
        NText,
        { style: { cursor: 'help', ...getTrendScoreStyle(score.total) } },
        { default: () => String(score.total) },
      )
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => text,
          default: () =>
            `${score.tooltip}\n综合权重：信号 45% · 动能 20% · 流动 20% · 形态 15%\n仅供趋势/突破筛选参考，非投资建议`,
        },
      )
    }
  },
  ]
}

/** Phase16.19-B2: Opportunity identity → StockLink → existing StockKlineModal. */
function renderOpportunityStockCell(row) {
  const topPick = isSnapshotTopPick(row)
  const model = toStockDisplayModel({
    stock_code: toFollowCodeFromRow(row),
    stock_name: resolveStrategyRowName(row),
  })
  const prefix = topPick
    ? h(NText, { type: 'warning', style: 'margin-right: 2px' }, { default: () => '★' })
    : null
  if (!model.klineKey) {
    return h('span', null, [
      prefix,
      h(NText, null, { default: () => model.displayText || resolveStrategyRowName(row) || '—' }),
    ])
  }
  return h('span', { class: 'opp-stock-cell', style: 'display:inline-flex;align-items:center;gap:2px;max-width:100%' }, [
    prefix,
    h(StockLink, {
      model,
      onOpen: () => showKline(row),
    }),
  ])
}

function buildOpportunitySignalColumn() {
  return {
    title: '信号',
    key: 'signal',
    width: 72,
    render(row) {
      const s = signalByCode.value.get(row.SECUCODE)
      if (signalScanLoading.value && !s) {
        return h(NText, { depth: 3 }, { default: () => '…' })
      }
      if (!s?.ok) {
        return h(NText, { depth: 3 }, { default: () => '—' })
      }
      if (s.tag) {
        const label = formatSignalTagLabel(s.tag, s.sellPositionPct)
        const tagEl = h(
          NTag,
          {
            size: 'small',
            bordered: true,
            color: getSignalTagColor(s.tag),
            style: 'cursor: pointer',
            onClick: () => showKline(row, s.tag),
          },
          { default: () => label },
        )
        if (s.statusText) {
          return h(NTooltip, { trigger: 'hover' }, {
            trigger: () => tagEl,
            default: () => s.statusText,
          })
        }
        return tagEl
      }
      return h(NText, { depth: 3 }, { default: () => '—' })
    },
  }
}

function buildOpportunityScoreColumn() {
  return {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', { class: 'col-tip' }, '评分'),
          default: () => '趋势综合分（筛选参考），非交易计划 item.score',
        },
      ),
    key: 'trendScore',
    width: 64,
    render(row) {
      if (signalScanLoading.value && !signalByCode.value.get(row.SECUCODE)) {
        return h(NText, { depth: 3 }, { default: () => '…' })
      }
      const s = signalByCode.value.get(row.SECUCODE)
      const score = calcTrendCompositeScore(row, s, technicalIndicatorReactive)
      if (!Number.isFinite(Number(score.total))) {
        return h(NText, { depth: 3 }, { default: () => '—' })
      }
      const text = h(
        NText,
        { style: { cursor: 'help', ...getTrendScoreStyle(score.total) } },
        { default: () => String(score.total) },
      )
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => text,
          default: () =>
            `${score.tooltip}\n综合权重：信号 45% · 动能 20% · 流动 20% · 形态 15%\n仅供趋势/突破筛选参考，非投资建议`,
        },
      )
    },
  }
}

function buildOpportunityRiskColumn() {
  return {
    title: '风险',
    key: 'risk',
    width: 120,
    ellipsis: { tooltip: true },
    render(row) {
      const code = toFollowCodeFromRow(row)
      const proj = code ? decisionProjectionByCode.value.get(code) : undefined
      if (decisionProjectionLoading.value && !proj) {
        return h(NText, { depth: 3 }, { default: () => '…' })
      }
      const riskCode = String(proj?.decision?.risk_code || '').trim()
      if (riskCode) {
        return h(NText, null, { default: () => riskCode })
      }
      const s = signalByCode.value.get(row.SECUCODE)
      const rowSignalTag = s?.tag ? formatSignalTagLabel(s.tag, s.sellPositionPct) : ''
      const view = buildDecisionBadgeView(proj, rowSignalTag, {
        loading: decisionProjectionLoading.value,
        error: decisionProjectionError.value,
      })
      // Fallback: decision tier as risk-context when risk_code absent (no blank/undefined).
      return h(NTooltip, { trigger: 'hover' }, {
        trigger: () =>
          h(NTag, { size: 'small', type: view.tagType, bordered: false }, { default: () => view.tagLabel || '—' }),
        default: () => view.tooltipLines.join('\n'),
      })
    },
  }
}

function buildOpportunityActionColumn() {
  return {
    title: '操作',
    key: 'actions',
    width: 268,
    fixed: 'right',
    render(row) {
      const entry = resolveOpportunityEntryForRow(row, opportunityEntryBySecucode.value)
      const watched = isOpportunityWatched(entry)
      const followed = isRowFollowed(row)
      return h(NFlex, { size: 4 }, {
        default: () => [
          h(
            NButton,
            {
              secondary: true,
              size: 'small',
              type: 'info',
              onClick: () => openProjectionDrawer(row),
            },
            { default: () => '查看解释' },
          ),
          h(
            NButton,
            {
              secondary: true,
              size: 'small',
              type: watched ? 'default' : 'primary',
              disabled: opportunityWatchSaving.value,
              loading: opportunityWatchSaving.value,
              onClick: () => watchOpportunityRow(row),
            },
            { default: () => (watched ? '取消跟踪' : '跟踪') },
          ),
          h(
            NButton,
            {
              secondary: true,
              size: 'small',
              type: followed ? 'default' : 'tertiary',
              disabled: followed,
              onClick: () => followRow(row),
            },
            { default: () => (followed ? '已自选' : '加入自选') },
          ),
        ],
      })
    },
  }
}

function projectionForOpportunityRow(row) {
  const code = toFollowCodeFromRow(row)
  const fromApi = code ? decisionProjectionByCode.value.get(code) : undefined
  if (fromApi) return fromApi
  // Snapshot hits not in CandidatePool batch → still show Strategy/理由 from in-memory signal.
  const summary = row?.SECUCODE ? signalByCode.value.get(row.SECUCODE) : null
  return buildSignalSnapshotDisplayProjection(
    summary,
    selectedScreenStrategy.value?.name || '',
  ) || undefined
}

function buildOpportunitySourceColumn() {
  return {
    title: '来源',
    key: 'explainSource',
    width: 88,
    render(row) {
      const proj = projectionForOpportunityRow(row)
      if (decisionProjectionLoading.value && !proj) {
        return h(NText, { depth: 3 }, { default: () => '…' })
      }
      const chip = opportunitySourceChipMeta(proj)
      return h(
        NTag,
        { size: 'tiny', bordered: false, type: chip.type },
        { default: () => chip.label },
      )
    },
  }
}

function buildOpportunityStrategyColumn() {
  return {
    title: '策略',
    key: 'explainStrategy',
    width: 110,
    ellipsis: { tooltip: true },
    render(row) {
      const proj = projectionForOpportunityRow(row)
      if (decisionProjectionLoading.value && !proj) {
        return h(NText, { depth: 3 }, { default: () => '…' })
      }
      return formatOpportunityTableCell(resolveOpportunityStrategyLabel(proj))
    },
  }
}

function buildOpportunityReasonColumn() {
  return {
    title: '入选理由',
    key: 'explainReason',
    width: 140,
    ellipsis: { tooltip: true },
    render(row) {
      const proj = projectionForOpportunityRow(row)
      if (decisionProjectionLoading.value && !proj) {
        return h(NText, { depth: 3 }, { default: () => '…' })
      }
      const summary = resolveOpportunityReasonSummary(proj)
      if (!summary) {
        return h(NText, { depth: 3 }, { default: () => '—' })
      }
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h(NText, null, { default: () => summary }),
          default: () =>
            str(proj?.signal?.trigger_reason) ||
            str(proj?.signal?.signal_tag) ||
            summary,
        },
      )
    },
  }
}

function str(v) {
  if (v == null) return ''
  return String(v).trim()
}

function buildActionColumn() {
  return {
    title: '操作',
    key: 'actions',
    width: 120,
    fixed: 'right',
    render(row) {
      const followed = isRowFollowed(row)
      return h(NFlex, { size: 4 }, {
        default: () => [
          h(
            NButton,
            {
              secondary: true,
              size: 'small',
              type: 'warning',
              onClick: () => showKline(row),
            },
            { default: () => '日K' },
          ),
          h(
            NButton,
            {
              secondary: true,
              size: 'small',
              type: followed ? 'default' : 'primary',
              disabled: followed,
              onClick: () => followRow(row),
            },
            { default: () => followed ? '已关注' : '关注' },
          ),
        ],
      })
    }
  }
}

const opportunityPriceColumns = [
  {
    title: priceColumnTitle(PRICE_KIND.signal),
    key: 'SIGNAL_PRICE',
    width: 110,
    render(row) {
      const s = signalByCode.value.get(row.SECUCODE)
      const { signalPrice, signalPriceStatus } = resolveSignalFields(row, s)
      if (signalPrice == null) {
        const label = isMissingSignalPriceStatus(signalPriceStatus) ? '未记录' : '—'
        return h(
          NTooltip,
          { trigger: 'hover' },
          {
            trigger: () => h(NText, { depth: 3 }, { default: () => label }),
            default: () => vsSignalReturnMissingHint(signalPrice, signalPriceStatus) || label,
          },
        )
      }
      const derived = isDerivedSignalPriceStatus(signalPriceStatus)
      const ctx = formatPriceWithContext(signalPrice, PRICE_KIND.signal, {
        digits: 2,
        extraTooltip: signalPriceTooltipForStatus(signalPriceStatus),
      })
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () =>
            h(
              NSpace,
              { size: 4, align: 'center', wrap: false },
              {
                default: () => [
                  h(NText, { type: 'info' }, { default: () => ctx.value }),
                  derived
                    ? h(NTag, { size: 'tiny', type: 'warning', bordered: false }, { default: () => '推算' })
                    : null,
                ],
              },
            ),
          default: () => ctx.tooltip,
        },
      )
    },
  },
  {
    title: '信号时间',
    key: 'SIGNAL_TIME',
    width: 108,
    render(row) {
      const s = signalByCode.value.get(row.SECUCODE)
      const { signalTime, signalDaysAgo } = resolveSignalFields(row, s)
      if (signalTime) {
        return h(NText, { depth: 2 }, { default: () => signalTime })
      }
      if (signalDaysAgo != null && signalDaysAgo > 0) {
        return h(NText, { depth: 3 }, { default: () => `${signalDaysAgo} 日前信号` })
      }
      return h(NText, { depth: 3 }, { default: () => '—' })
    },
  },
  {
    title: priceColumnTitle(PRICE_KIND.last),
    key: 'LIVE_PRICE',
    width: 110,
    render(row) {
      const live = getLiveQuote(row)
      const price = resolveLatestPrice(row, live, { preferLive: signalDataSource.value === 'snapshot' })
      if (price == null) {
        return h(NText, { depth: 3 }, { default: () => '—' })
      }
      const loading = signalDataSource.value === 'snapshot' && !live?.price
      const ctx = formatPriceWithContext(price, PRICE_KIND.last, {
        digits: 2,
        extraTooltip: loading ? '快照视图：最新行情价刷新中（*）' : undefined,
      })
      return h(NText, { type: 'info', depth: loading ? 3 : undefined }, {
        default: () => (loading ? `${ctx.value}*` : ctx.value),
      })
    },
  },
  {
    title: '信号以来涨跌',
    key: 'SIGNAL_RETURN',
    width: 108,
    render(row) {
      const s = signalByCode.value.get(row.SECUCODE)
      const { signalPrice, signalPriceStatus } = resolveSignalFields(row, s)
      const live = getLiveQuote(row)
      const latest = resolveLatestPrice(row, live, { preferLive: true })
      const ret = calcReturnSinceSignal(latest, signalPrice)
      const missingHint = vsSignalReturnMissingHint(signalPrice, signalPriceStatus)
      if (ret == null && missingHint) {
        return h(
          NTooltip,
          { trigger: 'hover' },
          {
            trigger: () => h(NText, { depth: 3 }, { default: () => '—' }),
            default: () => missingHint,
          },
        )
      }
      return renderPctCell(ret)
    },
  },
  {
    title: '快照涨幅',
    key: 'SNAPSHOT_CHANGE',
    width: 118,
    render(row) {
      return renderSnapshotPctCell(row)
    },
  },
  {
    title: '实时涨跌',
    key: 'LIVE_CHANGE',
    width: 92,
    render(row) {
      const live = getLiveQuote(row)
      const rate = resolveLiveChangeRate(row, live, { isSnapshotView: signalDataSource.value === 'snapshot' })
      return renderPctCell(rate)
    },
  },
  buildOpportunityActionColumn(),
]

const opportunityColumns = [
  {
    title: '股票',
    key: 'stock',
    width: 160,
    fixed: 'left',
    ellipsis: { tooltip: true },
    render(row) {
      return renderOpportunityStockCell(row)
    },
  },
  buildOpportunitySourceColumn(),
  buildOpportunityStrategyColumn(),
  buildOpportunitySignalColumn(),
  buildOpportunityReasonColumn(),
  buildOpportunityScoreColumn(),
  buildOpportunityRiskColumn(),
  buildDecisionTierColumn(),
  ...opportunityPriceColumns,
]

function buildDecisionTierColumn() {
  return {
    title: '决策',
    key: 'decisionTier',
    width: 80,
    render(row) {
      const code = toFollowCodeFromRow(row)
      const proj = code ? decisionProjectionByCode.value.get(code) : undefined
      const s = signalByCode.value.get(row.SECUCODE)
      const rowSignalTag = s?.tag ? formatSignalTagLabel(s.tag, s.sellPositionPct) : ''
      const view = buildDecisionBadgeView(proj, rowSignalTag, {
        loading: decisionProjectionLoading.value,
        error: decisionProjectionError.value,
      })
      const tagEl = h(
        NTag,
        {
          size: 'small',
          type: view.tagType,
          bordered: false,
          style: view.clickable ? 'cursor: pointer' : undefined,
          onClick: () => {
            if (view.retry) {
              loadDecisionProjectionsForTable()
              return
            }
            if (view.clickable) openProjectionDrawer(row)
          },
        },
        { default: () => view.tagLabel },
      )
      return h(NTooltip, { trigger: 'hover' }, {
        trigger: () => tagEl,
        default: () => view.tooltipLines.join('\n'),
      })
    },
  }
}

const baseColumns = [
  ...buildCodeNameSignalTrendColumns(),
  {
    title: '最新价',
    key: 'NEW_PRICE',
    width: 100,
    render(row) {
      const price = row.NEW_PRICE
      return h(NText, { type: "info" }, { default: () => isNumeric(price) ? price : '-' })
    }
  },
  {
    title: '涨跌幅(%)',
    key: 'CHANGE_RATE',
    width: 100,
    render(row) {
      const rate = toNumber(row.CHANGE_RATE, 0)
      const type = rate >= 0 ? 'error' : 'success'
      const sign = rate >= 0 ? '+' : ''
      return h(NText, { type: type }, { default: () => `${sign}${rate.toFixed(2)}%` })
    }
  },
  {
    title: '最高价',
    key: 'HIGH_PRICE',
    width: 100,
    render(row) {
      const price = row.HIGH_PRICE
      return h(NText, { type: "info" }, { default: () => isNumeric(price) ? price : '-' })
    }
  },
  {
    title: '最低价',
    key: 'LOW_PRICE',
    width: 100,
    render(row) {
      const price = row.LOW_PRICE
      return h(NText, { type: "info" }, { default: () => isNumeric(price) ? price : '-' })
    }
  },
  // {
  //   title: '鍓嶆敹浠?,
  //   key: 'PRE_CLOSE_PRICE',
  //   width: 100,
  //   render(row) {
  //     return h(NText, { type: "info" }, { default: () => row.PRE_CLOSE_PRICE.toFixed(2) })
  //   }
  // },
  {
    title: '成交量',
    key: 'VOLUME',
    width: 120,
    render(row) {
      const volume = toNumber(row.VOLUME, 0)
      let displayVolume = volume
      if (volume >= 100000000) {
        displayVolume = (volume / 100000000).toFixed(2) + '亿'
      } else if (volume >= 10000) {
        displayVolume = (volume / 10000).toFixed(2) + '万'
      }
      return h(NText, { type: "info" }, { default: () => displayVolume })
    }
  },
  {
    title: '成交额',
    key: 'DEAL_AMOUNT',
    width: 120,
    render(row) {
      const amount = toNumber(row.DEAL_AMOUNT, 0)
      let displayAmount = amount
      if (amount >= 100000000) {
        displayAmount = (amount / 100000000).toFixed(2) + '亿'
      } else if (amount >= 10000) {
        displayAmount = (amount / 10000).toFixed(2) + '万'
      }
      return h(NText, { type: "info" }, { default: () => displayAmount })
    }
  },
  {
    title: '换手率(%)',
    key: 'TURNOVERRATE',
    width: 80,
    render(row) {
      const rate = row.TURNOVERRATE
      return h(NText, { type: "info" }, { default: () => isNumeric(rate) ? rate : '-' })
    }
  },
  {
    title: '量比',
    key: 'VOLUME_RATIO',
    width: 80,
    render(row) {
      const ratio = row.VOLUME_RATIO
      return h(NText, { type: "info" }, { default: () => isNumeric(ratio) ? ratio : '-' })
    }
  },
  {
    title: '所属行业',
    key: 'INDUSTRY',
    width: 100,
    render(row) {
      return h(NTag, { type: "primary", size: "small" }, { default: () => row.INDUSTRY })
    }
  },
  {
    title: '所属概念',
    key: 'CONCEPT',
    width: 100,
    ellipsis: {
      tooltip: true
    },
    render(row) {
      if(typeof row.CONCEPT === 'string'){
        return h(NTag, { type: "info", size: "small" ,style: "margin-right: 4px;" }, { default: () => row.CONCEPT })
      }else{
        if (!row.CONCEPT || row.CONCEPT.length === 0) {
          return h(NText, { type: "secondary" }, { default: () => '无' })
        }
        return row.CONCEPT.map(concept =>
            h(NTag, { type: "info", size: "small", style: "margin-right: 4px;" }, { default: () => concept })
        )
      }
    }
  },
  // {
  //   title: '浜ゆ槗鎵€',
  //   key: 'MARKET',
  //   width: 100,
  //   render(row) {
  //     return h(NTag, { type: "warning", size: "small" }, { default: () => row.MARKET })
  //   }
  // },
  buildActionColumn(),
]

const tableColumns = computed(() => (showOpportunityColumns.value ? opportunityColumns : baseColumns))

const paginationReactive = reactive({
  keyword:"",
  page: 1,
  pageCount: 1,
  pageSize: 20,
  /** Phase16.20-A Beta RC: cap UI pageSize to avoid 200–500 DOM jank (audit H2). */
  pageSizes: [10, 20, 30, 50],
  showSizePicker: true,
  itemCount: 0,
  prefix({ itemCount }) {
    return formatOpportunityListCountLabel(itemCount)
  },
})

/** Phase16.26-B: opportunity/snapshot — paginate only when N > 50 (single local pager). */
const opportunityNeedsPagination = computed(() =>
  usesOpportunityRowSource.value &&
  shouldPaginateOpportunityList(signalFilteredRows.value.length, OPPORTUNITY_LIST_PAGINATE_THRESHOLD),
)

/** Table pagination: live = remote server; opportunity N≤50 = off; N>50 = local only (no pre-slice). */
const tablePagination = computed(() => {
  if (!usesOpportunityRowSource.value) {
    return paginationReactive
  }
  const n = signalFilteredRows.value.length
  if (!shouldPaginateOpportunityList(n, OPPORTUNITY_LIST_PAGINATE_THRESHOLD)) {
    return false
  }
  return {
    page: paginationReactive.page,
    pageSize: paginationReactive.pageSize,
    pageSizes: paginationReactive.pageSizes,
    showSizePicker: true,
    itemCount: n,
    prefix({ itemCount }) {
      return formatOpportunityListCountLabel(itemCount)
    },
  }
})

const opportunityListCountLabel = computed(() => {
  if (!usesOpportunityRowSource.value) return ''
  if (opportunityNeedsPagination.value) return ''
  return formatOpportunityListCountLabel(signalFilteredRows.value.length)
})
const optionsReactive= reactive([
  {
    label: '全部',
    value: ''
  },
 ])

function resetSearchOptions() {
  optionsReactive.splice(0, optionsReactive.length, { label: '全部', value: '' })
}

function buildHotIndustryRankMap(hotPlates, rankList) {
  const map = new Map()
  ;(hotPlates || []).forEach((item, idx) => {
    const name = String(item?.name || '').trim()
    if (name) map.set(name, idx + 1)
  })
  ;(rankList || []).forEach((item, idx) => {
    const name = String(item?.bd_name || '').trim()
    if (name && !map.has(name)) map.set(name, idx + 1)
  })
  return map
}

function resolveHotIndustryRank(name, hotRankMap) {
  const trimmed = String(name || '').trim()
  if (!trimmed || !hotRankMap.size) return 0
  if (hotRankMap.has(trimmed)) return hotRankMap.get(trimmed)
  for (const [hotName, rank] of hotRankMap) {
    if (hotName === trimmed || hotName.includes(trimmed) || trimmed.includes(hotName)) {
      return rank
    }
  }
  return 0
}

function loadIndustryOptions() {
  Promise.all([
    GetAllIndustries().catch(() => []),
    GetHotIndustryPlates(30).catch(() => []),
    GetIndustryRank('0', 30).catch(() => []),
  ]).then(([list, hotPlates, rankList]) => {
    const hotRankMap = buildHotIndustryRankMap(hotPlates, rankList)
    const rows = (list || []).map((name) => {
      const hotRank = resolveHotIndustryRank(name, hotRankMap)
      return {
        label: hotRank > 0 ? `${name} 🔥` : name,
        value: name,
        hot: hotRank > 0,
        hotRank,
      }
    })
    rows.sort((a, b) => {
      if (a.hot !== b.hot) return a.hot ? -1 : 1
      if (a.hot && b.hot) return a.hotRank - b.hotRank
      return a.label.localeCompare(b.label, 'zh-CN')
    })
    industryOptions.value = [{ label: '全部行业', value: '' }, ...rows]
  }).catch(() => {
    industryOptions.value = [{ label: '全部行业', value: '' }]
  })
}

async function fetchAllStocksMatchingFilters(onProgress) {
  const pageSize = 500
  const keyword = paginationReactive.keyword
  const industry = filterIndustry.value || ''
  const concept = ''
  const plateCode = ''
  let page = 1
  const all = []
  let total = 0
  while (true) {
    const res = await GetAllStocks(page, pageSize, keyword, industry, concept, plateCode, technicalIndicatorReactive)
    const batch = res?.result?.data || []
    total = res?.result?.count ?? total
    if (!batch.length) break
    all.push(...batch)
    onProgress?.({ phase: 'fetch', done: all.length, total: total || all.length })
    if (total > 0 && all.length >= total) break
    if (batch.length < pageSize) break
    page++
    if (page > 100) break
  }
  return all
}

async function scanPageSignals(rows) {
  if (!rows?.length) return
  if (!shouldRunLivePageSignalScan()) return
  const gen = signalScanGeneration.value
  signalScanLoading.value = true
  try {
    const mapObj = await scanRowsLastBarSignals(rows, {
      concurrency: SIGNAL_PAGE_SCAN_CONCURRENCY,
      signalSettings: selectedScreenStrategyParams.value,
    })
    if (!shouldRunLivePageSignalScan() || gen !== signalScanGeneration.value) return
    signalByCode.value = new Map(Object.entries(mapObj))
    dataRefreshKey.value++
  } finally {
    signalScanLoading.value = false
  }
}

async function loadWithSignalFilter() {
  signalScanLoading.value = true
  signalScanStatus.value = '正在拉取股票名单...'
  signalScanProgress.value = { phase: 'fetch', done: 0, total: 0 }
  signalByCode.value = new Map()
  try {
    const allRows = await fetchAllStocksMatchingFilters((p) => {
      signalScanProgress.value = p
      signalScanStatus.value = `正在拉取股票名单 ${p.done}/${p.total}...`
    })
    const all = filterRowsByMarketSegment(allRows)
    if (!all.length) {
      signalFilteredRows.value = []
      paginationReactive.page = 1
      paginationReactive.itemCount = 0
      paginationReactive.pageCount = 1
      message.warning('未获取到股票数据')
      return
    }
    signalScanProgress.value = { phase: 'scan', done: 0, total: all.length }
    signalScanStatus.value = `正在扫描股票 0/${all.length}...`
    const mapObj = await scanRowsLastBarSignals(all, {
      concurrency: SIGNAL_SCAN_CONCURRENCY,
      signalSettings: selectedScreenStrategyParams.value,
      onProgress: ({ done, total }) => {
        signalScanProgress.value = { phase: 'scan', done, total }
        signalScanStatus.value = `正在扫描股票 ${done}/${total}...`
      },
    })
    signalByCode.value = new Map(Object.entries(mapObj))
    const tags = filterSignalTags.value
    signalFilteredRows.value = all.filter((row) => passesSignalTagFilter(row, mapObj[row.SECUCODE], tags, signalFilterPassOptions.value))
    const reboundExcluded = hasReboundFilter.value
      ? all.filter((row) => {
          const s = mapObj[row.SECUCODE]
          return s?.tag === '弹' && !passesReboundScreenFilter(s, row, signalFilterPassOptions.value)
        }).length
      : 0
    paginationReactive.page = 1
    paginationReactive.itemCount = signalFilteredRows.value.length
    paginationReactive.pageCount = Math.max(1, Math.ceil(signalFilteredRows.value.length / paginationReactive.pageSize))
    dataRefreshKey.value++
    const reboundHint = reboundExcluded > 0
      ? `；弹已排除 ST/RSI>${reboundScreenMaxRsi.value} 共 ${reboundExcluded} 只`
      : ''
    const scopeLabel = filterIndustry.value ? `行业「${filterIndustry.value}」` : '全部行业'
    message.success(`${scopeLabel}扫描完成：${signalFilteredRows.value.length} 只含「${tags.join('、')}」信号（候选 ${all.length} 只${reboundHint}）`)
  } catch (err) {
    message.error('信号筛选失败: ' + (err?.message || err))
  } finally {
    signalScanLoading.value = false
    signalScanStatus.value = ''
    signalScanProgress.value = { phase: '', done: 0, total: 0 }
  }
}

function sortRowsByTrendScore(rows) {
  if (!rows?.length || signalScanLoading.value || signalByCode.value.size === 0) {
    return rows || []
  }
  return [...rows].sort((a, b) => {
    const sa = calcTrendCompositeScore(a, signalByCode.value.get(a.SECUCODE), technicalIndicatorReactive)
    const sb = calcTrendCompositeScore(b, signalByCode.value.get(b.SECUCODE), technicalIndicatorReactive)
    return sb.total - sa.total
  })
}

const displayData = computed(() => {
  void dataRefreshKey.value
  if (usesOpportunityRowSource.value) {
    // Phase16.26-B: pass FULL sorted rows. Naive local-paginates when N>50;
    // when N≤50 pagination is off. Do NOT pre-slice here (dual-pagination bug).
    return sortRowsByTrendScore(signalFilteredRows.value)
  }
  return sortRowsByTrendScore(dataRef.value || [])
})

/** Quote refresh target: current page only when opportunity list is paginated. */
function opportunityQuoteTargetRows() {
  const sorted = sortRowsByTrendScore(signalFilteredRows.value)
  return sliceOpportunityPage(sorted, {
    page: paginationReactive.page,
    pageSize: paginationReactive.pageSize,
    paginate: opportunityNeedsPagination.value,
  })
}

watch(
  () => [
    paginationReactive.page,
    paginationReactive.pageSize,
    signalDataSource.value,
    signalFilteredRows.value.length,
    snapshotMeta.value?.id,
  ],
  () => {
    if (
      shouldRefreshSnapshotQuotes({
        signalDataSource: signalDataSource.value,
        filteredRowCount: signalFilteredRows.value.length,
      })
    ) {
      const rows = usesOpportunityRowSource.value
        ? opportunityQuoteTargetRows()
        : displayData.value
      refreshLiveQuotesForRows(rows)
    }
  },
)

watch(
  () => [
    showOpportunityColumns.value,
    snapshotMeta.value?.tradeDate,
    snapshotMeta.value?.session,
    snapshotTradeDate.value,
    signalFilteredRows.value.length,
    selectedScreenStrategyId.value,
  ],
  () => {
    if (!showOpportunityColumns.value) return
    scheduleDecisionProjectionReload()
  },
)

async function loadStocks(page, pageSize) {
  if((vipLevel.value===""|| Number(vipLevel.value) <=0)){
    handleReset()
  }
  if (loadingRef.value) return
  loadingRef.value = true
  try {
    if (hasSignalFilter.value) {
      // Phase6.7-G: snapshot reuse only
      await ensureSnapshotForSignalFilter()
      return
    }
    const res = await GetAllStocks(page, pageSize, paginationReactive.keyword, filterIndustry.value || '', '', '', technicalIndicatorReactive)
    if (hasSignalFilter.value || isSnapshotSignalSource()) {
      return
    }
    signalFilteredRows.value = []
    signalByCode.value = new Map()
    signalDataSource.value = 'live_scan'
    if (res?.result) {
      const rows = filterRowsByMarketSegment(Array.isArray(res.result.data) ? res.result.data : [])
      dataRef.value = rows
      dataRefreshKey.value++
      paginationReactive.page = page
      paginationReactive.pageCount = Math.max(1, Math.ceil((res.result.count || rows.length) / pageSize))
      paginationReactive.itemCount = res.result.count ?? rows.length
      if (rows.length && shouldRunLivePageSignalScan()) {
        signalScanGeneration.value++
        scanPageSignals(rows)
      } else if (filterIndustry.value) {
        message.warning('未查到符合条件的股票，请稍后重试或更换行业')
      }
    } else {
      dataRef.value = []
      paginationReactive.page = 1
      paginationReactive.pageCount = 1
      paginationReactive.itemCount = 0
      message.error('获取股票数据失败')
    }
  } catch (err) {
    message.error('获取股票数据失败: ' + err.message)
  } finally {
    loadingRef.value = false
  }
}

function refreshStocks(page = paginationReactive.page) {
  loadStocks(page, paginationReactive.pageSize)
}

function handleExternalRefresh() {
  refreshStocks(paginationReactive.page)
}

function handleCheckedChange(checked) {

  if(checked&&(vipLevel.value===""|| Number(vipLevel.value) <=0)){
    handleReset()
    message.warning('未开通 VIP 或者已经过期，无法使用技术面筛选')
  }
}
function handlePageChange(currentPage) {
  if (usesOpportunityRowSource.value) {
    paginationReactive.page = currentPage
    return
  }
  loadStocks(currentPage, paginationReactive.pageSize)
}

function handlePageSizeChange(pageSize) {
  const MAX_UI_PAGE_SIZE = 50
  const next = Math.min(Math.max(1, Number(pageSize) || 20), MAX_UI_PAGE_SIZE)
  paginationReactive.pageSize = next
  if (usesOpportunityRowSource.value) {
    paginationReactive.page = 1
    paginationReactive.itemCount = signalFilteredRows.value.length
    paginationReactive.pageCount = Math.max(
      1,
      Math.ceil(signalFilteredRows.value.length / next) || 1,
    )
    return
  }
  loadStocks(1, next)
}
function handleSearch() {
  loadStocks(1, paginationReactive.pageSize)
}

function handleIndustryChange() {
  loadStocks(1, paginationReactive.pageSize)
}

function handleMarketSegmentChange() {
  loadStocks(1, paginationReactive.pageSize)
}

function handleSignalFilterChange() {
  loadStocks(1, paginationReactive.pageSize)
}

async function followRow(row) {
  const code = toFollowCodeFromRow(row)
  if (!code) {
    message.warning('无法识别股票代码')
    return
  }
  const normalizedCode = normalizeFollowCode(code)
  if (followedStockCodes.value.has(normalizedCode)) {
    message.info('已关注')
    return
  }
  const { followResult, groupInfo } = await followWithDateGroup(code, Follow)
  if (followResult === '关注成功' || String(followResult || '').includes('成功')) {
    followedStockCodes.value = new Set([...followedStockCodes.value, normalizedCode])
    message.success(formatFollowGroupMessage(followResult, groupInfo))
  } else if (followResult === '已经关注了' || String(followResult || '').includes('已') || String(followResult || '').includes('关注')) {
    followedStockCodes.value = new Set([...followedStockCodes.value, normalizedCode])
    message.info(followResult)
  } else {
    message.warning(followResult || '关注失败')
  }
}

function handleUpdateVal(value) {
  if (value === '') {
    resetSearchOptions()
  } else {
    GetAllStockInfoList({
      searchKeyWord: value
    }).then((res) => {
      if (res  && res.list) {
        optionsReactive.splice(0, optionsReactive.length)
        optionsReactive.push(...res.list.map(item => {
          return {
            label: item.SECURITY_NAME_ABBR,
            value: item.SECURITY_NAME_ABBR,
            obj: item,
          }
        }))
      }
    }).catch(err => {
      message.error('获取股票数据失败: ' + err.message)
    })
  }
}
const projectionDrawerVisible = ref(false)
const projectionDrawerRow = ref(null)

function openProjectionDrawer(row) {
  const stockCode = toFollowCodeFromRow(row)
  if (!stockCode) {
    message.warning('无法解析股票代码')
    return
  }
  projectionDrawerRow.value = {
    stockCode,
    stockName: resolveStrategyRowName(row),
    tradeDate: snapshotTradeDate.value || snapshotMeta.value?.tradeDate || '',
  }
  projectionDrawerVisible.value = true
}

const modalDataRef = reactive({
  visible: false,
  title: "",
  content: "",
  riskRemarks: "",
  stockCode: "",
  chartCode: "",
  stockName: "",
  narrativeCode: "",
  remarks: "",
  focusSignal: "",
})
function showKline(row, focusTag = '') {
  const model = toStockDisplayModel({
    stock_code: toFollowCodeFromRow(row),
    stock_name: resolveStrategyRowName(row),
  })
  if (!model.klineKey) {
    // Fallback: legacy resolve (should rarely hit for A-share rows).
    const code = resolveStrategyRowCode(row)
    const name = resolveStrategyRowName(row)
    if (!code) {
      message.warning('无法识别股票代码，请检查该行是否包含 SECUCODE')
      return
    }
    const em = toEastMoneyCode(code)
    modalDataRef.stockCode = em
    modalDataRef.stockName = name
    modalDataRef.narrativeCode = toFollowCodeFromRow(row)
    modalDataRef.focusSignal = focusTag || signalByCode.value.get(row.SECUCODE)?.tag || ''
    modalDataRef.title = `${name || em} ${em} — 多周期K线`
    modalDataRef.visible = true
    return
  }
  applyStockClickAction(model, modalDataRef)
  modalDataRef.narrativeCode = model.code
  modalDataRef.focusSignal = focusTag || signalByCode.value.get(row.SECUCODE)?.tag || ''
  // StockKlineModal binds :code="modalDataRef.stockCode" historically — map chartCode → stockCode.
  modalDataRef.stockCode = modalDataRef.chartCode || model.klineKey
}
const technicalIndicatorReactive = reactive({
  MACD_GOLDEN_FORK: false,
  KDJ_GOLDEN_FORK: false,
  BREAK_THROUGH: false,
  LOW_FUNDS_INFLOW: false,
  HIGH_FUNDS_OUTFLOW: false,
  BREAKUP_MA_5DAYS: false,
  LONG_AVG_ARRAY: false,
  SHORT_AVG_ARRAY: false,
  UPPER_LARGE_VOLUME: false,
  DOWN_NARROW_VOLUME: false,
  ONE_DAYANG_LINE: false,
  TWO_DAYANG_LINES: false,
  RISE_SUN: false,
  POWER_FULGUN: false,
  RESTORE_JUSTICE: false,
  DOWN_7DAYS: false,
  UPPER_8DAYS:false,
  UPPER_9DAYS:false,
  UPPER_4DAYS:false,
  HEAVEN_RULE:false,
  UPSIDE_VOLUME: false,
  BEARISH_ENGULFING: false,
  REVERSING_HAMMER: false,
  SHOOTING_STAR: false,
  EVENING_STAR: false,
  FIRST_DAWN: false,
  PREGNANT: false,
  BLACK_CLOUD_TOPS: false,
  MORNING_STAR: false,
  NARROW_FINISH: false,
  UPP_DAYS:0,
  CONCERN_RANK_7DAYS:0,
  UPNDAY:0,
  DOWNNDAY :0,
})

function handleReset(){
  technicalIndicatorReactive.MACD_GOLDEN_FORK = false
  technicalIndicatorReactive.KDJ_GOLDEN_FORK = false
  technicalIndicatorReactive.BREAK_THROUGH = false
  technicalIndicatorReactive.LOW_FUNDS_INFLOW = false
  technicalIndicatorReactive.HIGH_FUNDS_OUTFLOW = false
  technicalIndicatorReactive.BREAKUP_MA_5DAYS = false
  technicalIndicatorReactive.LONG_AVG_ARRAY = false
  technicalIndicatorReactive.SHORT_AVG_ARRAY = false
  technicalIndicatorReactive.UPPER_LARGE_VOLUME = false
  technicalIndicatorReactive.DOWN_NARROW_VOLUME = false
  technicalIndicatorReactive.ONE_DAYANG_LINE = false
  technicalIndicatorReactive.TWO_DAYANG_LINES = false
  technicalIndicatorReactive.RISE_SUN = false
  technicalIndicatorReactive.POWER_FULGUN = false
  technicalIndicatorReactive.RESTORE_JUSTICE = false
  technicalIndicatorReactive.DOWN_7DAYS = false
  technicalIndicatorReactive.UPPER_8DAYS=false
  technicalIndicatorReactive.UPPER_9DAYS=false
  technicalIndicatorReactive.UPPER_4DAYS=false
  technicalIndicatorReactive.HEAVEN_RULE=false
  technicalIndicatorReactive.ONE_DAYANG_LINE=false
  technicalIndicatorReactive.TWO_DAYANG_LINES= false
  technicalIndicatorReactive.RISE_SUN=false
  technicalIndicatorReactive.POWER_FULGUN=false
  technicalIndicatorReactive.RESTORE_JUSTICE=false
  technicalIndicatorReactive.DOWN_7DAYS=false
  technicalIndicatorReactive.UPPER_8DAYS=false
  technicalIndicatorReactive.UPPER_9DAYS=false
  technicalIndicatorReactive.UPPER_4DAYS=false
  technicalIndicatorReactive.HEAVEN_RULE=false
  technicalIndicatorReactive.UPSIDE_VOLUME=false
  technicalIndicatorReactive.BEARISH_ENGULFING=false
  technicalIndicatorReactive.REVERSING_HAMMER=false
  technicalIndicatorReactive.SHOOTING_STAR=false
  technicalIndicatorReactive.EVENING_STAR=false
  technicalIndicatorReactive.FIRST_DAWN=false
  technicalIndicatorReactive.PREGNANT=false
  technicalIndicatorReactive.BLACK_CLOUD_TOPS=false
  technicalIndicatorReactive.MORNING_STAR=false
  technicalIndicatorReactive.NARROW_FINISH=false
  technicalIndicatorReactive.UPP_DAYS=0
  technicalIndicatorReactive.CONCERN_RANK_7DAYS=0
  technicalIndicatorReactive.UPNDAY=0
  technicalIndicatorReactive.DOWNNDAY=0
  filterIndustry.value = ''
  filterMarketSegment.value = ''
  filterSignalTags.value = []
  signalFilteredRows.value = []
  lastSnapshotPayload.value = null
  paginationReactive.keyword = ''
}

// 鍒ゆ柇鏄惁鏄暟瀛?
const isNumeric = (value) => {
  if (value === null || value === undefined || value === '') {
    return false
  }
  return !isNaN(Number(value))
}

// 瀹夊叏杞崲鏁板瓧
const toNumber = (value, defaultValue = 0) => {
  const num = Number(value)
  return isNaN(num) ? defaultValue : num
}

</script>

<template>
  <div class="stock-screen-page">
    <div class="stock-screen-top">
      <n-collapse v-model:expanded-names="filterCollapseExpanded" class="stock-filter-collapse">
        <n-collapse-item title="技术面筛选条件" name="filters">
      <n-card size="small" class="filter-card" style="width: 100%; text-align: left">
        <n-space wrap :size="[12, 8]" align="center" item-style="display: flex;">
        <n-checkbox   @update:checked="handleCheckedChange" v-model:checked="technicalIndicatorReactive.MACD_GOLDEN_FORK">
        MACD金叉
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.KDJ_GOLDEN_FORK">
        KDJ金叉
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.BREAK_THROUGH">
        放量突破
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.LOW_FUNDS_INFLOW">
        低位资金净流入
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.HIGH_FUNDS_OUTFLOW">
        高位资金净流出
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.BREAKUP_MA_5DAYS">
        向上突破 5 日均线
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.LONG_AVG_ARRAY">
        均线多头排列
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.SHORT_AVG_ARRAY">
        均线空头排列
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.UPPER_LARGE_VOLUME">
        连涨放量
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.DOWN_NARROW_VOLUME">
        下跌无量
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.ONE_DAYANG_LINE">
        一根大阳线
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.TWO_DAYANG_LINES">
        两根大阳线
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"     v-model:checked="technicalIndicatorReactive.RISE_SUN">
        旭日东升
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.POWER_FULGUN">
        强势多方炮
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.RESTORE_JUSTICE">
        拨云见日
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"     v-model:checked="technicalIndicatorReactive.DOWN_7DAYS">
        七仙女下凡(七连阴)
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.UPPER_8DAYS">
        八仙过海(八连阳)
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.UPPER_9DAYS">
        九阳神功(九连阳)
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.UPPER_4DAYS">
        四连阳
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.HEAVEN_RULE">
        天量法则
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.UPSIDE_VOLUME">
        放量上攻
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.BEARISH_ENGULFING">
        空头破脚
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.REVERSING_HAMMER">
        倒转锤头
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.SHOOTING_STAR">
        射击之星
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.EVENING_STAR">
        黄昏之星
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.FIRST_DAWN">
        曙光初现
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.PREGNANT">
        身怀六甲
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.BLACK_CLOUD_TOPS">
        乌云盖顶
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.MORNING_STAR">
        早晨之星
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.NARROW_FINISH">
        窄幅整理
      </n-checkbox>
        </n-space>
      </n-card>
      <n-card size="small" class="filter-card filter-card--single-line">
        <div class="single-line-filter-row">
          <n-radio-group size="small"  @update:checked="handleCheckedChange" name="UPP_DAYS"   v-model:value="technicalIndicatorReactive.UPP_DAYS">
            <n-radio :value="3">人气排名连涨：3 天及以上</n-radio>
            <n-radio :value="5">人气排名连涨：5 天及以上</n-radio>
            <n-radio :value="7">人气排名连涨：7 天及以上</n-radio>
          </n-radio-group>
          <n-divider vertical/>
          <n-radio-group  size="small" @update:checked="handleCheckedChange"  name="CONCERN_RANK_7DAYS"  v-model:value="technicalIndicatorReactive.CONCERN_RANK_7DAYS">
            <n-radio :value="10">7 日关注排名前 10 名</n-radio>
            <n-radio :value="50">7 日关注排名前 50 名</n-radio>
            <n-radio :value="100">7 日关注排名前 100 名</n-radio>
          </n-radio-group>
          <n-divider vertical/>
          <n-radio-group  size="small" @update:checked="handleCheckedChange" name="UPNDAY"  v-model:value="technicalIndicatorReactive.UPNDAY">
            <n-radio :value="3">连涨天数：3 天及以上</n-radio>
            <n-radio :value="5">连涨天数：5 天及以上</n-radio>
            <n-radio :value="8">连涨天数：8 天及以上</n-radio>
          </n-radio-group>
          <n-divider vertical/>
          <n-radio-group  size="small" @update:checked="handleCheckedChange" name="DOWNNDAY"  v-model:value="technicalIndicatorReactive.DOWNNDAY">
            <n-radio :value="3">连跌天数：3 天及以上</n-radio>
            <n-radio :value="5">连跌天数：5 天及以上</n-radio>
            <n-radio :value="8">连跌天数：8 天及以上</n-radio>
            <n-radio :value="10">连跌天数：10 天及以上</n-radio>
            <n-radio :value="14">连跌天数：14 天及以上</n-radio>
          </n-radio-group>
        </div>
      </n-card>
        </n-collapse-item>
      </n-collapse>

    <div class="stock-toolbar stock-toolbar--snapshot">
      <n-text depth="3" class="toolbar-section-label">信号快照</n-text>
      <n-tag size="small" type="success" :bordered="false">盘后快照</n-tag>
      <n-select
        v-model:value="selectedScreenStrategyId"
        :options="screenStrategyOptions"
        placeholder="选股策略"
        style="width: 160px"
        @update:value="onScreenStrategyChange"
      />
      <n-select
        v-model:value="selectedSnapshotHistoryValue"
        placeholder="历史快照"
        :options="selectableSnapshotHistoryOptions"
        style="width: 280px"
        clearable
        @update:value="onSnapshotHistoryChange"
      />
      <n-button tertiary type="warning" :loading="backendScanLoading || (signalScanLoading && !!scanTaskView)" @click="runBackendSnapshotScan">
        生成快照
      </n-button>
      <n-tag size="small" :type="scanTaskStatusType" :bordered="false">
        生成状态：{{ scanTaskStatusLabel }}
      </n-tag>
      <n-text v-if="scanTaskSummaryText" depth="3" class="snapshot-hint">{{ scanTaskSummaryText }}</n-text>
      <n-button
        v-if="snapshotMeta && signalDataSource === 'snapshot'"
        tertiary
        type="warning"
        :loading="starBacktestLoading"
        @click="runStarBacktest"
      >
        星标回测
      </n-button>
      <n-button v-if="signalDataSource === 'snapshot' && snapshotMeta" quaternary @click="loadLiveSignalScan">退出快照</n-button>
      <div
        v-if="snapshotMetaDisplay && signalDataSource === 'snapshot'"
        class="snapshot-meta-banner"
      >
        <n-text type="success" strong class="snapshot-meta-line">
          {{ snapshotMetaDisplay.marketLine }}
        </n-text>
        <n-text depth="3" class="snapshot-meta-line">
          {{ snapshotMetaDisplay.generatedLine }} · {{ snapshotMetaDisplay.statsLine }}
        </n-text>
        <n-text
          v-if="snapshotMetaDisplay.nonTradingTip"
          depth="3"
          class="snapshot-meta-line snapshot-meta-tip"
        >
          {{ snapshotMetaDisplay.nonTradingTip }}
        </n-text>
      </div>
      <n-text v-if="snapshotMeta && signalDataSource === 'snapshot'" depth="3" class="snapshot-hint">
        信号触发价=出信号 K 线收盘；快照涨幅含扫描日；实时涨跌与最新行情价独立刷新
      </n-text>
      <n-text v-else-if="showOpportunityColumns" depth="3" class="snapshot-hint">
        机会视图：信号触发价与最新行情价分离展示；实时涨跌相对今昨收
      </n-text>
      <n-text v-else depth="3" class="snapshot-hint">点击生成盘后快照（后台执行）；按信号筛选须先有快照</n-text>
    </div>

    <div class="stock-toolbar">
      <n-select
        v-model:value="filterMarketSegment"
        :options="marketSegmentOptions"
        placeholder="市场分类"
        style="width: 120px"
        @update:value="handleMarketSegmentChange"
      />
      <n-select
        v-model:value="filterIndustry"
        :options="industryOptions"
        clearable
        filterable
        placeholder="所属行业"
        style="width: 240px"
        @update:value="handleIndustryChange"
      />
      <n-select
        v-model:value="filterSignalTags"
        :options="signalFilterOptions"
        multiple
        clearable
        :max-tag-count="2"
        placeholder="信号筛选（可多选）"
        style="width: 200px"
        @update:value="handleSignalFilterChange"
      />
      <n-text v-if="hasReboundFilter" depth="3" class="rebound-filter-hint">
        弹筛选：已排除 ST、RSI&gt;{{ reboundScreenMaxRsi }}
      </n-text>
      <n-auto-complete
        v-model:value="paginationReactive.keyword"
        class="toolbar-search"
        :input-props="{ autocomplete: 'disabled' }"
        :options="optionsReactive"
        placeholder="输入搜索关键词"
        clearable
        @input="handleUpdateVal"
        @select="(value) => { paginationReactive.keyword = value; handleSearch() }"
      />
      <n-button type="primary" ghost :loading="loadingRef" @click="handleSearch">搜索</n-button>
      <n-button tertiary type="info" :loading="loadingRef" @click="refreshStocks()">刷新</n-button>
      <n-button @click="handleReset">重置</n-button>
    </div>
    </div>
    <div v-if="signalScanLoading" class="signal-scan-progress">
      <n-progress
        type="line"
        :percentage="scanProgressPercent"
        :indicator-placement="'inside'"
        processing
      />
      <n-text depth="3" class="signal-scan-progress__text">{{ signalScanStatus }}</n-text>
    </div>
    <div class="stock-screen-table">
    <n-text
      v-if="opportunityListCountLabel"
      depth="3"
      class="opportunity-list-count"
    >
      {{ opportunityListCountLabel }}
    </n-text>
    <n-data-table
      :remote="!usesOpportunityRowSource"
      size="small"
      :columns="tableColumns"
      :data="displayData"
      :loading="tableLoading"
      :pagination="tablePagination"
      :row-key="(rowData) => rowData.SECUCODE"
      flex-height
      :scroll-x="showOpportunityColumns ? 1480 : 2100"
      style="height: 100%"
      @update:page="handlePageChange"
      @update:page-size="handlePageSizeChange"
    />
    <n-text v-if="showOpportunityColumns" depth="3" class="opportunity-price-footer">
      {{ OPPORTUNITY_PRICE_FOOTER }}
    </n-text>
    <n-text v-if="showOpportunityColumns" depth="3" class="opportunity-price-footer">
      {{ OPPORTUNITY_DECISION_FOOTER }}
    </n-text>
    <n-alert
      v-if="showOpportunityColumns && decisionProjectionError"
      type="warning"
      :bordered="false"
      closable
      class="opportunity-decision-alert"
      @close="decisionProjectionError = false"
    >
      决策态批量加载失败。
      <n-button text type="primary" size="small" @click="loadDecisionProjectionsForTable">刷新决策态</n-button>
    </n-alert>
    </div>

  <OpportunityProjectionDrawer
    v-model:show="projectionDrawerVisible"
    :row="projectionDrawerRow"
  />

  <stock-kline-modal
    v-model:show="modalDataRef.visible"
    :title="modalDataRef.title"
    :chart-key="'screen-kline-' + (modalDataRef.chartCode || modalDataRef.stockCode) + '-' + modalDataRef.focusSignal"
    :code="modalDataRef.chartCode || modalDataRef.stockCode"
    :stock-name="modalDataRef.stockName"
    :dark-theme="editorDataRef.darkTheme"
    :strategy-signals="true"
    :signal-strategy-id="selectedScreenStrategyId"
    :focus-signal-tag="modalDataRef.focusSignal"
  >
    <template v-if="modalDataRef.narrativeCode" #append>
      <n-divider style="margin: 16px 0 12px" />
      <InvestmentNarrativePanel :stock-code="modalDataRef.narrativeCode" embedded />
    </template>
  </stock-kline-modal>
  <n-modal v-model:show="starBacktestVisible" preset="card" title="星标回测" style="width: 760px" :bordered="false">
    <n-spin :show="starBacktestLoading">
      <n-alert v-if="starBacktestError" type="error" :bordered="false" style="margin-bottom: 12px">
        {{ starBacktestError }}
      </n-alert>
      <n-alert v-if="starBacktestNoDataHint" type="info" :bordered="false" style="margin-bottom: 12px">
        {{ starBacktestNoDataHint }}
      </n-alert>
      <n-text depth="3" class="star-backtest-note">
        按当前快照加星的 3 只股票，统计快照日收盘后未来 1/3/5 个交易日表现。
      </n-text>
      <n-table size="small" :bordered="false" single-line class="star-backtest-table">
        <thead>
          <tr>
            <th>周期</th>
            <th>样本</th>
            <th>上涨率</th>
            <th>平均收益</th>
            <th>最大回撤</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in starBacktestSummary" :key="item.days">
            <td>{{ item.days }}日</td>
            <td>{{ item.sample }}</td>
            <td>{{ fmtBacktestMetric(item.winRate) }}</td>
            <td :class="{ 'bt-up': item.avgReturn > 0, 'bt-down': item.avgReturn < 0 }">
              {{ pctText(item.avgReturn) }}
            </td>
            <td class="bt-down">{{ pctText(item.maxDrawdown) }}</td>
          </tr>
          <tr v-if="!starBacktestSummary.length && !starBacktestLoading">
            <td colspan="5">暂无回测结果</td>
          </tr>
        </tbody>
      </n-table>
      <n-table size="small" :bordered="false" single-line class="star-backtest-table">
        <thead>
          <tr>
            <th>股票</th>
            <th>信号</th>
            <th>基准日</th>
            <th>1日</th>
            <th>3日</th>
            <th>5日</th>
            <th>状态</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in starBacktestRows" :key="row.code">
            <td>{{ row.name }} {{ row.code }}</td>
            <td>{{ row.tag }}</td>
            <td>{{ row.baseDate || row.tradeDate }}</td>
            <td :class="{ 'bt-up': row.ret1 > 0, 'bt-down': row.ret1 < 0 }">{{ pctText(row.ret1) }}</td>
            <td :class="{ 'bt-up': row.ret3 > 0, 'bt-down': row.ret3 < 0 }">{{ pctText(row.ret3) }}</td>
            <td :class="{ 'bt-up': row.ret5 > 0, 'bt-down': row.ret5 < 0 }">{{ pctText(row.ret5) }}</td>
            <td>{{ row.status }}</td>
          </tr>
          <tr v-if="!starBacktestRows.length && !starBacktestLoading">
            <td colspan="7">暂无明细</td>
          </tr>
        </tbody>
      </n-table>
    </n-spin>
  </n-modal>
  </div>
</template>

<style scoped>
.stock-screen-page {
  width: 100%;
  flex: 1;
  height: 100%;
  min-height: 0;
  box-sizing: border-box;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.stock-screen-top {
  flex-shrink: 0;
}
.stock-filter-collapse :deep(.n-collapse-item__content-wrapper) {
  max-height: min(36vh, 320px);
  overflow-y: auto;
}
.stock-filter-collapse :deep(.n-collapse-item__header) {
  font-weight: 500;
}
.stock-filter-collapse :deep(.n-collapse-item__content-inner) {
  padding-top: 4px;
}
.filter-card {
  margin-bottom: 8px;
}
.filter-card:last-child {
  margin-bottom: 0;
}
.filter-card--single-line {
  width: 100%;
  text-align: left;
}
.single-line-filter-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.single-line-filter-row :deep(.n-radio-group) {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  flex: 0 1 auto;
  gap: 8px;
}
.single-line-filter-row :deep(.n-radio) {
  flex: 0 0 auto;
  white-space: nowrap;
}
.stock-screen-table {
  flex: 1;
  min-height: 320px;
  overflow: hidden;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
}
.opportunity-list-count {
  display: block;
  flex-shrink: 0;
  margin: 0 0 4px 2px;
  font-size: 12px;
  text-align: left;
}
.stock-screen-table :deep(.n-data-table) {
  flex: 1;
  min-height: 0;
  height: 100% !important;
}
.stock-screen-table :deep(.n-data-table-wrapper) {
  flex: 1;
  min-height: 0;
  height: auto;
  display: flex;
  flex-direction: column;
}
.stock-screen-table :deep(.n-data-table-base-table) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.stock-screen-table :deep(.n-data-table-base-table-body) {
  flex: 1;
  min-height: 0;
}
.stock-screen-table :deep(.n-data-table__pagination) {
  flex-shrink: 0;
  padding: 6px 0 0;
  margin-top: auto;
}
.stock-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  flex-wrap: wrap;
  margin-top: 8px;
}
.stock-toolbar--snapshot {
  margin-top: 10px;
  padding: 8px 10px;
  border-radius: 6px;
  background: rgba(34, 197, 94, 0.06);
  border: 1px solid rgba(34, 197, 94, 0.22);
}
.snapshot-meta-banner {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 4px 10px;
  border-radius: 6px;
  background: rgba(34, 197, 94, 0.08);
  border: 1px solid rgba(34, 197, 94, 0.28);
  max-width: min(420px, 100%);
}
.snapshot-meta-line {
  font-size: 12px;
  line-height: 1.45;
  white-space: nowrap;
}
.toolbar-section-label {
  font-size: 12px;
  white-space: nowrap;
  margin-right: 2px;
}
.snapshot-hint {
  font-size: 12px;
}
.opportunity-price-footer {
  display: block;
  margin-top: 8px;
  padding: 0 2px 4px;
  font-size: 12px;
  line-height: 1.5;
}
.signal-scan-progress {
  flex-shrink: 0;
  padding: 4px 0 0;
}
.signal-scan-progress__text {
  display: block;
  margin-top: 4px;
  font-size: 12px;
}
.toolbar-search {
  flex: 0 0 auto;
  width: 320px;
  min-width: 320px;
  max-width: 420px;
}
.rebound-filter-hint {
  font-size: 11px;
  white-space: nowrap;
}
.star-backtest-note {
  display: block;
  margin-bottom: 10px;
  font-size: 12px;
}
.star-backtest-table {
  margin-top: 10px;
}
.star-backtest-table th,
.star-backtest-table td {
  text-align: left;
  white-space: nowrap;
}
.bt-up {
  color: #ef5350;
}
.bt-down {
  color: #26a69a;
}
.opp-pct-cell {
  display: flex;
  flex-direction: column;
  line-height: 1.25;
  gap: 2px;
}
.opp-pct-cell__date {
  font-size: 11px;
}
</style>



