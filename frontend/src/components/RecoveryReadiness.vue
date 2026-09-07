<script setup>
import { computed, onMounted, ref } from 'vue'
import {
  NButton,
  NEmpty,
  NSpace,
  NSpin,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import { getRecoveryReadiness } from '../api/recoveryReadiness'

const message = useMessage()
const loading = ref(false)
const errorMessage = ref('')
const data = ref(null)

const hasData = computed(() => !!data.value)

function statusTagType(v) {
  const s = String(v || '')
  if (s === 'READY') return 'success'
  if (s === 'DEGRADED') return 'warning'
  if (s === 'BLOCKED') return 'error'
  return 'default'
}

function hintTagType(hint) {
  const s = String(hint || '').toUpperCase()
  if (s.includes('READY') || s.includes('OK') || s.includes('PASS')) return 'success'
  if (s.includes('DEGRADED') || s.includes('WARN') || s.includes('MISSING') || s.includes('LAG')) {
    return 'warning'
  }
  if (s.includes('BLOCK') || s.includes('FAIL') || s.includes('CORRUPT') || s.includes('GAP')) {
    return 'error'
  }
  return 'default'
}

function severityTagType(severity) {
  const s = String(severity || '').toUpperCase()
  if (s === 'ERROR' || s === 'CRITICAL' || s === 'BLOCKING') return 'error'
  if (s === 'WARN' || s === 'WARNING') return 'warning'
  return 'default'
}

function entriesOf(obj) {
  if (!obj || typeof obj !== 'object') return []
  return Object.entries(obj)
}

async function refresh() {
  loading.value = true
  errorMessage.value = ''
  try {
    data.value = await getRecoveryReadiness()
  } catch (e) {
    data.value = null
    errorMessage.value = e?.message || String(e)
    message.error(errorMessage.value)
  } finally {
    loading.value = false
  }
}

onMounted(refresh)
</script>

<template>
  <div class="recovery-readiness">
    <n-space justify="space-between" align="center" style="margin-bottom: 12px">
      <n-space align="center">
        <n-text strong>Recovery Readiness</n-text>
        <n-tag size="small" type="info" :bordered="false">只读</n-tag>
      </n-space>
      <n-button :loading="loading" @click="refresh">刷新</n-button>
    </n-space>

    <n-spin :show="loading">
      <template v-if="hasData">
        <!-- 总体 status -->
        <n-space align="center" :wrap="true" style="margin-bottom: 16px">
          <n-text>Status：</n-text>
          <n-tag size="medium" :type="statusTagType(data.status)" :bordered="false">
            {{ data.status || '—' }}
          </n-tag>
          <n-text depth="3">观测恢复就绪门禁；不触发 Repair / Replay / Execution</n-text>
        </n-space>

        <!-- checkpoint_status -->
        <n-text strong style="display: block; margin-bottom: 6px">Checkpoint Status</n-text>
        <n-space vertical :size="6" style="margin-bottom: 14px">
          <n-space align="center" :wrap="true">
            <n-text>Hint：</n-text>
            <n-tag
              size="small"
              :type="hintTagType(data.checkpoint_status?.status_hint)"
              :bordered="false"
            >
              {{ data.checkpoint_status?.status_hint || '—' }}
            </n-tag>
            <n-text depth="3">
              scopes={{ data.checkpoint_status?.scope_count ?? 0 }}
              · intact={{ data.checkpoint_status?.intact_count ?? 0 }}
              · corrupt={{ data.checkpoint_status?.corrupt_count ?? 0 }}
              · contract_mismatch={{ data.checkpoint_status?.contract_mismatch_count ?? 0 }}
            </n-text>
          </n-space>
          <n-space align="center" :wrap="true">
            <n-text>Baseline：</n-text>
            <n-tag
              size="small"
              :type="data.checkpoint_status?.missing_baseline ? 'warning' : 'success'"
              :bordered="false"
            >
              {{ data.checkpoint_status?.missing_baseline ? 'MISSING' : 'PRESENT' }}
            </n-tag>
            <n-text depth="3">max_lag_events={{ data.checkpoint_status?.max_lag_events ?? 0 }}</n-text>
          </n-space>
          <div v-if="(data.checkpoint_status?.samples || []).length" class="rr-list">
            <n-text depth="3" style="display: block; margin-bottom: 4px">Samples</n-text>
            <ul>
              <li
                v-for="(s, i) in data.checkpoint_status.samples"
                :key="'cp-' + i"
              >
                <n-space align="center" :wrap="true" :size="6">
                  <n-tag size="tiny" :type="s.intact ? 'success' : 'error'" :bordered="false">
                    {{ s.intact ? 'intact' : 'broken' }}
                  </n-tag>
                  <n-text>
                    {{ s.scope || '—' }} · audit={{ s.last_audit_id }} · v{{ s.version }}
                    · lag={{ s.lag_events }}
                  </n-text>
                  <n-text v-if="s.error" depth="3">{{ s.error }}</n-text>
                </n-space>
              </li>
            </ul>
          </div>
        </n-space>

        <!-- audit_continuity -->
        <n-text strong style="display: block; margin-bottom: 6px">Audit Continuity</n-text>
        <n-space vertical :size="6" style="margin-bottom: 14px">
          <n-space align="center" :wrap="true">
            <n-text>Hint：</n-text>
            <n-tag
              size="small"
              :type="hintTagType(data.audit_continuity?.status_hint)"
              :bordered="false"
            >
              {{ data.audit_continuity?.status_hint || '—' }}
            </n-tag>
          </n-space>
          <n-text depth="3">
            max_audit_id={{ data.audit_continuity?.max_audit_id ?? 0 }}
            · event_count={{ data.audit_continuity?.event_count ?? 0 }}
            · high_watermark={{ data.audit_continuity?.high_watermark ?? 0 }}
            · window_checked={{ data.audit_continuity?.window_checked ? 'true' : 'false' }}
            · gap_count={{ data.audit_continuity?.gap_count ?? 0 }}
            · anchor_mismatch={{ data.audit_continuity?.anchor_mismatch ?? 0 }}
          </n-text>
        </n-space>

        <!-- replay_verification -->
        <n-text strong style="display: block; margin-bottom: 6px">Replay Verification</n-text>
        <n-space vertical :size="6" style="margin-bottom: 14px">
          <n-space align="center" :wrap="true">
            <n-text>Hint：</n-text>
            <n-tag
              size="small"
              :type="hintTagType(data.replay_verification?.status_hint)"
              :bordered="false"
            >
              {{ data.replay_verification?.status_hint || '—' }}
            </n-tag>
            <n-text depth="3">mode={{ data.replay_verification?.mode || '—' }}</n-text>
          </n-space>
          <n-text depth="3">
            sample_size={{ data.replay_verification?.sample_size ?? 0 }}
            · passed={{ data.replay_verification?.passed_count ?? 0 }}
            · failed={{ data.replay_verification?.failed_count ?? 0 }}
            · skipped={{ data.replay_verification?.skipped_count ?? 0 }}
          </n-text>
          <n-text v-if="data.replay_verification?.last_error" type="error">
            last_error：{{ data.replay_verification.last_error }}
          </n-text>
        </n-space>

        <!-- divergence_summary -->
        <n-text strong style="display: block; margin-bottom: 6px">Divergence Summary</n-text>
        <n-space vertical :size="6" style="margin-bottom: 14px">
          <n-space align="center" :wrap="true">
            <n-text>Total：{{ data.divergence_summary?.total ?? 0 }}</n-text>
            <n-text depth="3">|</n-text>
            <n-text>Blocking：</n-text>
            <n-tag
              size="small"
              :type="(data.divergence_summary?.blocking_count || 0) > 0 ? 'error' : 'success'"
              :bordered="false"
            >
              {{ data.divergence_summary?.blocking_count ?? 0 }}
            </n-tag>
          </n-space>
          <n-space v-if="entriesOf(data.divergence_summary?.by_kind).length" align="center" :wrap="true">
            <n-text depth="3">by_kind：</n-text>
            <n-tag
              v-for="([k, v]) in entriesOf(data.divergence_summary.by_kind)"
              :key="'kind-' + k"
              size="tiny"
              :bordered="false"
            >
              {{ k }}={{ v }}
            </n-tag>
          </n-space>
          <n-space
            v-if="entriesOf(data.divergence_summary?.by_severity).length"
            align="center"
            :wrap="true"
          >
            <n-text depth="3">by_severity：</n-text>
            <n-tag
              v-for="([k, v]) in entriesOf(data.divergence_summary.by_severity)"
              :key="'sev-' + k"
              size="tiny"
              :type="severityTagType(k)"
              :bordered="false"
            >
              {{ k }}={{ v }}
            </n-tag>
          </n-space>
          <div v-if="(data.divergence_summary?.top || []).length" class="rr-list">
            <n-text depth="3" style="display: block; margin-bottom: 4px">Top</n-text>
            <ul>
              <li
                v-for="(d, i) in data.divergence_summary.top"
                :key="'div-' + i"
              >
                <n-space align="center" :wrap="true" :size="6">
                  <n-tag
                    size="tiny"
                    :type="d.blocking ? 'error' : severityTagType(d.severity)"
                    :bordered="false"
                  >
                    {{ d.blocking ? 'blocking' : d.severity || 'info' }}
                  </n-tag>
                  <n-text>
                    {{ d.kind || '—' }}
                    <template v-if="d.scope"> · {{ d.scope }}</template>
                    <template v-if="d.event_type"> · {{ d.event_type }}</template>
                    <template v-if="d.event_id"> · {{ d.event_id }}</template>
                  </n-text>
                  <n-text v-if="d.detail" depth="3">{{ d.detail }}</n-text>
                </n-space>
              </li>
            </ul>
          </div>
          <n-text v-else depth="3">无 divergence 条目</n-text>
        </n-space>
      </template>

      <n-empty
        v-else-if="errorMessage"
        :description="errorMessage"
        style="margin-top: 24px"
      />
      <n-empty
        v-else-if="!loading"
        description="暂无 Recovery Readiness 数据"
        style="margin-top: 24px"
      />
    </n-spin>
  </div>
</template>

<style scoped>
.recovery-readiness {
  padding: 12px 16px;
}
.rr-list ul {
  margin: 0;
  padding-left: 18px;
}
.rr-list li {
  margin-bottom: 4px;
}
</style>
