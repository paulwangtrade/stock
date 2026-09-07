<script setup>
/**
 * Portfolio holding provenance drawer (Phase15-A2 + Phase16-D4 Outcome Tab).
 * Read-only: provenance + lazy outcomes tab.
 */
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  NAlert,
  NButton,
  NDivider,
  NDrawer,
  NDrawerContent,
  NSpace,
  NSpin,
  NTabs,
  NTabPane,
  NText,
} from 'naive-ui'
import {
  PortfolioProvenanceNotFoundError,
  getPortfolioProvenance,
} from '../api/portfolioProvenance.ts'
import {
  OpportunityOutcomeNotFoundError,
  fetchOpportunityOutcome,
} from '../api/opportunityOutcome.ts'
import ExplanationEmpty from './explanation/ExplanationEmpty.vue'
import ExplanationTable from './explanation/ExplanationTable.vue'
import ExplanationHeader from './explanation/ExplanationHeader.vue'
import OutcomeTabContent from './OutcomeTabContent.vue'
import { buildExplanationDrawerTitleFromModel, buildProvenanceOriginFields } from '../utils/explanationKit.js'
import { fieldsToExplanationTableRows } from '../utils/explanationTable.js'
import { adaptProvenance } from '../utils/stockDisplayAdapters.js'
import { defaultLegIndex } from '../utils/opportunityOutcomeDisplay.js'
import {
  ORIGIN_SIGNAL_PRICE_FOOTER,
  ORIGIN_SIGNAL_PRICE_TOOLTIP,
  PROVENANCE_STATUS_LABEL,
  buildProvenanceSections,
  formatProvenanceMoney,
  formatProvenanceQty,
  orphanOrigins,
  provenanceStatusTagType,
} from '../utils/portfolioProvenanceDisplay.js'

const props = defineProps({
  show: { type: Boolean, default: false },
  row: { type: Object, default: null },
})

const emit = defineEmits(['update:show'])

const router = useRouter()
const loading = ref(false)
const loadError = ref('')
const notFound = ref(false)
const provenance = ref(null)

const activeTab = ref('provenance')
const outcomeLoading = ref(false)
const outcomeError = ref('')
const outcomeNotFound = ref(false)
const outcomeItems = ref([])
const outcomeLoaded = ref(false)
const outcomeGeneratedAt = ref('')
const selectedLegIndex = ref(0)

const stockDisplay = computed(() => adaptProvenance(provenance.value, props.row))

const stockCode = computed(() => stockDisplay.value.code)
const stockName = computed(() => stockDisplay.value.name)

const drawerTitle = computed(() =>
  buildExplanationDrawerTitleFromModel(stockDisplay.value, '解释'),
)

const spinShow = computed(
  () => loading.value || (activeTab.value === 'outcome' && outcomeLoading.value),
)

const positionDisplay = computed(() => {
  const pos = provenance.value?.position
  const row = props.row
  const m = stockDisplay.value
  return {
    stockCode: m.code || '—',
    stockName: m.name || '—',
    displayText: m.displayText || '—',
    quantity: formatProvenanceQty(pos?.quantity ?? row?.totalQty),
    avgCost: formatProvenanceMoney(pos?.avgCost ?? row?.avgCost),
  }
})

const statusLabel = computed(() => {
  const st = provenance.value?.provenanceStatus || 'partial'
  return PROVENANCE_STATUS_LABEL[st] || PROVENANCE_STATUS_LABEL.partial
})

const statusTagType = computed(() =>
  provenanceStatusTagType(provenance.value?.provenanceStatus || 'partial'),
)

const sections = computed(() =>
  buildProvenanceSections(provenance.value?.trades, provenance.value?.origins),
)

const extraOrigins = computed(() =>
  orphanOrigins(provenance.value?.trades, provenance.value?.origins),
)

const hasTrades = computed(() => (provenance.value?.trades || []).length > 0)

const disclaimer = computed(
  () =>
    provenance.value?.disclaimer ||
    'Paper 模拟持仓溯源（只读）。不代表真实券商流水。',
)

function provenanceOriginRows(origin) {
  return fieldsToExplanationTableRows(
    buildProvenanceOriginFields(origin, {
      signalPriceTooltip: ORIGIN_SIGNAL_PRICE_TOOLTIP,
    }),
    { group: 'provenance' },
  )
}

function resetOutcomeState() {
  outcomeLoading.value = false
  outcomeError.value = ''
  outcomeNotFound.value = false
  outcomeItems.value = []
  outcomeLoaded.value = false
  outcomeGeneratedAt.value = ''
  selectedLegIndex.value = 0
}

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

async function loadProvenance() {
  const code = String(props.row?.stockCode || '').trim()
  loadError.value = ''
  notFound.value = false
  provenance.value = null
  activeTab.value = 'provenance'
  resetOutcomeState()
  if (!code) {
    loadError.value = '股票代码无效'
    return
  }

  loading.value = true
  try {
    provenance.value = await getPortfolioProvenance(code)
  } catch (e) {
    if (e instanceof PortfolioProvenanceNotFoundError) {
      notFound.value = true
      loadError.value = e.message || '未找到该持仓'
      return
    }
    loadError.value = e?.message || String(e)
  } finally {
    loading.value = false
  }
}

async function loadOutcomes() {
  const code = stockCode.value || String(props.row?.stockCode || '').trim()
  outcomeError.value = ''
  outcomeNotFound.value = false
  if (!code) {
    outcomeError.value = '股票代码无效'
    return
  }

  outcomeLoading.value = true
  try {
    const result = await fetchOpportunityOutcome(code)
    outcomeItems.value = result.items || []
    outcomeGeneratedAt.value = result.generated_at || ''
    outcomeLoaded.value = true
    selectedLegIndex.value = defaultLegIndex(outcomeItems.value, provenance.value?.trades)
  } catch (e) {
    if (e instanceof OpportunityOutcomeNotFoundError) {
      outcomeNotFound.value = true
      outcomeError.value = e.message || '未找到该股票的交易结果'
      outcomeItems.value = []
      outcomeLoaded.value = true
      return
    }
    outcomeError.value = e?.message || String(e)
  } finally {
    outcomeLoading.value = false
  }
}

function onTabChange(name) {
  activeTab.value = name
  if (name === 'outcome' && !outcomeLoaded.value && provenance.value && !notFound.value) {
    loadOutcomes()
  }
}

watch(
  () => props.show,
  (visible) => {
    if (!visible) {
      provenance.value = null
      loadError.value = ''
      notFound.value = false
      activeTab.value = 'provenance'
      resetOutcomeState()
      return
    }
    loadProvenance()
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
      <n-spin :show="spinShow">
        <n-alert v-if="notFound" type="warning" :bordered="false" style="margin-bottom: 12px">
          {{ loadError || '未找到该持仓' }}
        </n-alert>
        <n-alert
          v-else-if="loadError"
          type="error"
          :bordered="false"
          style="margin-bottom: 12px"
        >
          {{ loadError }}
          <template #footer>
            <n-button size="small" @click="loadProvenance">重试</n-button>
          </template>
        </n-alert>

        <template v-if="provenance && !notFound">
          <n-tabs
            :value="activeTab"
            type="line"
            animated
            @update:value="onTabChange"
          >
            <n-tab-pane name="provenance" tab="来源">
              <ExplanationHeader
                :status-label="statusLabel"
                :status-type="statusTagType"
                :disclaimer="disclaimer"
                :show-title="false"
              />

              <n-text strong style="display: block; margin-bottom: 6px">一、当前持仓</n-text>
              <n-space vertical :size="4" style="margin-bottom: 14px">
                <n-text depth="3">股票：{{ positionDisplay.displayText }}</n-text>
                <n-text depth="3">数量：{{ positionDisplay.quantity }}</n-text>
                <n-text depth="3">成本：{{ positionDisplay.avgCost }}</n-text>
              </n-space>

              <n-divider style="margin: 12px 0" />

              <n-text strong style="display: block; margin-bottom: 6px">二、成交来源</n-text>
              <template v-if="hasTrades">
                <div
                  v-for="(sec, idx) in sections"
                  :key="`trade-${sec.planId}-${sec.trade.fillId || idx}`"
                  class="provenance-block"
                >
                  <n-space align="center" :size="8" style="margin-bottom: 4px">
                    <n-text depth="3">plan_id：</n-text>
                    <n-button
                      v-if="sec.planId > 0"
                      text
                      type="primary"
                      tag="a"
                      @click="goToPlan(sec.planId)"
                    >
                      #{{ sec.planId }}
                    </n-button>
                    <n-text v-else depth="3">—</n-text>
                  </n-space>
                  <n-text depth="3">成交价格：{{ sec.trade.fillPrice }}</n-text>
                  <n-text depth="3">成交时间：{{ sec.trade.filledAt }}</n-text>
                  <n-text v-if="sec.trade.fillVolume !== '—'" depth="3">
                    成交量：{{ sec.trade.fillVolume }}
                  </n-text>
                </div>
              </template>
              <n-text v-else depth="3" style="display: block; margin-bottom: 14px">
                暂无归因成交记录（持仓可能来自历史导入或未关联 fill）。
              </n-text>

              <n-divider style="margin: 12px 0" />

              <n-text strong style="display: block; margin-bottom: 6px">三、交易来源</n-text>
              <template v-if="sections.length">
                <div
                  v-for="(sec, idx) in sections"
                  :key="`origin-${sec.planId}-${idx}`"
                  class="provenance-block"
                >
                  <n-space align="center" :size="8" style="margin-bottom: 6px">
                    <n-text depth="2" style="font-size: 12px">计划</n-text>
                    <n-button
                      v-if="sec.planId > 0"
                      text
                      type="primary"
                      size="small"
                      @click="goToPlan(sec.planId)"
                    >
                      #{{ sec.planId }}
                    </n-button>
                  </n-space>
                  <ExplanationTable :rows="provenanceOriginRows(sec.origin)" />
                </div>
              </template>
              <template v-else-if="extraOrigins.length">
                <div
                  v-for="(orig, idx) in extraOrigins"
                  :key="`orphan-${orig.planId}-${idx}`"
                  class="provenance-block"
                >
                  <n-text depth="3">
                    plan_id：{{ orig.planId || '' }}
                    <ExplanationEmpty v-if="!orig.planId" inline />
                  </n-text>
                  <n-text depth="3">来源策略：{{ orig.strategy }}</n-text>
                </div>
              </template>
              <n-text v-else depth="3">暂无计划来源投影。</n-text>

              <n-text depth="3" class="origin-price-footer">
                {{ ORIGIN_SIGNAL_PRICE_FOOTER }}
              </n-text>
            </n-tab-pane>

            <n-tab-pane name="outcome" tab="结果">
              <OutcomeTabContent
                :items="outcomeItems"
                :selected-index="selectedLegIndex"
                :generated-at="outcomeGeneratedAt"
                :loading="outcomeLoading"
                :error="outcomeError"
                :not-found="outcomeNotFound"
                @update:selected-index="(v) => { selectedLegIndex = v }"
                @retry="loadOutcomes"
                @go-to-plan="goToPlan"
              />
            </n-tab-pane>
          </n-tabs>
        </template>
      </n-spin>

      <template v-if="provenance?.dataSourceNote" #footer>
        <n-text depth="3" style="font-size: 12px">
          {{ provenance.dataSourceNote }}
        </n-text>
      </template>
    </n-drawer-content>
  </n-drawer>
</template>

<style scoped>
.provenance-block {
  margin-bottom: 12px;
  padding-bottom: 10px;
  border-bottom: 1px dashed var(--n-border-color);
}
.provenance-block:last-child {
  border-bottom: none;
}
.origin-price-footer {
  display: block;
  margin-top: 12px;
  font-size: 11px;
  line-height: 1.45;
}
</style>
