<script setup>
/**
 * 归因观察：信号扫描快照命中 vs 随后 +1/+3/+10 个交易日，以及迄今。
 * 策略 → 信号标签级联后，表格、汇总和分组共性都只统计筛完的子集。
 * 假设沙盘只对照等权汇总，不改明细表，不写交易计划。
 */
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NAlert,
  NCheckbox,
  NDataTable,
  NEmpty,
  NPagination,
  NSelect,
  NSpace,
  NText,
} from 'naive-ui'
import { GetConfig, GetSignalScanAttribution, ListSignalScanSnapshots } from '../../wailsjs/go/main/App'
import { buildSnapshotHistoryLabel } from '../utils/snapshotDisplay.js'
import { getReboundScreenMaxRsi, getScreenStrategies, parseSignalParams } from '../utils/signalSettings.js'
import { marketColorCssVar } from '../utils/marketColor.js'
import { applyStockClickAction, toStockDisplay } from '../utils/stockDisplay.js'
import {
  reconcileScreenSignalTagSelection,
  signalTagsForScreenStrategy,
} from '../utils/screenStrategySignalFilter.js'
import {
  ATTRIBUTION_DISCLAIMER,
  LARGE_SAMPLE_WARNING,
  RESEARCH_STAT_LABEL,
  WHATIF_BASE_FALLBACK,
  WHATIF_DIFF_LABEL,
  WHATIF_IN_SAMPLE_MARK,
  WHATIF_SCENARIO_FALLBACK,
  WHATIF_TITLE,
  WHATIF_TOGGLE,
  buildAttributionSignalTagOptions,
  displayName,
  findHorizon,
  horizonCellText,
  horizonSummaryText,
  horizonTitle,
  strategyCell,
  whatIfArmLabel,
  whatIfColumns,
  whatIfCountDiff,
  whatIfDiffRate,
  whatIfDiffText,
  whatIfNote,
  whatIfReturnText,
  whatIfSlotStat,
} from '../utils/signalScanAttributionDisplay.js'
import StockKlineModal from './StockKlineModal.vue'
import StockLink from './StockLink.vue'

const loading = ref(false)
const errorText = ref('')
const view = ref(null)
const strategyId = ref('')
const strategies = ref([])
const screenSettings = ref(null)
const strategyOptions = ref([{ label: '全部策略', value: '' }])
const signalTags = ref([])
const snapshotId = ref(null)
const snapshotOptions = ref([])
const page = ref(1)
const pageSize = ref(50)
const sortKey = ref('')
const sortDesc = ref(true)
const whatIfSet = ref(false)
const whatIfKeys = ref([])

const klineModal = reactive({
  visible: false,
  title: '',
  chartCode: '',
  stockName: '',
})

const summaryStats = computed(() => view.value?.summary?.horizons || [])
const tableRows = computed(() =>
  (view.value?.rows || []).map((row, index) => ({
    ...row,
    _rowKey: `${row.code || 'row'}-${row.asOfDate || ''}-${index}`,
  })),
)
const total = computed(() => Number(view.value?.total) || 0)
const disclaimer = computed(() => view.value?.disclaimer || ATTRIBUTION_DISCLAIMER)
const researchLabel = computed(() => view.value?.researchStatLabel || view.value?.summary?.label || RESEARCH_STAT_LABEL)
const selectedStrategy = computed(() => {
  if (!strategyId.value) return null
  return strategies.value.find((item) => item.id === strategyId.value) || { id: strategyId.value }
})
const allowedSignalTags = computed(() => signalTagsForScreenStrategy(selectedStrategy.value))
const signalTagOptions = computed(() => buildAttributionSignalTagOptions(allowedSignalTags.value))
const strategyLabels = computed(() => {
  const map = {}
  for (const item of strategies.value) {
    if (item?.id) map[item.id] = item.name || item.id
  }
  return map
})
const filteredCount = computed(() => Number(view.value?.summary?.hitCount) || 0)
const snapshotHitCount = computed(() => Number(view.value?.summary?.snapshotHitCount) || 0)
const cohort = computed(() => view.value?.cohort || null)
const whatIf = computed(() => view.value?.whatIf || null)
const whatIfCols = computed(() => whatIfColumns())

function whatIfFilter(key) {
  const list = whatIf.value?.filters || []
  return list.find((item) => item.key === key) || null
}

function resetWhatIfSelection() {
  whatIfSet.value = false
  whatIfKeys.value = []
}

function syncSignalTagsToStrategy() {
  const next = reconcileScreenSignalTagSelection(signalTags.value, allowedSignalTags.value)
  const prev = signalTags.value
  const changed = next.length !== prev.length || next.some((tag, index) => tag !== prev[index])
  if (changed) signalTags.value = next
  return changed
}

function currentReboundMaxRsi() {
  const settings = selectedStrategy.value?.settings || screenSettings.value
  const n = Number(getReboundScreenMaxRsi(settings))
  return Number.isFinite(n) ? n : undefined
}

function openStockKline(model) {
  applyStockClickAction(model, klineModal)
}

function stockModel(row) {
  const name = String(row?.name || '').trim()
  return toStockDisplay({
    code: row?.klineCode || row?.code,
    name: name && name !== '—' ? name : '',
  })
}

function renderHorizon(horizon) {
  return (row) => {
    const cell = horizon === 'toDate' ? row.toDate : findHorizon(row, horizon)
    const text = horizonCellText(cell)
    if (!cell || cell.status !== 'ok') {
      return h('span', { title: horizonTitle(cell), style: 'opacity: 0.72' }, text)
    }
    return h(
      'span',
      { title: horizonTitle(cell), style: { color: marketColorCssVar(cell.returnRate) } },
      text,
    )
  }
}

const columns = computed(() => {
  const activeKey = sortKey.value
  const desc = sortDesc.value
  const titleOf = (label, key) => {
    const mark = activeKey === key ? (desc ? ' ↓' : ' ↑') : ''
    return h(
      'button',
      {
        type: 'button',
        class: 'attr-sort',
        title: activeKey === key ? '切换排序方向' : '按该列涨跌排序',
        onClick: () => toggleSort(key),
      },
      `${label}${mark}`,
    )
  }
  return [
  { title: '代码', key: 'code', width: 110 },
  {
    title: '名称',
    key: 'name',
    width: 160,
    render: (row) => {
      const model = stockModel(row)
      if (!model?.klineKey) return displayName(row.name)
      return h(StockLink, { model, onOpen: openStockKline })
    },
  },
  {
    title: '策略',
    key: 'strategyId',
    width: 180,
    ellipsis: { tooltip: true },
    render: (row) => strategyCell(row, strategyLabels.value),
  },
  { title: '对照日', key: 'asOfDate', width: 120 },
  {
    title: '对照价',
    key: 'closeText',
    width: 120,
    render: (row) =>
      h('div', [
        h('div', row.closeText || '数据不足'),
        row.priceBasisLabel
          ? h('div', { style: 'font-size: 12px; opacity: 0.7' }, row.priceBasisLabel)
          : null,
      ]),
  },
  { title: titleOf('+1 交易日', '1'), key: 'h1', width: 120, render: renderHorizon(1) },
  { title: titleOf('+3 交易日', '3'), key: 'h3', width: 120, render: renderHorizon(3) },
  { title: titleOf('+10 交易日', '10'), key: 'h10', width: 130, render: renderHorizon(10) },
  { title: titleOf('迄今', 'toDate'), key: 'toDate', width: 110, render: renderHorizon('toDate') },
]
})

async function loadStrategies() {
  try {
    const cfg = await GetConfig()
    const parsed = parseSignalParams(cfg?.signalParams)
    screenSettings.value = parsed
    const list = getScreenStrategies(parsed)
    strategies.value = list
    strategyOptions.value = [
      { label: '全部策略', value: '' },
      ...list.map((item) => ({ label: item.name || item.id, value: item.id })),
    ]
  } catch {
    strategies.value = []
    screenSettings.value = null
    strategyOptions.value = [{ label: '全部策略', value: '' }]
  }
}

async function loadSnapshots() {
  errorText.value = ''
  try {
    const res = await ListSignalScanSnapshots({
      page: 1,
      pageSize: 50,
      strategyId: strategyId.value || '',
    })
    const opts = (res?.data || [])
      .filter((snap) => snap?.id)
      .map((snap) => ({
        label: buildSnapshotHistoryLabel(snap),
        value: snap.id,
      }))
    snapshotOptions.value = opts
    if (!opts.some((item) => item.value === snapshotId.value)) {
      snapshotId.value = opts[0]?.value ?? null
    }
  } catch (err) {
    snapshotOptions.value = []
    snapshotId.value = null
    errorText.value = err?.message || '无法读取快照列表'
  }
}

async function loadTable() {
  if (!snapshotId.value) {
    view.value = null
    return
  }
  loading.value = true
  errorText.value = ''
  try {
    const res = await GetSignalScanAttribution({
      snapshotId: snapshotId.value,
      page: page.value,
      pageSize: pageSize.value,
      signalTags: signalTags.value.slice(),
      reboundMaxRsi: currentReboundMaxRsi(),
      sortKey: sortKey.value,
      sortDesc: sortDesc.value,
      whatIfSet: whatIfSet.value,
      whatIfKeys: whatIfSet.value ? whatIfKeys.value.slice() : [],
    })
    view.value = res || null
    if (res && res.ok === false) {
      errorText.value = res.message || '无法生成对照表'
    }
  } catch (err) {
    view.value = null
    errorText.value = err?.message || '无法读取归因对照'
  } finally {
    loading.value = false
  }
}

async function onStrategyChange(value) {
  strategyId.value = value || ''
  syncSignalTagsToStrategy()
  resetWhatIfSelection()
  page.value = 1
  await loadSnapshots()
  await loadTable()
}

async function onSignalTagsChange(value) {
  const raw = Array.isArray(value) ? value.filter(Boolean) : []
  const next = reconcileScreenSignalTagSelection(raw, allowedSignalTags.value)
  const prev = signalTags.value
  const changed = next.length !== prev.length || next.some((tag, index) => tag !== prev[index])
  if (!changed) return
  signalTags.value = next
  resetWhatIfSelection()
  page.value = 1
  await loadTable()
}

async function onSnapshotChange(value) {
  snapshotId.value = value || null
  resetWhatIfSelection()
  page.value = 1
  await loadTable()
}

async function onWhatIfToggle(key, checked) {
  const current = new Set(
    whatIfSet.value
      ? whatIfKeys.value
      : (whatIf.value?.filters || []).filter((item) => item.enabled).map((item) => item.key),
  )
  if (checked) current.add(key)
  else current.delete(key)
  whatIfKeys.value = [...current]
  whatIfSet.value = true
  await loadTable()
}

function whatIfCellText(arm, key) {
  return whatIfReturnText(whatIfSlotStat(arm, key))
}

function whatIfDiffCell(key) {
  return whatIfDiffText(whatIfSlotStat(whatIf.value?.baseline, key), whatIfSlotStat(whatIf.value?.scenario, key))
}

function whatIfDiffStyle(key) {
  const rate = whatIfDiffRate(whatIfSlotStat(whatIf.value?.baseline, key), whatIfSlotStat(whatIf.value?.scenario, key))
  if (rate == null) return {}
  return { color: marketColorCssVar(rate) }
}

async function onPageChange(next) {
  page.value = next
  await loadTable()
}

async function toggleSort(key) {
  if (sortKey.value === key) sortDesc.value = !sortDesc.value
  else {
    sortKey.value = key
    sortDesc.value = true
  }
  page.value = 1
  await loadTable()
}

onMounted(async () => {
  await loadStrategies()
  syncSignalTagsToStrategy()
  await loadSnapshots()
  await loadTable()
})
</script>

<template>
  <div class="attribution-panel">
    <n-space vertical :size="10">
      <div>
        <n-text strong style="font-size: 15px">归因观察</n-text>
        <n-text depth="3" style="margin-left: 8px; font-size: 12px">
          快照命中与随后交易日涨跌对照。{{ researchLabel }}。
        </n-text>
      </div>

      <n-alert type="info" :bordered="false">
        {{ view?.scopeNote || '只对照信号扫描快照里的命中，不连接入场、持仓或退出，也不写入交易计划。' }}
      </n-alert>

      <n-space align="center" :wrap="true">
        <n-select
          :value="strategyId"
          :options="strategyOptions"
          size="small"
          style="width: 200px"
          placeholder="策略"
          @update:value="onStrategyChange"
        />
        <n-select
          :value="signalTags"
          :options="signalTagOptions"
          multiple
          clearable
          size="small"
          style="min-width: 220px; max-width: 420px"
          :placeholder="signalTagOptions.length ? '信号标签（可多选）' : '该策略没有可筛信号'"
          :disabled="!signalTagOptions.length"
          @update:value="onSignalTagsChange"
        />
        <n-select
          :value="snapshotId"
          :options="snapshotOptions"
          size="small"
          style="min-width: 320px; max-width: 520px"
          placeholder="选择快照"
          @update:value="onSnapshotChange"
        />
        <n-text v-if="snapshotOptions.length >= 50" depth="3" style="font-size: 12px">只列出最近 50 条快照</n-text>
      </n-space>

      <n-alert v-if="errorText" type="warning" :bordered="false">{{ errorText }}</n-alert>
      <n-alert v-if="view?.largeSample" type="warning" :bordered="false">
        {{ view.largeSampleWarning || LARGE_SAMPLE_WARNING }}
      </n-alert>

      <n-space v-if="view?.ok" :size="16" :wrap="true">
        <n-text depth="3">筛选后 {{ filteredCount }} 只</n-text>
        <n-text v-if="snapshotHitCount && snapshotHitCount !== filteredCount" depth="3">
          快照共 {{ snapshotHitCount }} 只
        </n-text>
        <n-text depth="3">三档都齐全 {{ view.summary?.completeAll ?? 0 }} 只</n-text>
        <n-text v-for="stat in summaryStats" :key="stat.horizon" depth="3">
          +{{ stat.horizon }} {{ horizonSummaryText(stat) }}
        </n-text>
        <n-text v-if="view.summary?.toDate" depth="3">迄今 {{ horizonSummaryText(view.summary.toDate) }}</n-text>
      </n-space>

      <div v-if="view?.ok && cohort" class="cohort">
        <n-text strong>分组共性（+1 上涨 / 下跌）</n-text>
        <n-text depth="3" style="display: block; margin-top: 4px">{{ cohort.note }}</n-text>
        <n-text v-if="cohort.sizeNote" depth="3" style="display: block; margin-top: 2px">{{ cohort.sizeNote }}</n-text>
        <n-alert v-if="cohort.warning" type="warning" :bordered="false" style="margin-top: 8px">
          {{ cohort.warning }}
        </n-alert>
        <n-text v-if="cohort.browseNote" depth="3" style="display: block; margin-top: 6px">{{ cohort.browseNote }}</n-text>
        <n-text depth="3" style="display: block; margin-top: 6px">
          上涨 {{ cohort.up?.count ?? 0 }} · 下跌 {{ cohort.down?.count ?? 0 }} · 持平 {{ cohort.flat?.count ?? 0 }} · +1 数据不足 {{ cohort.excluded ?? 0 }}
        </n-text>

        <template v-if="cohort.ok">
          <div class="cohort-grid">
            <div class="cohort-card">
              <div class="cohort-card__title">上涨组 {{ cohort.up.count }} 只</div>
              <div>行业 {{ cohort.up.topIndustryShareText }}</div>
              <div>量比中位数 {{ cohort.up.volumeRatioText }}</div>
              <div>前20日 {{ cohort.up.prior20Text }}</div>
              <div>距MA20 {{ cohort.up.distMa20Text }}</div>
              <div>RSI {{ cohort.up.rsiText }}</div>
            </div>
            <div class="cohort-card">
              <div class="cohort-card__title">下跌组 {{ cohort.down.count }} 只</div>
              <div>行业 {{ cohort.down.topIndustryShareText }}</div>
              <div>量比中位数 {{ cohort.down.volumeRatioText }}</div>
              <div>前20日 {{ cohort.down.prior20Text }}</div>
              <div>距MA20 {{ cohort.down.distMa20Text }}</div>
              <div>RSI {{ cohort.down.rsiText }}</div>
            </div>
          </div>
          <div v-if="cohort.contrasts?.length" class="cohort-contrasts">
            <div
              v-for="item in cohort.contrasts"
              :key="item.key"
              class="cohort-contrast"
              :class="{ 'cohort-contrast--hot': item.highlight }"
            >
              <span class="cohort-contrast__label">{{ item.label }}</span>
              <span>上涨 {{ item.upText }}</span>
              <span>下跌 {{ item.downText }}</span>
              <span>{{ item.diffText }}</span>
              <n-checkbox
                v-if="whatIfFilter(item.key)"
                size="small"
                :checked="!!whatIfFilter(item.key).enabled"
                :disabled="loading"
                @update:checked="(checked) => onWhatIfToggle(item.key, checked)"
              >
                {{ WHATIF_TOGGLE }} · {{ whatIfFilter(item.key).rule }}
              </n-checkbox>
            </div>
          </div>
        </template>
        <n-empty v-else :description="cohort.message || '无法分组'" style="margin-top: 8px" />
      </div>

      <div v-if="view?.ok && filteredCount > 0 && whatIf" class="whatif">
        <n-text strong>{{ WHATIF_TITLE }}</n-text>
        <n-text depth="3" style="display: block; margin-top: 4px">{{ whatIfNote(whatIf) }}</n-text>
        <n-text v-if="whatIf.inSampleNote" depth="3" style="display: block; margin-top: 2px">{{ whatIf.inSampleNote }}</n-text>
        <n-text v-if="whatIf.weightNote" depth="3" style="display: block; margin-top: 2px">{{ whatIf.weightNote }}</n-text>
        <n-alert v-if="!whatIf.ok && whatIf.message" type="warning" :bordered="false" style="margin-top: 8px">
          {{ whatIf.message }}
        </n-alert>
        <n-alert v-if="whatIf.warning" type="warning" :bordered="false" style="margin-top: 8px">
          {{ whatIf.warning }}
        </n-alert>
        <table v-if="whatIf.ok" class="whatif-table">
          <thead>
            <tr>
              <th>对照</th>
              <th>只数</th>
              <th v-for="col in whatIfCols" :key="col.key">
                {{ col.label }}
                <span v-if="col.inSample" class="whatif-insample">{{ WHATIF_IN_SAMPLE_MARK }}</span>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>{{ whatIfArmLabel(whatIf.baseline, WHATIF_BASE_FALLBACK) }}</td>
              <td>{{ whatIf.baseline?.hitCount ?? 0 }}</td>
              <td v-for="col in whatIfCols" :key="`base-${col.key}`">{{ whatIfCellText(whatIf.baseline, col.key) }}</td>
            </tr>
            <tr>
              <td>{{ whatIfArmLabel(whatIf.scenario, WHATIF_SCENARIO_FALLBACK) }}</td>
              <td>{{ whatIf.scenario?.hitCount ?? 0 }}</td>
              <td v-for="col in whatIfCols" :key="`sc-${col.key}`">{{ whatIfCellText(whatIf.scenario, col.key) }}</td>
            </tr>
            <tr>
              <td>{{ WHATIF_DIFF_LABEL }}</td>
              <td>{{ whatIfCountDiff(whatIf.baseline, whatIf.scenario) }}</td>
              <td v-for="col in whatIfCols" :key="`diff-${col.key}`" :style="whatIfDiffStyle(col.key)">
                {{ whatIfDiffCell(col.key) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <n-data-table
        v-if="snapshotId && view?.rows?.length"
        size="small"
        :columns="columns"
        :data="tableRows"
        :loading="loading"
        :bordered="false"
        :single-line="false"
        :scroll-x="1180"
        :row-key="(row) => row._rowKey"
      />
      <n-text v-else-if="loading" depth="3">正在读取本地快照…</n-text>
      <n-empty
        v-else
        :description="snapshotOptions.length ? (view?.message || '该快照没有可对照的命中') : '还没有研究快照。请先在机会列表生成信号扫描快照，并保留本地日线。'"
      />

      <n-pagination
        v-if="total > pageSize"
        :page="page"
        :page-size="pageSize"
        :item-count="total"
        @update:page="onPageChange"
      />

      <n-text v-if="view?.barNote" depth="3" style="font-size: 12px; display: block">
        {{ view.barNote }}
      </n-text>
      <n-text v-if="view?.toDateNote" depth="3" style="font-size: 12px; display: block">
        {{ view.toDateNote }}
      </n-text>
      <n-text v-if="view?.calendarNote" depth="3" style="font-size: 12px; display: block">
        {{ view.calendarNote }}
      </n-text>
      <n-text depth="3" style="font-size: 12px; display: block">
        {{ disclaimer }}
      </n-text>
    </n-space>

    <StockKlineModal
      v-model:show="klineModal.visible"
      :title="klineModal.title"
      :code="klineModal.chartCode"
      :stock-name="klineModal.stockName"
    />
  </div>
</template>

<style scoped>
.attribution-panel {
  height: 100%;
  min-height: 0;
  overflow: auto;
  padding: 4px 2px 16px;
  box-sizing: border-box;
}
.attr-sort {
  margin: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  cursor: pointer;
  text-align: left;
}
.attr-sort:hover {
  color: var(--n-primary-color, #18a058);
}
.cohort-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 10px;
  margin-top: 8px;
}
.cohort-card {
  border: 1px solid rgba(128, 128, 128, 0.25);
  border-radius: 8px;
  padding: 8px 10px;
  line-height: 1.6;
  font-size: 13px;
}
.cohort-card__title {
  font-weight: 600;
  margin-bottom: 4px;
}
.cohort-contrasts {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.cohort-contrast {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  font-size: 13px;
  padding: 4px 8px;
  border-radius: 6px;
}
.cohort-contrast--hot {
  background: rgba(24, 160, 88, 0.12);
  font-weight: 600;
}
.cohort-contrast__label {
  min-width: 8em;
}
.whatif {
  margin-top: 4px;
}
.whatif-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  margin-top: 8px;
}
.whatif-table th,
.whatif-table td {
  text-align: left;
  padding: 4px 8px;
  border-bottom: 1px solid rgba(128, 128, 128, 0.2);
  white-space: nowrap;
}
.whatif-insample {
  margin-left: 4px;
  font-size: 12px;
  font-weight: 400;
  opacity: 0.72;
}
</style>
