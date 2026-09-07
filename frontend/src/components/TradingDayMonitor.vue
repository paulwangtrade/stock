<script setup>
/**
 * Phase11-C.1 Trading Day Monitor UI — read-only.
 * Data: GET /api/trading/day-monitor only. No trade actions.
 */
import { computed, onMounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NEmpty,
  NSpace,
  NSpin,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import { getTradingDayMonitor } from '../api/tradingDayMonitor'

const message = useMessage()
const loading = ref(false)
const errorMessage = ref('')
const view = ref(null)

const morning = computed(() => view.value?.morning || null)
const execution = computed(() => view.value?.execution || null)
const settlement = computed(() => view.value?.settlement || null)

function statusTagType(status) {
  const s = String(status || '').toUpperCase()
  if (s === 'PASS') return 'success'
  if (s === 'FAIL') return 'error'
  if (s === 'SKIP') return 'warning'
  if (s === 'PENDING') return 'info'
  return 'default'
}

function formatPlanId(id) {
  const n = Number(id)
  if (!Number.isFinite(n) || n <= 0) return '—'
  return String(n)
}

async function refresh() {
  loading.value = true
  errorMessage.value = ''
  try {
    view.value = await getTradingDayMonitor()
  } catch (e) {
    view.value = null
    errorMessage.value = e?.message || String(e)
    message.error(errorMessage.value)
  } finally {
    loading.value = false
  }
}

onMounted(refresh)
</script>

<template>
  <div class="trading-day-monitor">
    <n-space justify="space-between" align="center" style="margin-bottom: 12px">
      <n-space align="center" :wrap="true">
        <n-text strong style="font-size: 16px">Trading Day Monitor</n-text>
        <n-tag size="small" type="info" :bordered="false">只读</n-tag>
        <n-text v-if="view?.tradeDate" depth="3">业务日 {{ view.tradeDate }}</n-text>
      </n-space>
      <n-button :loading="loading" @click="refresh">刷新</n-button>
    </n-space>

    <n-alert type="warning" :bordered="false" style="margin-bottom: 14px">
      {{ view?.disclaimer || '只读交易日观察。不修改 Gateway / Automation / Settlement。' }}
    </n-alert>

    <n-spin :show="loading">
      <template v-if="errorMessage && !view">
        <n-empty :description="errorMessage" />
      </template>

      <template v-else-if="view">
        <!-- Morning -->
        <n-text strong style="display: block; margin-bottom: 8px">Morning</n-text>
        <div class="section-block">
          <div class="row">
            <n-text class="label">Materialize</n-text>
            <n-tag size="small" :type="statusTagType(morning?.materialize?.status)" :bordered="false">
              {{ morning?.materialize?.status || '—' }}
            </n-tag>
            <n-text depth="3" class="meta">
              plan_id={{ formatPlanId(morning?.materialize?.planId) }}
              <template v-if="morning?.materialize?.reason"> · {{ morning.materialize.reason }}</template>
              <template v-if="morning?.materialize?.source"> · {{ morning.materialize.source }}</template>
            </n-text>
          </div>
          <div class="row">
            <n-text class="label">Approve</n-text>
            <n-tag size="small" :type="statusTagType(morning?.approve?.status)" :bordered="false">
              {{ morning?.approve?.status || '—' }}
            </n-tag>
            <n-text depth="3" class="meta">
              plan_id={{ formatPlanId(morning?.approve?.planId) }}
              <template v-if="morning?.approve?.reason"> · {{ morning.approve.reason }}</template>
              <template v-if="morning?.approve?.source"> · {{ morning.approve.source }}</template>
            </n-text>
          </div>
          <div class="row">
            <n-text class="label">Freeze</n-text>
            <n-tag size="small" :type="statusTagType(morning?.freeze?.status)" :bordered="false">
              {{ morning?.freeze?.status || '—' }}
            </n-tag>
            <n-text depth="3" class="meta">
              plan_id={{ formatPlanId(morning?.freeze?.planId) }}
              <template v-if="morning?.freeze?.reason"> · {{ morning.freeze.reason }}</template>
              <template v-if="morning?.freeze?.source"> · {{ morning.freeze.source }}</template>
            </n-text>
          </div>
        </div>

        <!-- Execution -->
        <n-text strong style="display: block; margin: 16px 0 8px">Execution</n-text>
        <div class="section-block">
          <div class="row">
            <n-text class="label">Session</n-text>
            <n-text>{{ execution?.session || '—' }}</n-text>
          </div>
          <div class="row">
            <n-text class="label">Status</n-text>
            <n-tag size="small" :type="statusTagType(execution?.status)" :bordered="false">
              {{ execution?.status || '—' }}
            </n-tag>
          </div>
          <div class="row">
            <n-text class="label">plan_id</n-text>
            <n-text>{{ formatPlanId(execution?.planId) }}</n-text>
          </div>
          <div class="row">
            <n-text class="label">reason</n-text>
            <n-text>{{ execution?.reason || '—' }}</n-text>
          </div>
        </div>

        <!-- Settlement -->
        <n-text strong style="display: block; margin: 16px 0 8px">Settlement</n-text>
        <div class="section-block">
          <div class="row">
            <n-text class="label">Status</n-text>
            <n-tag size="small" :type="statusTagType(settlement?.status)" :bordered="false">
              {{ settlement?.status || '—' }}
            </n-tag>
            <n-text v-if="settlement?.reason" depth="3" class="meta">· {{ settlement.reason }}</n-text>
          </div>
        </div>

        <n-text depth="3" style="display: block; margin-top: 16px; font-size: 12px">
          {{ view.dataSourceNote }}
        </n-text>
      </template>
    </n-spin>
  </div>
</template>

<style scoped>
.trading-day-monitor {
  padding: 12px 16px 24px;
  max-width: 900px;
}
.section-block {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 10px 12px;
  background: rgba(128, 128, 128, 0.06);
  border-radius: 6px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
}
.label {
  min-width: 100px;
  font-weight: 600;
}
.meta {
  font-size: 12px;
}
</style>
