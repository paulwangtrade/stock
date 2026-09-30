<script setup>
/**
 * 归因观察：信号扫描快照命中 vs 随后 +1/+3/+10 个交易日。
 * 只读本地快照与日线，不写交易计划。
 */
import { computed, h, onMounted, ref } from 'vue'
import {
  NAlert,
  NDataTable,
  NEmpty,
  NPagination,
  NSelect,
  NSpace,
  NText,
} from 'naive-ui'
import { GetConfig, GetSignalScanAttribution, ListSignalScanSnapshots } from '../../wailsjs/go/main/App'
import { buildSnapshotHistoryLabel } from '../utils/snapshotDisplay.js'
import { getScreenStrategies, parseSignalParams } from '../utils/signalSettings.js'
import { marketColorCssVar } from '../utils/marketColor.js'
import {
  ATTRIBUTION_DISCLAIMER,
  RESEARCH_STAT_LABEL,
  displayName,
  findHorizon,
  horizonCellText,
  horizonSummaryText,
  horizonTitle,
  strategyCell,
} from '../utils/signalScanAttributionDisplay.js'

const loading = ref(false)
const errorText = ref('')
const view = ref(null)
const strategyId = ref('')
const strategyOptions = ref([{ label: '全部策略', value: '' }])
const snapshotId = ref(null)
const snapshotOptions = ref([])
const page = ref(1)
const pageSize = ref(50)

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

function renderHorizon(horizon) {
  return (row) => {
    const cell = findHorizon(row, horizon)
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

const columns = [
  { title: '代码', key: 'code', width: 110 },
  {
    title: '名称',
    key: 'name',
    width: 120,
    render: (row) => displayName(row.name),
  },
  {
    title: '策略',
    key: 'strategyId',
    width: 180,
    ellipsis: { tooltip: true },
    render: (row) => strategyCell(row),
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
  { title: '+1 交易日', key: 'h1', width: 110, render: renderHorizon(1) },
  { title: '+3 交易日', key: 'h3', width: 110, render: renderHorizon(3) },
  { title: '+10 交易日', key: 'h10', width: 120, render: renderHorizon(10) },
]

async function loadStrategies() {
  try {
    const cfg = await GetConfig()
    const strategies = getScreenStrategies(parseSignalParams(cfg?.signalParams))
    strategyOptions.value = [
      { label: '全部策略', value: '' },
      ...strategies.map((item) => ({ label: item.name || item.id, value: item.id })),
    ]
  } catch {
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
  page.value = 1
  await loadSnapshots()
  await loadTable()
}

async function onSnapshotChange(value) {
  snapshotId.value = value || null
  page.value = 1
  await loadTable()
}

async function onPageChange(next) {
  page.value = next
  await loadTable()
}

onMounted(async () => {
  await loadStrategies()
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

      <n-space v-if="view?.ok" :size="16" :wrap="true">
        <n-text depth="3">命中 {{ view.summary?.hitCount ?? 0 }} 只</n-text>
        <n-text depth="3">三档都齐全 {{ view.summary?.completeAll ?? 0 }} 只</n-text>
        <n-text v-for="stat in summaryStats" :key="stat.horizon" depth="3">
          +{{ stat.horizon }} {{ horizonSummaryText(stat) }}
        </n-text>
      </n-space>

      <n-data-table
        v-if="snapshotId && view?.rows?.length"
        size="small"
        :columns="columns"
        :data="tableRows"
        :loading="loading"
        :bordered="false"
        :single-line="false"
        :scroll-x="990"
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
      <n-text v-if="view?.calendarNote" depth="3" style="font-size: 12px; display: block">
        {{ view.calendarNote }}
      </n-text>
      <n-text depth="3" style="font-size: 12px; display: block">
        {{ disclaimer }}
      </n-text>
    </n-space>
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
</style>
