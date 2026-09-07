<!--
  @deprecated Phase16.5 — 生产入口已迁移至 ResearchCandidatePool.vue（researchIndex.vue 挂载）。
  本组件无路由 / 无 import 引用，保留供历史对照；新功能请勿 import 本文件。
  参见 PHASE16_CODE_QUALITY_AUDIT.md §4 orphan 组件说明。
-->
<script setup>
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NDataTable,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NSkeleton,
  NSpace,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import { getCandidatePool } from '../api/candidatePool'
import {
  createResearchTradeIntent,
  confirmResearchTradeIntent,
  executeResearchTradeIntent,
} from '../api/researchTrade'
import { evaluateTradeRiskGate } from '../utils/riskGate'
import { getLastMarketModeKey } from '../utils/marketStatusBar'

const message = useMessage()
const loading = ref(false)
const submitting = ref(false)
const threshold = ref(60)
const snapshotTime = ref('')
const snapshotId = ref(0)
const hintMessage = ref('')
const rows = ref([])
const buyVisible = ref(false)
const buyForm = reactive({
  stockCode: '',
  stockName: '',
  price: 0,
  volume: 100,
  reason: '候选池模拟买入',
  signalScore: 0,
  signalTag: '',
})

const hasData = computed(() => rows.value.length > 0)

const columns = [
  { title: '代码', key: 'stock_code', width: 110 },
  { title: '名称', key: 'stock_name', width: 100 },
  {
    title: '信号分',
    key: 'signal_score',
    width: 90,
    sorter: (a, b) => a.signal_score - b.signal_score,
  },
  {
    title: '预测方向',
    key: 'direction',
    width: 100,
    render(row) {
      const type = row.direction === '买入' || row.direction === '看多'
        ? 'error'
        : row.direction === '卖出' || row.direction === '看空'
          ? 'success'
          : 'default'
      return h(NTag, { size: 'small', type, bordered: false }, { default: () => row.direction || '—' })
    },
  },
  {
    title: '理由',
    key: 'reason',
    ellipsis: { tooltip: true },
    render(row) {
      return row.reason || row.signal_tag || '—'
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 120,
    fixed: 'right',
    render(row) {
      return h(
        NButton,
        {
          size: 'small',
          type: 'warning',
          secondary: true,
          onClick: () => openBuy(row),
        },
        { default: () => '模拟买入' },
      )
    },
  },
]

async function refresh() {
  loading.value = true
  try {
    const res = await getCandidatePool()
    threshold.value = Number(res.threshold) || 60
    snapshotTime.value = res.snapshot_time || ''
    snapshotId.value = Number(res.snapshot_id) || 0
    hintMessage.value = res.message || ''
    rows.value = Array.isArray(res.data) ? res.data : []
  } catch (e) {
    message.error(e?.message || String(e))
    rows.value = []
    hintMessage.value = '加载候选池失败'
  } finally {
    loading.value = false
  }
}

function openBuy(row) {
  buyForm.stockCode = String(row.stock_code || '')
  buyForm.stockName = String(row.stock_name || '')
  buyForm.price = Number(row.price) || 0
  buyForm.volume = 100
  buyForm.reason = '候选池模拟买入'
  buyForm.signalScore = Number(row.signal_score) || 0
  buyForm.signalTag = String(row.signal_tag || '')
  buyVisible.value = true
}

async function submitBuy() {
  const gate = evaluateTradeRiskGate({
    marketModeKey: getLastMarketModeKey(),
    direction: '买入',
  })
  if (!gate.ok) {
    message.error(gate.reason)
    return
  }
  if (!buyForm.stockCode.trim()) {
    message.warning('股票代码不能为空')
    return
  }
  if (!(Number(buyForm.volume) > 0) || !(Number(buyForm.price) > 0)) {
    message.warning('请填写有效的价格与数量')
    return
  }
  submitting.value = true
  try {
    const created = await createResearchTradeIntent({
      symbol: buyForm.stockCode.trim(),
      stock_name: buyForm.stockName.trim() || buyForm.stockCode.trim(),
      price: Number(buyForm.price),
      volume: Number(buyForm.volume),
      candidate_snapshot_id: Number(snapshotId.value) || 0,
      signal_score: Number(buyForm.signalScore) || 0,
      signal_tag: buyForm.signalTag || '',
      reason: buyForm.reason || '候选池模拟买入',
    })
    await confirmResearchTradeIntent(created.id)
    const exec = await executeResearchTradeIntent(created.id)
    if (exec.status === 'submitted') {
      message.success(`模拟买入成功（订单 #${exec.order_id || ''}）`)
      buyVisible.value = false
      await refresh()
      return
    }
    message.error(
      exec.error_message ||
        exec.error_code ||
        `模拟买入失败（${exec.status || 'rejected'}）`,
    )
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    submitting.value = false
  }
}

function formatSnapshotTime(v) {
  if (!v) return '暂无'
  try {
    const d = new Date(v)
    if (Number.isNaN(d.getTime())) return v
    return d.toLocaleString()
  } catch {
    return v
  }
}

onMounted(refresh)
</script>

<template>
  <div class="candidate-pool">
    <div class="toolbar">
      <n-text depth="3" class="hint">
        基于最新 done 信号快照只读解析，信号分 &gt; 阈值，按分降序
      </n-text>
      <n-button type="primary" secondary :loading="loading" @click="refresh">刷新</n-button>
    </div>

    <n-space v-if="!loading" class="meta" align="center">
      <n-text depth="3">快照时间：{{ formatSnapshotTime(snapshotTime) }}</n-text>
      <n-text depth="3">阈值：{{ threshold }}</n-text>
      <n-text depth="3">候选：{{ rows.length }}</n-text>
    </n-space>

    <div v-if="loading" class="skeleton-wrap">
      <n-skeleton text :repeat="3" />
      <n-skeleton height="180px" style="margin-top: 12px" />
    </div>

    <template v-else>
      <n-empty
        v-if="!hasData"
        class="empty"
        :description="hintMessage || '暂无候选股（请先生成盘后信号快照，或降低阈值）'"
      >
        <template #extra>
          <n-button size="small" @click="refresh">重新加载</n-button>
        </template>
      </n-empty>
      <n-data-table
        v-else
        size="small"
        flex-height
        :columns="columns"
        :data="rows"
        :row-key="(r) => r.stock_code"
        :scroll-x="780"
        style="height: calc(100% - 72px)"
      />
    </template>

    <n-modal
      v-model:show="buyVisible"
      preset="card"
      title="模拟买入"
      style="width: 420px"
      :mask-closable="!submitting"
    >
      <n-form label-placement="left" label-width="72">
        <n-form-item label="代码">
          <n-input :value="buyForm.stockCode" disabled />
        </n-form-item>
        <n-form-item label="名称">
          <n-input :value="buyForm.stockName" disabled />
        </n-form-item>
        <n-form-item label="价格">
          <n-input-number v-model:value="buyForm.price" :min="0.01" :step="0.01" style="width: 100%" />
        </n-form-item>
        <n-form-item label="数量">
          <n-input-number v-model:value="buyForm.volume" :min="100" :step="100" style="width: 100%" />
        </n-form-item>
        <n-form-item label="备注">
          <n-input v-model:value="buyForm.reason" />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button :disabled="submitting" @click="buyVisible = false">取消</n-button>
          <n-button type="warning" :loading="submitting" @click="submitBuy">确认买入</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.candidate-pool {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  box-sizing: border-box;
  padding: 4px 0;
}
.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.hint {
  font-size: 12px;
}
.meta {
  font-size: 12px;
}
.skeleton-wrap {
  flex: 1;
  padding: 8px 0;
}
.empty {
  margin-top: 48px;
}
</style>
