<script setup>
/**
 * Phase16-H0 · Unified explanation field display (grid or stacked with source).
 */
import { computed } from 'vue'
import { NTag, NText, NTooltip } from 'naive-ui'
import { normalizeExplanationFields } from '../../utils/explanationKit.js'
import ExplanationEmpty from './ExplanationEmpty.vue'

const props = defineProps({
  fields: { type: Array, default: () => [] },
  /** grid: meta-grid (Provenance); stack: label:value + source line (Projection) */
  variant: { type: String, default: 'grid' },
  /** Optional label mapper (Projection field keys → 中文) */
  labelFn: { type: Function, default: null },
  sourcePrefix: { type: String, default: '字段来源：' },
})

const rows = computed(() => {
  const normalized = normalizeExplanationFields(props.fields)
  if (typeof props.labelFn === 'function') {
    return normalized.map((row) => ({
      ...row,
      label: props.labelFn(row.label) || row.label,
    }))
  }
  return normalized
})
</script>

<template>
  <div v-if="variant === 'stack'" class="explanation-fields explanation-fields--stack">
    <div v-for="row in rows" :key="row.key" class="explanation-field-stack">
      <n-text
        depth="3"
        class="explanation-field-stack__line"
        :class="{ 'is-missing': row.missing }"
      >
        {{ row.label }}：{{ row.value }}
      </n-text>
      <n-text v-if="row.source" depth="3" class="explanation-field-stack__source">
        {{ sourcePrefix }}{{ row.source }}
      </n-text>
    </div>
  </div>

  <div v-else class="explanation-fields explanation-fields--grid meta-grid">
    <div
      v-for="row in rows"
      :key="row.key"
      class="meta-item"
      :class="{ 'meta-item-wide': row.wide }"
    >
      <span class="meta-k">
        <n-tooltip
          v-if="row.tooltip"
          trigger="hover"
          :style="{ maxWidth: '320px' }"
        >
          <template #trigger>
            <span class="col-tip">{{ row.label }}</span>
          </template>
          {{ row.tooltip }}
        </n-tooltip>
        <template v-else>{{ row.label }}</template>
      </span>
      <span class="meta-v" :class="{ 'meta-v-missing': row.missing }">
        <n-tag v-if="row.showTag && !row.missing" size="small" :bordered="false">
          {{ row.value }}
        </n-tag>
        <template v-else-if="row.missing && !row.value">
          <ExplanationEmpty inline />
        </template>
        <template v-else>{{ row.value }}</template>
      </span>
    </div>
  </div>
</template>

<style scoped>
.explanation-fields--stack .explanation-field-stack {
  margin-bottom: 6px;
}
.explanation-field-stack__line {
  font-size: 13px;
}
.explanation-field-stack__line.is-missing {
  font-style: italic;
}
.explanation-field-stack__source {
  display: block;
  font-size: 11px;
  margin-left: 0;
}
.meta-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 8px 16px;
}
.meta-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.meta-item-wide {
  grid-column: 1 / -1;
}
.meta-k {
  font-size: 12px;
  color: var(--n-text-color-3);
}
.meta-v {
  font-size: 13px;
  word-break: break-word;
}
.meta-v-missing {
  color: var(--n-text-color-3);
  font-style: italic;
}
.col-tip {
  cursor: help;
  border-bottom: 1px dotted var(--n-text-color-3);
}
</style>
