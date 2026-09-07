<script setup>
/**
 * Phase16-H0 · Decision pipeline tag row (发现 → 关注 → 决策 → 计划 → 持仓).
 */
import { computed } from 'vue'
import { NSpace, NTag, NText } from 'naive-ui'
import {
  EXPLANATION_PIPELINE_CAPTION,
  EXPLANATION_PIPELINE_TITLE,
  normalizePipelineSteps,
} from '../../utils/explanationKit.js'

const props = defineProps({
  steps: { type: Array, default: () => [] },
  title: { type: String, default: EXPLANATION_PIPELINE_TITLE },
  caption: { type: String, default: EXPLANATION_PIPELINE_CAPTION },
  showTitle: { type: Boolean, default: true },
  showCaption: { type: Boolean, default: true },
})

const normalizedSteps = computed(() => normalizePipelineSteps(props.steps))
</script>

<template>
  <div v-if="normalizedSteps.length" class="explanation-pipeline">
    <n-text v-if="showTitle" strong class="explanation-pipeline__title">
      {{ title }}
    </n-text>
    <n-space :size="6" :wrap="true" class="explanation-pipeline__tags">
      <template v-for="(step, idx) in normalizedSteps" :key="step.key">
        <n-tag
          :type="step.active ? 'success' : 'default'"
          size="small"
          :bordered="false"
        >
          {{ step.label }}
        </n-tag>
        <n-text v-if="idx < normalizedSteps.length - 1" depth="3">→</n-text>
      </template>
    </n-space>
    <n-text
      v-if="showCaption && caption"
      depth="3"
      class="explanation-pipeline__caption"
    >
      {{ caption }}
    </n-text>
  </div>
</template>

<style scoped>
.explanation-pipeline__title {
  display: block;
  margin-bottom: 8px;
}
.explanation-pipeline__tags {
  margin-bottom: 14px;
}
.explanation-pipeline__caption {
  display: block;
  margin-bottom: 14px;
  font-size: 12px;
}
</style>
