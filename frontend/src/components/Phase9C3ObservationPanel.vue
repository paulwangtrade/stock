<script setup>
/**
 * Phase9-C.3 Shadow Evaluation — observation-only panel.
 * Read-only: no authority switch, no migration, no execution controls.
 */
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NCode,
  NDescriptions,
  NDescriptionsItem,
  NSpace,
  NTag,
  NText,
  NThing,
  useMessage,
} from 'naive-ui'
import { EventsOn, EventsOff } from '../../wailsjs/runtime'
import {
  getPhase9C3ObservationBundle,
  exportPhase9C3ObservationBundleJSON,
  getLastPhase9C3ObservationCheckpoint,
  PHASE9_C3_OBSERVATION_CHECKPOINT_EVENT,
} from '../utils/riskintel/phase9C3ObservationBundle.js'

const message = useMessage()
const loading = ref(false)
const bundle = ref(null)
const lastAutoCheckpointAt = ref(null)
const copyPreview = ref('')

const statusTagType = computed(() => {
  const s = String(bundle.value?.status?.status || '')
  if (s === 'HEALTHY') return 'success'
  if (s === 'DEGRADED') return 'warning'
  if (s === 'ROLLBACK_RECOMMENDED') return 'error'
  return 'default'
})

const windowTagType = computed(() => {
  const label = String(bundle.value?.healthyWindowHint?.label || '')
  if (label === 'HEALTHY') return 'success'
  if (label === 'DEGRADED') return 'warning'
  if (label === 'INSUFFICIENT SAMPLE') return 'default'
  return 'error'
})

function pct(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return `${(n * 100).toFixed(2)}%`
}

/** UI-only: display UTC RFC3339 as Beijing time. Does not alter bundle/JSON. */
const CN_TZ = 'Asia/Shanghai'
function formatBeijingTime(v) {
  if (v == null || v === '') return '—'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return String(v)
  return d.toLocaleString('zh-CN', {
    timeZone: CN_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  })
}

function refresh(options = {}) {
  loading.value = true
  try {
    const next = getPhase9C3ObservationBundle({
      recordGate: options.recordGate !== false,
      meta: {
        trigger: 'manual_refresh',
        sameProcessScan: true,
        ...(options.meta || {}),
      },
    })
    bundle.value = next
    copyPreview.value = JSON.stringify(next, null, 2)
    const last = getLastPhase9C3ObservationCheckpoint()
    if (last?.capturedAt) lastAutoCheckpointAt.value = last.capturedAt
  } catch (e) {
    message.error(`刷新失败：${e?.message || e}`)
  } finally {
    loading.value = false
  }
}

async function copyJson() {
  try {
    const text = exportPhase9C3ObservationBundleJSON({
      recordGate: false,
      meta: { trigger: 'manual_copy', sameProcessScan: true },
    })
    copyPreview.value = text
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
      message.success('已复制 Observation JSON')
    } else {
      message.warning('剪贴板不可用，请从下方预览手动复制')
    }
  } catch (e) {
    message.error(`复制失败：${e?.message || e}`)
  }
}

function onCheckpoint(payload) {
  try {
    if (payload?.capturedAt) lastAutoCheckpointAt.value = payload.capturedAt
    // Keep panel in sync if user is watching during scan
    if (bundle.value) {
      bundle.value = payload
      copyPreview.value = JSON.stringify(payload, null, 2)
    }
  } catch (_) {
    /* observation only */
  }
}

onMounted(() => {
  refresh({ recordGate: true })
  try {
    EventsOn(PHASE9_C3_OBSERVATION_CHECKPOINT_EVENT, onCheckpoint)
  } catch (_) {
    /* non-wails */
  }
})

onBeforeUnmount(() => {
  try {
    EventsOff(PHASE9_C3_OBSERVATION_CHECKPOINT_EVENT)
  } catch (_) {
    /* ignore */
  }
})
</script>

<template>
  <div style="padding: 16px 20px; max-width: 1100px;">
    <NThing>
      <template #header>C.3 Shadow Evaluation</template>
      <template #description>
        <NText depth="3">
          Phase9-C.3 只读观察面板 · 不改变 gate 状态语义 · 不触发 migration · 不影响 execution · 指标随进程重启清零
        </NText>
      </template>
      <template #header-extra>
        <NSpace>
          <NButton :loading="loading" type="primary" secondary @click="refresh({ recordGate: true })">
            刷新观察
          </NButton>
          <NButton :disabled="loading" @click="copyJson">复制 JSON</NButton>
        </NSpace>
      </template>
    </NThing>

    <NAlert style="margin-top: 12px;" type="info" title="Observation only" :bordered="false">
      本页仅聚合内存中的 controlled switch / gate / shadow metrics。Scan 成功后会自动 emit checkpoint（失败不影响主流程）；仍保留手动刷新。
      <br />界面时间显示为北京时间（UTC+8），复制 JSON 保留 UTC 时间
      <template v-if="lastAutoCheckpointAt">
        <br />最近自动 checkpoint：{{ formatBeijingTime(lastAutoCheckpointAt) }}
      </template>
    </NAlert>

    <NSpace vertical style="margin-top: 16px;" :size="16">
      <NCard size="small" title="Controlled Switch Status">
        <NSpace align="center" style="margin-bottom: 12px;">
          <NTag :type="statusTagType" size="small">{{ bundle?.status?.status || '—' }}</NTag>
          <NTag :type="windowTagType" size="small">
            Window: {{ bundle?.healthyWindowHint?.label || '—' }}
          </NTag>
          <NText depth="3" style="font-size: 12px;">
            countsTowardHealthyWindow={{ bundle?.healthyWindowHint?.countsTowardHealthyWindow ? 'true' : 'false' }}
          </NText>
        </NSpace>
        <NDescriptions label-placement="left" :column="2" size="small" bordered>
          <NDescriptionsItem label="source">{{ bundle?.status?.source || '—' }}</NDescriptionsItem>
          <NDescriptionsItem label="activeSince">{{ formatBeijingTime(bundle?.status?.activeSince) }}</NDescriptionsItem>
          <NDescriptionsItem label="totalDecisions">{{ bundle?.status?.totalDecisions ?? '—' }}</NDescriptionsItem>
          <NDescriptionsItem label="projectionUsageRate">{{ pct(bundle?.status?.projectionUsageRate) }}</NDescriptionsItem>
          <NDescriptionsItem label="fallbackRate">{{ pct(bundle?.status?.fallbackRate) }}</NDescriptionsItem>
          <NDescriptionsItem label="diffRate">{{ pct(bundle?.status?.diffRate) }}</NDescriptionsItem>
          <NDescriptionsItem label="unavailableRate">{{ pct(bundle?.status?.unavailableRate) }}</NDescriptionsItem>
          <NDescriptionsItem label="capturedAt">{{ formatBeijingTime(bundle?.capturedAt) }}</NDescriptionsItem>
        </NDescriptions>
        <div v-if="bundle?.healthyWindowHint?.reasons?.length" style="margin-top: 10px;">
          <NText depth="3" style="font-size: 12px;">Eligibility reasons:</NText>
          <ul style="margin: 4px 0 0; padding-left: 18px; font-size: 12px; color: var(--n-text-color-3);">
            <li v-for="(r, i) in bundle.healthyWindowHint.reasons" :key="i">{{ r }}</li>
          </ul>
        </div>
      </NCard>

      <NCard size="small" title="Metrics (legacy vs projection)">
        <NDescriptions label-placement="left" :column="2" size="small" bordered>
          <NDescriptionsItem label="projectionSelectedCount">
            {{ bundle?.metrics?.projectionSelectedCount ?? '—' }}
          </NDescriptionsItem>
          <NDescriptionsItem label="legacyFallbackCount">
            {{ bundle?.metrics?.legacyFallbackCount ?? '—' }}
          </NDescriptionsItem>
          <NDescriptionsItem label="preferredSource">
            {{ bundle?.metrics?.preferredSource || '—' }}
          </NDescriptionsItem>
          <NDescriptionsItem label="decisionDiffRate">
            {{ pct(bundle?.metrics?.decisionDiffRate) }}
          </NDescriptionsItem>
        </NDescriptions>
        <div style="margin-top: 10px;">
          <NText depth="3" style="font-size: 12px;">projectionUnavailableReasons</NText>
          <pre style="margin: 4px 0 0; font-size: 12px; white-space: pre-wrap;">{{
            JSON.stringify(bundle?.metrics?.projectionUnavailableReasons || {}, null, 2)
          }}</pre>
        </div>
      </NCard>

      <NCard size="small" title="Migration Gate Snapshot (record on refresh)">
        <NDescriptions label-placement="left" :column="2" size="small" bordered>
          <NDescriptionsItem label="sampleCount">{{ bundle?.gate?.sampleCount ?? '—' }}</NDescriptionsItem>
          <NDescriptionsItem label="recommendation">{{ bundle?.gate?.recommendation || '—' }}</NDescriptionsItem>
          <NDescriptionsItem label="exactMatchRate">{{ pct(bundle?.gate?.readiness?.exactMatchRate) }}</NDescriptionsItem>
          <NDescriptionsItem label="alignedMatchRate">{{ pct(bundle?.gate?.readiness?.alignedMatchRate) }}</NDescriptionsItem>
          <NDescriptionsItem label="diffRate">{{ pct(bundle?.gate?.readiness?.diffRate) }}</NDescriptionsItem>
          <NDescriptionsItem label="authoritySource">{{ bundle?.gate?.authoritySource || '—' }}</NDescriptionsItem>
        </NDescriptions>
      </NCard>

      <NCard size="small" title="Shadow Observation (summary)">
        <NDescriptions label-placement="left" :column="2" size="small" bordered>
          <NDescriptionsItem label="shadow generated">
            {{ bundle?.shadowObservation?.metrics?.generated ?? '—' }}
          </NDescriptionsItem>
          <NDescriptionsItem label="shadow failed">
            {{ bundle?.shadowObservation?.metrics?.failed ?? '—' }}
          </NDescriptionsItem>
          <NDescriptionsItem label="matchRate">
            {{ pct(bundle?.shadowObservation?.summary?.matchRate) }}
          </NDescriptionsItem>
          <NDescriptionsItem label="mismatchRate">
            {{ pct(bundle?.shadowObservation?.summary?.mismatchRate) }}
          </NDescriptionsItem>
          <NDescriptionsItem label="migrationRecommendation">
            {{ bundle?.shadowObservation?.readiness?.migrationRecommendation || '—' }}
          </NDescriptionsItem>
          <NDescriptionsItem label="controlledSwitchStatus">
            {{ bundle?.shadowObservation?.controlledSwitchStatus?.status || '—' }}
          </NDescriptionsItem>
        </NDescriptions>
      </NCard>

      <NCard size="small" title="JSON Preview (for ENTRY)">
        <NCode :code="copyPreview || '(empty)'" language="json" word-wrap />
      </NCard>
    </NSpace>
  </div>
</template>
