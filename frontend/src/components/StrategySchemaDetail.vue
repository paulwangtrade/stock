<script setup>
/**
 * Phase13-B3-C：Schema 详情 + Revision 时间线 + Create New Revision
 */
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NEmpty, NSkeleton, NSpace, NTag, NText, NThing, useMessage } from 'naive-ui'
import {
  createStrategySchemaDraft,
  getStrategySchema,
  revStatusLabel,
} from '../api/strategySchemas'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const loading = ref(false)
const forking = ref(false)
const detail = ref(null)

const strategyId = computed(() => String(route.query.id || ''))

const hasWorkingCopy = computed(() =>
  (detail.value?.revisions || []).some((r) => r.status === 'draft' || r.status === 'reviewing'),
)

const activeRevision = computed(() =>
  (detail.value?.revisions || []).find((r) => r.status === 'active'),
)

async function load() {
  if (!strategyId.value) {
    detail.value = null
    return
  }
  loading.value = true
  try {
    detail.value = await getStrategySchema(strategyId.value)
  } catch (e) {
    message.error(e?.message || String(e))
    detail.value = null
  } finally {
    loading.value = false
  }
}

function openRevision(rev) {
  router.push({
    name: 'strategyRevisionView',
    query: { id: strategyId.value, version: rev.revision },
  })
}

async function createNewRevision() {
  if (!detail.value?.definition || !activeRevision.value) {
    message.warning('需要已发布的 active revision')
    return
  }
  if (hasWorkingCopy.value) {
    message.warning('已有 draft/reviewing，请先完成或打开该工作副本')
    const work = (detail.value.revisions || []).find(
      (r) => r.status === 'draft' || r.status === 'reviewing',
    )
    if (work) openRevision(work)
    return
  }
  forking.value = true
  try {
    const def = detail.value.definition
    const parent = activeRevision.value.revision
    const out = await createStrategySchemaDraft({
      strategy_id: def.strategy_id,
      name: def.name,
      description: def.description,
      parent_revision: parent,
      revision_note: `fork of ${parent}`,
      // empty content → server copies parent
      universe: {},
      signals: {},
      filters: {},
      ranking: {},
      risk_profile_ref: { mode: 'inherit' },
    })
    message.success(`已创建新草稿 ${out.revision.revision}`)
    await load()
    router.push({
      name: 'strategyRevisionView',
      query: { id: out.definition.strategy_id, version: out.revision.revision },
    })
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    forking.value = false
  }
}

function formatTime(v) {
  if (!v) return '—'
  try {
    const d = new Date(v)
    if (Number.isNaN(d.getTime())) return v
    return d.toLocaleString()
  } catch {
    return v
  }
}

function shortHash(h) {
  if (!h) return '—'
  return h.length > 12 ? `${h.slice(0, 10)}…` : h
}

watch(strategyId, load)
onMounted(load)
</script>

<template>
  <div class="schema-detail">
    <n-space justify="space-between" align="center">
      <n-button quaternary @click="router.push({ name: 'strategySchemas' })">← 返回列表</n-button>
      <n-text depth="3">无 AI / Intent / Promote / 下单</n-text>
    </n-space>

    <n-skeleton v-if="loading" text :repeat="8" />
    <n-empty v-else-if="!strategyId" description="缺少 id 参数" />
    <n-empty v-else-if="!detail" description="未找到策略模板" />
    <template v-else>
      <n-thing :title="detail.definition.name" :description="detail.definition.description || '无描述'">
        <template #header-extra>
          <n-space>
            <n-tag size="small" :bordered="false">{{ detail.definition.status }}</n-tag>
            <n-button
              v-if="activeRevision"
              size="small"
              type="primary"
              secondary
              :loading="forking"
              @click="createNewRevision"
            >
              Create New Revision
            </n-button>
          </n-space>
        </template>
        <n-space vertical :size="4">
          <n-text depth="3">ID：{{ detail.definition.strategy_id }}</n-text>
          <n-text depth="3">契约：{{ detail.definition.schema_version }}</n-text>
          <n-text depth="3">当前 Revision：{{ detail.definition.current_revision || '—' }}</n-text>
          <n-text depth="3">更新：{{ formatTime(detail.definition.updated_at) }}</n-text>
          <n-text v-if="activeRevision" depth="3" style="font-size: 12px">
            active 内容冻结；修改请 Create New Revision（生成新 draft）
          </n-text>
        </n-space>
      </n-thing>

      <n-text strong style="margin-top: 16px; display: block">Revision 时间线</n-text>
      <n-empty v-if="!(detail.revisions || []).length" description="无 Revision" />
      <div v-for="rev in detail.revisions" :key="rev.revision_id" class="rev-row">
        <n-space justify="space-between" align="center">
          <div>
            <n-text>{{ rev.revision }}</n-text>
            <n-tag size="tiny" :bordered="false" style="margin-left: 8px">
              {{ revStatusLabel(rev.status) }}
            </n-tag>
            <n-text depth="3" style="display: block; font-size: 12px">
              {{ rev.revision_note || '—' }} · params={{ shortHash(rev.params_hash) }} ·
              rev={{ shortHash(rev.revision_hash) }}
            </n-text>
          </div>
          <n-button size="small" secondary @click="openRevision(rev)">
            {{ rev.status === 'draft' ? '编辑' : '查看' }}
          </n-button>
        </n-space>
      </div>
    </template>
  </div>
</template>

<style scoped>
.schema-detail {
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  height: 100%;
  box-sizing: border-box;
  overflow: auto;
}
.rev-row {
  padding: 10px 0;
  border-bottom: 1px solid rgba(128, 128, 128, 0.2);
}
</style>
