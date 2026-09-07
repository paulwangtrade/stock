<script setup>
/**
 * OpportunityProjection read-only drawer (Phase16-C2.1).
 * GET /api/opportunities/projections?stock_code=…
 */
import { computed, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  NAlert,
  NButton,
  NDivider,
  NDrawer,
  NDrawerContent,
  NSpace,
  NSpin,
  NTag,
  NText,
} from 'naive-ui'
import {
  OpportunityProjectionNotFoundError,
  fetchOpportunityProjection,
} from '../api/opportunityProjection.ts'
import ExplanationTable from './explanation/ExplanationTable.vue'
import ExplanationHeader from './explanation/ExplanationHeader.vue'
import ExplanationPipeline from './explanation/ExplanationPipeline.vue'
import StockKlineModal from './StockKlineModal.vue'
import StockLink from './StockLink.vue'
import { buildExplanationDrawerTitleFromModel } from '../utils/explanationKit.js'
import { adaptOpportunity, applyStockClickAction } from '../utils/stockDisplayAdapters.js'
import { fieldsToExplanationTableRows } from '../utils/explanationTable.js'
import {
  PROJECTION_DISCLAIMER,
  buildOpportunityProjectionView,
  projectionFieldLabel,
  projectionQualityLabel,
  projectionQualityTagType,
} from '../utils/opportunityProjectionDisplay.js'
import { opportunitySourceChipMeta } from '../utils/opportunityExplanationColumns.js'
import { priceSourceLabel, PRICE_KIND } from '../utils/priceDisplay.js'

/** Phase16.19-B2: empty / missing snapshot copy (never blank or undefined). */
const SNAPSHOT_MISSING_MSG = '暂无历史解释'
const SIGNAL_MISSING_MSG = '暂无信号数据'

const props = defineProps({
  show: { type: Boolean, default: false },
  row: { type: Object, default: null },
})

const emit = defineEmits(['update:show'])

const router = useRouter()
const loading = ref(false)
const loadError = ref('')
const notFound = ref(false)
const projection = ref(null)

const stockDisplay = computed(() => adaptOpportunity(props.row, projection.value))

const stockCode = computed(() => stockDisplay.value.code)
const stockName = computed(() => stockDisplay.value.name)

const drawerTitle = computed(() =>
  buildExplanationDrawerTitleFromModel(stockDisplay.value, '解释'),
)

const klineModal = reactive({
  visible: false,
  title: '',
  chartCode: '',
  stockName: '',
})

function openStockKline(model) {
  applyStockClickAction(model || stockDisplay.value, klineModal)
}

/** Phase16-G3 + Phase16.28 unified field labels. */
const EXPLANATION_FIELD_LABEL = {
  trigger_reason: '发现依据 / 入选说明',
  signal_price: priceSourceLabel(PRICE_KIND.signal),
  signal_time: '信号时间',
  signal_tag: '信号类型',
  strategy_source: '策略来源标识',
  strategy_name: '策略名称',
  pool_source: '发现来源',
}

function explanationFieldLabel(key) {
  return EXPLANATION_FIELD_LABEL[key] || projectionFieldLabel(key)
}

function sectionTableRows(section) {
  return fieldsToExplanationTableRows(section?.fields || [], {
    labelFn: explanationFieldLabel,
    group: section?.id || '',
  })
}

const view = computed(() => buildOpportunityProjectionView(projection.value))

const sourceChip = computed(() => opportunitySourceChipMeta(projection.value))

/** True when there is no projection payload to explain (notFound / empty). */
const snapshotMissing = computed(() => {
  if (notFound.value) return true
  const p = projection.value
  if (!p) return false
  // Signal present ⇒ discovery data is on-screen; do not banner「暂无历史解释」
  // merely because snapshot_id is absent (ProjectOne may still fill signal fields).
  if (p.signal?.present) return false
  return true
})

const signalMissing = computed(() => {
  const p = projection.value
  if (!p) return false
  return !p.signal?.present
})

const snapshotMissingMessage = computed(() => {
  if (notFound.value) return loadError.value || SNAPSHOT_MISSING_MSG
  if (signalMissing.value) return SIGNAL_MISSING_MSG
  if (snapshotMissing.value) return SNAPSHOT_MISSING_MSG
  return ''
})

const qualityTagType = computed(() => projectionQualityTagType(view.value?.quality || 'partial'))
const qualityLabel = computed(() => projectionQualityLabel(view.value?.quality || 'partial'))

const contextText = computed(() =>
  view.value?.tradeDate ? `交易日 ${view.value.tradeDate}` : '',
)

function close() {
  emit('update:show', false)
}

function goToPlan(planId) {
  const id = Math.trunc(Number(planId) || 0)
  if (id <= 0) return
  close()
  router.push({
    name: 'tradePlanUpcoming',
    query: { plan_id: String(id) },
  })
}

async function loadProjection() {
  const code = String(props.row?.stockCode || '').trim()
  loadError.value = ''
  notFound.value = false
  projection.value = null
  if (!code) {
    loadError.value = '股票代码无效'
    return
  }

  loading.value = true
  try {
    const q = {}
    const tradeDate = String(props.row?.tradeDate || '').trim()
    if (tradeDate) q.tradeDate = tradeDate
    projection.value = await fetchOpportunityProjection(code, q)
  } catch (e) {
    if (e instanceof OpportunityProjectionNotFoundError) {
      notFound.value = true
      loadError.value = SNAPSHOT_MISSING_MSG
      return
    }
    loadError.value = e?.message || String(e) || SNAPSHOT_MISSING_MSG
  } finally {
    loading.value = false
  }
}

watch(
  () => props.show,
  (visible) => {
    if (!visible) {
      projection.value = null
      loadError.value = ''
      notFound.value = false
      return
    }
    loadProjection()
  },
)
</script>

<template>
  <n-drawer
    :show="show"
    :width="520"
    placement="right"
    @update:show="(v) => { if (!v) close() }"
  >
    <n-drawer-content :title="drawerTitle" closable>
      <n-spin :show="loading">
        <n-space v-if="stockDisplay.displayText" align="center" style="margin-bottom: 10px" :wrap="true">
          <n-text depth="3">股票</n-text>
          <StockLink
            v-if="stockDisplay.klineKey"
            :model="stockDisplay"
            @open="openStockKline"
          />
          <n-text v-else>{{ stockDisplay.displayText }}</n-text>
          <n-tag size="small" :type="sourceChip.type" :bordered="false">
            {{ sourceChip.label }}
          </n-tag>
        </n-space>

        <n-text depth="3" style="display: block; margin-bottom: 10px; font-size: 12px">
          数据链路：SignalSnapshot → CandidatePool → TradePlan（只读解释，不改交易状态）
        </n-text>

        <n-alert v-if="notFound || (snapshotMissing && !loading && !loadError)" type="warning" :bordered="false" style="margin-bottom: 12px">
          {{ snapshotMissingMessage || '暂无历史解释' }}
        </n-alert>
        <n-alert
          v-else-if="loadError && !notFound"
          type="error"
          :bordered="false"
          style="margin-bottom: 12px"
        >
          {{ loadError }}
          <template #footer>
            <n-button size="small" @click="loadProjection">重试</n-button>
          </template>
        </n-alert>

        <template v-if="view && !notFound">
          <ExplanationHeader
            :status-label="qualityLabel"
            :status-type="qualityTagType"
            :context-text="contextText"
            :disclaimer="PROJECTION_DISCLAIMER"
            :show-title="false"
          />

          <ExplanationPipeline :steps="view.pipeline" />

          <n-divider style="margin: 12px 0" />

          <template v-for="section in view.sections" :key="section.id">
            <div v-if="!section.optional || section.present" style="margin-bottom: 14px">
              <n-space align="center" justify="space-between" style="margin-bottom: 4px">
                <n-text strong>{{ section.title }}</n-text>
                <n-text depth="3" style="font-size: 12px">{{ section.sourceBanner }}</n-text>
              </n-space>

              <ExplanationTable
                :rows="sectionTableRows(section)"
                :show-source="true"
              />

              <n-button
                v-if="section.id === 'trade_plan' && section.present && projection?.trade_plan?.plan_id"
                text
                type="primary"
                size="small"
                style="margin-top: 6px"
                @click="goToPlan(projection.trade_plan.plan_id)"
              >
                查看 TradePlan #{{ projection.trade_plan.plan_id }}
              </n-button>

              <n-divider style="margin: 12px 0" />
            </div>
          </template>
        </template>
      </n-spin>
    </n-drawer-content>
  </n-drawer>

  <stock-kline-modal
    v-model:show="klineModal.visible"
    :title="klineModal.title"
    :chart-key="'opportunity-drawer-kline-' + klineModal.chartCode"
    :code="klineModal.chartCode"
    :stock-name="klineModal.stockName"
  />
</template>
