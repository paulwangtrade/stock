<script setup>
import { computed, defineAsyncComponent, onBeforeMount, onBeforeUnmount, ref, watch } from 'vue'
import { EventsOff, EventsOn } from '../../wailsjs/runtime'
import { useRoute, useRouter } from 'vue-router'

const ResearchReport = defineAsyncComponent(() => import('./researchReport.vue'))
const AiRecommendStocksList = defineAsyncComponent(() => import('./aiRecommendStocksList.vue'))
const PromptTemplateList = defineAsyncComponent(() => import('./promptTemplateList.vue'))
const CronTaskManager = defineAsyncComponent(() => import('./cron-task-manager.vue'))
const TradingRecordManager = defineAsyncComponent(() => import('./TradingRecordManager.vue'))
const StockChangesMonitor = defineAsyncComponent(() => import('./stockChangesMonitor.vue'))
const StockStrategyManager = defineAsyncComponent(() => import('./stockStrategyManager.vue'))
const SignalBacktestPanel = defineAsyncComponent(() => import('./SignalBacktestPanel.vue'))
const CandidatePool = defineAsyncComponent(() => import('./ResearchCandidatePool.vue'))
const PaperTradingPanel = defineAsyncComponent(() => import('./PaperTradingPanel.vue'))
const RealOrders = defineAsyncComponent(() => import('./RealOrders.vue'))

/** Phase15-A: hide from ordinary nav; keep panes for hash debug `#/research?name=模拟盘`. */
const LEGACY_PAPER_TAB = '模拟盘'
const LEGACY_REALSTUB_TAB = '模拟券商(RealStub)'

const nowTab = ref('AI分析报告')
const route = useRoute()
const router = useRouter()
const paperPrefill = ref(null)
const showLegacyPaperTab = computed(() => nowTab.value === LEGACY_PAPER_TAB)
const showLegacyRealStubTab = computed(() => nowTab.value === LEGACY_REALSTUB_TAB)

onBeforeMount(() => {
  const name = route.query.name
  if (name === '股票信息筛选') {
    router.replace({ name: 'stockScreen' })
    return
  }
  if (name) {
    // Phase13-A5：旧 Tab「候选池」→「研究候选」
    nowTab.value = name === '候选池' ? '研究候选' : name
  }
  if (route.query.code) {
    paperPrefill.value = {
      stockCode: String(route.query.code || ''),
      stockName: String(route.query.stockName || ''),
      price: Number(route.query.price) || 0,
    }
  }
})

onBeforeUnmount(() => {
  EventsOff('changeResearchTab')
  EventsOff('paperBuyPrefill')
})

watch(
  () => route.query.name,
  (name) => {
    if (!name) return
    nowTab.value = name === '候选池' ? '研究候选' : String(name)
  },
)

EventsOn('changeResearchTab', async (msg) => {
  if (msg?.name === '股票信息筛选') {
    router.push({ name: 'stockScreen' })
    return
  }
  const tabName = msg?.name === '候选池' ? '研究候选' : msg.name
  updateTab(tabName)
  if (msg?.code) {
    paperPrefill.value = {
      stockCode: String(msg.code || ''),
      stockName: String(msg.stockName || ''),
      price: Number(msg.price) || 0,
    }
  }
})

EventsOn('paperBuyPrefill', (payload) => {
  if (!payload) return
  paperPrefill.value = {
    stockCode: String(payload.stockCode || ''),
    stockName: String(payload.stockName || ''),
    price: Number(payload.price) || 0,
  }
  nowTab.value = '模拟盘'
})

function updateTab(name) {
  nowTab.value = name
}
</script>

<template>
  <n-card class="research-shell" :bordered="false">
    <n-tabs
      type="line"
      animated
      class="research-tabs"
      @update-value="updateTab"
      :value="nowTab"
      style="--wails-draggable: no-drag"
      :pane-wrapper-style="{ flex: 1, minHeight: 0, overflow: 'hidden' }"
      :pane-style="{ height: '100%', overflow: 'hidden', boxSizing: 'border-box' }"
    >
      <n-tab-pane name="AI分析报告" display-directive="if">
        <ResearchReport />
      </n-tab-pane>
      <n-tab-pane name="股票推荐记录" display-directive="if">
        <AiRecommendStocksList />
      </n-tab-pane>
      <n-tab-pane name="异动监控" display-directive="if">
        <StockChangesMonitor />
      </n-tab-pane>
      <n-tab-pane name="提示词模板" display-directive="if">
        <PromptTemplateList />
      </n-tab-pane>
      <n-tab-pane name="我的策略" display-directive="if">
        <StockStrategyManager />
      </n-tab-pane>
      <n-tab-pane name="信号回测" display-directive="if">
        <SignalBacktestPanel />
      </n-tab-pane>
      <n-tab-pane name="研究候选" display-directive="if">
        <CandidatePool />
      </n-tab-pane>
      <n-tab-pane v-if="showLegacyPaperTab" :name="LEGACY_PAPER_TAB" display-directive="if">
        <PaperTradingPanel :prefill="paperPrefill" />
      </n-tab-pane>
      <n-tab-pane v-if="showLegacyRealStubTab" :name="LEGACY_REALSTUB_TAB" display-directive="if">
        <RealOrders />
      </n-tab-pane>
      <n-tab-pane name="定时任务" display-directive="if">
        <CronTaskManager />
      </n-tab-pane>
      <n-tab-pane name="交易日志" display-directive="if">
        <TradingRecordManager />
      </n-tab-pane>
    </n-tabs>
  </n-card>
</template>

<style scoped>
.research-shell {
  /* Phase16.21-C: 与视口对齐，让研究候选表可占满剩余高度 */
  height: 100%;
  min-height: calc(100vh - 120px);
  max-height: calc(100vh - 120px);
  overflow: hidden;
}
.research-shell :deep(.n-card__content) {
  height: 100%;
  padding: 4px 8px 8px;
  box-sizing: border-box;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.research-tabs {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.research-tabs :deep(.n-tabs-nav) {
  flex-shrink: 0;
}
.research-tabs :deep(.n-tabs-pane-wrapper) {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.research-tabs :deep(.n-tab-pane) {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
</style>
