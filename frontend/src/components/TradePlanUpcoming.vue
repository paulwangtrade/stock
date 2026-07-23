<script setup>
import { computed, h, onMounted, ref } from 'vue'
import {
  NButton,
  NDataTable,
  NEmpty,
  NInput,
  NSpace,
  NSpin,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import {
  TRADE_PLAN_CODE_NO_UPCOMING,
  TRADE_PLAN_CODE_OK,
  getUpcomingTradePlan,
} from '../api/tradePlans'

const message = useMessage()
const loading = ref(false)
const tradeDateInput = ref('')
const requestTradeDate = ref('')
const emptyMessage = ref('')
const plan = ref(null)

const hasPlan = computed(() => !!plan.value)

const sourceLabel = computed(() => {
  const s = String(plan.value?.source_session || '').trim()
  if (s === 'after_close') return '盘后计划'
  if (s === 'morning_rebuild') return '早盘重建'
  return s || '—'
})

const itemColumns = [
  { title: '代码', key: 'stock_code', width: 110 },
  { title: '名称', key: 'stock_name', width: 100 },
  {
    title: '方向',
    key: 'side',
    width: 80,
    render(row) {
      const side = String(row.side || '')
      const type = side === 'buy' ? 'error' : side === 'sell' ? 'success' : 'default'
      return h(NTag, { size: 'small', type, bordered: false }, { default: () => side || '—' })
    },
  },
  { title: '优先级', key: 'priority', width: 80 },
  {
    title: '计划金额',
    key: 'target_amount',
    width: 110,
    render(row) {
      const n = Number(row.target_amount) || 0
      return n.toLocaleString()
    },
  },
  { title: '状态', key: 'status', width: 90 },
  {
    title: '得分',
    key: 'score',
    width: 80,
    render(row) {
      return Number(row.score || 0).toFixed(2)
    },
  },
  {
    title: '风控',
    key: 'risk',
    ellipsis: { tooltip: true },
    render(row) {
      const code = String(row.risk_code || '').trim()
      const msg = String(row.risk_message || '').trim()
      if (!code && !msg) return '—'
      return code && msg ? `${code}: ${msg}` : code || msg
    },
  },
  {
    title: '策略',
    key: 'strategy_name',
    ellipsis: { tooltip: true },
    render(row) {
      return row.strategy_name || '—'
    },
  },
]

async function refresh() {
  loading.value = true
  emptyMessage.value = ''
  try {
    const res = await getUpcomingTradePlan(tradeDateInput.value || undefined)
    requestTradeDate.value = res.trade_date || tradeDateInput.value || ''
    if (res.code === TRADE_PLAN_CODE_NO_UPCOMING) {
      plan.value = null
      emptyMessage.value = res.message || '暂无即将交易的计划'
      return
    }
    if (res.code !== TRADE_PLAN_CODE_OK) {
      plan.value = null
      emptyMessage.value = res.message || `加载失败 (code=${res.code})`
      message.error(emptyMessage.value)
      return
    }
    plan.value = res.plan
    if (!res.plan) {
      emptyMessage.value = res.message || '暂无即将交易的计划'
    }
  } catch (e) {
    plan.value = null
    emptyMessage.value = e?.message || String(e)
    message.error(emptyMessage.value)
  } finally {
    loading.value = false
  }
}

onMounted(refresh)
</script>

<template>
  <div class="trade-plan-upcoming">
    <n-space justify="space-between" align="center" style="margin-bottom: 12px">
      <n-space align="center">
        <n-text strong>明日交易计划</n-text>
        <n-tag size="small" type="info" :bordered="false">只读</n-tag>
      </n-space>
      <n-space align="center">
        <n-input
          v-model:value="tradeDateInput"
          placeholder="trade_date YYYY-MM-DD（可选）"
          clearable
          style="width: 220px"
        />
        <n-button :loading="loading" @click="refresh">刷新</n-button>
      </n-space>
    </n-space>

    <n-spin :show="loading">
      <template v-if="hasPlan">
        <n-space vertical :size="10" style="margin-bottom: 14px">
          <n-space align="center" :wrap="true">
            <n-text>TradeDate：{{ plan.trade_date || '—' }}</n-text>
            <n-text depth="3">|</n-text>
            <n-text>PlanVersion：{{ plan.plan_version }}</n-text>
            <n-text depth="3">|</n-text>
            <n-text>Status：</n-text>
            <n-tag size="small" :bordered="false">{{ plan.status || '—' }}</n-tag>
            <n-text depth="3">|</n-text>
            <n-text>来源：{{ sourceLabel }}</n-text>
            <n-text depth="3">|</n-text>
            <n-text depth="3">PoolID：{{ plan.pool_id || '—' }}</n-text>
          </n-space>

          <n-space align="center" :wrap="true">
            <n-text>Risk Status：</n-text>
            <n-tag
              size="small"
              :type="plan.risk?.passed ? 'success' : 'warning'"
              :bordered="false"
            >
              {{ plan.risk?.passed ? '通过' : '未通过' }}
            </n-tag>
            <n-text depth="3">|</n-text>
            <n-text>Freeze Status：</n-text>
            <n-tag
              size="small"
              :type="plan.freeze?.is_frozen ? 'success' : 'default'"
              :bordered="false"
            >
              {{ plan.freeze?.is_frozen ? '已冻结' : '未冻结' }}
            </n-tag>
            <template v-if="plan.freeze?.freeze_at">
              <n-text depth="3">|</n-text>
              <n-text>FreezeAt：{{ plan.freeze.freeze_at }}</n-text>
            </template>
            <template v-if="plan.freeze?.freeze_by">
              <n-text depth="3">|</n-text>
              <n-text depth="3">FreezeBy：{{ plan.freeze.freeze_by }}</n-text>
            </template>
          </n-space>

          <div v-if="(plan.risk?.reasons || []).length" class="risk-reasons">
            <n-text depth="3">Risk Reasons：</n-text>
            <ul>
              <li v-for="(r, i) in plan.risk.reasons" :key="i">{{ r }}</li>
            </ul>
          </div>
          <n-text v-else depth="3">Risk Reasons：无</n-text>

          <n-text v-if="!plan.freeze?.is_frozen" depth="3" style="display: block">
            当前计划未冻结，仅供观测，不作为执行依据。
          </n-text>
        </n-space>

        <n-data-table
          v-if="(plan.items || []).length"
          size="small"
          :columns="itemColumns"
          :data="plan.items"
          :row-key="(row) => `${row.stock_code}-${row.priority}`"
        />
        <n-empty v-else description="计划无明细 items" />
      </template>

      <n-empty
        v-else
        :description="emptyMessage || (requestTradeDate ? `无 upcoming（${requestTradeDate}）` : '暂无即将交易的计划')"
      />
    </n-spin>
  </div>
</template>

<style scoped>
.trade-plan-upcoming {
  height: 100%;
  overflow: auto;
  padding: 4px 2px 12px;
  box-sizing: border-box;
}
.risk-reasons ul {
  margin: 4px 0 0;
  padding-left: 18px;
}
</style>
