<script setup>
/**
 * Phase16-H0 · Explanation badge row (quality + readonly + context) and optional disclaimer.
 */
import { computed } from 'vue'
import { NAlert, NSpace, NTag, NText } from 'naive-ui'
import { EXPLANATION_READONLY_LABEL } from '../../utils/explanationKit.js'

const props = defineProps({
  /** Full drawer title when stockName/code not split */
  title: { type: String, default: '' },
  stockName: { type: String, default: '' },
  stockCode: { type: String, default: '' },
  theme: { type: String, default: '' },
  /** Optional quality / completeness badge */
  statusLabel: { type: String, default: '' },
  statusType: { type: String, default: 'default' },
  showReadonlyTag: { type: Boolean, default: true },
  contextText: { type: String, default: '' },
  disclaimer: { type: String, default: '' },
  showDisclaimer: { type: Boolean, default: true },
  showTitle: { type: Boolean, default: true },
})

const readonlyLabel = EXPLANATION_READONLY_LABEL

const displayTitle = computed(() => {
  const explicit = String(props.title || '').trim()
  if (explicit) return explicit
  const name = String(props.stockName || '').trim() || String(props.stockCode || '').trim() || '—'
  const code = String(props.stockCode || '').trim()
  const topic = String(props.theme || '').trim()
  if (topic && code) return `${name}（${code}）· ${topic}`
  if (code) return `${name}（${code}）`
  return name
})

const hasStatus = computed(() => String(props.statusLabel || '').trim().length > 0)
const hasDisclaimer = computed(
  () => props.showDisclaimer && String(props.disclaimer || '').trim().length > 0,
)
</script>

<template>
  <div class="explanation-header">
    <n-text v-if="showTitle && displayTitle" strong class="explanation-header__title">
      {{ displayTitle }}
    </n-text>

    <n-space align="center" :wrap="true" class="explanation-header__badges">
      <n-tag v-if="hasStatus" :type="statusType" :bordered="false">
        {{ statusLabel }}
      </n-tag>
      <n-tag v-if="showReadonlyTag" size="small" type="info" :bordered="false">
        {{ readonlyLabel }}
      </n-tag>
      <n-text v-if="contextText" depth="3">{{ contextText }}</n-text>
    </n-space>

    <n-alert
      v-if="hasDisclaimer"
      type="info"
      :bordered="false"
      class="explanation-header__disclaimer"
    >
      {{ disclaimer }}
    </n-alert>
  </div>
</template>

<style scoped>
.explanation-header__title {
  display: block;
  margin-bottom: 8px;
}
.explanation-header__badges {
  margin-bottom: 12px;
}
.explanation-header__disclaimer {
  margin-bottom: 14px;
}
</style>
