/**
 * Apply Phase6.7-G-only hunks to HEAD allStockList.vue.
 * No Meta / snapshotHistorySelect / snapshotDetailCache / screenTablePerf.
 */
const fs = require('fs')
const p = 'D:/stock/frontend/src/components/allStockList.vue'
const raw = fs.readFileSync(p, 'utf8')
const eol = raw.includes('\r\n') ? '\r\n' : '\n'
let t = raw.replace(/\r\n/g, '\n')

function mustInclude(s) {
  if (!t.includes(s)) throw new Error('expected snippet missing: ' + JSON.stringify(s.slice(0, 80)))
}
function replaceOnce(oldStr, newStr, label) {
  const n = t.split(oldStr).length - 1
  if (n !== 1) throw new Error(`${label}: expected 1 occurrence, got ${n}`)
  t = t.replace(oldStr, newStr)
  console.log('ok', label)
}

// 1) imports
replaceOnce(
  `  GetLatestSignalScanSnapshotByStrategy,
  ParseSignalScanSnapshotPayload,
  RunSignalScanSnapshot,
  ListSignalScanSnapshots,
  IsSignalScanRunning,
} from "../../wailsjs/go/main/App";`,
  `  GetLatestSignalScanSnapshotByStrategy,
  ParseSignalScanSnapshotPayload,
  StartSignalScanSnapshot,
  GetLatestSignalScanTask,
  GetSignalScanTask,
  ListSignalScanSnapshots,
  IsSignalScanRunning,
} from "../../wailsjs/go/main/App";`,
  'imports',
)

// 2) onMounted / onBeforeUnmount
replaceOnce(
  `onMounted(() => {
  loadIndustryOptions()
  loadFollowedStockCodes()
  refreshStocks(1)
  EventsOn('allStockListRefresh', handleExternalRefresh)
  EventsOn('signalScanProgress', onBackendSignalScanProgress)
  EventsOn('signalScanDone', onBackendSignalScanDone)
})

onBeforeUnmount(() => {
  EventsOff('allStockListRefresh')
  EventsOff('signalScanProgress')
  EventsOff('signalScanDone')
})`,
  `onMounted(() => {
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
})`,
  'lifecycle',
)

// 3) state after signalScanProgress — match without relying on garbled comment
{
  const marker = 'const signalScanProgress = ref({ phase: \'\', done: 0, total: 0 })'
  const idx = t.indexOf(marker)
  if (idx < 0) throw new Error('signalScanProgress missing')
  const after = t.indexOf('\nconst SIGNAL_SCAN_CONCURRENCY', idx)
  if (after < 0) throw new Error('SIGNAL_SCAN_CONCURRENCY missing')
  t =
    t.slice(0, idx) +
    `const signalScanProgress = ref({ phase: '', done: 0, total: 0 })
/** Phase6.7-G: async snapshot task view */
const scanTaskView = ref(null)
let scanTaskPollTimer = null
const SIGNAL_SCAN_CONCURRENCY` +
    t.slice(after + '\nconst SIGNAL_SCAN_CONCURRENCY'.length)
  // insert lastSnapshotPayload near signalDataSource
  replaceOnce(
    `const signalDataSource = ref('')
const backendScanLoading = ref(false)`,
    `const signalDataSource = ref('')
const lastSnapshotPayload = ref(null)
const backendScanLoading = ref(false)`,
    'lastSnapshotPayload state',
  )
  console.log('ok scanTask state')
}

// 4) status computeds after snapshotMetaLabel
replaceOnce(
  `const snapshotMetaLabel = computed(() => {
  if (!snapshotMeta.value) return ''
  const s = snapshotMeta.value.session === 'midday' ? '午盘' : '盘后'
  return \`\${snapshotMeta.value.tradeDate} \${s} · 扫 \${snapshotMeta.value.scannedTotal} 只 · 命中 \${snapshotMeta.value.hitTotal} 只\`
})

const starBacktestNoDataHint = computed(() => {`,
  `const snapshotMetaLabel = computed(() => {
  if (!snapshotMeta.value) return ''
  const s = snapshotMeta.value.session === 'midday' ? '午盘' : '盘后'
  return \`\${snapshotMeta.value.tradeDate} \${s} · 扫 \${snapshotMeta.value.scannedTotal} 只 · 命中 \${snapshotMeta.value.hitTotal} 只\`
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
  if (task.startTime) parts.push(\`开始 \${task.startTime}\`)
  if (task.durationMs > 0) parts.push(\`耗时 \${(Number(task.durationMs) / 1000).toFixed(0)}s\`)
  if (task.hitTotal != null && task.status === 'completed') parts.push(\`命中 \${task.hitTotal}\`)
  if (task.message) parts.push(task.message)
  return parts.join(' · ')
})

const starBacktestNoDataHint = computed(() => {`,
  'task status computeds',
)

// 5) applySnapshotPayload — keep last payload + allow empty tag show-all
{
  const start = t.indexOf('function applySnapshotPayload(payload, snap) {')
  const end = t.indexOf('\nasync function refreshSnapshotBanner', start)
  if (start < 0 || end < 0) throw new Error('applySnapshotPayload block missing')
  t =
    t.slice(0, start) +
    `function applySnapshotPayload(payload, snap) {
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
` +
    t.slice(end + 1)
  console.log('ok applySnapshotPayload')
}

// 6) replace progress/done/run/loadLive block with G versions + ensureSnapshot
{
  const start = t.indexOf('function onBackendSignalScanProgress(p) {')
  const end = t.indexOf('\nconst vipLevel = ref', start)
  if (start < 0 || end < 0) throw new Error('scan handlers block missing')
  t =
    t.slice(0, start) +
    `function onBackendSignalScanProgress(p) {
  if (!p) return
  backendScanLoading.value = true
  signalScanLoading.value = true
  signalScanProgress.value = { phase: p.phase || 'scan', done: p.done || 0, total: p.total || 0 }
  const phaseLabel = p.phase === 'fetch' ? '拉取名单' : p.phase === 'compute' ? '计算信号' : '扫描'
  signalScanStatus.value = \`\${phaseLabel} \${p.done || 0}/\${p.total || 0}...\`
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
    selectedSnapshotHistoryValue.value = \`\${tradeDate}|\${session}\`
    snapshotTradeDate.value = tradeDate
    snapshotSession.value = session
    await tryLoadSignalSnapshot()
    message.success(\`快照完成：命中 \${ev?.hitTotal ?? scanTaskView.value?.hitTotal ?? 0} 只有信号\`)
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
      selectedSnapshotHistoryValue.value = \`\${snap.tradeDate}|\${snap.session || snapshotSession.value}\`
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

` +
    t.slice(end + 1)
  console.log('ok scan handlers')
}

// 7) loadStocks signal branch
replaceOnce(
  `    if (hasSignalFilter.value) {
      if (selectedSnapshotHistoryValue.value) {
        const loaded = await tryLoadSignalSnapshot()
        if (loaded) return
      }
      await loadWithSignalFilter()
      signalDataSource.value = 'live'
      snapshotMeta.value = null
      return
    }`,
  `    if (hasSignalFilter.value) {
      // Phase6.7-G: snapshot reuse only
      await ensureSnapshotForSignalFilter()
      return
    }`,
  'loadStocks',
)

// 8) clear helpers
replaceOnce(
  `    snapshotMeta.value = null
    signalDataSource.value = ''
    if (hasSignalFilter.value) refreshStocks()`,
  `    snapshotMeta.value = null
    lastSnapshotPayload.value = null
    signalDataSource.value = ''
    if (hasSignalFilter.value) refreshStocks()`,
  'history clear',
)

{
  const idx = t.indexOf('function handleReset')
  const end = t.indexOf('\n}', idx)
  const block = t.slice(idx, end)
  if (!block.includes('lastSnapshotPayload')) {
    t =
      t.slice(0, idx) +
      block.replace(
        'signalFilteredRows.value = []',
        'signalFilteredRows.value = []\n  lastSnapshotPayload.value = null',
      ) +
      t.slice(end)
    console.log('ok handleReset')
  }
}

// 9) template
replaceOnce(
  `<n-button tertiary type="warning" :loading="backendScanLoading || signalScanLoading" @click="runBackendSnapshotScan">
        生成快照
      </n-button>`,
  `<n-button tertiary type="warning" :loading="backendScanLoading || (signalScanLoading && !!scanTaskView)" @click="runBackendSnapshotScan">
        生成快照
      </n-button>
      <n-tag size="small" :type="scanTaskStatusType" :bordered="false">
        生成状态：{{ scanTaskStatusLabel }}
      </n-tag>
      <n-text v-if="scanTaskSummaryText" depth="3" class="snapshot-hint">{{ scanTaskSummaryText }}</n-text>`,
  'template status',
)

replaceOnce(
  `<n-button v-if="hasSignalFilter && snapshotMeta" quaternary @click="loadLiveSignalScan">实时扫描</n-button>`,
  `<n-button v-if="signalDataSource === 'snapshot' && snapshotMeta" quaternary @click="loadLiveSignalScan">退出快照</n-button>`,
  'template exit',
)

replaceOnce(
  `点击生成盘后快照，系统不会自动扫描`,
  `点击生成盘后快照（后台执行）；按信号筛选须先有快照`,
  'template hint',
)

const outText = eol === '\r\n' ? t.replace(/\n/g, '\r\n') : t
fs.writeFileSync(p, outText, 'utf8')

const out = fs.readFileSync(p, 'utf8')
const checks = {
  Start: out.includes('StartSignalScanSnapshot'),
  GetLatest: out.includes('GetLatestSignalScanTask'),
  GetTask: out.includes('GetSignalScanTask'),
  scanTaskView: out.includes('const scanTaskView'),
  poll: out.includes('startScanTaskPoll') && out.includes('stopScanTaskPoll'),
  ensure: out.includes('ensureSnapshotForSignalFilter'),
  loadStocksEnsure: out.includes('await ensureSnapshotForSignalFilter()'),
  noRun: !out.includes('RunSignalScanSnapshot'),
  noMeta: !out.includes('SnapshotMeta'),
  noHistorySelect: !out.includes('snapshotHistorySelect'),
  noDetailCache: !out.includes('snapshotDetailCache'),
  noScreenPerf: !out.includes('screenTablePerf'),
  noMeasure: !out.includes('measurePerformance'),
  exitBtn: out.includes('退出快照'),
  noLiveBtn: !out.includes('>实时扫描<'),
  statusLabel: out.includes('scanTaskStatusLabel'),
  bad: (out.match(/\uFFFD/g) || []).length,
  loadWithStillDefined: out.includes('async function loadWithSignalFilter'),
}
console.log(JSON.stringify(checks, null, 2))
if (!checks.scanTaskView || !checks.loadStocksEnsure || checks.bad > 0 || !checks.noMeta) {
  process.exitCode = 1
}
