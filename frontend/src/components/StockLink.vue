<script setup>
/**
 * Phase16.14 StockLink — click opens multi-period K-line via klineKey.
 */
import { computed } from 'vue'
import { CLICK_KLINE_MODAL, toStockKlineLinkModel } from '../utils/stockDisplay.js'
import StockDisplay from './StockDisplay.vue'

const props = defineProps({
  model: {
    type: Object,
    required: true,
  },
})

const emit = defineEmits(['open'])

const linkModel = computed(() => toStockKlineLinkModel(props.model) || props.model)

const canOpen = computed(() => {
  const m = linkModel.value
  return !!(m?.klineKey || (m?.click_action?.type === CLICK_KLINE_MODAL && m?.click_action?.chart_code))
})

function onOpen() {
  if (!canOpen.value) return
  emit('open', linkModel.value)
}
</script>

<template>
  <button
    v-if="linkModel"
    type="button"
    class="stock-link"
    :class="{ 'stock-link--disabled': !canOpen }"
    :disabled="!canOpen"
    :title="canOpen ? (linkModel.click_action?.title || '查看多周期K线') : '无法打开K线'"
    @click.stop="onOpen"
  >
    <stock-display :model="linkModel" />
  </button>
</template>

<style scoped>
.stock-link {
  display: inline-flex;
  align-items: center;
  margin: 0;
  padding: 0;
  border: 0;
  background: transparent;
  cursor: pointer;
  text-align: left;
  max-width: 100%;
}
.stock-link--disabled {
  cursor: default;
  opacity: 0.85;
}
.stock-link:not(.stock-link--disabled):hover :deep(.stock-display__name),
.stock-link:not(.stock-link--disabled):hover :deep(.stock-display__text) {
  color: var(--n-primary-color, #18a058);
}
</style>
