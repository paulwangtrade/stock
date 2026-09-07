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
import { DIVERGENCE_KINDS, getBrokerReconcile } from '../api/brokerReconcile'

const message = useMessage()
const loading = ref(false)
const errorMessage = ref('')
const data = ref(null)

const hasData = computed(() => !!data.value)

const kindRows = computed(() => {
  const by = data.value?.by_kind || {}
  return DIVERGENCE_KINDS.map((kind) => ({
    kind,
    count: Number(by[kind]) || 0,
  }))
})

function statusTagType(v) {
  const s = String(v || '')
  if (s === 'READY') return 'success'
  if (s === 'DEGRADED') return 'warning'
  if (s === 'ATTENTION' || s === 'OBSERVATION_ERROR') return 'error'
  return 'default'
}

function severityTagType(severity) {
  const s = String(severity || '').toLowerCase()
  if (s === 'attention' || s === 'error' || s === 'critical') return 'error'
  if (s === 'warn' || s === 'warning') return 'warning'
  return 'default'
}

function formatCheckedAt(v) {
  if (!v) return '—'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return String(v)
  return d.toLocaleString()
}

async function refresh() {
  loading.value = true
  errorMessage.value = ''
  try {
    data.value = await getBrokerReconcile()
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
  <div class="broker-reconcile">
    <n-space justify="space-between" align="center" style="margin-bottom: 12px">
      <n-space align="center">
        <n-text strong>Broker 对账</n-text>
        <n-tag size="small" type="info" :bordered="false">只读</n-tag>
      </n-space>
      <n-button :loading="loading" @click="refresh">刷新</n-button>
    </n-space>

    <n-spin :show="loading">
      <template v-if="hasData">
        <n-space align="center" :wrap="true" style="margin-bottom: 16px">
          <n-text>Status：</n-text>
          <n-tag size="medium" :type="statusTagType(data.status)" :bordered="false">
            {{ data.status || '—' }}
          </n-tag>
          <n-text depth="3">通道观察对账；不触发 Repair / Retry / Cancel / 交易</n-text>
        </n-space>

        <n-text strong style="display: block; margin-bottom: 6px">检查摘要</n-text>
        <n-space vertical :size="6" style="margin-bottom: 14px">
          <n-text>
            总检查订单数：{{ data.total_orders ?? 0 }}
          </n-text>
          <n-text>
            divergence 总数：{{ data.divergence_count ?? 0 }}
          </n-text>
          <n-text depth="3">
            checked_at：{{ formatCheckedAt(data.checked_at) }}
          </n-text>
          <n-text v-if="data.observation_error" type="error">
            observation_error：{{ data.observation_error }}
          </n-text>
        </n-space>

        <n-text strong style="display: block; margin-bottom: 6px">Divergence 分类</n-text>
        <n-space align="center" :wrap="true" style="margin-bottom: 14px">
          <n-tag
            v-for="row in kindRows"
            :key="'kind-' + row.kind"
            size="small"
            :type="row.count > 0 ? 'warning' : 'default'"
            :bordered="false"
          >
            {{ row.kind }}={{ row.count }}
          </n-tag>
        </n-space>

        <n-text strong style="display: block; margin-bottom: 6px">Divergence 明细</n-text>
        <div v-if="(data.divergences || []).length" class="br-list">
          <ul>
            <li
              v-for="(d, i) in data.divergences"
              :key="'div-' + i"
            >
              <n-space align="center" :wrap="true" :size="6">
                <n-tag size="tiny" :type="severityTagType(d.severity)" :bordered="false">
                  {{ d.severity || 'info' }}
                </n-tag>
                <n-text>
                  {{ d.kind || '—' }}
                  <template v-if="d.order_id"> · {{ d.order_id }}</template>
                </n-text>
                <n-text v-if="d.detail" depth="3">{{ d.detail }}</n-text>
              </n-space>
            </li>
          </ul>
        </div>
        <n-text v-else depth="3">无 divergence 条目</n-text>
      </template>

      <n-empty
        v-else-if="errorMessage"
        :description="errorMessage"
        style="margin-top: 24px"
      />
      <n-empty
        v-else-if="!loading"
        description="暂无 Broker Reconcile 数据"
        style="margin-top: 24px"
      />
    </n-spin>
  </div>
</template>

<style scoped>
.broker-reconcile {
  padding: 12px 16px;
}
.br-list ul {
  margin: 0;
  padding-left: 18px;
}
.br-list li {
  margin-bottom: 6px;
}
</style>
