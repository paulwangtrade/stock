<script setup>
/**
 * Investment Narrative read-only panel (Phase14-G0.5b).
 * Five sections + E1 price labels; no speculative copy.
 */
import { computed, ref, watch } from 'vue'
import { NAlert, NDivider, NSpin, NText } from 'naive-ui'
import { fetchInvestmentNarrative } from '../api/investmentNarrative.ts'
import { fetchOpportunityList } from '../api/opportunities.ts'
import { OPPORTUNITY_PRICE_FOOTER } from '../utils/opportunityListMetrics.js'
import {
  CURRENT_PRICE_LABEL,
  SIGNAL_PRICE_LABEL,
  buildNarrativeDisplayModel,
  narrativeField,
} from '../utils/investmentNarrativeDisplay.js'

const props = defineProps({
  stockCode: { type: String, default: '' },
  accountId: { type: Number, default: 0 },
  /** When true, hide outer title (embedded in drawer). */
  embedded: { type: Boolean, default: false },
})

const loading = ref(false)
const loadError = ref('')
const narrative = ref(null)
const latestUserAction = ref(null)

const display = computed(() =>
  buildNarrativeDisplayModel(narrative.value, latestUserAction.value),
)

const stockLabel = computed(() => narrativeField(props.stockCode))

async function load() {
  const code = String(props.stockCode || '').trim()
  loadError.value = ''
  narrative.value = null
  latestUserAction.value = null
  if (!code) return

  loading.value = true
  try {
    const [narr, pool] = await Promise.all([
      fetchInvestmentNarrative(code, props.accountId || undefined),
      fetchOpportunityList({ includeUserAction: true, accountId: props.accountId || undefined }).catch(
        () => ({ entries: [] }),
      ),
    ])
    narrative.value = narr
    const norm = code.toLowerCase()
    const entry = (pool.entries || []).find(
      (e) => String(e.stock_code || '').toLowerCase() === norm,
    )
    latestUserAction.value = entry?.latest_user_action || null
  } catch (e) {
    loadError.value = e?.message || String(e)
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.stockCode, props.accountId],
  () => {
    load()
  },
  { immediate: true },
)
</script>

<template>
  <div class="investment-narrative-panel">
    <n-text v-if="!embedded" strong style="display: block; margin-bottom: 8px">
      投资叙事 · {{ stockLabel }}
    </n-text>
    <n-alert type="info" :bordered="false" style="margin-bottom: 10px">
      只读投影：汇总信号、计划、持仓与复评记录，不构成买卖建议。
    </n-alert>

    <n-spin :show="loading">
      <n-text v-if="loadError" type="error" style="display: block; margin-bottom: 8px">
        {{ loadError }}
      </n-text>

      <section class="narrative-section">
        <n-text strong>① 为什么发现</n-text>
        <n-text depth="3" class="narrative-line">信号时间：{{ display.discovery.signalTime }}</n-text>
        <n-text depth="3" class="narrative-line">
          {{ SIGNAL_PRICE_LABEL }}：{{ display.discovery.signalPrice }}
        </n-text>
        <n-text depth="3" class="narrative-line">信号标签：{{ display.discovery.signalTag }}</n-text>
      </section>

      <n-divider style="margin: 10px 0" />

      <section class="narrative-section">
        <n-text strong>② 为什么进入机会</n-text>
        <n-text depth="3" class="narrative-line">用户行为：{{ display.opportunity.actionLabel }}</n-text>
        <n-text depth="3" class="narrative-line">跟踪状态：{{ display.opportunity.watchState }}</n-text>
      </section>

      <n-divider style="margin: 10px 0" />

      <section class="narrative-section">
        <n-text strong>③ 为什么产生计划</n-text>
        <n-text depth="3" class="narrative-line">策略：{{ display.planOrigin.strategy }}</n-text>
        <n-text depth="3" class="narrative-line">计划原因：{{ display.planOrigin.reason }}</n-text>
        <n-text depth="3" class="narrative-line">计划 ID：{{ display.planOrigin.planId }}</n-text>
      </section>

      <n-divider style="margin: 10px 0" />

      <section class="narrative-section">
        <n-text strong>④ 为什么持有</n-text>
        <n-text depth="3" class="narrative-line">持仓数量：{{ display.holding.quantity }}</n-text>
        <n-text depth="3" class="narrative-line">成本价：{{ display.holding.avgCost }}</n-text>
      </section>

      <n-divider style="margin: 10px 0" />

      <section class="narrative-section">
        <n-text strong>⑤ 为什么需要复评</n-text>
        <n-text depth="3" class="narrative-line">复评状态：{{ display.exitReview.status }}</n-text>
        <n-text depth="3" class="narrative-line">最近结论：{{ display.exitReview.outcome }}</n-text>
        <n-text
          v-if="display.exitReview.outcomeReason !== '暂无记录'"
          depth="3"
          class="narrative-line"
        >
          结论备注：{{ display.exitReview.outcomeReason }}
        </n-text>
      </section>

      <n-divider style="margin: 10px 0" />

      <section class="narrative-section">
        <n-text strong>价格参考（Phase14-E1）</n-text>
        <n-text depth="3" class="narrative-line">
          {{ SIGNAL_PRICE_LABEL }}：{{ display.priceStory.signalPrice }}
        </n-text>
        <n-text depth="3" class="narrative-line">
          {{ CURRENT_PRICE_LABEL }}：{{ display.priceStory.currentPrice }}
        </n-text>
        <n-text depth="3" class="narrative-line">
          信号以来涨跌：{{ display.priceStory.vsSignalPct }}
        </n-text>
        <n-text depth="3" style="display: block; margin-top: 8px; font-size: 12px">
          {{ OPPORTUNITY_PRICE_FOOTER }}
        </n-text>
      </section>
    </n-spin>
  </div>
</template>

<style scoped>
.investment-narrative-panel {
  padding: 4px 0 8px;
}
.narrative-section {
  margin-bottom: 4px;
}
.narrative-line {
  display: block;
  margin-top: 4px;
  font-size: 13px;
}
</style>
