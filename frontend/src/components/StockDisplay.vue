<script setup>
defineProps({
  model: {
    type: Object,
    required: true,
  },
})
</script>

<template>
  <span v-if="model" class="stock-display">
    <!-- Phase16.14: single-line 名称(code) -->
    <span v-if="model.displayText" class="stock-display__text">{{ model.displayText }}</span>
    <template v-else>
      <span class="stock-display__name">{{ model.display_name }}</span>
      <span class="stock-display__code">{{ model.display_code }}</span>
    </template>
  </span>
</template>

<style scoped>
.stock-display {
  display: inline-flex;
  align-items: baseline;
  gap: 6px;
  line-height: 1.4;
  max-width: 100%;
}
.stock-display__text,
.stock-display__name {
  font-weight: 600;
  color: var(--n-text-color, inherit);
}
.stock-display__code {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--n-text-color-3, rgba(128, 128, 128, 0.82));
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  letter-spacing: 0.02em;
}
</style>
