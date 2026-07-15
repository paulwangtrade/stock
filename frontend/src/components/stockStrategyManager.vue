<script setup>
import { h, onBeforeMount, onBeforeUnmount, reactive, ref, computed } from 'vue'
import {
  CreateStockStrategy,
  DeleteStockStrategy,
  Follow,
  GetConfig,
  GetAllIndustries,
  GetStockEastMoneyKLine,
  GetStockStrategyList,
  GetStockStrategyRunDetail,
  GetStockStrategyRunList,
  GetStockStrategySummary,
  RunStockStrategy,
  UpdateStockStrategy,
} from '../../wailsjs/go/main/App'
import { EventsEmit, EventsOff, EventsOn } from '../../wailsjs/runtime'
import { NButton, NCheckbox, NFlex, NSelect, NTag, NText, NTooltip, NDataTable, useDialog, useMessage } from 'naive-ui'
import { AddOutline } from '@vicons/ionicons5'
import TechnicalIndicatorFields from './TechnicalIndicatorFields.vue'
import StockKlineModal from './StockKlineModal.vue'
import { resolveStrategyRowCode, resolveStrategyRowName, toEastMoneyCode, toFollowCodeFromRow, eastMoneyCodeVariants } from '../utils/stockCode'
import { followWithDateGroup, formatFollowGroupMessage } from '../utils/followDateGroup'
import { formatPercent2, formatPercent2Signed } from '../utils/formatNumber'
import { summarizeBuySignal, buildIndexMa20ByDay, normalizeDayKey, parseRunDayKey, truncateBarsToDay, truncateIndexMa20ByDay } from '../utils/icePointSignals'
import { getSignalOptions } from '../utils/signalSettingsStore'
import { resolveEffectiveSignalDayKey, resolveSignalLastBarIndex } from '../utils/tradingSession'
import {
  applyTechnicalIndicators,
  defaultTechnicalIndicators,
  hasActiveTechnicalIndicator,
  resetTechnicalIndicators,
  STRATEGY_PRESETS,
} from '../utils/technicalIndicators'
import { getSignalTagColor, formatSignalTagLabel, buildSignalFilterOptions, passesSignalTagFilter } from '../utils/signalBuyGuide'
import {
  describeSignalPattern,
  extractSignalPattern,
  isSimilarSignalPattern,
  PEAK_PULLBACK_REDUCE_PATTERN,
} from '../utils/signalPatternMatch'

const message = useMessage()
const dialog = useDialog()
const loading = ref(false)
const strategyList = ref([])
const showEdit = ref(false)
const editingId = ref(0)
const showResult = ref(false)
const showHistory = ref(false)
const resultView = ref(null)
const historyRuns = ref([])
const historyStrategyId = ref(0)
const historyStrategyName = ref('')
const historyLoading = ref(false)
const historyPagination = reactive({
  page: 1,
  pageCount: 1,
  pageSize: 10,
  pageSizes: [10, 15, 20, 30],
  showSizePicker: true,
  itemCount: 0,
  prefix({ itemCount }) {
    return `共 ${itemCount} 次执行`
  },
})
const historyColumns = [
  {
    title: '序号',
    key: 'index',
    width: 52,
    render(_row, index) {
      return (historyPagination.page - 1) * historyPagination.pageSize + index + 1
    },
  },
  {
    title: '执行时间',
    key: 'createdAt',
    width: 168,
    render(row) {
      return formatRunTime(row.createdAt)
    },
  },
  {
    title: '股票数',
    key: 'stockCount',
    width: 80,
    align: 'center',
    render(row) {
      const n = row.stockCount ?? 0
      return h(NTag, { size: 'small', type: n > 0 ? 'success' : 'default', bordered: false }, { default: () => `${n} 只` })
    },
  },
  {
    title: '说明',
    key: 'message',
    minWidth: 120,
    ellipsis: { tooltip: true },
    render(row) {
      const msg = String(row.message || '').trim()
      if (!msg) return h(NText, { depth: 3 }, { default: () => '—' })
      return h(NText, { depth: 2 }, { default: () => msg })
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 72,
    fixed: 'right',
    render(row) {
      return h(
        NButton,
        { size: 'small', type: 'primary', tertiary: true, onClick: () => viewRun(row) },
        { default: () => '查看' },
      )
    },
  },
]
const resultColumns = ref([])
const resultData = ref([])
const showKline = ref(false)
const klineCode = ref('')
const klineName = ref('')
/** 从执行结果带入 K 线，定位与列表一致的信号柱 */
const klineFocusSignal = ref('')
const darkTheme = ref(false)
/** 行代码 → 冰点/买点扫描结果 */
const signalByCode = ref(new Map())
const scanBuyLoading = ref(false)
const showBuyOnly = ref(false)
const showSignalHitsOnly = ref(false)
const filterSignalTags = ref([])
const filterSimilarOnly = ref(false)
const referencePattern = ref(null)
const referencePatternLabel = ref('')
const signalFilterOptions = buildSignalFilterOptions()
const SIGNAL_COL_KEY = '_buySignal'
/** 历史回放截止日 YYYY-MM-DD；空=实时（未开盘按昨收 K 线） */
const resultReplayDayKey = ref('')
/** 实时扫描时 K 线/信号截止日（未开盘为昨收） */
const liveSignalAsOfDay = ref('')

const queryTypeOptions = [
  { label: '自然语言（指标选股）', value: 'eastmoney_nl' },
  { label: '技术面（股票筛选）', value: 'technical' },
]

const schedulePresets = [
  { label: '不启用定时', value: '' },
  { label: '每个交易日 9:35', value: '0 35 9 * * 1-5' },
  { label: '每个交易日 15:05', value: '0 5 15 * * 1-5' },
  { label: '每天 9:35', value: '0 35 9 * * *' },
]

const form = reactive({
  name: '',
  queryType: 'eastmoney_nl',
  queryText: '',
  keyword: '',
  industry: '',
  /** 自然语言策略：多选行业，保存为 industry JSON */
  nlIndustries: [],
  cronExpr: '',
  enable: false,
  pageSize: 50,
  description: '',
})

const technical = reactive(defaultTechnicalIndicators())
const technicalTouched = ref(false)

const industryOptions = ref([])

function parseNlIndustriesFromStorage(str) {
  const raw = String(str || '').trim()
  if (!raw) return []
  try {
    const arr = JSON.parse(raw)
    if (Array.isArray(arr)) return arr.map((s) => String(s).trim()).filter(Boolean)
  } catch (_) { /* legacy plain text */ }
  return [raw]
}

const nlMergedPreview = computed(() => {
  const parts = [...(form.nlIndustries || [])]
  const tail = String(form.queryText || '').trim()
  if (tail) parts.push(tail)
  return parts.filter(Boolean).join('；')
})

async function loadIndustries() {
  try {
    const list = await GetAllIndustries()
    industryOptions.value = (list || []).map((name) => ({ label: name, value: name }))
  } catch {
    industryOptions.value = []
  }
}

const pagination = reactive({
  page: 1,
  pageSize: 10,
  itemCount: 0,
  pageCount: 1,
})

const columns = [
  { title: '名称', key: 'name', minWidth: 120 },
  {
    title: '类型',
    key: 'queryType',
    width: 100,
    render(row) {
      return row.queryType === 'technical'
        ? h(NTag, { size: 'small', type: 'info' }, { default: () => '技术面' })
        : h(NTag, { size: 'small', type: 'warning' }, { default: () => '自然语言' })
    },
  },
  {
    title: '条件摘要',
    key: 'summary',
    minWidth: 200,
    ellipsis: { tooltip: true },
    render(row) {
      return h(NText, { depth: 2 }, { default: () => row._summary || '…' })
    },
  },
  {
    title: '定时',
    key: 'cronExpr',
    width: 100,
    render(row) {
      return row.enable && row.cronExpr
        ? h(NTag, { size: 'small', type: 'success' }, { default: () => '已启用' })
        : h(NText, { depth: 3 }, { default: () => '未启用' })
    },
  },
  {
    title: '上次运行',
    key: 'lastRunAt',
    width: 200,
    render(row) {
      const t = row.lastRunAt ? String(row.lastRunAt).substring(0, 19).replace('T', ' ') : '-'
      const c = row.lastRunCount != null ? ` / ${row.lastRunCount}只` : ''
      return h(NText, { style: 'white-space: nowrap' }, { default: () => t + c })
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 280,
    fixed: 'right',
    render(row) {
      return [
        h(NButton, { size: 'small', type: 'primary', tertiary: true, onClick: () => runNow(row) }, { default: () => '运行' }),
        h(NButton, { size: 'small', tertiary: true, onClick: () => openHistory(row) }, { default: () => '历史' }),
        h(NButton, { size: 'small', tertiary: true, onClick: () => openEdit(row) }, { default: () => '编辑' }),
        h(NButton, { size: 'small', type: 'error', tertiary: true, onClick: () => remove(row) }, { default: () => '删除' }),
      ]
    },
  },
]

function resetTechnical() {
  resetTechnicalIndicators(technical)
  technicalTouched.value = false
}

function resetForm() {
  editingId.value = 0
  form.name = ''
  form.queryType = 'eastmoney_nl'
  form.queryText = ''
  form.keyword = ''
  form.industry = ''
  form.nlIndustries = []
  form.cronExpr = ''
  form.enable = false
  form.pageSize = 50
  form.description = ''
  resetTechnical()
}

function openCreate() {
  resetForm()
  loadIndustries()
  showEdit.value = true
}

function applyStrategyPreset(presetId) {
  const p = STRATEGY_PRESETS.find((x) => x.id === presetId)
  if (!p) return
  resetForm()
  form.name = p.name
  form.queryType = p.queryType
  form.queryText = p.queryText || ''
  form.description = p.description || ''
  form.pageSize = p.pageSize || 50
  form.cronExpr = p.cronExpr || ''
  form.enable = !!p.enable && !!p.cronExpr
  if (p.applyTechnical) {
    p.applyTechnical(technical)
    technicalTouched.value = true
  }
  showEdit.value = true
}

const presetOptions = STRATEGY_PRESETS.map((p) => ({
  label: p.name,
  key: p.id,
}))

function openEdit(row) {
  editingId.value = row.id
  form.name = row.name
  form.queryType = row.queryType
  form.queryText = row.queryText || ''
  form.keyword = row.keyword || ''
  form.industry = row.industry || ''
  form.nlIndustries = row.queryType === 'eastmoney_nl' ? parseNlIndustriesFromStorage(row.industry) : []
  form.cronExpr = row.cronExpr || ''
  form.enable = row.enable
  form.pageSize = row.pageSize || 50
  form.description = row.description || ''
  resetTechnical()
  if (row.queryType === 'technical' && row.queryJson) {
    try {
      applyTechnicalIndicators(technical, JSON.parse(row.queryJson))
      technicalTouched.value = true
    } catch (e) {
      message.warning('技术面参数解析失败')
    }
  }
  loadIndustries()
  showEdit.value = true
}

function buildPayload() {
  const payload = {
    id: editingId.value,
    name: form.name.trim(),
    queryType: form.queryType,
    queryText: form.queryType === 'eastmoney_nl' ? form.queryText.trim() : '',
    queryJson: form.queryType === 'technical' ? JSON.stringify(technical) : '',
    keyword: form.queryType === 'technical' ? form.keyword.trim() : '',
    industry:
      form.queryType === 'technical'
        ? (form.industry || '').trim()
        : form.nlIndustries?.length
          ? JSON.stringify(form.nlIndustries)
          : '',
    cronExpr: form.cronExpr,
    enable: form.enable && !!form.cronExpr,
    pageSize: form.pageSize,
    description: form.description,
  }
  return payload
}

async function saveStrategy() {
  if (!form.name.trim()) {
    message.warning('请填写策略名称')
    return
  }
  if (form.queryType === 'eastmoney_nl' && !form.queryText.trim() && !(form.nlIndustries?.length)) {
    message.warning('请填写选股条件或选择板块/行业')
    return
  }
  if (form.queryType === 'technical' && !hasActiveTechnicalIndicator(technical)) {
    message.warning('请至少勾选一项技术条件')
    return
  }
  const s = buildPayload()
  const msg = editingId.value
    ? await UpdateStockStrategy(s)
    : await CreateStockStrategy(s)
  if (msg.includes('成功')) {
    message.success(msg)
    showEdit.value = false
    loadList()
  } else {
    message.error(msg)
  }
}

async function loadList() {
  loading.value = true
  try {
    const res = await GetStockStrategyList({
      page: pagination.page,
      pageSize: pagination.pageSize,
      name: '',
      queryType: '',
    })
    const rows = res?.data || []
    await Promise.all(
      rows.map(async (row) => {
        try {
          row._summary = await GetStockStrategySummary(row.id)
        } catch {
          row._summary = '—'
        }
      }),
    )
    strategyList.value = rows
    pagination.itemCount = res?.total || 0
    pagination.pageCount = Math.max(1, Math.ceil(pagination.itemCount / pagination.pageSize))
  } catch (e) {
    strategyList.value = []
    message.error(`加载策略列表失败：${e}`)
  } finally {
    loading.value = false
  }
}

async function runNow(row) {
  const loadingMsg = message.loading('正在执行策略…', { duration: 0 })
  try {
    const view = await RunStockStrategy(row.id)
    loadingMsg.destroy()
    if (!view || view.code !== 0) {
      message.error(view?.message || '执行失败')
      return
    }
    showRunResult(view)
    loadList()
  } catch (e) {
    loadingMsg.destroy()
    message.error(String(e))
  }
}

function buildNlColumns(cols) {
  if (!cols) return []
  const percentCol = (item) => {
    const k = String(item.key || '').toUpperCase()
    const t = String(item.title || '')
    return k === 'CHANGE_RATE' || k.includes('CHANGERATE') || t.includes('涨跌幅') || item.unit === '%'
  }
  const percentRender = (row, key) => {
    const rate = Number(row[key])
    if (!Number.isFinite(rate)) return h(NText, { depth: 3 }, { default: () => '—' })
    const type = rate >= 0 ? 'error' : 'success'
    return h(NText, { type }, { default: () => `${formatPercent2Signed(rate)}%` })
  }
  return cols
    .filter((item) => !item.hiddenNeed && item.title !== '市场码' && item.title !== '市场简称')
    .map((item) => {
      if (item.children) {
        return {
          title: item.title + (item.unit ? `[${item.unit}]` : ''),
          key: item.key,
          children: item.children.filter((c) => !c.hiddenNeed).map((c) => ({
            title: c.dateMsg,
            key: c.key,
            minWidth: 100,
            ...(percentCol(c) ? { render: (row) => percentRender(row, c.key) } : {}),
          })),
        }
      }
      const col = {
        title: item.title + (item.unit ? `[${item.unit}]` : ''),
        key: item.key,
        minWidth: 110,
      }
      if (percentCol(item)) {
        col.render = (row) => percentRender(row, item.key)
      }
      return col
    })
}

function rowKey(row) {
  return resolveStrategyRowCode(row) || ''
}

function daySortKey(dayStr) {
  const s = String(dayStr || '').trim().replace(/\//g, '-')
  const m = s.match(/^(\d{4})-(\d{2})-(\d{2})/)
  if (m) return Number(m[1] + m[2] + m[3])
  const c = s.match(/^(\d{4})(\d{2})(\d{2})/)
  if (c) return Number(c[1] + c[2] + c[3])
  return 0
}

let indexMa20ByDayCache = null
let indexMa20FetchPromise = null

async function ensureIndexMa20ByDay(asOfDayKey = '') {
  if (indexMa20ByDayCache && !asOfDayKey) return indexMa20ByDayCache
  if (indexMa20FetchPromise && !asOfDayKey) return indexMa20FetchPromise
  indexMa20FetchPromise = (async () => {
    try {
      const raw = await GetStockEastMoneyKLine('000001.SH', '上证指数', '101', 800)
      const list = Array.isArray(raw) ? raw : []
      const closeByDay = new Map()
      for (const r of list) {
        const k = normalizeDayKey(r.day)
        const c = Number(r.close)
        if (k && Number.isFinite(c)) closeByDay.set(k, c)
      }
      let map = buildIndexMa20ByDay(closeByDay, 20)
      if (asOfDayKey) {
        map = truncateIndexMa20ByDay(map, asOfDayKey)
      } else {
        indexMa20ByDayCache = map
      }
      return map
    } catch {
      return new Map()
    }
  })()
  const map = await indexMa20FetchPromise
  if (!asOfDayKey) {
    indexMa20ByDayCache = map
  }
  indexMa20FetchPromise = null
  return map
}

async function fetchDailyBars(row, asOfDayKey = '') {
  const name = resolveStrategyRowName(row)
  const barCount = asOfDayKey ? 800 : 120
  const variants = eastMoneyCodeVariants(rowKey(row))
  for (const code of variants) {
    try {
      const raw = await GetStockEastMoneyKLine(code, name, '101', barCount)
      const list = Array.isArray(raw) ? raw : []
      if (!list.length) continue
      const sorted = [...list].sort((a, b) => daySortKey(a.day) - daySortKey(b.day))
      const closes = []
      const opens = []
      const highs = []
      const lows = []
      const volumes = []
      const dayKeys = []
      for (const r of sorted) {
        const c = Number(r.close)
        const o = Number(r.open)
        const h = Number(r.high)
        const l = Number(r.low)
        const v = Number(r.volume)
        if (!Number.isFinite(c)) continue
        closes.push(c)
        opens.push(Number.isFinite(o) ? o : c)
        highs.push(Number.isFinite(h) ? h : c)
        lows.push(Number.isFinite(l) ? l : c)
        volumes.push(Number.isFinite(v) ? v : 0)
        dayKeys.push(normalizeDayKey(r.day))
      }
      if (closes.length < 20) continue
      let bars = { closes, opens, highs, lows, volumes, dayKeys }
      if (asOfDayKey) {
        bars = truncateBarsToDay(bars, asOfDayKey)
        if (!bars) continue
      }
      return bars
    } catch {
      /* try next code variant */
    }
  }
  return null
}

async function fetchLiveSignalAsOfDay() {
  try {
    const raw = await GetStockEastMoneyKLine('000001.SH', '上证指数', '101', 30)
    const list = Array.isArray(raw) ? raw : []
    if (!list.length) return ''
    const sorted = [...list].sort((a, b) => daySortKey(a.day) - daySortKey(b.day))
    const dayKeys = sorted.map((r) => normalizeDayKey(r.day)).filter(Boolean)
    return resolveEffectiveSignalDayKey(dayKeys) || ''
  } catch {
    return ''
  }
}

async function runPool(tasks, concurrency = 4) {
  const results = []
  let i = 0
  async function worker() {
    while (i < tasks.length) {
      const idx = i++
      results[idx] = await tasks[idx]()
    }
  }
  const workers = Array.from({ length: Math.min(concurrency, tasks.length) }, () => worker())
  await Promise.all(workers)
  return results
}

async function scanBuySignals() {
  const rows = resultData.value || []
  if (!rows.length) {
    message.info('暂无结果可扫描')
    return
  }
  scanBuyLoading.value = true
  signalByCode.value = new Map()
  const replayDay = resultReplayDayKey.value
  liveSignalAsOfDay.value = replayDay || ''
  try {
    if (!replayDay) {
      liveSignalAsOfDay.value = await fetchLiveSignalAsOfDay()
    }
    const indexMa20ByDay = await ensureIndexMa20ByDay(replayDay)
    const tasks = rows.map((row) => async () => {
      const key = rowKey(row)
      if (!key) {
        return [key, { ok: false }]
      }
      const bars = await fetchDailyBars(row, replayDay)
      if (!bars) {
        return [key, { ok: false }]
      }
      const summary = summarizeBuySignal(
        { ...bars, indexMa20ByDay },
        {
          ...getSignalOptions(),
          recentBuyDays: 0,
          recentSellDays: 0,
          includeSell: true,
          signalLastIndex: replayDay ? undefined : resolveSignalLastBarIndex(bars.dayKeys),
        },
      )
      const lastIdx =
        summary.signalLastIndex ??
        (replayDay ? bars.closes.length - 1 : resolveSignalLastBarIndex(bars.dayKeys))
      try {
        summary.signalPattern = extractSignalPattern(summary, lastIdx, bars.closes)
      } catch {
        summary.signalPattern = null
      }
      return [key, { ok: true, ...summary }]
    })
    const pairs = await runPool(tasks, 4)
    const map = new Map()
    for (const pair of pairs) {
      if (pair && pair[0]) map.set(pair[0], pair[1])
    }
    signalByCode.value = map
    const hitCount = rows.filter((r) => {
      const s = map.get(rowKey(r))
      return s?.ok && s.tag
    }).length
    const buyCount = rows.filter((r) => {
      const s = map.get(rowKey(r))
      return s?.ok && s.hasRecentBuy && s.recentSignalDaysAgo === 0
    }).length
    const reduceCount = rows.filter((r) => {
      const s = map.get(rowKey(r))
      return s?.ok && s.tag === '减' && s.recentSignalDaysAgo === 0
    }).length
    const replayHint = replayDay ? `（回放至 ${replayDay}）` : ''
    message.success(
      `扫描完成${replayHint}：${hitCount} 只有信号，${buyCount} 只买点，${reduceCount} 只今日减`,
    )
    resultColumns.value = injectSignalColumn(resultColumns.value)
  } finally {
    scanBuyLoading.value = false
  }
}

function findNameColumnIndex(cols) {
  const nameKeys = [
    'name',
    'SECURITY_SHORT_NAME',
    'SECURITY_NAME_ABBR',
    'SECURITY_NAME',
    'STOCK_NAME',
  ]
  let idx = cols.findIndex((c) => nameKeys.includes(c.key))
  if (idx >= 0) return idx
  return cols.findIndex((c) => {
    const t = String(c.title || '')
    return t.startsWith('名称') || t.includes('股票名称') || t.includes('证券名称')
  })
}

function injectSignalColumn(cols) {
  const list = [...(cols || [])].filter((c) => c.key !== SIGNAL_COL_KEY)
  const signalCol = {
    title: resultReplayDayKey.value ? '买点(回放)' : '买点信号',
    key: SIGNAL_COL_KEY,
    width: 96,
    render(row) {
      const s = signalByCode.value.get(rowKey(row))
      if (!s) {
        return h(NText, { depth: 3 }, { default: () => (scanBuyLoading.value ? '…' : '—') })
      }
      if (!s.ok) {
        return h(NText, { depth: 3 }, { default: () => '无K线' })
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
            onClick: () => openStrategyKline(row, s.tag),
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
    },
  }
  const nameIdx = findNameColumnIndex(list)
  if (nameIdx >= 0) {
    list.splice(nameIdx + 1, 0, signalCol)
  } else {
    const idx = list.findIndex((c) => c.key === 'actions' || c.key === 'op')
    if (idx >= 0) {
      list.splice(idx, 0, signalCol)
    } else {
      list.push(signalCol)
    }
  }
  return list
}

function rowHasSignal(row) {
  const s = signalByCode.value.get(rowKey(row))
  return s?.ok && !!s.tag
}

function rowIsBuyCandidate(row) {
  const s = signalByCode.value.get(rowKey(row))
  return s?.ok && s.hasRecentBuy && s.recentSignalDaysAgo === 0
}

function setReferenceFromRow(row) {
  const s = signalByCode.value.get(rowKey(row))
  if (!s?.signalPattern?.tag) {
    message.warning('请先扫描信号，且该行须有有效标签')
    return
  }
  referencePattern.value = { ...s.signalPattern }
  referencePatternLabel.value = resolveStrategyRowName(row) || rowKey(row)
  filterSimilarOnly.value = true
  message.info(`参考形态：${describeSignalPattern(referencePattern.value)}（${referencePatternLabel.value}）`)
}

function applyPeakPullbackPreset() {
  referencePattern.value = { ...PEAK_PULLBACK_REDUCE_PATTERN }
  referencePatternLabel.value = '主升后破MA20·今日减'
  filterSimilarOnly.value = true
  showSignalHitsOnly.value = false
  showBuyOnly.value = false
  message.info('已启用「主升后破MA20·今日减」形态（类似九联科技）')
}

function clearReferencePattern() {
  referencePattern.value = null
  referencePatternLabel.value = ''
  filterSimilarOnly.value = false
  filterSignalTags.value = []
}

function applySimilarFromKline() {
  const code = klineCode.value
  if (!code) return
  const row = (resultData.value || []).find((r) => {
    const c = resolveStrategyRowCode(r)
    return c && toEastMoneyCode(c) === toEastMoneyCode(code)
  })
  if (row) {
    setReferenceFromRow(row)
    return
  }
  message.warning('当前 K 线股票不在本次策略结果中')
}

const similarMatchCount = computed(() => {
  if (!referencePattern.value || !filterSimilarOnly.value) return 0
  return (resultData.value || []).filter((r) => {
    const s = signalByCode.value.get(rowKey(r))
    return s?.signalPattern && isSimilarSignalPattern(referencePattern.value, s.signalPattern)
  }).length
})

const displayResultData = computed(() => {
  let rows = [...(resultData.value || [])]
  if (signalByCode.value.size > 0) {
    if (showBuyOnly.value) {
      rows = rows.filter((r) => rowIsBuyCandidate(r))
    } else if (filterSimilarOnly.value && referencePattern.value) {
      rows = rows.filter((r) => {
        const s = signalByCode.value.get(rowKey(r))
        return s?.signalPattern && isSimilarSignalPattern(referencePattern.value, s.signalPattern)
      })
    } else if (filterSignalTags.value.length) {
      rows = rows.filter((r) =>
        passesSignalTagFilter(r, signalByCode.value.get(rowKey(r)), filterSignalTags.value),
      )
    } else if (showSignalHitsOnly.value) {
      rows = rows.filter((r) => rowHasSignal(r))
    }
    if (signalByCode.value.size > 0) {
      rows.sort((a, b) => {
        const sa = signalByCode.value.get(rowKey(a))
        const sb = signalByCode.value.get(rowKey(b))
        const ra = sa?.ok && sa.tag ? sa.sortRank : 9
        const rb = sb?.ok && sb.tag ? sb.sortRank : 9
        if (ra !== rb) return ra - rb
        return 0
      })
    }
  }
  return rows
})

const signalHitCount = computed(() => {
  return (resultData.value || []).filter((r) => rowHasSignal(r)).length
})

const buyCandidateCount = computed(() => {
  return (resultData.value || []).filter((r) => rowIsBuyCandidate(r)).length
})

const resultModalTitle = computed(() => {
  const n = resultView.value?.stockCount ?? 0
  const scanned = signalByCode.value.size > 0
  const shown = displayResultData.value.length
  if (resultReplayDayKey.value) {
    return scanned
      ? `执行结果（回放 ${resultReplayDayKey.value}）· 显示 ${shown} / ${n} 只`
      : `执行结果（回放 ${resultReplayDayKey.value}）· ${n} 只`
  }
  return scanned ? `执行结果 · 显示 ${shown} / ${n} 只` : `执行结果 · ${n} 只`
})

const klineModalTitle = computed(() => {
  const base = `${klineName.value} ${klineCode.value} — 策略K线`
  if (resultReplayDayKey.value) {
    return `${base} · 回放 ${resultReplayDayKey.value}`
  }
  return base
})

async function followAllBuyCandidates() {
  const rows = (resultData.value || []).filter((r) => rowIsBuyCandidate(r))
  if (!rows.length) {
    message.warning('请先扫描，且存在买点股票')
    return
  }
  let ok = 0
  for (const row of rows) {
    const code = toFollowCodeFromRow(row)
    if (!code) continue
    const { followResult } = await followWithDateGroup(code, Follow)
    if (followResult === '关注成功') ok++
  }
  message.success(`已关注 ${ok} 只买点候选股`)
}

function renderRowActions(row) {
  return h(NFlex, { size: 4 }, {
    default: () => [
      h(
        NButton,
        { size: 'small', type: 'primary', tertiary: true, onClick: () => openStrategyKline(row) },
        { default: () => 'K线' },
      ),
      h(
        NButton,
        {
          size: 'small',
          type: 'info',
          tertiary: true,
          disabled: !signalByCode.value.get(rowKey(row))?.signalPattern?.tag,
          onClick: () => setReferenceFromRow(row),
        },
        { default: () => '相似' },
      ),
      h(
        NButton,
        { size: 'small', type: 'warning', tertiary: true, onClick: () => followRow(row) },
        { default: () => '关注' },
      ),
    ],
  })
}

function openStrategyKline(row, focusTag = '') {
  const code = resolveStrategyRowCode(row)
  const name = resolveStrategyRowName(row)
  if (!code) {
    message.warning('无法识别股票代码，请检查该行是否包含 SECURITY_CODE / SECUCODE')
    return
  }
  const scanned = signalByCode.value.get(rowKey(row))
  klineFocusSignal.value = focusTag || scanned?.tag || ''
  klineCode.value = toEastMoneyCode(code)
  klineName.value = name
  showKline.value = true
}

function buildTechnicalColumns() {
  return [
    { title: '代码', key: 'SECURITY_CODE', width: 90 },
    { title: '名称', key: 'SECURITY_NAME_ABBR', width: 100 },
    { title: '最新价', key: 'NEW_PRICE', width: 80 },
    { title: '涨跌幅%', key: 'CHANGE_RATE', width: 90, render(row) {
      const rate = Number(row.CHANGE_RATE)
      if (!Number.isFinite(rate)) return h(NText, { depth: 3 }, { default: () => '—' })
      const type = rate >= 0 ? 'error' : 'success'
      return h(NText, { type }, { default: () => `${formatPercent2Signed(rate)}%` })
    } },
    { title: '行业', key: 'INDUSTRY', minWidth: 100, ellipsis: { tooltip: true } },
    {
      title: '操作',
      key: 'op',
      width: 170,
      fixed: 'right',
      render(row) {
        return renderRowActions(row)
      },
    },
  ]
}

function showRunResult(view, replayDayKey = '') {
  resultView.value = view
  resultReplayDayKey.value = replayDayKey || parseRunDayKey(view?.runAt) || ''
  signalByCode.value = new Map()
  showBuyOnly.value = false
  showSignalHitsOnly.value = false
  filterSignalTags.value = []
  filterSimilarOnly.value = false
  referencePattern.value = null
  referencePatternLabel.value = ''
  indexMa20ByDayCache = null
  indexMa20FetchPromise = null
  if (view.queryType === 'eastmoney_nl') {
    resultColumns.value = buildNlColumns(view.columns || [])
    if (!resultColumns.value.find((c) => c.key === 'actions')) {
      resultColumns.value.push({
        title: '操作',
        key: 'actions',
        width: 170,
        fixed: 'right',
        render(row) {
          return renderRowActions(row)
        },
      })
    }
    resultColumns.value = injectSignalColumn(resultColumns.value)
    resultData.value = view.dataList || []
  } else {
    resultColumns.value = injectSignalColumn(buildTechnicalColumns())
    resultData.value = view.dataList || []
  }
  showResult.value = true
  scanBuySignals()
}

function followRow(row) {
  const code = toFollowCodeFromRow(row)
  if (!code) {
    message.warning('无法识别股票代码')
    return
  }
  followWithDateGroup(code, Follow).then(({ followResult, groupInfo }) => {
    if (followResult === '关注成功') {
      message.success(formatFollowGroupMessage(followResult, groupInfo))
    } else {
      message.error(followResult)
    }
  })
}

async function openHistory(row) {
  historyStrategyId.value = row.id
  historyStrategyName.value = row.name || '未命名策略'
  historyPagination.page = 1
  showHistory.value = true
  await loadHistoryRuns(1)
}

async function loadHistoryRuns(page) {
  if (!historyStrategyId.value) return
  historyLoading.value = true
  try {
    const res = await GetStockStrategyRunList({
      strategyId: historyStrategyId.value,
      page,
      pageSize: historyPagination.pageSize,
    })
    historyRuns.value = res?.data || []
    historyPagination.page = page
    historyPagination.itemCount = res?.total ?? 0
    historyPagination.pageCount = Math.max(1, Math.ceil(historyPagination.itemCount / historyPagination.pageSize))
  } finally {
    historyLoading.value = false
  }
}

function handleHistoryPageChange(page) {
  loadHistoryRuns(page)
}

function handleHistoryPageSizeChange(pageSize) {
  historyPagination.pageSize = pageSize
  loadHistoryRuns(1)
}

function formatRunTime(createdAt) {
  return String(createdAt || '').substring(0, 19).replace('T', ' ')
}

async function viewRun(run) {
  const view = await GetStockStrategyRunDetail(run.id)
  if (!view) {
    message.error('加载失败')
    return
  }
  showHistory.value = false
  const replayDay = parseRunDayKey(view.runAt || run.createdAt)
  showRunResult(view, replayDay)
}

function remove(row) {
  dialog.warning({
    title: '删除策略',
    content: `确定删除「${row.name}」？历史记录将一并删除。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      const msg = await DeleteStockStrategy(row.id)
      if (msg.includes('成功')) {
        message.success(msg)
        loadList()
      } else message.error(msg)
    },
  })
}

function onSchedulePreset(v) {
  form.cronExpr = v
  form.enable = !!v
}

function openSaveFromExternal(payload) {
  resetForm()
  if (payload.queryType === 'technical') {
    form.queryType = 'technical'
    if (payload.queryJson) {
      try {
        applyTechnicalIndicators(technical, JSON.parse(payload.queryJson))
        technicalTouched.value = true
        message.success('已从「股票筛选」带入技术条件')
      } catch (e) {
        message.warning('技术条件解析失败')
      }
    }
    form.keyword = payload.keyword || ''
    form.industry = payload.industry || ''
  } else {
    form.queryType = 'eastmoney_nl'
    form.queryText = payload.queryText || ''
  }
  form.name = payload.name || ''
  if (!form.name && form.queryType === 'eastmoney_nl') {
    form.name = '指标选股策略'
  }
  showEdit.value = true
}

onBeforeMount(() => {
  GetConfig().then((res) => {
    darkTheme.value = !!res?.darkTheme
  })
  loadList()
  loadIndustries()
  EventsOn('openSaveStockStrategy', openSaveFromExternal)
})

onBeforeUnmount(() => {
  EventsOff('openSaveStockStrategy')
})
</script>

<template>
  <div class="strategy-page">
    <div class="strategy-page-header">
    <n-space justify="space-between" align="center" wrap>
      <n-text depth="2">保存自然语言或技术面条件，支持定时执行、冰点/买点扫描与历史记录。</n-text>
      <n-space wrap>
        <n-dropdown
          trigger="click"
          :options="presetOptions"
          @select="applyStrategyPreset"
        >
          <n-button>从模板创建</n-button>
        </n-dropdown>
        <n-button type="primary" @click="openCreate">
          <template #icon><n-icon :component="AddOutline" /></template>
          新建策略
        </n-button>
      </n-space>
    </n-space>
    </div>

    <div class="strategy-page-table">
    <n-data-table
      :columns="columns"
      :data="strategyList"
      :loading="loading"
      :pagination="pagination"
      remote
      size="small"
      flex-height
      :scroll-x="900"
      style="height: calc(100vh - 280px); min-height: 360px"
      @update:page="(p) => { pagination.page = p; loadList() }"
    />
    </div>

    <n-modal
      v-model:show="showEdit"
      preset="card"
      :title="editingId ? '编辑策略' : '新建策略'"
      style="width: 880px; max-width: 95vw"
      :content-style="{ maxHeight: 'min(85vh, 820px)', overflowY: 'auto' }"
    >
      <n-form label-placement="left" label-width="100">
        <n-form-item label="策略名称">
          <n-input v-model:value="form.name" placeholder="例如：冰点超跌" />
        </n-form-item>
        <n-form-item label="策略类型">
          <n-radio-group v-model:value="form.queryType">
            <n-radio value="eastmoney_nl">自然语言</n-radio>
            <n-radio value="technical">技术面</n-radio>
          </n-radio-group>
        </n-form-item>
        <n-form-item v-if="form.queryType === 'eastmoney_nl'" label="板块/行业">
          <n-space vertical :size="6" style="width: 100%">
            <n-select
              v-model:value="form.nlIndustries"
              :options="industryOptions"
              multiple
              filterable
              tag
              max-tag-count="responsive"
              placeholder="可多选，如：半导体、银行（可选）"
              style="width: 100%; max-width: 520px"
            />
            <n-text v-if="!industryOptions.length" depth="3" style="font-size: 12px">
              暂无行业列表，请先在「股票信息」页加载数据后再选；也可直接在下方条件里写「半导体」
            </n-text>
            <n-text v-else-if="nlMergedPreview" depth="3" style="font-size: 12px">
              提交东财：{{ nlMergedPreview }}
            </n-text>
          </n-space>
        </n-form-item>
        <n-form-item v-if="form.queryType === 'eastmoney_nl'" label="选股条件">
          <n-input
            v-model:value="form.queryText"
            type="textarea"
            :rows="3"
            placeholder="如：RSI小于30；60日新低；非ST；成交额大于5000万（需配置东财 qgqp_b_id）"
          />
        </n-form-item>
        <n-form-item v-if="form.queryType === 'technical'" label="板块/行业">
          <n-space vertical :size="4" style="width: 100%">
            <n-select
              v-model:value="form.industry"
              :options="industryOptions"
              placeholder="全部行业（可选，如：半导体、银行）"
              clearable
              filterable
              style="width: 100%; max-width: 420px"
            />
            <n-text v-if="!industryOptions.length" depth="3" style="font-size: 12px">
              暂无行业列表，请先在「股票信息」页加载数据后再选
            </n-text>
          </n-space>
        </n-form-item>
        <n-form-item v-if="form.queryType === 'technical'" label="关键词">
          <n-input v-model:value="form.keyword" placeholder="可选，按股票名称筛选" clearable />
        </n-form-item>
        <n-form-item v-if="form.queryType === 'technical'" label="技术条件">
          <n-alert
            v-if="!hasActiveTechnicalIndicator(technical)"
            type="warning"
            :bordered="false"
            style="margin-bottom: 8px"
            title="未选择任何条件"
          >
            技术面策略须至少勾选一项，否则会筛出无效结果。可从「股票筛选」页勾选后点「保存为策略」自动带入。
          </n-alert>
          <TechnicalIndicatorFields :model="technical" />
        </n-form-item>
        <n-form-item label="每页数量">
          <n-input-number v-model:value="form.pageSize" :min="10" :max="500" style="width: 160px" />
        </n-form-item>
        <n-form-item label="定时执行">
          <n-space vertical>
            <n-select
              :value="form.cronExpr"
              :options="schedulePresets"
              placeholder="选择或下方自定义"
              @update:value="onSchedulePreset"
            />
            <n-input v-model:value="form.cronExpr" placeholder="Cron 如 0 35 9 * * 1-5" clearable />
            <n-checkbox v-model:checked="form.enable">启用定时（需填写 Cron）</n-checkbox>
          </n-space>
        </n-form-item>
        <n-form-item label="备注">
          <n-input v-model:value="form.description" type="textarea" :rows="2" />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showEdit = false">取消</n-button>
          <n-button type="primary" @click="saveStrategy">保存</n-button>
        </n-space>
      </template>
    </n-modal>

    <n-modal v-model:show="showResult" preset="card" :title="resultModalTitle" style="width: 92vw; max-width: 1200px">
      <n-text v-if="resultView?.traceInfo" type="info" style="display: block; margin-bottom: 8px">
        条件：{{ resultView.traceInfo }}
      </n-text>
      <n-alert
        v-if="resultReplayDayKey"
        type="warning"
        :bordered="false"
        style="margin-bottom: 8px"
        :title="'历史回放 · 截止 ' + resultReplayDayKey"
      >
        表格价格为执行当时快照；买点/K 线按该日及之前 K 线回放，不含之后行情。
      </n-alert>
      <n-space vertical :size="8" style="margin-bottom: 8px">
        <n-space wrap align="center">
          <n-button size="small" type="primary" :loading="scanBuyLoading" @click="scanBuySignals">
            扫描K线信号
          </n-button>
          <n-select
            v-model:value="filterSignalTags"
            :options="signalFilterOptions"
            multiple
            clearable
            placeholder="信号筛选"
            size="small"
            style="min-width: 140px"
            :disabled="!signalByCode.size || showBuyOnly || filterSimilarOnly"
            @update:value="() => { showBuyOnly = false; showSignalHitsOnly = false }"
          />
          <n-button
            size="small"
            type="error"
            tertiary
            :disabled="!signalByCode.size"
            @click="applyPeakPullbackPreset"
          >
            九联形态
          </n-button>
          <n-checkbox
            v-model:checked="filterSimilarOnly"
            :disabled="!referencePattern || !signalByCode.size"
          >
            仅相似{{ similarMatchCount ? `（${similarMatchCount}）` : '' }}
          </n-checkbox>
          <n-button
            v-if="referencePattern"
            size="small"
            quaternary
            @click="clearReferencePattern"
          >
            清除参考
          </n-button>
          <n-checkbox
            v-model:checked="showSignalHitsOnly"
            :disabled="!signalByCode.size || showBuyOnly || filterSimilarOnly || filterSignalTags.length > 0"
          >
            仅有信号（{{ signalHitCount }}）
          </n-checkbox>
          <n-checkbox v-model:checked="showBuyOnly" :disabled="!signalByCode.size || filterSimilarOnly">
            仅买点（{{ buyCandidateCount }}）
          </n-checkbox>
          <n-button size="small" type="warning" :disabled="buyCandidateCount === 0" @click="followAllBuyCandidates">
            一键关注买点
          </n-button>
        </n-space>
        <n-text v-if="referencePattern && filterSimilarOnly" depth="3" style="font-size: 12px">
          参考：{{ referencePatternLabel }} · {{ describeSignalPattern(referencePattern) }}
        </n-text>
        <n-text depth="3" style="font-size: 12px">
          <template v-if="resultReplayDayKey">
            买/趋/减/止=信号当日 · 强/突=确认完成 · 冰=RSI&lt;30 · 回放至 {{ resultReplayDayKey }}
          </template>
          <template v-else>
            买/趋/减/止=最后一根有效K线 · 强(+2)/突(+3)=确认完成 · 九联形态=今日减+曾趋/突+曾止+MA20下
          </template>
        </n-text>
      </n-space>
      <n-data-table
        :columns="resultColumns"
        :data="displayResultData"
        :loading="scanBuyLoading"
        :pagination="{ pageSize: 15 }"
        size="small"
        :scroll-x="900"
        max-height="60vh"
      />
      <n-text
        v-if="resultData.length && !displayResultData.length && signalByCode.size"
        depth="3"
        style="display: block; margin-top: 8px; font-size: 12px"
      >
        当前筛选无匹配项（共 {{ resultData.length }} 只）。可取消「仅相似 / 信号筛选 / 仅有信号」或点「清除参考」。
      </n-text>
    </n-modal>

    <n-modal
      v-model:show="showHistory"
      preset="card"
      :title="`执行历史 · ${historyStrategyName}`"
      style="width: min(720px, 94vw)"
      @after-leave="historyRuns = []; historyStrategyName = ''"
    >
      <n-data-table
        size="small"
        :columns="historyColumns"
        :data="historyRuns"
        :loading="historyLoading"
        :pagination="historyPagination"
        :row-key="(row) => row.id"
        :scroll-x="520"
        @update:page="handleHistoryPageChange"
        @update:page-size="handleHistoryPageSizeChange"
      />
    </n-modal>

    <stock-kline-modal
      v-model:show="showKline"
      :title="klineModalTitle"
      :chart-key="'strategy-kline-' + klineCode + '-' + klineFocusSignal + '-' + (resultReplayDayKey || 'live')"
      :code="klineCode"
      :stock-name="klineName"
      :dark-theme="darkTheme"
      :chart-height="520"
      :strategy-signals="true"
      :focus-signal-tag="klineFocusSignal"
      :signal-as-of-day="resultReplayDayKey || liveSignalAsOfDay"
      :realtime-interval-ms="resultReplayDayKey ? 0 : undefined"
      @after-leave="klineFocusSignal = ''"
    >
      <template #footer>
        <n-space justify="end">
          <n-button size="small" @click="showKline = false">关闭</n-button>
          <n-button size="small" type="primary" @click="applySimilarFromKline(); showKline = false">
            在结果中找相似
          </n-button>
        </n-space>
      </template>
    </stock-kline-modal>
  </div>
</template>

<style scoped>
.strategy-page {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
  overflow: hidden;
  --wails-draggable: no-drag;
}
.strategy-page-header {
  flex-shrink: 0;
}
.strategy-page-table {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
</style>
