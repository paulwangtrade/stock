<script setup>
/**
 * TradePlan Origin panel (Phase14-G2.2 + Phase16.28-B).
 * GET /api/tradeplans/{id}/origin → 横向摘要表 + 同源 Origin Drawer（不请求 strategy-explanation）.
 */
import { computed, h, ref, watch } from 'vue'
import { NAlert, NButton, NDataTable, NSpace, NSpin, NTag, NText, NTooltip } from 'naive-ui'
import { getTradePlanOrigin } from '../api/tradePlans.ts'
import {
  ORIGIN_SIGNAL_PRICE_FOOTER,
  buildOriginPanelModel,
} from '../utils/tradePlanOriginDisplay.js'
import { adaptTradePlanOrigin } from '../utils/stockDisplayAdapters.js'
import StockLink from './StockLink.vue'
import TradePlanOriginExplanationDrawer from './tradeplan/TradePlanOriginExplanationDrawer.vue'

const props = defineProps({
  planId: { type: Number, default: 0 },
  /** trade_plans.source_session — Source chip（与 Drawer 同源） */
  sourceSession: { type: String, default: '' },
  /** optional plan-level source hint */
  source: { type: String, default: '' },
  /** stock_code → stock_name from plan items (Origin API 无 name) */
  nameByCode: { type: Object, default: () => ({}) },
})

const emit = defineEmits(['open-stock'])

const loading = ref(false)
const loadError = ref('')
const originItems = ref([])
const drawerShow = ref(false)
const drawerEntry = ref(null)

const panel = computed(() =>
  buildOriginPanelModel(originItems.value, {
    sourceSession: props.sourceSession,
    source: props.source,
    nameByCode: props.nameByCode,
  }),
)

const panelTitle = computed(() => {
  const items = panel.value.items
  if (items.length === 1) {
    const row = items[0]
    const name = row.stockName || row.stockCode
    return name && row.stockCode && name !== row.stockCode
      ? `${name}（${row.stockCode}）· 入选解释`
      : `${row.stockCode || '解释'} · 入选解释`
  }
  return '入选解释'
})

const columns = [
  {
    title: '股票',
    key: 'stock',
    width: 160,
    ellipsis: { tooltip: true },
    render(row) {
      const code = row.stockCode && row.stockCode !== '暂无记录' ? row.stockCode : ''
      const name = row.stockName || ''
      if (!code && !name) return h(NText, { depth: 3 }, { default: () => '—' })
      return h(StockLink, {
        model: adaptTradePlanOrigin({ stock_code: code, stock_name: name }),
        onOpen: (model) => emit('open-stock', model),
      })
    },
  },
  {
    title: '来源',
    key: 'source',
    width: 96,
    render(row) {
      return h(
        NTag,
        { size: 'tiny', bordered: false, type: row.sourceChipType || 'default' },
        { default: () => row.sourceChipLabel || '未知来源' },
      )
    },
  },
  {
    title: '策略',
    key: 'strategy',
    minWidth: 100,
    ellipsis: { tooltip: true },
    render(row) {
      const v = row.strategyNameLabel || '—'
      return h(NText, { depth: v === '—' ? 3 : undefined }, { default: () => v })
    },
  },
  {
    title: '信号',
    key: 'signal',
    width: 72,
    render(row) {
      if (!row.signalTagMissing && row.signalTagLabel && row.signalTagLabel !== '—') {
        return h(NTag, { size: 'small', bordered: false, type: 'info' }, { default: () => row.signalTagLabel })
      }
      return h(NText, { depth: 3 }, { default: () => '—' })
    },
  },
  {
    title: '评分',
    key: 'score',
    width: 72,
    render(row) {
      const v = row.scoreLabel || '—'
      return h(NText, { depth: v === '—' ? 3 : undefined }, { default: () => v })
    },
  },
  {
    title: '入选理由',
    key: 'reason',
    minWidth: 140,
    ellipsis: { tooltip: true },
    render(row) {
      const summary = row.reasonSummary || '暂无说明'
      const full =
        (!row.selectionReasonMissing && row.selectionReason) ||
        (!row.sourceReasonMissing && row.sourceReason) ||
        summary
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () =>
            h(NText, { depth: summary === '暂无说明' ? 3 : undefined }, { default: () => summary }),
          default: () => full,
        },
      )
    },
  },
  {
    title: '操作',
    key: '_action',
    width: 96,
    render(row) {
      return h(
        NButton,
        {
          size: 'tiny',
          secondary: true,
          onClick: () => openDrawer(row),
        },
        { default: () => '查看解释' },
      )
    },
  },
]

function openDrawer(row) {
  drawerEntry.value = row || null
  drawerShow.value = true
}

async function load() {
  const id = Math.trunc(Number(props.planId) || 0)
  loadError.value = ''
  originItems.value = []
  drawerShow.value = false
  drawerEntry.value = null
  if (id <= 0) return

  loading.value = true
  try {
    const res = await getTradePlanOrigin(id)
    if (!res.ok) {
      loadError.value = res.message || '计划来源不可用'
      return
    }
    originItems.value = Array.isArray(res.items) ? res.items : []
  } catch (e) {
    loadError.value = e?.message || String(e)
  } finally {
    loading.value = false
  }
}

watch(
  () => props.planId,
  () => {
    load()
  },
  { immediate: true },
)
</script>

<template>
  <div
    v-if="planId > 0"
    class="execution-window-panel trade-plan-origin-panel"
    role="region"
    aria-label="入选解释"
    data-phase="PHASE16-28-B"
  >
    <n-space align="center" style="margin-bottom: 8px">
      <n-text strong>{{ panelTitle }}</n-text>
      <n-tag size="small" type="info" :bordered="false">只读</n-tag>
      <n-tag v-if="panel.items.length" size="small" :bordered="false">
        {{ panel.items.length }} 只
      </n-tag>
    </n-space>

    <n-alert type="info" :bordered="false" style="margin-bottom: 10px">
      解释「为何从机会/候选进入本计划」；横向摘要 +「查看解释」详情。不触发审批、冻结或执行。
    </n-alert>

    <n-spin :show="loading">
      <n-tag v-if="loadError" type="warning" :bordered="false" style="margin-bottom: 8px">
        {{ loadError }}
      </n-tag>

      <template v-if="panel.items.length">
        <n-data-table
          size="small"
          :columns="columns"
          :data="panel.items"
          :row-key="(row, i) => `${row.stockCode}-${i}`"
          :pagination="false"
          :single-line="true"
          :scroll-x="760"
        />
        <n-text depth="3" class="origin-price-footer">
          {{ ORIGIN_SIGNAL_PRICE_FOOTER }}
        </n-text>
      </template>

      <n-text v-else-if="!loading && !loadError" depth="3" style="font-size: 12px">
        暂无计划项来源数据。
      </n-text>
    </n-spin>

    <TradePlanOriginExplanationDrawer
      v-model:show="drawerShow"
      :entry="drawerEntry"
      @open-stock="(m) => emit('open-stock', m)"
    />
  </div>
</template>

<style scoped>
.trade-plan-origin-panel {
  margin-bottom: 10px;
}

.origin-price-footer {
  display: block;
  margin-top: 8px;
  font-size: 11px;
  line-height: 1.45;
}
</style>
