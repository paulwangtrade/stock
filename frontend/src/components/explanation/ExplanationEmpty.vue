<script setup>
/**
 * Phase16-H0 · Unified empty / missing field placeholder.
 */
import { ORIGIN_EMPTY } from '../../utils/tradePlanOriginDisplay.js'

defineProps({
  inline: { type: Boolean, default: false },
  text: { type: String, default: ORIGIN_EMPTY },
})
</script>

<template>
  <span :class="inline ? 'explanation-empty explanation-empty--inline' : 'explanation-empty'">
    {{ text }}
  </span>
</template>

<style scoped>
.explanation-empty {
  color: var(--n-text-color-3);
  font-style: italic;
}
.explanation-empty--inline {
  display: inline;
}
</style>
