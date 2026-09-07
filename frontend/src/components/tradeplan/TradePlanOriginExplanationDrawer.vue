<script setup>
/**
 * Phase16.28-B — Origin Explanation Drawer (同源 Origin 行，不请求 strategy-explanation).
 */
import { computed } from 'vue'
import { NDrawer, NDrawerContent, NSpace, NTag, NText } from 'naive-ui'
import ExplanationTable from '../explanation/ExplanationTable.vue'
import StockLink from '../StockLink.vue'
import { adaptTradePlanOrigin } from '../../utils/stockDisplayAdapters.js'
import { buildOriginExplanationRows } from '../../utils/explanationTable.js'
import { ORIGIN_SIGNAL_PRICE_FOOTER } from '../../utils/tradePlanOriginDisplay.js'

const props = defineProps({
  show: { type: Boolean, default: false },
  /** buildOriginItemDisplay result */
  entry: { type: Object, default: null },
})

const emit = defineEmits(['update:show', 'open-stock'])

const entry = computed(() => (props.entry && typeof props.entry === 'object' ? props.entry : null))

const stockModel = computed(() =>
  adaptTradePlanOrigin({
    stock_code: entry.value?.stockCode,
    stock_name: entry.value?.stockName,
  }),
)

const drawerTitle = computed(() => {
  const code = String(entry.value?.stockCode || '').trim()
  const name = String(entry.value?.stockName || '').trim()
  if (name && code && name !== code) return `${name}（${code}）· 入选解释`
  if (code) return `${code} · 入选解释`
  return '入选解释'
})

const tableRows = computed(() => buildOriginExplanationRows(entry.value || {}))
</script>

<template>
  <n-drawer
    :show="show"
    :width="520"
    placement="right"
    @update:show="(v) => emit('update:show', v)"
  >
    <n-drawer-content :title="drawerTitle" closable>
      <template v-if="entry">
        <n-space align="center" style="margin-bottom: 10px" :wrap="true">
          <n-text depth="3">股票</n-text>
          <StockLink
            v-if="stockModel.klineKey || stockModel.code"
            :model="stockModel"
            @open="(m) => emit('open-stock', m)"
          />
          <n-text v-else>{{ entry.stockCode || '—' }}</n-text>
          <n-tag size="small" :type="entry.sourceChipType || 'default'" :bordered="false">
            {{ entry.sourceChipLabel || '未知来源' }}
          </n-tag>
        </n-space>

        <n-text depth="3" style="display: block; margin-bottom: 10px; font-size: 12px">
          数据来自 TradePlan Origin（只读）；与主表同源，不改交易状态。
        </n-text>

        <ExplanationTable :rows="tableRows" :show-source="false" />

        <n-text depth="3" style="display: block; margin-top: 10px; font-size: 11px; line-height: 1.45">
          {{ ORIGIN_SIGNAL_PRICE_FOOTER }}
        </n-text>
      </template>
      <n-text v-else depth="3">暂无 Origin 行数据。</n-text>
    </n-drawer-content>
  </n-drawer>
</template>
