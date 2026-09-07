<script setup>
/**
 * Phase16 Kline Modal contract:
 * - Unique stock identity prop: `code` (required for embedded chart).
 * - Other chart options (stockName, strategySignals, costPrice, …) pass through attrs.
 * - Do not use chart-code / stock_code as Modal chart identity.
 */
import { computed, ref, useAttrs, watch } from 'vue'
import { ExpandOutline, ContractOutline } from '@vicons/ionicons5'
import { NAlert, NButton, NIcon, NModal } from 'naive-ui'
import StockLightweightKlineChart from './StockLightweightKlineChart.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps({
  show: { type: Boolean, default: false },
  title: { type: String, default: '' },
  /**
   * Unique chart identity — East-money / display code for StockLightweightKlineChart.
   * Callers must bind `:code="..."`. Do not pass chart-code.
   */
  code: { type: String, default: '' },
  chartKey: { type: String, default: 'kline' },
  chartHeight: { type: Number, default: 500 },
  /** 非最大化时的弹窗宽度 */
  modalWidth: { type: String, default: 'min(1100px, 96vw)' },
  /** 最大化时图表高度 = innerHeight - maxHeightOffset */
  maxHeightOffset: { type: Number, default: 200 },
  /** false 时不渲染内置 K 线（使用 default 插槽自定义内容） */
  embedChart: { type: Boolean, default: true },
})

const emit = defineEmits(['update:show', 'after-leave'])

const attrs = useAttrs()
const maximized = ref(false)

const resolvedCode = computed(() => String(props.code || '').trim())
const hasCode = computed(() => !!resolvedCode.value)

const visible = computed({
  get: () => props.show,
  set: (v) => emit('update:show', v),
})

watch(
  () => props.show,
  (v) => {
    if (!v) maximized.value = false
  },
)

const modalStyle = computed(() => {
  if (maximized.value) {
    return { width: '100vw', maxWidth: '100vw', height: '100vh', margin: 0, borderRadius: 0 }
  }
  return { width: props.modalWidth, maxWidth: props.modalWidth, boxSizing: 'border-box' }
})

const contentStyle = computed(() => {
  if (maximized.value) {
    return {
      maxHeight: 'calc(100vh - 72px)',
      height: 'calc(100vh - 72px)',
      overflow: 'auto',
      minWidth: 0,
      boxSizing: 'border-box',
    }
  }
  return {
    maxHeight: 'min(85vh, 820px)',
    overflowY: 'auto',
    overflowX: 'hidden',
    minWidth: 0,
    boxSizing: 'border-box',
  }
})

const effectiveChartHeight = computed(() => {
  if (maximized.value && typeof window !== 'undefined') {
    return Math.max(560, window.innerHeight - props.maxHeightOffset)
  }
  return props.chartHeight
})

const chartMountKey = computed(
  () => `${props.chartKey}-${resolvedCode.value || 'nocode'}-${maximized.value ? 'max' : 'normal'}`,
)

function toggleMaximize() {
  maximized.value = !maximized.value
}

function onAfterLeave() {
  maximized.value = false
  emit('after-leave')
}
</script>

<template>
  <n-modal
    v-model:show="visible"
    :title="title"
    preset="card"
    class="stock-kline-modal"
    :class="{ 'stock-kline-modal--max': maximized }"
    :style="modalStyle"
    :content-style="contentStyle"
    @after-leave="onAfterLeave"
  >
    <template #header-extra>
      <n-button
        quaternary
        circle
        size="small"
        :title="maximized ? '还原' : '最大化'"
        @click="toggleMaximize"
      >
        <n-icon :component="maximized ? ContractOutline : ExpandOutline" />
      </n-button>
    </template>

    <slot name="prepend" />

    <n-alert
      v-if="embedChart && show && !hasCode"
      type="warning"
      :bordered="false"
      title="未设置股票代码"
      style="margin-bottom: 8px"
    >
      请通过 StockKlineModal 的
      <code>code</code>
      属性传入（例如
      <code>:code="klineModal.chartCode"</code>
      ）。不要使用
      <code>chart-code</code>
      或其它别名作为图表标识。
    </n-alert>

    <stock-lightweight-kline-chart
      v-else-if="embedChart && show && hasCode"
      v-bind="attrs"
      :key="chartMountKey"
      :code="resolvedCode"
      :chart-height="effectiveChartHeight"
    />

    <slot />

    <slot name="append" />

    <template v-if="$slots.footer" #footer>
      <slot name="footer" />
    </template>
  </n-modal>
</template>

<style scoped>
:deep(.stock-kline-modal--max.n-modal) {
  padding: 0;
}
:deep(.stock-kline-modal--max .n-card) {
  border-radius: 0;
  height: 100vh;
  max-height: 100vh;
}
</style>
