<script setup>
/**
 * Phase16.15-A / 16.24 — Level 2 策略解释 Drawer（投研分节表格）.
 * 复用 ExplanationTable + ExplanationHeader；不改业务逻辑。
 */
import { computed } from 'vue'
import { NAlert, NDrawer, NDrawerContent, NDivider, NEmpty, NSpace, NText } from 'naive-ui'
import ExplanationTable from '../explanation/ExplanationTable.vue'
import ExplanationHeader from '../explanation/ExplanationHeader.vue'
import {
  buildStrategyExplanationRows,
  strategyExplanationStatusHint,
  strategyExplanationStatusLabel,
  strategyExplanationStatusType,
} from '../../utils/strategyExplanationDisplay.js'

const props = defineProps({
  show: { type: Boolean, default: false },
  /** { itemId, code, name, explanation } */
  entry: { type: Object, default: null },
})

const emit = defineEmits(['update:show'])

const explanation = computed(() => props.entry?.explanation || null)
const code = computed(() => String(props.entry?.code || '').trim())
const name_ = computed(() => String(props.entry?.name || '').trim())

const drawerTitle = computed(() => {
  const n = name_.value || code.value || '—'
  const c = code.value
  return c && n !== c ? `${n}（${c}）· 策略解释` : `${n} · 策略解释`
})

const rawStatus = computed(() => String(explanation.value?.status || '').trim())
const statusLabel = computed(() => strategyExplanationStatusLabel(rawStatus.value))
const statusType = computed(() => strategyExplanationStatusType(rawStatus.value))
const statusHint = computed(() => strategyExplanationStatusHint(rawStatus.value))
const isMissing = computed(() => String(rawStatus.value).toLowerCase() === 'missing')
const headline = computed(() => {
  const h = String(explanation.value?.headline || '').trim()
  return h || statusHint.value || '—'
})

const allRows = computed(() => buildStrategyExplanationRows(explanation.value))

const signalRows = computed(() => allRows.value.filter((r) => r.group === 'signal'))
const riskRows = computed(() => allRows.value.filter((r) => r.group === 'risk'))
const entryRows = computed(() => allRows.value.filter((r) => r.group === 'entry'))
const exitRows = computed(() => allRows.value.filter((r) => r.group === 'exit'))
const evidenceRows = computed(() => allRows.value.filter((r) => r.group === 'evidence'))
const disclaimerRows = computed(() => allRows.value.filter((r) => r.group === 'disclaimer'))

const hasAnySection = computed(
  () =>
    signalRows.value.length ||
    riskRows.value.length ||
    entryRows.value.length ||
    exitRows.value.length ||
    evidenceRows.value.length ||
    disclaimerRows.value.length,
)
</script>

<template>
  <n-drawer
    :show="show"
    :width="520"
    placement="right"
    @update:show="(v) => emit('update:show', v)"
  >
    <n-drawer-content :title="drawerTitle" closable>
      <div v-if="explanation" class="expl-drawer-body">
        <ExplanationHeader
          :stock-name="name_"
          :stock-code="code"
          theme="策略解释"
          :status-label="statusLabel"
          :status-type="statusType"
          :show-title="false"
          :show-readonly-tag="true"
          :show-disclaimer="false"
        />

        <n-alert
          v-if="isMissing"
          type="warning"
          :bordered="false"
          style="margin-bottom: 12px"
        >
          该计划创建时未保存完整策略解释，无法恢复历史决策依据。
        </n-alert>

        <n-space v-else-if="headline && headline !== '—'" vertical :size="4" style="margin-bottom: 12px">
          <n-text strong>{{ headline }}</n-text>
          <n-text
            v-if="statusHint && headline !== statusHint"
            depth="3"
            style="font-size: 12px; line-height: 1.5"
          >
            {{ statusHint }}
          </n-text>
        </n-space>

        <template v-if="hasAnySection">
          <div v-if="signalRows.length" class="expl-section">
            <n-text class="expl-section__title">一、信号依据</n-text>
            <ExplanationTable :rows="signalRows" :show-header="true" empty-text="—" />
          </div>

          <div v-if="riskRows.length" class="expl-section">
            <n-divider style="margin: 12px 0" />
            <n-text class="expl-section__title">二、风险分析</n-text>
            <ExplanationTable :rows="riskRows" :show-header="true" empty-text="—" />
          </div>

          <div v-if="entryRows.length" class="expl-section">
            <n-divider style="margin: 12px 0" />
            <n-text class="expl-section__title">三、入场计划</n-text>
            <ExplanationTable :rows="entryRows" :show-header="true" empty-text="—" />
          </div>

          <div v-if="evidenceRows.length" class="expl-section">
            <n-divider style="margin: 12px 0" />
            <n-text class="expl-section__title">四、决策依据</n-text>
            <ExplanationTable :rows="evidenceRows" :show-header="true" empty-text="—" />
          </div>

          <div v-if="exitRows.length" class="expl-section">
            <n-divider style="margin: 12px 0" />
            <n-text class="expl-section__title">退出评估（只读）</n-text>
            <ExplanationTable :rows="exitRows" :show-header="true" empty-text="—" />
          </div>

          <div v-if="disclaimerRows.length" class="expl-section expl-section--disclaimer">
            <n-divider style="margin: 12px 0" />
            <n-text class="expl-section__title">五、免责声明</n-text>
            <ExplanationTable :rows="disclaimerRows" :show-header="true" empty-text="—" />
          </div>
        </template>

        <n-empty
          v-else-if="!isMissing"
          size="small"
          description="暂无结构化解释字段"
        />
      </div>

      <n-empty
        v-else
        size="small"
        description="暂无解释数据。请从主表点击「查看解释」按需加载。"
      />
    </n-drawer-content>
  </n-drawer>
</template>

<style scoped>
.expl-drawer-body {
  padding: 4px 0;
}
.expl-section {
  margin-bottom: 8px;
}
.expl-section__title {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: var(--n-text-color-2);
  margin-bottom: 8px;
}
.expl-section--disclaimer {
  opacity: 0.85;
}
</style>
