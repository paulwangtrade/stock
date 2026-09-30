<script setup>
/**
 * Shared ExitWatch list. 观察≠卖出指令.
 * external_mirror never offers a sell draft.
 */
import { computed } from 'vue'
import { NAlert, NButton, NEmpty, NTag, NText } from 'naive-ui'
import {
  EXIT_WATCH_DISCLAIMER,
  canOfferExitWatchSell,
  exitWatchTagType,
  readExitWatchItem,
} from '../utils/exitWatchDisplay.js'

const props = defineProps({
  items: { type: Array, default: () => [] },
  phase: { type: String, default: 'ready' },
  source: { type: String, required: true },
  policyRef: { type: String, default: '' },
  asOf: { type: String, default: '' },
  sourceMessage: { type: String, default: '' },
})

const emit = defineEmits(['sell-intent'])

const rows = computed(() => {
  const list = Array.isArray(props.items) ? props.items : []
  return list.map((raw) => readExitWatchItem(raw, props.phase === 'error' ? 'ready' : props.phase))
})

const failed = computed(() => props.phase === 'error' || !!props.sourceMessage)
</script>

<template>
  <section class="exit-watch">
    <n-alert type="warning" :bordered="false" style="margin-bottom: 8px">
      {{ EXIT_WATCH_DISCLAIMER }}。这里只列出观察状态，不会自动卖出。
    </n-alert>
    <n-text depth="3" style="display: block; margin-bottom: 8px; font-size: 12px">
      <span v-if="policyRef">策略 {{ policyRef }}</span>
      <span v-if="asOf"> · 截至 {{ asOf }}</span>
    </n-text>
    <n-alert v-if="failed" type="default" :bordered="false" style="margin-bottom: 8px">
      {{ sourceMessage || '退出观察读取失败，已失败关闭为数据不足' }}。{{ EXIT_WATCH_DISCLAIMER }}
    </n-alert>
    <n-empty v-if="phase === 'loading'" description="退出观察计算中" />
    <n-empty v-else-if="!rows.length && !failed" description="暂无退出观察" />
    <ul v-else-if="rows.length" class="exit-watch-list">
      <li v-for="row in rows" :key="row.dedupKey || row.positionId || row.stockCode" class="exit-watch-row">
        <div class="exit-watch-main">
          <n-tag size="small" :bordered="false" :type="row.pending ? 'default' : exitWatchTagType(row.class)">
            {{ row.label }}
          </n-tag>
          <span class="code">{{ row.stockCode || '—' }}</span>
          <span>{{ row.stockName || '—' }}</span>
          <n-text depth="3">{{ row.reasonText || '—' }}</n-text>
        </div>
        <n-text depth="3" style="display: block; font-size: 12px">{{ row.summary }}</n-text>
        <n-text v-if="row.dedupKey" depth="3" style="display: block; font-size: 11px">
          {{ row.dedupKey }}<span v-if="row.policyRef"> · {{ row.policyRef }}</span>
        </n-text>
        <n-button
          v-if="canOfferExitWatchSell(row)"
          size="tiny"
          quaternary
          type="warning"
          style="margin-top: 4px"
          @click="emit('sell-intent', row)"
        >
          {{ row.manualSellDraftRef ? '人工确认后打开模拟卖出草稿' : '人工确认后填写模拟卖出' }}
        </n-button>
        <n-text v-else-if="source === 'external_mirror'" depth="3" style="display: block; font-size: 12px">
          镜像观察不可生成卖出意图
        </n-text>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.exit-watch { margin: 8px 0 14px; }
.exit-watch-list { list-style: none; margin: 0; padding: 0; }
.exit-watch-row {
  padding: 8px 0;
  border-bottom: 1px solid rgba(128, 128, 128, 0.16);
}
.exit-watch-main { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; margin-bottom: 2px; }
.code { font-family: Consolas, monospace; }
</style>
