<script setup>
/**
 * Research Experiment Foundation — record hypothesis against finding_id.
 * Session memory only · no auto-run · no backtest · no strategy.
 */
import { computed, ref } from 'vue'
import {
  NAlert,
  NButton,
  NDataTable,
  NEmpty,
  NInput,
  NSpace,
  NText,
  useMessage,
} from 'naive-ui'
import {
  createResearchExperiment,
  listResearchExperiments,
} from '../utils/researchExperimentProjection.js'

const message = useMessage()
const findingId = ref('rf:signal_type=BREAKOUT')
const hypothesis = ref('')
const rows = ref(listResearchExperiments())

const columns = [
  { title: '实验 ID', key: 'experiment_id', ellipsis: { tooltip: true } },
  { title: 'finding_id', key: 'finding_id', width: 200, ellipsis: { tooltip: true } },
  { title: '假设', key: 'hypothesis', ellipsis: { tooltip: true } },
  { title: '状态', key: 'status', width: 90 },
]

const tableRows = computed(() =>
  (rows.value || []).map((card) => ({
    experiment_id: card.experiment_id || '—',
    finding_id: card.finding_id || '—',
    hypothesis: card.hypothesis || '—',
    status: card.status || '—',
  })),
)

function refresh() {
  rows.value = listResearchExperiments()
}

function onCreate() {
  const card = createResearchExperiment({
    finding_id: findingId.value,
    hypothesis: hypothesis.value,
  })
  if (!card.available) {
    message.warning('需要假设句子与 finding_id。不自动跑实验。')
    return
  }
  refresh()
  message.success(`已记录 ${card.experiment_id}`)
}
</script>

<template>
  <div class="research-experiment-foundation">
    <n-space justify="space-between" align="center" style="margin-bottom: 10px">
      <div>
        <n-text strong style="font-size: 15px">研究实验（基础）</n-text>
        <n-text depth="3" style="margin-left: 8px; font-size: 12px">
          引用 finding_id · status=recorded · 会话内存 · 非策略
        </n-text>
      </div>
      <n-button size="small" type="primary" @click="onCreate">记录实验</n-button>
    </n-space>

    <n-alert type="info" :bordered="false" style="margin-bottom: 10px">
      Experiment 只验证人对发现写下的假设。不运行回测、不调参、不生成 Strategy Version。
    </n-alert>

    <n-space vertical style="margin-bottom: 12px">
      <n-input v-model:value="findingId" size="small" placeholder="finding_id（如 rf:…）" />
      <n-input
        v-model:value="hypothesis"
        type="textarea"
        size="small"
        :rows="2"
        placeholder="人工假设句子（必填）"
      />
    </n-space>

    <n-data-table
      v-if="tableRows.length"
      size="small"
      :columns="columns"
      :data="tableRows"
      :bordered="false"
      :single-line="false"
    />
    <n-empty v-else description="尚无会话内实验。填写 finding_id 与假设后记录。" />
  </div>
</template>

<style scoped>
.research-experiment-foundation {
  padding: 4px 2px 12px;
}
</style>
