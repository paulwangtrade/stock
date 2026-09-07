<script setup>
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import {
  NButton,
  NEmpty,
  NInput,
  NSpace,
  NSpin,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import {
  OPS_TD_CODE_BAD_TRADE_DATE,
  OPS_TD_CODE_NOT_FOUND,
  OPS_TD_CODE_OK,
  getTradingDayStatus,
} from '../api/tradingDayStatus'

const message = useMessage()
const loading = ref(false)
const tradeDateInput = ref('')
const requestTradeDate = ref('')
const emptyMessage = ref('')
const status = ref(null)

const hasStatus = computed(() => !!status.value)

function schemaTagType(v) {
  const s = String(v || '')
  if (s === 'READY') return 'success'
  if (s.includes('BLOCKED')) return 'error'
  return 'default'
}

function afterCloseTagType(v) {
  const s = String(v || '')
  if (s === 'COMPLETED') return 'success'
  if (s === 'RISK_FAILED') return 'error'
  if (s === 'NOT_FOUND' || s === 'SKIPPED_DISABLED') return 'warning'
  return 'default'
}

function morningTagType(v) {
  return String(v || '') === 'adopt_frozen' ? 'success' : 'warning'
}

function guardTagType(v) {
  const s = String(v || '')
  if (s === 'WOULD_PASS') return 'success'
  if (s === 'N/A') return 'default'
  return 'error'
}

async function refresh() {
  loading.value = true
  emptyMessage.value = ''
  try {
    const res = await getTradingDayStatus(tradeDateInput.value || undefined)
    requestTradeDate.value = res.trade_date || tradeDateInput.value || ''

    if (res.code === OPS_TD_CODE_NOT_FOUND) {
      status.value = null
      emptyMessage.value = res.message || '暂无交易日状态'
      return
    }
    if (res.code === OPS_TD_CODE_BAD_TRADE_DATE) {
      status.value = null
      emptyMessage.value = res.message || 'trade_date 无效'
      message.error(emptyMessage.value)
      return
    }
    if (res.code !== OPS_TD_CODE_OK) {
      status.value = null
      emptyMessage.value = res.message || `加载失败 (code=${res.code})`
      message.error(emptyMessage.value)
      return
    }
    status.value = res.data
    if (!res.data) {
      emptyMessage.value = res.message || '暂无交易日状态'
    }
  } catch (e) {
    status.value = null
    emptyMessage.value = e?.message || String(e)
    message.error(emptyMessage.value)
  } finally {
    loading.value = false
  }
}

onMounted(refresh)
</script>

<template>
  <div class="production-readiness">
    <n-space justify="space-between" align="center" style="margin-bottom: 12px">
      <n-space align="center">
        <n-text strong>生产就绪</n-text>
        <n-tag size="small" type="info" :bordered="false">只读</n-tag>
      </n-space>
      <n-space align="center">
        <n-input
          v-model:value="tradeDateInput"
          placeholder="trade_date YYYY-MM-DD（可选）"
          clearable
          style="width: 220px"
        />
        <n-button :loading="loading" @click="refresh">刷新</n-button>
      </n-space>
    </n-space>

    <n-spin :show="loading">
      <template v-if="hasStatus">
        <!-- 结论条 -->
        <n-space vertical :size="10" style="margin-bottom: 16px">
          <n-space align="center" :wrap="true">
            <n-text>ExecutionReady：</n-text>
            <n-tag
              size="small"
              :type="status.execution_ready?.ready ? 'success' : 'error'"
              :bordered="false"
            >
              {{ status.execution_ready?.ready ? 'READY' : 'NOT_READY' }}
            </n-tag>
            <n-text depth="3">|</n-text>
            <n-text>Guard（须 Frozen）：</n-text>
            <n-tag
              size="small"
              :type="guardTagType(status.execution_ready?.guard_status)"
              :bordered="false"
            >
              {{ status.execution_ready?.guard_status || '—' }}
            </n-tag>
            <template v-if="status.execution_ready?.reason">
              <n-text depth="3">|</n-text>
              <n-text depth="3">{{ status.execution_ready.reason }}</n-text>
            </template>
          </n-space>
          <n-space align="center" :wrap="true">
            <n-text>TradeDate：{{ status.date || requestTradeDate || '—' }}</n-text>
            <n-text depth="3">|</n-text>
            <n-text>Schema：</n-text>
            <n-tag
              size="small"
              :type="schemaTagType(status.schema?.validation_status)"
              :bordered="false"
            >
              {{ status.schema?.validation_status || '—' }}
              v{{ status.schema?.registry_version ?? '—' }}
            </n-tag>
            <n-text depth="3">|</n-text>
            <n-text>Cron：</n-text>
            <n-tag
              size="small"
              :type="status.cron?.after_close_registered ? 'success' : 'warning'"
              :bordered="false"
            >
              after_close {{ status.cron?.after_close_registered ? '✓' : '×' }}
            </n-tag>
            <n-tag
              size="small"
              :type="status.cron?.morning_registered ? 'success' : 'warning'"
              :bordered="false"
            >
              morning {{ status.cron?.morning_registered ? '✓' : '×' }}
            </n-tag>
          </n-space>
        </n-space>

        <!-- Infra -->
        <n-text strong style="display: block; margin-bottom: 6px">Infra</n-text>
        <n-space vertical :size="6" style="margin-bottom: 14px">
          <n-space align="center" :wrap="true">
            <n-text>Schema validation：</n-text>
            <n-tag
              size="small"
              :type="schemaTagType(status.schema?.validation_status)"
              :bordered="false"
            >
              {{ status.schema?.validation_status || '—' }}
            </n-tag>
            <n-text depth="3">registry_version={{ status.schema?.registry_version ?? '—' }}</n-text>
          </n-space>
          <n-space align="center" :wrap="true">
            <n-text>Cron registered：</n-text>
            <n-text depth="3">
              after_close={{ status.cron?.after_close_registered ? 'true' : 'false' }}
              · morning={{ status.cron?.morning_registered ? 'true' : 'false' }}
            </n-text>
          </n-space>
        </n-space>

        <!-- Pipeline -->
        <n-text strong style="display: block; margin-bottom: 6px">Pipeline</n-text>
        <n-space vertical :size="8" style="margin-bottom: 14px">
          <n-space align="center" :wrap="true">
            <n-text>AfterClose：</n-text>
            <n-tag
              size="small"
              :type="afterCloseTagType(status.after_close?.status)"
              :bordered="false"
            >
              {{ status.after_close?.status || '—' }}
            </n-tag>
            <n-text depth="3">pool_id={{ status.after_close?.pool_id || '—' }}</n-text>
            <n-text depth="3">plan_id={{ status.after_close?.plan_id || '—' }}</n-text>
            <n-text v-if="status.after_close?.last_run" depth="3">
              last_run={{ status.after_close.last_run }}
            </n-text>
          </n-space>

          <n-space align="center" :wrap="true">
            <n-text>TradePlan：</n-text>
            <n-tag
              size="small"
              :type="status.plan?.exists ? 'success' : 'warning'"
              :bordered="false"
            >
              {{ status.plan?.exists ? 'exists' : 'empty' }}
            </n-tag>
            <template v-if="status.plan?.exists">
              <n-tag size="small" :bordered="false">{{ status.plan.status || '—' }}</n-tag>
              <n-text depth="3">v{{ status.plan.plan_version }}</n-text>
              <n-text depth="3">{{ status.plan.source_session || '—' }}</n-text>
            </template>
          </n-space>

          <n-space align="center" :wrap="true">
            <n-text>Risk：</n-text>
            <n-tag
              size="small"
              :type="status.risk?.passed ? 'success' : 'error'"
              :bordered="false"
            >
              {{ status.risk?.passed ? 'passed' : 'not_passed' }}
            </n-tag>
            <n-text depth="3">{{ status.risk?.status || '—' }}</n-text>
            <n-text v-if="status.risk?.reason" depth="3">{{ status.risk.reason }}</n-text>
          </n-space>

          <n-space align="center" :wrap="true">
            <n-text>Freeze：</n-text>
            <n-tag
              size="small"
              :type="status.freeze?.is_frozen ? 'success' : 'warning'"
              :bordered="false"
            >
              {{ status.freeze?.is_frozen ? 'FROZEN' : 'NOT_FROZEN' }}
            </n-tag>
            <n-text v-if="status.freeze?.freeze_at" depth="3">
              {{ status.freeze.freeze_at }}
            </n-text>
            <n-text v-if="status.freeze?.freeze_by" depth="3">
              by {{ status.freeze.freeze_by }}
            </n-text>
          </n-space>

          <n-space align="center" :wrap="true">
            <n-text>Morning：</n-text>
            <n-tag
              size="small"
              :type="morningTagType(status.morning?.mode)"
              :bordered="false"
            >
              {{ status.morning?.mode || '—' }}
            </n-tag>
          </n-space>

          <n-space align="center" :wrap="true">
            <n-text>ExecutionReady：</n-text>
            <n-tag
              size="small"
              :type="status.execution_ready?.ready ? 'success' : 'error'"
              :bordered="false"
            >
              {{ status.execution_ready?.ready ? 'READY' : 'NOT_READY' }}
            </n-tag>
            <n-text depth="3">={{ status.execution_ready?.guard_status || '—' }}</n-text>
          </n-space>
        </n-space>

        <n-text depth="3" style="display: block; margin-bottom: 8px">
          本页仅观测；不触发 Workflow / Approve / Freeze / Execute。
        </n-text>
        <RouterLink :to="{ name: 'tradePlanUpcoming' }">查看交易计划 →</RouterLink>
      </template>

      <n-empty
        v-else
        :description="emptyMessage || (requestTradeDate ? `无状态（${requestTradeDate}）` : '暂无交易日状态')"
      />
    </n-spin>
  </div>
</template>

<style scoped>
.production-readiness {
  height: 100%;
  overflow: auto;
  padding: 4px 2px 12px;
  box-sizing: border-box;
}
</style>
