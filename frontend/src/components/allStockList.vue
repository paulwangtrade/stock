<script setup>
import {h, onBeforeMount, onMounted, onBeforeUnmount, ref, reactive, computed} from 'vue'
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
} from "../../wailsjs/go/main/App";
import { followWithDateGroup, formatFollowGroupMessage } from "../utils/followDateGroup"
import {NButton, NInput, NTag, NText, NTooltip, NProgress, useMessage, useNotification, NDataTable, NSpace, NPagination, NFlex, NSelect, NIcon, NModal, NCard, NTable, NSpin, NAlert} from "naive-ui";
import StockKlineModal from "./StockKlineModal.vue"
import { resolveStrategyRowCode, resolveStrategyRowName, toEastMoneyCode, toFollowCodeFromRow } from "../utils/stockCode"
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
    if (!hasSignalFilter.value) {
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
    const st = String(task?.status || '').toLowerCase()
    if (st === 'running' || st === 'pending') {
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
const dataRefreshKey = ref(0)
const signalByCode = ref(new Map())
const signalScanLoading = ref(false)
const signalScanStatus = ref('')
const signalScanProgress = ref({ phase: '', done: 0, total: 0 })
/** Phase6.7-G: async snapshot task view */
const scanTaskView = ref(null)
let scanTaskPollTimer = null
const SIGNAL_SCAN_CONCURRENCY = 24
const SIGNAL_PAGE_SCAN_CONCURRENCY = 12
const signalFilteredRows = ref([])
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

const snapshotMetaLabel = computed(() => {
  if (!snapshotMeta.value) return ''
  const s = snapshotMeta.value.session === 'midday' ? '午盘' : '盘后'
  return `${snapshotMeta.value.tradeDate} ${s} · 扫 ${snapshotMeta.value.scannedTotal} 只 · 命中 ${snapshotMeta.value.hitTotal} 只`
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
  return {
    ok: true,
    tag: normalizeScreenSignalTag(hit.tag),
    recentSignalDaysAgo: hit.recentSignalDaysAgo ?? hit.daysAgo ?? null,
    statusText: hit.statusText,
    sortRank: hit.sortRank ?? 0,
    latestStatus: { rsi: hit.rsi ?? null },
  }
}

function hitToRow(hit) {
  return {
    SECUCODE: hit.SECUCODE,
    SECURITY_CODE: hit.SECURITY_CODE,
    SECURITY_NAME_ABBR: hit.SECURITY_NAME_ABBR,
    NEW_PRICE: hit.NEW_PRICE,
    CHANGE_RATE: hit.CHANGE_RATE,
    HIGH_PRICE: hit.HIGH_PRICE,
    LOW_PRICE: hit.LOW_PRICE,
    PRE_CLOSE_PRICE: hit.PRE_CLOSE_PRICE,
    VOLUME: hit.VOLUME,
    DEAL_AMOUNT: hit.DEAL_AMOUNT,
    TURNOVERRATE: hit.TURNOVERRATE,
    VOLUME_RATIO: hit.VOLUME_RATIO,
    INDUSTRY: hit.INDUSTRY,
    CONCEPT: hit.CONCEPT,
    MARKET: hit.MARKET,
  }
}

function applySnapshotPayload(payload, snap) {
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
    .map(hitToRow)
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
  signalDataSource.value = 'snapshot'
  dataRefreshKey.value++
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
      const label = `${s.tradeDate} ${s.session === 'midday' ? '午盘' : '盘后'}${s.strategyName ? ` · ${s.strategyName}` : ''} (${s.hitTotal}只)`
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

function startScanTaskPoll() {
  stopScanTaskPoll()
  scanTaskPollTimer = setInterval(async () => {
    const task = await refreshScanTaskView()
    const st = String(task?.status || '').toLowerCase()
    if (st === 'completed' || st === 'failed' || !st) {
      stopScanTaskPoll()
      if (st !== 'running' && st !== 'pending') {
        backendScanLoading.value = false
        if (st !== 'completed') {
          signalScanLoading.value = false
          signalScanStatus.value = ''
        }
      }
    }
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
      if (rows.length && !hasSignalFilter.value) {
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
const isValidVip=ref(false) // 鏄惁鏄細鍛?
const columnsRef = ref([
  // {
  //   title: '鏁版嵁鏃堕棿',
  //   key: 'MAX_TRADE_DATE',
  //   width: 120,
  // },
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
    width: 100,
    fixed: 'left',
    render(row) {
      const topPick = isSnapshotTopPick(row)
      return h(
        NButton,
        {
          text: true,
          type: 'success',
          style: 'font-weight: 600',
          onClick: () => showKline(row),
        },
        { default: () => `${topPick ? '★ ' : ''}${row.SECURITY_NAME_ABBR}` },
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
  {
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
  },
])

const paginationReactive = reactive({
  keyword:"",
  page: 1,
  pageCount: 1,
  pageSize: 20,
  pageSizes: [10, 20, 30, 50, 100, 200, 300, 500],
  showSizePicker: true,
  itemCount: 0,
  prefix({ itemCount }) {
    return `${itemCount} 只股票`
  },
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
  signalScanLoading.value = true
  try {
    const mapObj = await scanRowsLastBarSignals(rows, {
      concurrency: SIGNAL_PAGE_SCAN_CONCURRENCY,
      signalSettings: selectedScreenStrategyParams.value,
    })
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
  if (hasSignalFilter.value) {
    const start = (paginationReactive.page - 1) * paginationReactive.pageSize
    const sorted = sortRowsByTrendScore(signalFilteredRows.value)
    return sorted.slice(start, start + paginationReactive.pageSize)
  }
  return sortRowsByTrendScore(dataRef.value || [])
})

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
    signalFilteredRows.value = []
    signalByCode.value = new Map()
    const res = await GetAllStocks(page, pageSize, paginationReactive.keyword, filterIndustry.value || '', '', '', technicalIndicatorReactive)
    if (res?.result) {
      const rows = filterRowsByMarketSegment(Array.isArray(res.result.data) ? res.result.data : [])
      dataRef.value = rows
      dataRefreshKey.value++
      paginationReactive.page = page
      paginationReactive.pageCount = Math.max(1, Math.ceil((res.result.count || rows.length) / pageSize))
      paginationReactive.itemCount = res.result.count ?? rows.length
      if (rows.length) {
        // 鍒楄〃鍏堝睍绀猴紝淇″彿/K 绾垮湪鍚庡彴鎵紙閬垮厤閫夎涓氬悗鏁撮〉鍗′綇锛?
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
  if (hasSignalFilter.value) {
    paginationReactive.page = currentPage
    return
  }
  loadStocks(currentPage, paginationReactive.pageSize)
}

function handlePageSizeChange(pageSize) {
  paginationReactive.pageSize = pageSize
  if (hasSignalFilter.value) {
    paginationReactive.page = 1
    paginationReactive.pageCount = Math.max(1, Math.ceil(signalFilteredRows.value.length / pageSize))
    return
  }
  loadStocks(1, pageSize)
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
const modalDataRef = reactive({
  visible: false,
  title: "",
  content: "",
  riskRemarks: "",
  stockCode: "",
  stockName: "",
  remarks: "",
  focusSignal: "",
})
function showKline(row, focusTag = '') {
  const code = resolveStrategyRowCode(row)
  const name = resolveStrategyRowName(row)
  if (!code) {
    message.warning('无法识别股票代码，请检查该行是否包含 SECUCODE')
    return
  }
  const em = toEastMoneyCode(code)
  modalDataRef.stockCode = em
  modalDataRef.stockName = name
  modalDataRef.focusSignal = focusTag || signalByCode.value.get(row.SECUCODE)?.tag || ''
  modalDataRef.title = `${name} ${em} - 日K`
  modalDataRef.visible = true
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
        style="width: 168px"
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
      <n-tag v-if="snapshotMeta && signalDataSource === 'snapshot'" type="success" size="small" :bordered="false">
        {{ snapshotMetaLabel }}
      </n-tag>
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
      <n-button tertiary type="info" :loading="loadingRef || signalScanLoading" @click="refreshStocks()">刷新</n-button>
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
    <n-data-table
      :remote="!hasSignalFilter"
      size="small"
      :columns="columnsRef"
      :data="displayData"
      :loading="loadingRef || signalScanLoading"
      :pagination="paginationReactive"
      :row-key="(rowData) => rowData.SECUCODE"
      flex-height
      :scroll-x="2100"
      style="height: 100%"
      @update:page="handlePageChange"
      @update:page-size="handlePageSizeChange"
    />
    </div>

  <stock-kline-modal
    v-model:show="modalDataRef.visible"
    :title="modalDataRef.title"
    :chart-key="'screen-kline-' + modalDataRef.stockCode + '-' + modalDataRef.focusSignal"
    :code="modalDataRef.stockCode"
    :stock-name="modalDataRef.stockName"
    :dark-theme="editorDataRef.darkTheme"
    :strategy-signals="true"
    :signal-strategy-id="selectedScreenStrategyId"
    :focus-signal-tag="modalDataRef.focusSignal"
  />
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
.toolbar-section-label {
  font-size: 12px;
  white-space: nowrap;
  margin-right: 2px;
}
.snapshot-hint {
  font-size: 12px;
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
</style>



