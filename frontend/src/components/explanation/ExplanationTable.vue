<script setup>
/**
 * Phase16.14 ExplanationTable — 字段 | 内容 双列解释表（只读展示）.
 */
import { computed, useSlots } from 'vue'
import { NTag, NTooltip } from 'naive-ui'
import {
  EXPLANATION_TABLE_COLUMNS,
  normalizeExplanationTableRows,
} from '../../utils/explanationTable.js'
import { EXPLANATION_SOURCE_PREFIX } from '../../utils/explanationKit.js'
import ExplanationEmpty from './ExplanationEmpty.vue'

const props = defineProps({
  rows: { type: Array, default: () => [] },
  columns: {
    type: Object,
    default: () => ({ ...EXPLANATION_TABLE_COLUMNS }),
  },
  emptyText: { type: String, default: '暂无记录' },
  showHeader: { type: Boolean, default: true },
  dense: { type: Boolean, default: true },
  groups: { type: Array, default: null },
  showSource: { type: Boolean, default: false },
  sourcePrefix: { type: String, default: EXPLANATION_SOURCE_PREFIX },
})

const slots = useSlots()

const tableRows = computed(() =>
  normalizeExplanationTableRows(props.rows, { groups: props.groups }),
)

const fieldHeader = computed(
  () => String(props.columns?.field || EXPLANATION_TABLE_COLUMNS.field).trim() || '字段',
)
const contentHeader = computed(
  () => String(props.columns?.content || EXPLANATION_TABLE_COLUMNS.content).trim() || '内容',
)

function hasFieldSlot(key) {
  return typeof slots[`field-${key}`] === 'function'
}

function hasContentSlot(key) {
  return typeof slots[`content-${key}`] === 'function'
}

function contentClass(row) {
  return {
    'explanation-table__content': true,
    'is-missing': row.missing,
    'is-risk': row.kind === 'risk' || row.emphasis === 'warn' || row.emphasis === 'danger',
    'is-muted': row.emphasis === 'muted',
  }
}
</script>

<template>
  <div
    class="explanation-table"
    :class="{ 'explanation-table--dense': dense }"
    role="table"
    aria-label="解释字段表"
  >
    <div v-if="showHeader" class="explanation-table__head" role="row">
      <div class="explanation-table__th explanation-table__th--field" role="columnheader">
        {{ fieldHeader }}
      </div>
      <div class="explanation-table__th explanation-table__th--content" role="columnheader">
        {{ contentHeader }}
      </div>
    </div>

    <div class="explanation-table__body" role="rowgroup">
      <div
        v-for="(row, idx) in tableRows"
        :key="row.key"
        class="explanation-table__row"
        :class="{ 'explanation-table__row--alt': idx % 2 === 1 }"
        role="row"
      >
      <div class="explanation-table__td explanation-table__td--field" role="cell">
        <slot v-if="hasFieldSlot(row.key)" :name="`field-${row.key}`" :row="row" />
        <template v-else>
          <n-tooltip
            v-if="row.tooltip"
            trigger="hover"
            :style="{ maxWidth: '320px' }"
          >
            <template #trigger>
              <span class="explanation-table__field-tip">{{ row.field }}</span>
            </template>
            {{ row.tooltip }}
          </n-tooltip>
          <span v-else>{{ row.field }}</span>
        </template>
      </div>

      <div class="explanation-table__td explanation-table__td--content" role="cell">
        <slot v-if="hasContentSlot(row.key)" :name="`content-${row.key}`" :row="row" />
        <template v-else>
          <n-tag
            v-if="row.kind === 'tag' && !row.missing && row.content"
            size="small"
            :bordered="false"
          >
            {{ row.content }}
          </n-tag>
          <ExplanationEmpty
            v-else-if="row.missing && (!row.content || row.content === emptyText)"
            inline
            :text="emptyText"
          />
          <span v-else :class="contentClass(row)">{{ row.content || emptyText }}</span>
          <span
            v-if="showSource && row.source"
            class="explanation-table__source"
          >
            {{ sourcePrefix }}{{ row.source }}
          </span>
        </template>
      </div>
      </div>
    </div>

    <div v-if="$slots.append" class="explanation-table__append">
      <slot name="append" />
    </div>
  </div>
</template>

<style scoped>
.explanation-table {
  width: 100%;
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  overflow: hidden;
  font-size: 13px;
}
.explanation-table--dense .explanation-table__th,
.explanation-table--dense .explanation-table__td {
  padding: 6px 10px;
}
.explanation-table__head {
  display: grid;
  grid-template-columns: 112px minmax(0, 1fr);
  background: var(--n-action-color, rgba(0, 0, 0, 0.04));
  border-bottom: 1px solid var(--n-border-color);
}
.explanation-table__row {
  display: grid;
  grid-template-columns: 112px minmax(0, 1fr);
  border-bottom: 1px solid var(--n-border-color);
}
.explanation-table__row:last-child {
  border-bottom: none;
}
.explanation-table__row--alt {
  background: rgba(128, 128, 128, 0.04);
}
.explanation-table__th {
  font-size: 12px;
  font-weight: 600;
  color: var(--n-text-color-2);
  padding: 8px 10px;
}
.explanation-table__td {
  padding: 8px 10px;
  min-width: 0;
  word-break: break-word;
  line-height: 1.45;
}
.explanation-table__td--field {
  color: var(--n-text-color-3);
  font-size: 12px;
  align-self: start;
}
.explanation-table__td--content {
  color: var(--n-text-color);
}
.explanation-table__field-tip {
  cursor: help;
  border-bottom: 1px dotted var(--n-text-color-3);
}
.explanation-table__content.is-missing {
  color: var(--n-text-color-3);
  font-style: italic;
}
.explanation-table__content.is-risk {
  color: var(--n-error-color, #d03050);
}
.explanation-table__content.is-muted {
  color: var(--n-text-color-3);
}
.explanation-table__source {
  display: block;
  margin-top: 2px;
  font-size: 11px;
  color: var(--n-text-color-3);
}
.explanation-table__append {
  padding: 8px 10px;
  border-top: 1px solid var(--n-border-color);
  font-size: 11px;
  color: var(--n-text-color-3);
  line-height: 1.45;
}
</style>
