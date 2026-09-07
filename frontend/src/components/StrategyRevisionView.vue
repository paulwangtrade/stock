<script setup>
/**
 * Phase13-B3-C：Revision 查看 / Draft 编辑 / Submit / Activate / Create New Revision
 */
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NButton,
  NCode,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NSkeleton,
  NSpace,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import {
  activateStrategyRevision,
  createStrategySchemaDraft,
  getStrategyRevision,
  getStrategySchema,
  isDraftEditable,
  isRevisionFrozen,
  revStatusLabel,
  submitStrategyRevision,
  updateStrategySchemaDraft,
} from '../api/strategySchemas'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const loading = ref(false)
const saving = ref(false)
const submitting = ref(false)
const activating = ref(false)
const forking = ref(false)
const rev = ref(null)
const defName = ref('')

const strategyId = computed(() => String(route.query.id || ''))
const version = computed(() => String(route.query.version || ''))
const editable = computed(() => isDraftEditable(rev.value?.status || ''))
const frozen = computed(() => isRevisionFrozen(rev.value?.status || ''))

const edit = reactive({
  revision_note: '',
  universe_json: '',
  signals_json: '',
  filters_json: '',
  ranking_json: '',
  risk_json: '',
  knobs_json: '',
})

function pretty(obj) {
  try {
    return JSON.stringify(obj ?? {}, null, 2)
  } catch {
    return String(obj)
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

function fillEdit(r) {
  edit.revision_note = r.revision_note || ''
  edit.universe_json = pretty(r.universe)
  edit.signals_json = pretty(r.signals)
  edit.filters_json = pretty(r.filters)
  edit.ranking_json = pretty(r.ranking)
  edit.risk_json = pretty(r.risk_profile_ref)
  edit.knobs_json = pretty(r.parameters?.knobs || {})
}

function parseJSON(label, text) {
  try {
    return JSON.parse(text || '{}')
  } catch {
    throw new Error(`${label} JSON 无效`)
  }
}

async function load() {
  if (!strategyId.value || !version.value) {
    rev.value = null
    return
  }
  loading.value = true
  try {
    rev.value = await getStrategyRevision(strategyId.value, version.value)
    fillEdit(rev.value)
    try {
      const detail = await getStrategySchema(strategyId.value)
      defName.value = detail.definition?.name || ''
    } catch {
      defName.value = ''
    }
  } catch (e) {
    message.error(e?.message || String(e))
    rev.value = null
  } finally {
    loading.value = false
  }
}

async function saveDraft(opts = { quiet: false }) {
  if (!editable.value) {
    message.warning('仅 draft 可编辑')
    return false
  }
  saving.value = true
  try {
    const payload = {
      revision_note: edit.revision_note,
      universe: parseJSON('universe', edit.universe_json),
      signals: parseJSON('signals', edit.signals_json),
      filters: parseJSON('filters', edit.filters_json),
      ranking: parseJSON('ranking', edit.ranking_json),
      risk_profile_ref: parseJSON('risk_profile_ref', edit.risk_json),
      knobs: parseJSON('knobs', edit.knobs_json),
    }
    const out = await updateStrategySchemaDraft(strategyId.value, version.value, payload)
    rev.value = out.revision
    fillEdit(out.revision)
    if (!opts.quiet) message.success('Draft 已保存')
    return true
  } catch (e) {
    message.error(e?.message || String(e))
    return false
  } finally {
    saving.value = false
  }
}

async function onSubmit() {
  submitting.value = true
  try {
    if (editable.value) {
      const ok = await saveDraft({ quiet: true })
      if (!ok) return
    }
    const out = await submitStrategyRevision(strategyId.value, version.value)
    rev.value = out.revision
    fillEdit(out.revision)
    message.success('已提交审阅（reviewing）')
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    submitting.value = false
  }
}

async function onActivate() {
  activating.value = true
  try {
    const out = await activateStrategyRevision(strategyId.value, version.value)
    rev.value = out.revision
    fillEdit(out.revision)
    message.success('已发布（active）')
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    activating.value = false
  }
}

async function createNewRevision() {
  if (rev.value?.status !== 'active') {
    message.warning('仅 active 可通过 Create New Revision 派生草稿')
    return
  }
  forking.value = true
  try {
    const out = await createStrategySchemaDraft({
      strategy_id: strategyId.value,
      name: defName.value || strategyId.value,
      parent_revision: version.value,
      revision_note: `fork of ${version.value}`,
      universe: {},
      signals: {},
      filters: {},
      ranking: {},
      risk_profile_ref: { mode: 'inherit' },
    })
    message.success(`已创建新草稿 ${out.revision.revision}`)
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

watch([strategyId, version], load)
onMounted(load)
</script>

<template>
  <div class="rev-view">
    <n-space justify="space-between" align="center">
      <n-button
        quaternary
        @click="router.push({ name: 'strategySchemaDetail', query: { id: strategyId } })"
      >
        ← 返回详情
      </n-button>
      <n-text depth="3">
        {{ editable ? 'Draft 可编辑' : '只读（冻结）' }} · 无 AI / Promote
      </n-text>
    </n-space>

    <n-skeleton v-if="loading" text :repeat="10" />
    <n-empty v-else-if="!strategyId || !version" description="缺少 id/version" />
    <n-empty v-else-if="!rev" description="未找到 Revision" />
    <template v-else>
      <n-space align="center" wrap>
        <n-text strong>version：{{ rev.revision }}</n-text>
        <n-tag size="small" :bordered="false">{{ revStatusLabel(rev.status) }}</n-tag>
        <n-tag size="small" :bordered="false" type="info">{{ rev.source || 'manual' }}</n-tag>
      </n-space>

      <div class="meta">
        <n-text depth="3">status：{{ rev.status }}</n-text>
        <n-text depth="3">revision_id：{{ rev.revision_id }}</n-text>
        <n-text depth="3">params_hash：{{ rev.parameters?.params_hash || '—' }}</n-text>
        <n-text depth="3">revision_hash：{{ rev.revision_hash || '—' }}</n-text>
        <n-text depth="3">created_at：{{ formatTime(rev.created_at) }}</n-text>
        <n-text depth="3">updated_at：{{ formatTime(rev.updated_at) }}</n-text>
        <n-text v-if="rev.parent_revision" depth="3">parent：{{ rev.parent_revision }}</n-text>
      </div>

      <n-space wrap>
        <n-button
          v-if="editable"
          type="primary"
          :loading="saving"
          @click="saveDraft"
        >
          保存 Draft
        </n-button>
        <n-button
          v-if="editable"
          secondary
          type="warning"
          :loading="submitting"
          @click="onSubmit"
        >
          Submit Review
        </n-button>
        <n-button
          v-if="rev.status === 'reviewing'"
          type="primary"
          :loading="activating"
          @click="onActivate"
        >
          Activate
        </n-button>
        <n-button
          v-if="rev.status === 'active'"
          secondary
          :loading="forking"
          @click="createNewRevision"
        >
          Create New Revision
        </n-button>
      </n-space>

      <n-text v-if="frozen && rev.status !== 'draft'" depth="3" style="font-size: 12px">
        当前状态禁止直接编辑内容；active 请用 Create New Revision 生成新 draft。
      </n-text>

      <template v-if="editable">
        <n-form label-placement="top">
          <n-form-item label="revision_note">
            <n-input v-model:value="edit.revision_note" />
          </n-form-item>
          <n-form-item label="universe (JSON)">
            <n-input v-model:value="edit.universe_json" type="textarea" :rows="5" />
          </n-form-item>
          <n-form-item label="signals (JSON)">
            <n-input v-model:value="edit.signals_json" type="textarea" :rows="5" />
          </n-form-item>
          <n-form-item label="filters (JSON)">
            <n-input v-model:value="edit.filters_json" type="textarea" :rows="4" />
          </n-form-item>
          <n-form-item label="ranking (JSON)">
            <n-input v-model:value="edit.ranking_json" type="textarea" :rows="4" />
          </n-form-item>
          <n-form-item label="risk_profile_ref (JSON)">
            <n-input v-model:value="edit.risk_json" type="textarea" :rows="3" />
          </n-form-item>
          <n-form-item label="parameters.knobs (JSON)">
            <n-input v-model:value="edit.knobs_json" type="textarea" :rows="4" />
          </n-form-item>
        </n-form>
      </template>
      <template v-else>
        <n-text v-if="rev.revision_note">备注：{{ rev.revision_note }}</n-text>
        <n-text strong>universe</n-text>
        <n-code :code="pretty(rev.universe)" language="json" word-wrap />
        <n-text strong>signals</n-text>
        <n-code :code="pretty(rev.signals)" language="json" word-wrap />
        <n-text strong>filters</n-text>
        <n-code :code="pretty(rev.filters)" language="json" word-wrap />
        <n-text strong>ranking</n-text>
        <n-code :code="pretty(rev.ranking)" language="json" word-wrap />
        <n-text strong>risk_profile_ref</n-text>
        <n-code :code="pretty(rev.risk_profile_ref)" language="json" word-wrap />
        <n-text strong>parameters</n-text>
        <n-code :code="pretty(rev.parameters)" language="json" word-wrap />
      </template>
    </template>
  </div>
</template>

<style scoped>
.rev-view {
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  height: 100%;
  box-sizing: border-box;
  overflow: auto;
}
.meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 12px;
}
</style>
