<script setup>
/**
 * Phase16.26-C2.3.2-B1 — 我的跟踪机会（跟踪列表 + 查看/创建交易计划 Draft）
 * 数据：user_opportunity_actions latest=WATCH；取消写 IGNORE；与 followed_stock 无关。
 * 计划：按 stock_code 代表 plan；NONE→创建 Draft；已有→仅查看（禁止覆盖）。
 */
import { computed, h, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  NButton,
  NDataTable,
  NEmpty,
  NFlex,
  NSpace,
  NTag,
  NText,
  useDialog,
  useMessage,
} from 'naive-ui'
import { fetchWatchlist } from '../api/watchlist.ts'
import {
  createWatchlistTradePlanDraft,
  WatchlistDraftError,
  WATCHLIST_DRAFT_ERROR_CODES,
} from '../api/watchlistCreatePlan.ts'
import { postOpportunityAction, OPPORTUNITY_ACTION } from '../api/opportunities.ts'
import { buildIgnoreFromWatchlistItem } from '../utils/opportunityUserAction.js'
import { adaptOpportunity, applyStockClickAction } from '../utils/stockDisplayAdapters.js'
import StockLink from './StockLink.vue'
import StockKlineModal from './StockKlineModal.vue'

const message = useMessage()
const dialog = useDialog()
const router = useRouter()
const loading = ref(false)
const cancelSaving = ref(false)
const createSaving = ref(false)
const watchingCount = ref(0)
const rows = ref([])

const klineModal = reactive({
  visible: false,
  title: '',
  chartCode: '',
  stockName: '',
})

function openStockKline(model) {
  applyStockClickAction(model, klineModal)
}

function formatWatchTime(v) {
  const s = String(v || '').trim()
  if (!s) return '—'
  return s.replace('T', ' ').replace(/\+08:00$/, '').replace(/Z$/, '')
}

function hasExistingPlan(row) {
  const status = String(row?.trade_plan_status || '').toUpperCase()
  return Boolean(row?.in_trade_plan) && status !== 'NONE'
}

function goToTradePlan(rowOrId) {
  const planId =
    typeof rowOrId === 'number' || typeof rowOrId === 'string'
      ? Math.trunc(Number(rowOrId) || 0)
      : Math.trunc(Number(rowOrId?.trade_plan_id) || 0)
  if (planId <= 0) {
    message.warning('无法打开计划：缺少计划标识')
    return
  }
  router.push({
    name: 'tradePlanUpcoming',
    query: { plan_id: String(planId) },
  })
}

async function loadList() {
  loading.value = true
  try {
    const view = await fetchWatchlist({ limit: 100 })
    watchingCount.value = view.watching_count
    rows.value = view.items || []
  } catch (e) {
    rows.value = []
    watchingCount.value = 0
    message.error(e?.message || String(e))
  } finally {
    loading.value = false
  }
}

async function cancelWatchRow(row) {
  if (cancelSaving.value) return
  const payload = buildIgnoreFromWatchlistItem(row)
  if (!payload) {
    message.warning('无法取消跟踪：缺少机会标识或扫描批次')
    return
  }
  cancelSaving.value = true
  try {
    await postOpportunityAction({
      action: OPPORTUNITY_ACTION.IGNORE,
      scanBatchKey: payload.scanBatchKey,
      opportunityId: payload.opportunityId,
      stockCode: payload.stockCode,
      createdBy: 'ui:watched-opportunities',
    })
    message.success('已取消跟踪')
    await loadList()
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    cancelSaving.value = false
  }
}

function confirmCreateTradePlan(row) {
  if (createSaving.value) return
  if (hasExistingPlan(row)) {
    message.warning('该股票已有交易计划，请查看计划')
    return
  }
  const code = String(row?.stock_code || '').trim()
  dialog.warning({
    title: '创建模拟买入计划',
    content: () =>
      h('div', { style: 'line-height: 1.7' }, [
        h('div', `将为 ${code || '该股票'} 生成买入草稿（Draft）。`),
        h('div', { style: 'margin-top: 6px; color: #666' }, '不会批准、锁定、物化或真实下单。'),
        h('div', { style: 'color: #666' }, '默认使用配置买入金额；限价与股数稍后在计划页处理。'),
      ]),
    positiveText: '确认创建草稿',
    negativeText: '取消',
    maskClosable: false,
    onPositiveClick: async () => {
      await createTradePlanRow(row)
    },
  })
}

async function createTradePlanRow(row) {
  if (createSaving.value) return
  const opportunityId = String(row?.opportunity_id || '').trim()
  const scanBatchKey = String(row?.scan_batch_key || '').trim()
  const stockCode = String(row?.stock_code || '').trim()
  if (!opportunityId || !scanBatchKey || !stockCode) {
    message.warning('无法创建：缺少机会标识或扫描批次')
    return
  }
  createSaving.value = true
  try {
    const res = await createWatchlistTradePlanDraft({
      stock_code: stockCode,
      stock_name: row?.stock_name ? String(row.stock_name) : undefined,
      opportunity_id: opportunityId,
      scan_batch_key: scanBatchKey,
      actor: 'ui:watched-opportunities',
    })
    message.success(`已创建草稿计划 #${res.plan_id}`)
    await loadList()
    if (res.plan_id > 0) {
      goToTradePlan(res.plan_id)
    }
  } catch (e) {
    if (e instanceof WatchlistDraftError) {
      message.error(e.userMessage)
      if (e.code === WATCHLIST_DRAFT_ERROR_CODES.PLAN_EXISTS && e.planId) {
        goToTradePlan(e.planId)
      }
    } else {
      message.error(e?.message || String(e))
    }
  } finally {
    createSaving.value = false
  }
}

function goStockScreen() {
  router.push({ name: 'stockScreen' })
}

const columns = computed(() => [
  {
    title: '股票',
    key: 'stock',
    minWidth: 160,
    render(row) {
      const model = adaptOpportunity(row)
      if (!model?.klineKey && !model?.displayText) {
        return h(NText, null, { default: () => row.stock_code || '—' })
      }
      return h(StockLink, {
        model,
        onOpen: openStockKline,
      })
    },
  },
  {
    title: '来源',
    key: 'source',
    minWidth: 160,
    ellipsis: { tooltip: true },
    render(row) {
      return row.source || '选股机会'
    },
  },
  {
    title: '扫描批次',
    key: 'scan_batch_key',
    minWidth: 140,
    ellipsis: { tooltip: true },
    render(row) {
      return row.scan_batch_key || '—'
    },
  },
  {
    title: '加入时间',
    key: 'watch_time',
    width: 170,
    render(row) {
      return formatWatchTime(row.watch_time)
    },
  },
  {
    title: '状态',
    key: 'status',
    width: 200,
    render(row) {
      const tags = [
        h(NTag, { size: 'small', type: 'info', bordered: false }, { default: () => '观察中' }),
      ]
      const status = String(row.trade_plan_status || '').toUpperCase()
      const planId = Math.trunc(Number(row.trade_plan_id) || 0)
      if (hasExistingPlan(row)) {
        tags.push(
          h(
            NTag,
            {
              size: 'small',
              type: 'warning',
              bordered: false,
              style: planId > 0 ? 'cursor: pointer' : undefined,
              title:
                planId > 0
                  ? `查看交易计划 #${planId}（${status}）`
                  : `已生成交易计划（${status}）`,
              onClick: (e) => {
                e?.stopPropagation?.()
                goToTradePlan(row)
              },
            },
            { default: () => '查看计划' },
          ),
        )
      }
      return h(NFlex, { size: 6, align: 'center' }, { default: () => tags })
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 280,
    render(row) {
      const model = adaptOpportunity(row)
      const buttons = [
        h(
          NButton,
          {
            size: 'small',
            tertiary: true,
            type: 'info',
            disabled: !model?.klineKey,
            onClick: () => openStockKline(model),
          },
          { default: () => '查看详情' },
        ),
      ]
      if (hasExistingPlan(row)) {
        buttons.push(
          h(
            NButton,
            {
              size: 'small',
              tertiary: true,
              type: 'warning',
              disabled: !(Math.trunc(Number(row.trade_plan_id) || 0) > 0),
              onClick: () => goToTradePlan(row),
            },
            { default: () => '查看计划' },
          ),
        )
      } else {
        buttons.push(
          h(
            NButton,
            {
              size: 'small',
              type: 'primary',
              secondary: true,
              disabled: createSaving.value,
              loading: createSaving.value,
              onClick: () => confirmCreateTradePlan(row),
            },
            { default: () => '创建交易计划' },
          ),
        )
      }
      buttons.push(
        h(
          NButton,
          {
            size: 'small',
            secondary: true,
            disabled: cancelSaving.value,
            loading: cancelSaving.value,
            onClick: () => cancelWatchRow(row),
          },
          { default: () => '取消跟踪' },
        ),
      )
      return h(NFlex, { size: 4, wrap: true }, { default: () => buttons })
    },
  },
])

onMounted(() => {
  loadList()
})
</script>

<template>
  <div class="watched-opportunities">
    <n-space vertical :size="16">
      <div>
        <n-text strong style="font-size: 18px">我的跟踪机会</n-text>
        <div style="margin-top: 6px">
          <n-text depth="3">
            从「选股 / 机会」标记的机会观察列表（非自选）。当前跟踪：{{ watchingCount }}
          </n-text>
        </div>
      </div>

      <n-data-table
        v-if="rows.length || loading"
        :columns="columns"
        :data="rows"
        :loading="loading"
        :bordered="false"
        :single-line="false"
        size="small"
      />
      <n-empty
        v-else
        description="暂无跟踪中的机会。请在「选股 / 机会」对信号结果点击「跟踪」。跟踪不会加入「自选」。"
      >
        <template #extra>
          <n-button type="primary" secondary @click="goStockScreen">去选股 / 机会</n-button>
        </template>
      </n-empty>
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
.watched-opportunities {
  padding: 16px 20px;
  max-width: 1100px;
}
</style>
