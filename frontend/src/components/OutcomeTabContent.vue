<script setup>
/**
 * Phase16-D4 — Shared Outcome Tab content (Portfolio + Opportunity drawers).
 * Read-only; all performance metrics from Outcome API.
 */
import { computed } from 'vue'
import {
  NAlert,
  NButton,
  NDivider,
  NRadioButton,
  NRadioGroup,
  NSpace,
  NTag,
  NText,
} from 'naive-ui'
import ExplanationFieldGrid from './explanation/ExplanationFieldGrid.vue'
import ExplanationEmpty from './explanation/ExplanationEmpty.vue'
import {
  OUTCOME_DISCLAIMER,
  OUTCOME_EMPTY_MESSAGE,
  buildOutcomeTabView,
  outcomeQualityTagType,
  outcomeStatusTagType,
} from '../utils/opportunityOutcomeDisplay.js'

const props = defineProps({
  items: { type: Array, default: () => [] },
  selectedIndex: { type: Number, default: 0 },
  generatedAt: { type: String, default: '' },
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
  notFound: { type: Boolean, default: false },
})

const emit = defineEmits(['update:selectedIndex', 'retry', 'goToPlan'])

const view = computed(() => buildOutcomeTabView(props.items, props.selectedIndex, props.generatedAt))

const legValue = computed({
  get: () => props.selectedIndex,
  set: (v) => emit('update:selectedIndex', Number(v) || 0),
})

function onGoToPlan(planId) {
  const id = Math.trunc(Number(planId) || 0)
  if (id > 0) emit('goToPlan', id)
}
</script>

<template>
  <div class="outcome-tab">
    <n-alert
      v-if="notFound"
      type="warning"
      :bordered="false"
      style="margin-bottom: 12px"
    >
      {{ error || '未找到该股票的交易结果' }}
    </n-alert>
    <n-alert
      v-else-if="error"
      type="error"
      :bordered="false"
      style="margin-bottom: 12px"
    >
      {{ error }}
      <template #footer>
        <n-button size="small" @click="emit('retry')">重试</n-button>
      </template>
    </n-alert>

    <template v-else-if="!loading">
      <template v-if="view.isEmpty">
        <n-text depth="3">{{ OUTCOME_EMPTY_MESSAGE }}</n-text>
      </template>

      <template v-else>
        <n-space align="center" :wrap="true" style="margin-bottom: 12px">
          <n-tag
            v-if="view.selected"
            :type="outcomeStatusTagType(view.selected.status)"
            :bordered="false"
          >
            {{ view.selected.statusLabel }}
          </n-tag>
          <n-tag :type="outcomeQualityTagType(view.quality)" :bordered="false">
            {{ view.qualityLabel }}
          </n-tag>
          <n-tag size="small" type="info" :bordered="false">只读</n-tag>
          <n-text v-if="view.generatedAt" depth="3" style="font-size: 12px">
            更新 {{ view.generatedAt }}
          </n-text>
        </n-space>

        <n-alert type="info" :bordered="false" style="margin-bottom: 14px">
          {{ OUTCOME_DISCLAIMER }}
        </n-alert>

        <n-radio-group
          v-if="view.legs.length > 1"
          v-model:value="legValue"
          size="small"
          style="margin-bottom: 14px"
        >
          <n-space :wrap="true">
            <n-radio-button
              v-for="leg in view.legs"
              :key="leg.outcomeId || leg.index"
              :value="leg.index"
            >
              Leg {{ leg.index + 1 }} · {{ leg.summary }}
            </n-radio-button>
          </n-space>
        </n-radio-group>

        <n-text strong style="display: block; margin-bottom: 8px">解释</n-text>
        <template v-for="section in view.explainSections" :key="section.key">
          <div style="margin-bottom: 14px">
            <n-space align="center" justify="space-between" style="margin-bottom: 4px">
              <n-text strong>{{ section.title }}</n-text>
              <n-text depth="3" style="font-size: 12px">{{ section.sourceBanner }}</n-text>
            </n-space>
            <ExplanationFieldGrid variant="stack" :fields="section.fields" />
            <n-button
              v-if="section.key === 'plan' && section.planId"
              text
              type="primary"
              size="small"
              style="margin-top: 6px"
              @click="onGoToPlan(section.planId)"
            >
              查看 TradePlan #{{ section.planId }}
            </n-button>
          </div>
        </template>

        <n-divider style="margin: 12px 0" />

        <n-text strong style="display: block; margin-bottom: 8px">结果</n-text>

        <div v-if="view.resultSections" style="margin-bottom: 14px">
          <n-text depth="2" style="display: block; margin-bottom: 6px; font-size: 13px">买入 Entry</n-text>
          <ExplanationFieldGrid
            v-if="view.resultSections.entry.present || view.selected?.status === 'NO_TRADE'"
            variant="stack"
            :fields="view.resultSections.entry.fields"
          />
          <n-text v-else depth="3"><ExplanationEmpty inline /></n-text>

          <n-text depth="2" style="display: block; margin: 10px 0 6px; font-size: 13px">卖出 Exit</n-text>
          <ExplanationFieldGrid variant="stack" :fields="view.resultSections.exit.fields" />

          <n-text depth="2" style="display: block; margin: 10px 0 6px; font-size: 13px">绩效 Performance</n-text>
          <ExplanationFieldGrid
            v-if="view.resultSections.performance.fields.length"
            variant="stack"
            :fields="view.resultSections.performance.fields"
          />
          <n-text v-else depth="3"><ExplanationEmpty inline /></n-text>
        </div>
      </template>
    </template>
  </div>
</template>

<style scoped>
.outcome-tab {
  min-height: 120px;
}
</style>
