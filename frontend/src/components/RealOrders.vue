<script setup>
import { computed, h, onMounted, ref } from 'vue'
import {
  NButton,
  NDataTable,
  NEmpty,
  NSelect,
  NSpace,
  NSpin,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'

const message = useMessage()
const loading = ref(false)
const orders = ref([])
const statusFilter = ref('')
const expandedRowKeys = ref([])
const fillsByOrder = ref({})
const fillsLoading = ref({})

const statusOptions = [
  { label: '全部', value: '' },
  { label: 'pending', value: 'pending' },
  { label: 'filled', value: 'filled' },
  { label: 'cancelled', value: 'cancelled' },
  { label: 'rejected', value: 'rejected' },
  { label: 'partially_filled', value: 'partially_filled' },
]

const filteredOrders = computed(() => {
  const list = orders.value || []
  if (!statusFilter.value) return list
  return list.filter(
    o => o.status === statusFilter.value || o.brokerStatus === statusFilter.value,
  )
})

const fillColumns = [
  { title: 'exec_id', key: 'execId', ellipsis: { tooltip: true } },
  { title: 'qty', key: 'fillQty', width: 80 },
  {
    title: 'price',
    key: 'fillPrice',
    width: 100,
    render(row) {
      return Number(row.fillPrice || 0).toFixed(4)
    },
  },
  { title: 'cumQty', key: 'cumQty', width: 80 },
  {
    title: 'avgPrice',
    key: 'avgPrice',
    width: 100,
    render(row) {
      return Number(row.avgPrice || 0).toFixed(4)
    },
  },
]

function renderFillsExpand(row) {
  const key = row.id
  const busy = !!fillsLoading.value[key]
  const fills = fillsByOrder.value[key] || []
  return h('div', { class: 'fills-expand' }, [
    h(NText, { depth: 3, style: 'margin-bottom: 8px; display: block' }, () => '成交明细 (fills)'),
    h(NSpin, { show: busy }, {
      default: () =>
        fills.length
          ? h(NDataTable, {
              size: 'small',
              bordered: false,
              columns: fillColumns,
              data: fills,
              pagination: false,
            })
          : h(NEmpty, { description: '暂无成交', size: 'small' }),
    }),
  ])
}

const columns = computed(() => [
  {
    type: 'expand',
    renderExpand: row => renderFillsExpand(row),
  },
  { title: '代码', key: 'stockCode', width: 100 },
  { title: '方向', key: 'side', width: 60 },
  {
    title: 'client_order_id',
    key: 'clientOrderId',
    ellipsis: { tooltip: true },
    minWidth: 160,
  },
  {
    title: 'broker_order_id',
    key: 'brokerOrderId',
    ellipsis: { tooltip: true },
    minWidth: 140,
  },
  { title: 'OMS状态', key: 'status', width: 100 },
  { title: 'broker_status', key: 'brokerStatus', width: 130 },
  { title: 'filled_volume', key: 'filledVolume', width: 110 },
  {
    title: 'avg_price',
    key: 'filledPrice',
    width: 100,
    render(row) {
      return Number(row.filledPrice || 0).toFixed(4)
    },
  },
  { title: 'leaves', key: 'leavesQuantity', width: 80 },
])

async function fetchOrders() {
  loading.value = true
  try {
    const res = await fetch('/api/real/orders')
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`)
    }
    const body = await res.json()
    orders.value = body.orders || []
    fillsByOrder.value = {}
    expandedRowKeys.value = []
  } catch (e) {
    message.error(`加载 RealStub 订单失败: ${e.message || e}`)
    orders.value = []
  } finally {
    loading.value = false
  }
}

async function loadFills(order) {
  const key = order.id || order.clientOrderId
  if (!key || fillsByOrder.value[key]) return
  fillsLoading.value = { ...fillsLoading.value, [key]: true }
  try {
    const res = await fetch(`/api/real/order/${encodeURIComponent(key)}`)
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`)
    }
    const body = await res.json()
    fillsByOrder.value = {
      ...fillsByOrder.value,
      [key]: body.fills || [],
    }
  } catch (e) {
    message.error(`加载成交失败: ${e.message || e}`)
    fillsByOrder.value = { ...fillsByOrder.value, [key]: [] }
  } finally {
    fillsLoading.value = { ...fillsLoading.value, [key]: false }
  }
}

function onExpandedKeysUpdate(keys) {
  const prev = new Set(expandedRowKeys.value)
  expandedRowKeys.value = keys
  for (const id of keys) {
    if (prev.has(id)) continue
    const order = (orders.value || []).find(o => o.id === id)
    if (order) loadFills(order)
  }
}

onMounted(fetchOrders)
</script>

<template>
  <div class="real-orders-panel">
    <n-space justify="space-between" align="center" style="margin-bottom: 12px">
      <n-space align="center">
        <n-text strong>模拟券商 (RealStub)</n-text>
        <n-tag size="small" type="warning" :bordered="false">与 Paper 隔离</n-tag>
      </n-space>
      <n-space>
        <n-select
          v-model:value="statusFilter"
          :options="statusOptions"
          style="width: 180px"
          placeholder="按状态筛选"
        />
        <n-button :loading="loading" @click="fetchOrders">刷新</n-button>
      </n-space>
    </n-space>

    <n-spin :show="loading">
      <n-data-table
        v-if="filteredOrders.length"
        size="small"
        :columns="columns"
        :data="filteredOrders"
        :row-key="row => row.id"
        :expanded-row-keys="expandedRowKeys"
        @update:expanded-row-keys="onExpandedKeysUpdate"
      />
      <n-empty v-else description="暂无 RealStub 订单" />
    </n-spin>
  </div>
</template>

<style scoped>
.real-orders-panel {
  height: 100%;
  overflow: auto;
  padding: 4px 2px 12px;
  box-sizing: border-box;
}
</style>

<style>
.fills-expand {
  padding: 8px 12px 12px 36px;
  background: rgba(128, 128, 128, 0.06);
}
</style>
