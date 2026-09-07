<script setup>
/**
 * PositionOriginDrawer — Phase17.5「为什么买入」解释层。
 * 复用 GET /api/portfolio/positions/{code}/provenance；可选读取 TradePlan.source_session。
 * 不新增交易、不改 provenance 模型。
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
  NTag,
  NText,
} from 'naive-ui'
import {
  PortfolioProvenanceNotFoundError,
  getPortfolioProvenance,
} from '../api/portfolioProvenance.ts'
import { getTradePlanById, TRADE_PLAN_CODE_OK } from '../api/tradePlans.ts'
import ExplanationHeader from './explanation/ExplanationHeader.vue'
import ExplanationEmpty from './explanation/ExplanationEmpty.vue'
import { buildExplanationDrawerTitleFromModel } from '../utils/explanationKit.js'
import { adaptProvenance } from '../utils/stockDisplayAdapters.js'
import {
  ORIGIN_SIGNAL_PRICE_FOOTER,
  buildPositionOriginCards,
  pickPrimaryOriginCard,
} from '../utils/positionOriginDisplay.js'
import {
  PROVENANCE_STATUS_LABEL,
  provenanceStatusTagType,
} from '../utils/portfolioProvenanceDisplay.js'

const props = defineProps({
  show: { type: Boolean, default: false },
  row: { type: Object, default: null },
  /** Optional pre-resolved { bucket, planId } from portfolio table chip enrich. */
  sourceHint: { type: Object, default: null },
})

const emit = defineEmits(['update:show', 'open-full-provenance'])

const router = useRouter()
const loading = ref(false)
const loadError = ref('')
const notFound = ref(false)
const provenance = ref(null)
/** @type {import('vue').Ref<Record<number, string>>} */
const sessionByPlan = ref({})

const stockDisplay = computed(() => adaptProvenance(provenance.value, props.row))
const drawerTitle = computed(() =>
  buildExplanationDrawerTitleFromModel(stockDisplay.value, '为什么买入'),
)

const statusLabel = computed(() => {
  const st = provenance.value?.provenanceStatus || 'partial'
  return PROVENANCE_STATUS_LABEL[st] || PROVENANCE_STATUS_LABEL.partial
})

const statusTagType = computed(() =>
  provenanceStatusTagType(provenance.value?.provenanceStatus || 'partial'),
)

const cards = computed(() =>
  buildPositionOriginCards(provenance.value, sessionByPlan.value),
)

const primary = computed(() => pickPrimaryOriginCard(cards.value))

const disclaimer = computed(
  () =>
    provenance.value?.disclaimer ||
    '只读解释：说明该持仓为何进入组合。不代表真实券商流水，不生成买卖单。',
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

function openFullProvenance() {
  emit('open-full-provenance', props.row)
}

async function loadSessionsForPlans(planIds) {
  const ids = [...new Set((planIds || []).filter((id) => id > 0))]
  const next = {}
  await Promise.all(
    ids.map(async (planId) => {
      try {
        const res = await getTradePlanById(planId)
        if (res.code !== TRADE_PLAN_CODE_OK || !res.plan) return
        next[planId] = String(res.plan.source_session || '').trim()
      } catch (_) {
        /* leave missing */
      }
    }),
  )
  sessionByPlan.value = next
}

async function load() {
  const code = String(props.row?.stockCode || '').trim()
  loadError.value = ''
  notFound.value = false
  provenance.value = null
  sessionByPlan.value = {}
  if (!code) {
    loadError.value = '股票代码无效'
    return
  }

  loading.value = true
  try {
    const view = await getPortfolioProvenance(code)
    provenance.value = view
    const planIds = [
      ...new Set([
        ...(view.trades || []).map((t) => Math.trunc(Number(t.planId) || 0)),
        ...(view.origins || []).map((o) => Math.trunc(Number(o.planId) || 0)),
        Math.trunc(Number(props.sourceHint?.planId) || 0),
      ]),
    ].filter((id) => id > 0)
    await loadSessionsForPlans(planIds)
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

watch(
  () => props.show,
  (visible) => {
    if (!visible) {
      provenance.value = null
      loadError.value = ''
      notFound.value = false
      sessionByPlan.value = {}
      return
    }
    load()
  },
)
</script>

<template>
  <n-drawer
    :show="show"
    :width="460"
    placement="right"
    @update:show="(v) => { if (!v) close() }"
  >
    <n-drawer-content :title="drawerTitle" closable>
      <n-spin :show="loading">
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
            <n-button size="small" @click="load">重试</n-button>
          </template>
        </n-alert>

        <template v-if="provenance && !notFound">
          <ExplanationHeader
            :status-label="statusLabel"
            :status-type="statusTagType"
            :disclaimer="disclaimer"
            context-text="买入来源解释 · 只读"
            :show-title="false"
          />

          <n-alert type="info" :bordered="false" style="margin: 12px 0">
            <template v-if="primary">
              {{ primary.whySummary }}
            </template>
            <template v-else>
              暂无归因计划，无法解释买入来源（可能为导入仓或未关联成交）。
            </template>
          </n-alert>

          <template v-if="cards.length">
            <div
              v-for="(card, idx) in cards"
              :key="`origin-${card.planId}-${idx}`"
              class="origin-card"
            >
              <n-space align="center" :size="8" style="margin-bottom: 8px">
                <n-text depth="2" style="font-size: 12px">来源</n-text>
                <n-tag size="small" :type="card.sourceChipType" :bordered="false">
                  {{ card.sourceChipLabel }}
                </n-tag>
                <n-button
                  v-if="card.planId > 0"
                  text
                  type="primary"
                  size="small"
                  @click="goToPlan(card.planId)"
                >
                  计划 #{{ card.planId }}
                </n-button>
              </n-space>

              <n-divider style="margin: 8px 0">信号</n-divider>
              <n-space vertical :size="4">
                <n-text depth="3">
                  SignalPrice：
                  <template v-if="!card.signalPriceMissing">{{ card.signalPrice }}</template>
                  <ExplanationEmpty v-else inline />
                </n-text>
                <n-text depth="3">
                  SignalTime：
                  <template v-if="!card.signalTimeMissing">{{ card.signalTime }}</template>
                  <ExplanationEmpty v-else inline />
                </n-text>
                <n-text v-if="!card.signalTagMissing" depth="3">
                  标签：{{ card.signalTag }}
                </n-text>
                <n-text v-if="!card.snapshotIdMissing" depth="3">
                  快照 #{{ card.snapshotId }}
                </n-text>
              </n-space>

              <n-divider style="margin: 8px 0">TradePlan</n-divider>
              <n-space vertical :size="4">
                <n-text depth="3">
                  Plan 来源：
                  <template v-if="!card.planSourceSessionMissing">{{ card.planSourceSession }}</template>
                  <ExplanationEmpty v-else inline />
                </n-text>
                <n-text depth="3">
                  策略：
                  <template v-if="!card.strategyMissing">{{ card.strategy }}</template>
                  <ExplanationEmpty v-else inline />
                </n-text>
                <n-text depth="3">
                  发现依据：
                  <template v-if="!card.buyReasonMissing">{{ card.buyReason }}</template>
                  <ExplanationEmpty v-else inline />
                </n-text>
                <n-text v-if="!card.selectionReasonMissing" depth="3">
                  入选说明：{{ card.selectionReason }}
                </n-text>
                <n-text v-if="card.trade" depth="3">
                  关联成交：{{ card.trade.fillPrice }} · {{ card.trade.filledAt }}
                </n-text>
              </n-space>
            </div>
          </template>
          <n-text v-else depth="3" style="display: block; margin-top: 8px">
            暂无计划来源投影。
          </n-text>

          <n-text depth="3" class="origin-price-footer">
            {{ ORIGIN_SIGNAL_PRICE_FOOTER }}
          </n-text>

          <n-space style="margin-top: 16px" :size="8">
            <n-button size="small" secondary @click="openFullProvenance">
              完整溯源
            </n-button>
          </n-space>
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
.origin-card {
  margin-bottom: 14px;
  padding-bottom: 12px;
  border-bottom: 1px dashed var(--n-border-color);
}
.origin-card:last-of-type {
  border-bottom: none;
}
.origin-price-footer {
  display: block;
  margin-top: 12px;
  font-size: 11px;
  line-height: 1.45;
}
</style>
