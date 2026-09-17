<script setup>
import { defineAsyncComponent, onBeforeMount, onBeforeUnmount, ref } from 'vue'
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
const PaperTradingPanel = defineAsyncComponent(() => import('./PaperTradingPanel.vue'))
const ResearchExperimentFoundationPanel = defineAsyncComponent(() => import('./ResearchExperimentFoundationPanel.vue'))

const nowTab = ref('AI分析报告')
const route = useRoute()
const router = useRouter()

onBeforeMount(() => {
  const name = route.query.name
  if (name === '股票信息筛选') {
    router.replace({ name: 'stockScreen' })
    return
  }
  if (name) {
    nowTab.value = name
  }
})

onBeforeUnmount(() => {
  EventsOff('changeResearchTab')
})

EventsOn('changeResearchTab', async (msg) => {
  if (msg?.name === '股票信息筛选') {
    router.push({ name: 'stockScreen' })
    return
  }
  updateTab(msg.name)
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
      <n-tab-pane name="模拟盘" display-directive="if">
        <PaperTradingPanel />
      </n-tab-pane>
      <n-tab-pane name="研究实验" display-directive="if">
        <ResearchExperimentFoundationPanel />
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
  /* 填满主滚动区内容高度（底栏留白由 App padding-bottom 负责，此处不再重复扣减） */
  height: 100%;
  min-height: calc(92vh - 52px);
  max-height: calc(92vh - 52px);
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
