<script setup>
/**
 * Holding Health Explanation drawer (Phase17-C4).
 * Display-only: grade / supporting & risk factors / signal / eval time.
 * No sell button, no trade actions.
 */
import { computed } from 'vue'
import {
  NAlert,
  NDivider,
  NDrawer,
  NDrawerContent,
  NSpace,
  NTag,
  NText,
} from 'naive-ui'
import ExplanationHeader from './explanation/ExplanationHeader.vue'
import {
  formatHealthEvalTime,
  healthFactorLabelZH,
  healthGradeListLabel,
  healthGradeTagType,
} from '../utils/holdingHealthDisplay.js'

const props = defineProps({
  show: { type: Boolean, default: false },
  /** Snapshot position row (stockCode / stockName). */
  row: { type: Object, default: null },
  /** Mapped HoldingHealthScore from exit-evaluation. */
  health: { type: Object, default: null },
})

const emit = defineEmits(['update:show'])

const visible = computed({
  get: () => props.show,
  set: (v) => emit('update:show', v),
})

const stockCode = computed(() => String(props.row?.stockCode || props.health?.stockCode || '').trim())
const stockName = computed(() => String(props.row?.stockName || '').trim())

const explanation = computed(
  () => props.health?.explanation || null,
)

const grade = computed(() => String(props.health?.grade || '').trim())
const score = computed(() => {
  const n = Number(props.health?.score)
  return Number.isFinite(n) ? n : null
})

const supporting = computed(() =>
  Array.isArray(props.health?.supportingFactors) ? props.health.supportingFactors : [],
)
const risks = computed(() =>
  Array.isArray(props.health?.riskFactors) ? props.health.riskFactors : [],
)

const evalTime = computed(() =>
  formatHealthEvalTime(
    props.health?.evaluationTime || explanation.value?.evaluationTime,
  ),
)

const sourceType = computed(() => String(explanation.value?.sourceType || '').trim() || '—')

const signal = computed(() => explanation.value?.signalContext || {})

const statusLabel = computed(() => {
  if (!grade.value) return '暂无健康评分'
  const base = healthGradeListLabel(grade.value)
  return score.value != null ? `${base} · ${score.value}` : base
})
</script>

<template>
  <n-drawer v-model:show="visible" :width="420" placement="right">
    <n-drawer-content closable :title="stockName || stockCode || '持仓健康'">
      <explanation-header
        :stock-name="stockName"
        :stock-code="stockCode"
        theme="持仓健康"
        :status-label="statusLabel"
        :status-type="healthGradeTagType(grade)"
        context-text="持仓质量评价 · 非卖出建议"
        disclaimer="本抽屉仅展示评价结果，不生成买卖单，不修改持仓。"
      />

      <n-alert v-if="!health" type="default" :bordered="false" style="margin-top: 12px">
        暂无健康评分（评价接口不可用或该票无持仓评价）。
      </n-alert>

      <template v-else>
        <n-divider style="margin: 14px 0 10px">支持因素</n-divider>
        <n-space v-if="supporting.length" :wrap="true" size="small">
          <n-tag
            v-for="code in supporting"
            :key="'s-' + code"
            size="small"
            type="success"
            :bordered="false"
          >
            ✓ {{ healthFactorLabelZH(code) }}
          </n-tag>
        </n-space>
        <n-text v-else depth="3">暂无支持因素</n-text>

        <n-divider style="margin: 14px 0 10px">风险因素</n-divider>
        <n-space v-if="risks.length" :wrap="true" size="small">
          <n-tag
            v-for="code in risks"
            :key="'r-' + code"
            size="small"
            type="warning"
            :bordered="false"
          >
            ⚠ {{ healthFactorLabelZH(code) }}
          </n-tag>
        </n-space>
        <n-text v-else depth="3">暂无风险提示</n-text>

        <n-divider style="margin: 14px 0 10px">信号来源</n-divider>
        <n-space vertical :size="4">
          <n-text depth="3">来源类型：{{ sourceType }}</n-text>
          <n-text depth="3">
            信号：
            <template v-if="signal.present">
              {{ signal.signalTag || '有' }}
              <template v-if="signal.signalPrice"> · 价 {{ signal.signalPrice }}</template>
              <template v-if="signal.signalTime"> · {{ signal.signalTime }}</template>
              <template v-if="signal.signalSnapshotId"> · 快照 #{{ signal.signalSnapshotId }}</template>
            </template>
            <template v-else>无关联信号</template>
          </n-text>
          <n-text depth="3">评价时间：{{ evalTime }}</n-text>
          <n-text v-if="explanation?.positionDays != null" depth="3">
            持仓天数：{{ explanation.positionDays }}
          </n-text>
        </n-space>

        <n-text
          v-if="health.dataSourceNote"
          depth="3"
          style="display: block; margin-top: 16px; font-size: 12px"
        >
          {{ health.dataSourceNote }}
        </n-text>
      </template>
    </n-drawer-content>
  </n-drawer>
</template>
