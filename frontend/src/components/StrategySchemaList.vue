<script setup>
/**
 * Phase13-B3-C：Strategy Schema 列表 + Create Draft
 */
import { h, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  NButton,
  NDataTable,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NSkeleton,
  NSpace,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import { createStrategySchemaDraft, listStrategySchemas } from '../api/strategySchemas'

const message = useMessage()
const router = useRouter()
const loading = ref(false)
const rows = ref([])
const hint = ref('')
const showCreate = ref(false)
const creating = ref(false)
const form = reactive({
  name: '',
  slug: '',
  description: '',
  revision_note: '',
  universe_source: 'scan',
  signals_kind: 'deterministic',
  top_n: 20,
  knobs_json: '{\n  "lookback_days": 20\n}',
})

const columns = [
  { title: '名称', key: 'name', ellipsis: { tooltip: true } },
  {
    title: '状态',
    key: 'status',
    width: 120,
    render(row) {
      return h(NTag, { size: 'small', bordered: false }, { default: () => statusLabel(row.status) })
    },
  },
  { title: '当前 Revision', key: 'current_revision', width: 120, render: (r) => r.current_revision || '—' },
  {
    title: '更新',
    key: 'updated_at',
    width: 180,
    render(row) {
      return formatTime(row.updated_at)
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 100,
    render(row) {
      return h(
        NButton,
        {
          size: 'small',
          secondary: true,
          onClick: () =>
            router.push({ name: 'strategySchemaDetail', query: { id: row.strategy_id } }),
        },
        { default: () => '详情' },
      )
    },
  },
]

function statusLabel(s) {
  switch (s) {
    case 'active':
      return '已发布'
    case 'draft_only':
      return '仅草稿'
    case 'archived':
      return '已归档'
    default:
      return s || '—'
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

function resetForm() {
  form.name = ''
  form.slug = ''
  form.description = ''
  form.revision_note = ''
  form.universe_source = 'scan'
  form.signals_kind = 'deterministic'
  form.top_n = 20
  form.knobs_json = '{\n  "lookback_days": 20\n}'
}

async function refresh() {
  loading.value = true
  try {
    const res = await listStrategySchemas()
    rows.value = res.items || []
    hint.value = res.message || ''
  } catch (e) {
    message.error(e?.message || String(e))
    rows.value = []
  } finally {
    loading.value = false
  }
}

async function submitCreate() {
  if (!form.name.trim()) {
    message.warning('请填写名称')
    return
  }
  let knobs = {}
  try {
    knobs = form.knobs_json.trim() ? JSON.parse(form.knobs_json) : {}
  } catch {
    message.error('knobs JSON 无效')
    return
  }
  creating.value = true
  try {
    const out = await createStrategySchemaDraft({
      name: form.name.trim(),
      slug: form.slug.trim() || undefined,
      description: form.description.trim() || undefined,
      revision_note: form.revision_note.trim() || undefined,
      universe: { source: form.universe_source.trim() || 'scan' },
      signals: { kind: form.signals_kind.trim() || 'deterministic' },
      filters: {},
      ranking: { top_n: Number(form.top_n) || 20 },
      risk_profile_ref: { mode: 'inherit' },
      knobs,
    })
    message.success(`已创建草稿 ${out.revision?.revision}`)
    showCreate.value = false
    resetForm()
    await refresh()
    router.push({
      name: 'strategyRevisionView',
      query: { id: out.definition.strategy_id, version: out.revision.revision },
    })
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    creating.value = false
  }
}

onMounted(refresh)
</script>

<template>
  <div class="schema-list">
    <div class="toolbar">
      <div>
        <n-text strong>策略模板（Schema）</n-text>
        <n-text depth="3" class="hint">手工草稿 → 审阅 → 发布 · 无 AI / Intent / Promote / 下单</n-text>
      </div>
      <n-space>
        <n-button type="primary" @click="showCreate = true">新建 Draft</n-button>
        <n-button secondary :loading="loading" @click="refresh">刷新</n-button>
      </n-space>
    </div>
    <n-text v-if="hint && !loading" depth="3">{{ hint }}</n-text>
    <n-skeleton v-if="loading" text :repeat="5" />
    <n-empty v-else-if="!rows.length" description="暂无策略模板" />
    <n-data-table v-else size="small" :columns="columns" :data="rows" :bordered="false" />

    <n-modal
      v-model:show="showCreate"
      preset="card"
      title="Create Draft"
      style="width: 560px"
      :bordered="false"
      @after-leave="resetForm"
    >
      <n-text depth="3" style="display: block; margin-bottom: 12px">
        仅创建 status=draft；发布须经 Submit Review → Activate
      </n-text>
      <n-form label-placement="top">
        <n-form-item label="名称" required>
          <n-input v-model:value="form.name" placeholder="策略模板名称" />
        </n-form-item>
        <n-form-item label="slug（可选）">
          <n-input v-model:value="form.slug" placeholder="留空则由名称生成" />
        </n-form-item>
        <n-form-item label="描述">
          <n-input v-model:value="form.description" type="textarea" :rows="2" />
        </n-form-item>
        <n-form-item label="revision_note">
          <n-input v-model:value="form.revision_note" />
        </n-form-item>
        <n-form-item label="universe.source">
          <n-input v-model:value="form.universe_source" />
        </n-form-item>
        <n-form-item label="signals.kind">
          <n-input v-model:value="form.signals_kind" />
        </n-form-item>
        <n-form-item label="ranking.top_n">
          <n-input-number v-model:value="form.top_n" :min="1" :max="500" style="width: 100%" />
        </n-form-item>
        <n-form-item label="parameters.knobs (JSON)">
          <n-input v-model:value="form.knobs_json" type="textarea" :rows="4" />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showCreate = false">取消</n-button>
          <n-button type="primary" :loading="creating" @click="submitCreate">创建 Draft</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.schema-list {
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  height: 100%;
  box-sizing: border-box;
}
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}
.hint {
  display: block;
  font-size: 12px;
  margin-top: 2px;
}
</style>
