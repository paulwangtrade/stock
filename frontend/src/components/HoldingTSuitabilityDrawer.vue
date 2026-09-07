<script setup>
/**
 * Holding T-Suitability drawer (Phase17.1).
 * Display-only: level / reasons / freshness / volatility / health.
 * No buy/sell buttons, no trade advice.
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
  formatTSuitEvalTime,
  freshnessLabelZH,
  tSuitLevelLabel,
  tSuitLevelTagType,
  tSuitReasonLabelZH,
  volatilityLabelZH,
} from '../utils/holdingTSuitabilityDisplay.js'

const props = defineProps({
  show: { type: Boolean, default: false },
  row: { type: Object, default: null },
  /** Mapped HoldingTSuitability from exit-evaluation. */
  suitability: { type: Object, default: null },
})

const emit = defineEmits(['update:show'])

const visible = computed({
  get: () => props.show,
  set: (v) => emit('update:show', v),
})

const stockCode = computed(() =>
  String(props.row?.stockCode || props.suitability?.stockCode || '').trim(),
)
const stockName = computed(() => String(props.row?.stockName || '').trim())

const level = computed(() => String(props.suitability?.level || '').trim().toLowerCase())
const reasons = computed(() =>
  Array.isArray(props.suitability?.reasons) ? props.suitability.reasons : [],
)

const statusLabel = computed(() => {
  if (!level.value) return '暂无做 T 评价'
  return tSuitLevelLabel(level.value)
})
</script>

<template>
  <n-drawer v-model:show="visible" :width="420" placement="right">
    <n-drawer-content closable :title="stockName || stockCode || '做 T 适宜性'">
      <explanation-header
        :stock-name="stockName"
        :stock-code="stockCode"
        theme="做 T 适宜性"
        :status-label="statusLabel"
        :status-type="tSuitLevelTagType(level)"
        context-text="持仓做 T 适宜性评价 · 非交易指令"
        disclaimer="本抽屉仅展示评价原因，不生成买卖单，不修改持仓。"
      />

      <n-alert v-if="!suitability" type="default" :bordered="false" style="margin-top: 12px">
        暂无做 T 适宜性评价（评价未就绪或不适用）。
      </n-alert>

      <template v-else>
        <n-divider style="margin: 14px 0 10px">评价等级</n-divider>
        <n-tag size="small" :type="tSuitLevelTagType(level)" :bordered="false">
          {{ tSuitLevelLabel(level) }}
        </n-tag>

        <n-divider style="margin: 14px 0 10px">原因</n-divider>
        <n-space v-if="reasons.length" vertical :size="4">
          <n-text v-for="code in reasons" :key="code" depth="3">
            · {{ tSuitReasonLabelZH(code) }}
          </n-text>
        </n-space>
        <n-text v-else depth="3">暂无额外原因码（操作门槛与环境均可通过）。</n-text>

        <n-divider style="margin: 14px 0 10px">环境</n-divider>
        <n-space vertical :size="4">
          <n-text depth="3">可卖：{{ suitability.canSell ? '是' : '否' }}</n-text>
          <n-text depth="3">行情新鲜度：{{ freshnessLabelZH(suitability.freshness) }}</n-text>
          <n-text depth="3">波动状态：{{ volatilityLabelZH(suitability.volatilityStatus) }}</n-text>
          <n-text depth="3">健康等级：{{ suitability.healthGrade || '—' }}</n-text>
          <n-text depth="3">评价时间：{{ formatTSuitEvalTime(suitability.evaluatedAt) }}</n-text>
        </n-space>

        <n-text
          v-if="suitability.dataSourceNote"
          depth="3"
          style="display: block; margin-top: 16px; font-size: 12px"
        >
          {{ suitability.dataSourceNote }}
        </n-text>
      </template>
    </n-drawer-content>
  </n-drawer>
</template>
