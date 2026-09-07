<script setup>
import { computed, h, onMounted, ref } from 'vue'
import { NButton, NCard, NDataTable, NEmpty, NSpin, NTag, NText, NSpace } from 'naive-ui'
import { loadModelObservation } from '../utils/watchlistModelObservation.js'

const emit = defineEmits(['loaded'])

const loading = ref(false)
const error = ref('')
const payload = ref(null)

const rows = computed(() => payload.value?.rows || [])
const hasRows = computed(() => rows.value.length > 0)

const columns = [
  { title: '排名', key: 'rank', width: 56 },
  { title: '股票', key: 'stockCode', width: 110 },
  {
    title: '名称',
    key: 'stockName',
    width: 120,
    ellipsis: { tooltip: true },
    render(row) {
      return row.stockName || '—'
    },
  },
  {
    title: '来源',
    key: 'source',
    width: 160,
    ellipsis: { tooltip: true },
    render(row) {
      return h(NTag, { size: 'tiny', bordered: false, type: 'info' }, { default: () => row.source || '—' })
    },
  },
  {
    title: '状态',
    key: 'status',
    width: 100,
    render(row) {
      const s = String(row.status || '')
      const type = s === 'gap_skip' ? 'warning' : s === 'priced' || s === 'ready' ? 'success' : 'default'
      return h(NTag, { size: 'tiny', bordered: false, type }, { default: () => s || '—' })
    },
  },
]

async function refresh() {
  loading.value = true
  error.value = ''
  try {
    const next = await loadModelObservation()
    payload.value = next
    emit('loaded', next)
  } catch (e) {
    error.value = e?.message || String(e)
    payload.value = null
    emit('loaded', { rows: [] })
  } finally {
    loading.value = false
  }
}

onMounted(refresh)

defineExpose({ refresh })
</script>

<template>
  <n-card size="small" class="model-obs-card" :bordered="true" content-style="padding: 10px 12px">
    <n-space align="center" justify="space-between" :wrap="true" style="margin-bottom: 8px">
      <n-space align="center" :size="8">
        <n-text strong>今日模型观察</n-text>
        <n-tag size="tiny" type="info" :bordered="false">只读 · 不自动加入自选</n-tag>
        <n-text v-if="payload?.tradeDate" depth="3" style="font-size: 12px">
          {{ payload.tradeDate }}
          <template v-if="payload.planId"> · plan #{{ payload.planId }}</template>
        </n-text>
      </n-space>
      <n-button size="tiny" quaternary :loading="loading" @click="refresh">刷新</n-button>
    </n-space>
    <n-text depth="3" style="display: block; font-size: 12px; margin-bottom: 8px">
      系统发现（CandidatePool / TradePlan 投影）。仅观察，不产生买卖建议。
      <template v-if="payload?.sourceLabel">来源：{{ payload.sourceLabel }}</template>
    </n-text>
    <n-spin :show="loading">
      <n-tag v-if="error" type="warning" :bordered="false" style="margin-bottom: 8px">{{ error }}</n-tag>
      <n-data-table
        v-if="hasRows"
        size="small"
        :columns="columns"
        :data="rows"
        :bordered="false"
        :single-line="false"
        :pagination="false"
      />
      <n-empty
        v-else-if="!loading"
        size="small"
        :description="payload?.emptyMessage || '暂无模型观察'"
      />
    </n-spin>
  </n-card>
</template>

<style scoped>
.model-obs-card {
  margin-bottom: 8px;
  background: rgba(32, 128, 240, 0.04);
  border-color: rgba(32, 128, 240, 0.22);
}
</style>
