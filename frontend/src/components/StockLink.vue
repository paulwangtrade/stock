<script setup>
import { CLICK_KLINE_MODAL } from '../utils/stockDisplay.js'
import StockDisplay from './StockDisplay.vue'

const props = defineProps({
  model: {
    type: Object,
    required: true,
  },
})

const emit = defineEmits(['open'])

function onOpen() {
  const action = props.model?.click_action
  if (!action || action.type !== CLICK_KLINE_MODAL || !action.chart_code) return
  emit('open', props.model)
}
</script>

<template>
  <button
    v-if="model"
    type="button"
    class="stock-link"
    :title="model.click_action?.title || model.display_code"
    @click="onOpen"
  >
    <stock-display :model="model" />
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
.stock-link:hover :deep(.stock-display__name) {
  color: var(--n-primary-color, #18a058);
}
</style>
