<script setup>
/**
 * Phase13-A8/B1-B：研究候选 + Explain + Strategy Intent
 * - List / Detail / 标注
 * - Explain Detail
 * - Intent 手工工作流（无 Promote / AI / 下单）
 */
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NDataTable,
  NDivider,
  NDrawer,
  NDrawerContent,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NSelect,
  NSkeleton,
  NSpace,
  NTag,
  NText,
  NTooltip,
  useMessage,
} from 'naive-ui'
import {
  getResearchCandidate,
  getResearchExplain,
  listResearchCandidates,
  updateResearchCandidate,
  updateResearchExplain,
} from '../api/researchCandidates'
import {
  approveStrategyIntent,
  createStrategyIntentDraft,
  getStrategyIntent,
  intentStatusLabel,
  isIntentDraftEditable,
  listStrategyIntents,
  submitStrategyIntent,
  updateStrategyIntentDraft,
} from '../api/strategyIntents'
import {
  adaptResearchCandidate,
  applyStockClickAction,
} from '../utils/stockDisplayAdapters.js'
import { formatStatus, formatFieldTooltip } from '../utils/statusDisplay.js'
import StockKlineModal from './StockKlineModal.vue'
import StockLink from './StockLink.vue'

const message = useMessage()
const loading = ref(false)
const detailLoading = ref(false)
const saving = ref(false)
const explainLoading = ref(false)
const explainSaving = ref(false)
const threshold = ref(60)
const tradeDate = ref('')
const asOf = ref('')
const hintMessage = ref('')
const snapshotId = ref(0)
const loadError = ref(false)
const rows = ref([])
const detailOpen = ref(false)
const detail = ref(null)
const statusFilter = ref(null)
const explainOpen = ref(false)
const explain = ref(null)

/** Phase16.18-B1: page-local K-line modal (same pattern as TradePlan / Portfolio). */
const klineModal = reactive({
  visible: false,
  title: '',
  chartCode: '',
  stockName: '',
})

function openStockKline(model) {
  applyStockClickAction(model, klineModal)
}

function openRowKline(row) {
  openStockKline(adaptResearchCandidate(row))
}

function renderStockCodeKlineLink(row) {
  const model = adaptResearchCandidate(row)
  if (!model.code && !model.klineKey) return '—'
  if (!model.klineKey) return model.code || '—'
  return h(
    'button',
    {
      type: 'button',
      class: 'research-kline-code',
      title: '查看多周期K线',
      onClick: (e) => {
        e.stopPropagation()
        openStockKline(model)
      },
    },
    model.code || '—',
  )
}

function renderStockNameKlineLink(row) {
  const model = adaptResearchCandidate(row)
  if (!model?.displayText && !model?.code) return '—'
  if (!model.klineKey) return model.displayText || model.code || '—'
  return h(StockLink, {
    model,
    onOpen: openStockKline,
  })
}

const intentLoading = ref(false)
const intentSaving = ref(false)
const intentSubmitting = ref(false)
const intentApproving = ref(false)
const intent = ref(null)

const editForm = reactive({
  status: 'new',
  note: '',
  tagsText: '',
})

const explainForm = reactive({
  summary: '',
  reasonText: '',
  riskSeverity: 'info',
  riskText: '',
})

const intentForm = reactive({
  summary: '',
  verb: 'watch',
  actionText: '',
  unbound: true,
  strategyId: 'sdef:trend_breakout',
  schemaRevision: 'v1',
  session: 'open',
})

const verbOptions = [
  { label: 'watch', value: 'watch' },
  { label: 'consider_buy', value: 'consider_buy' },
  { label: 'consider_sell', value: 'consider_sell' },
  { label: 'avoid', value: 'avoid' },
  { label: 'defer', value: 'defer' },
]

const intentEditable = computed(() => isIntentDraftEditable(intent.value?.status || ''))
const canCreateIntent = computed(() => {
  if (!detail.value?.candidate?.id) return false
  if (!intent.value) return true
  return ['expired', 'discarded'].includes(intent.value.status)
})

const statusOptions = [
  { label: '新建', value: 'new' },
  { label: '观察中', value: 'watching' },
  { label: '已复盘', value: 'reviewed' },
  { label: '已丢弃', value: 'discarded' },
]

const riskOptions = [
  { label: '提示', value: 'info' },
  { label: '注意', value: 'warn' },
  { label: '较高', value: 'high' },
]

const filterOptions = [{ label: '全部状态', value: null }, ...statusOptions]

const hasData = computed(() => rows.value.length > 0)

const statusFilterLabel = computed(() => {
  if (statusFilter.value == null) return '全部'
  const opt = statusOptions.find((o) => o.value === statusFilter.value)
  return opt?.label || String(statusFilter.value).toUpperCase()
})

const isFilterEmpty = computed(
  () =>
    statusFilter.value != null &&
    rows.value.length === 0 &&
    snapshotId.value > 0 &&
    Boolean(tradeDate.value),
)

const isSnapshotThresholdEmpty = computed(
  () => snapshotId.value > 0 && rows.value.length === 0 && !isFilterEmpty.value && !loadError.value,
)

const emptyState = computed(() => {
  if (loadError.value) {
    return {
      description: '加载研究候选失败，请稍后重试',
      showRefresh: true,
      showClearFilter: false,
      reasons: [],
    }
  }
  if (isFilterEmpty.value) {
    return {
      description: `当前没有符合条件的研究候选（筛选：${statusFilterLabel.value}）`,
      showRefresh: false,
      showClearFilter: true,
      reasons: ['可清除筛选查看全部候选'],
    }
  }
  if (isSnapshotThresholdEmpty.value) {
    const reasons = ['当前阈值过高，可稍后再看或等待新扫描']
    const hint = hintMessage.value.trim()
    if (hint) reasons.push(hint)
    return {
      description: '当前没有符合条件的研究候选',
      showRefresh: false,
      showClearFilter: false,
      reasons,
    }
  }
  return {
    description: '当前没有符合条件的研究候选',
    showRefresh: false,
    showClearFilter: false,
    reasons: ['今日尚未生成盘后扫描快照', '或当前阈值下没有入选标的'],
  }
})

const columns = [
  {
    title: '代码',
    key: 'stock_code',
    width: 100,
    fixed: 'left',
    render(row) {
      return renderStockCodeKlineLink(row)
    },
  },
  {
    title: '名称',
    key: 'stock_name',
    width: 120,
    fixed: 'left',
    ellipsis: { tooltip: true },
    render(row) {
      return renderStockNameKlineLink(row)
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, '信号'),
          default: () => formatFieldTooltip('signal'),
        },
      ),
    key: 'signal_tag',
    width: 72,
    render(row) {
      const tag = String(row.signal_tag || '').trim()
      if (!tag) return '—'
      return h(
        NTag,
        { size: 'small', bordered: false, type: 'info' },
        { default: () => tag },
      )
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, '评分'),
          default: () => formatFieldTooltip('score'),
        },
      ),
    key: 'signal_score',
    width: 72,
    sorter: (a, b) => (Number(a.signal_score) || 0) - (Number(b.signal_score) || 0),
    render(row) {
      return row.signal_score != null && Number.isFinite(Number(row.signal_score))
        ? Number(row.signal_score)
        : '—'
    },
  },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', null, '风险'),
          default: () => formatFieldTooltip('risk'),
        },
      ),
    key: 'risk_display',
    width: 100,
    ellipsis: { tooltip: true },
    render(row) {
      return formatCandidateRisk(row)
    },
  },
  {
    title: '状态',
    key: 'status',
    width: 88,
    render(row) {
      const st = formatStatus(row.status, 'research')
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () =>
            h(
              NTag,
              { size: 'small', bordered: false, type: st.type },
              { default: () => st.label },
            ),
          default: () => st.tooltip || st.label,
        },
      )
    },
  },
  {
    title: '解释摘要',
    key: 'explain_summary',
    minWidth: 160,
    ellipsis: { tooltip: true },
    render(row) {
      return row.explain_summary || '—'
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 168,
    fixed: 'right',
    render(row) {
      return h(
        NSpace,
        { size: 4, wrap: false },
        {
          default: () => [
            h(
              NButton,
              { size: 'tiny', secondary: true, onClick: () => openDetail(row) },
              { default: () => '详情' },
            ),
            h(
              NButton,
              {
                size: 'tiny',
                tertiary: true,
                disabled: !adaptResearchCandidate(row).klineKey,
                onClick: () => openRowKline(row),
              },
              { default: () => 'K线' },
            ),
            h(
              NButton,
              {
                size: 'tiny',
                tertiary: true,
                disabled: !row.explain_ref,
                onClick: () => openExplain(row.id),
              },
              { default: () => '解释' },
            ),
          ],
        },
      )
    },
  },
]

/** List DTO has no risk_code; surface tags/note when present (never invent). */
function formatCandidateRisk(row) {
  const tags = Array.isArray(row?.tags) ? row.tags.map((t) => String(t || '').trim()).filter(Boolean) : []
  const note = String(row?.note || '').trim()
  const riskish = tags.find((t) => /风险|拒|超|限|risk|block|reject/i.test(t))
  if (riskish) return riskish
  if (tags.length) return tags.slice(0, 2).join(' · ')
  if (note) return note.length > 24 ? `${note.slice(0, 24)}…` : note
  return '—'
}

const poolSummaryLine = computed(() => {
  const day = tradeDate.value || '—'
  const n = rows.value.length
  if (snapshotId.value > 0) {
    return `当前研究池：${day} · ${n} 只股票 · 来源：盘后扫描快照 #${snapshotId.value}`
  }
  return `当前研究池：${day} · ${n} 只股票`
})

async function refresh() {
  loading.value = true
  try {
    const res = await listResearchCandidates({
      status: statusFilter.value || undefined,
    })
    tradeDate.value = res.trade_date || ''
    asOf.value = res.as_of || ''
    threshold.value = Number(res.threshold) || 60
    hintMessage.value = res.message || ''
    snapshotId.value = Number(res.snapshot_id) || 0
    rows.value = Array.isArray(res.items) ? res.items : []
    loadError.value = false
  } catch (e) {
    message.error(e?.message || String(e))
    rows.value = []
    hintMessage.value = '加载研究候选失败'
    loadError.value = true
  } finally {
    loading.value = false
  }
}

function clearStatusFilter() {
  statusFilter.value = null
  refresh()
}

function intentTagType(status) {
  switch (String(status || '').toLowerCase()) {
    case 'approved':
      return 'success'
    case 'reviewing':
      return 'warning'
    case 'draft':
      return 'info'
    case 'discarded':
    case 'expired':
      return 'default'
    default:
      return 'default'
  }
}

async function loadIntentForCandidate(candidateId) {
  intent.value = null
  if (!candidateId) return
  intentLoading.value = true
  try {
    const res = await listStrategyIntents({ candidate_id: candidateId })
    const items = res.items || []
    if (!items.length) {
      intentForm.summary = ''
      intentForm.verb = 'watch'
      intentForm.actionText = ''
      intentForm.unbound = true
      intentForm.strategyId = 'sdef:trend_breakout'
      intentForm.schemaRevision = 'v1'
      intentForm.session = 'open'
      return
    }
    intent.value = items[0]
    const full = await getStrategyIntent(items[0].id)
    intent.value = {
      id: full.id,
      candidate_id: full.candidate_id,
      status: full.status,
      intent_type: full.intent_type,
      summary: full.summary,
      schema_revision: full.schema_revision,
      strategy_id: full.strategy_schema_ref?.strategy_id,
      unbound: full.strategy_schema_ref?.unbound,
      updated_at: full.updated_at,
    }
    intentForm.summary = full.summary || ''
    intentForm.verb = full.action?.verb || 'watch'
    intentForm.actionText = full.action?.text || ''
    intentForm.unbound = full.strategy_schema_ref?.unbound ?? true
    intentForm.strategyId = full.strategy_schema_ref?.strategy_id || 'sdef:trend_breakout'
    intentForm.schemaRevision = full.schema_revision || full.strategy_schema_ref?.revision || 'v1'
    intentForm.session = String(full.conditions?.session || 'open')
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    intentLoading.value = false
  }
}

function buildSchemaRef() {
  if (intentForm.unbound) {
    return { unbound: true, note: '手工未绑定 Schema' }
  }
  return {
    unbound: false,
    strategy_id: intentForm.strategyId.trim(),
    revision: intentForm.schemaRevision.trim(),
  }
}

async function createIntent() {
  if (!detail.value?.candidate?.id) return
  intentSaving.value = true
  try {
    const out = await createStrategyIntentDraft({
      candidate_id: detail.value.candidate.id,
      explain_ref: detail.value.candidate.explain_ref || undefined,
      strategy_schema_ref: buildSchemaRef(),
      schema_revision: intentForm.unbound ? undefined : intentForm.schemaRevision.trim(),
      intent_type: 'manual',
      summary: intentForm.summary.trim(),
      conditions: { session: intentForm.session || 'any' },
      action: { verb: intentForm.verb, text: intentForm.actionText.trim() || undefined },
      risk_constraints: { severity_cap: 'info' },
    })
    intent.value = {
      id: out.intent.id,
      candidate_id: out.intent.candidate_id,
      status: out.intent.status,
      intent_type: out.intent.intent_type,
      summary: out.intent.summary,
      schema_revision: out.intent.schema_revision,
      strategy_id: out.intent.strategy_schema_ref?.strategy_id,
      unbound: out.intent.strategy_schema_ref?.unbound,
      updated_at: out.intent.updated_at,
    }
    message.success('已创建策略意图草稿')
    await loadIntentForCandidate(detail.value.candidate.id)
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    intentSaving.value = false
  }
}

async function saveIntentDraft() {
  if (!intent.value?.id || !intentEditable.value) return
  intentSaving.value = true
  try {
    const out = await updateStrategyIntentDraft(intent.value.id, {
      summary: intentForm.summary.trim(),
      strategy_schema_ref: buildSchemaRef(),
      schema_revision: intentForm.unbound ? '' : intentForm.schemaRevision.trim(),
      conditions: { session: intentForm.session || 'any' },
      action: { verb: intentForm.verb, text: intentForm.actionText.trim() || undefined },
    })
    message.success('策略意图草稿已保存')
    await loadIntentForCandidate(out.intent.candidate_id)
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    intentSaving.value = false
  }
}

async function submitIntent() {
  if (!intent.value?.id) return
  intentSubmitting.value = true
  try {
    if (intentEditable.value) {
      await saveIntentDraft()
    }
    const out = await submitStrategyIntent(intent.value.id)
    message.success('已提交审阅')
    await loadIntentForCandidate(out.intent.candidate_id)
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    intentSubmitting.value = false
  }
}

async function approveIntent() {
  if (!intent.value?.id) return
  intentApproving.value = true
  try {
    const out = await approveStrategyIntent(intent.value.id)
    message.success('策略意图已批准')
    await loadIntentForCandidate(out.intent.candidate_id)
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    intentApproving.value = false
  }
}

async function openDetail(row) {
  detailOpen.value = true
  detail.value = null
  intent.value = null
  detailLoading.value = true
  try {
    const res = await getResearchCandidate(row.id)
    detail.value = res
    editForm.status = res.candidate.status || 'new'
    editForm.note = res.candidate.note || ''
    editForm.tagsText = Array.isArray(res.candidate.tags) ? res.candidate.tags.join(', ') : ''
    await loadIntentForCandidate(res.candidate.id)
  } catch (e) {
    message.error(e?.message || String(e))
    detailOpen.value = false
  } finally {
    detailLoading.value = false
  }
}

async function openExplain(candidateId) {
  explainOpen.value = true
  explain.value = null
  explainLoading.value = true
  try {
    const res = await getResearchExplain(candidateId)
    explain.value = res
    explainForm.summary = res.summary || ''
    explainForm.reasonText = res.research_reason?.text || ''
    explainForm.riskSeverity = res.risk_note?.severity || 'info'
    explainForm.riskText = res.risk_note?.text || ''
  } catch (e) {
    message.error(e?.message || String(e))
    explainOpen.value = false
  } finally {
    explainLoading.value = false
  }
}

async function saveAnnotation() {
  if (!detail.value?.candidate?.id) return
  saving.value = true
  try {
    const tags = editForm.tagsText
      .split(/[,，]/)
      .map((s) => s.trim())
      .filter(Boolean)
    const res = await updateResearchCandidate(detail.value.candidate.id, {
      status: editForm.status,
      note: editForm.note,
      tags,
    })
    detail.value = res
    message.success('研究备注已保存')
    await refresh()
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    saving.value = false
  }
}

async function saveExplain() {
  if (!explain.value?.candidate_id) return
  explainSaving.value = true
  try {
    const res = await updateResearchExplain(explain.value.candidate_id, {
      summary: explainForm.summary,
      research_reason: {
        kind: 'analyst_note',
        text: explainForm.reasonText,
      },
      risk_note: explainForm.riskText.trim()
        ? { severity: explainForm.riskSeverity, text: explainForm.riskText }
        : null,
      clear_risk_note: !explainForm.riskText.trim(),
    })
    explain.value = res
    message.success('研究解释已保存（人工层）')
    await refresh()
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    explainSaving.value = false
  }
}

function formatTime(v) {
  if (!v) return '暂无'
  try {
    const d = new Date(v)
    if (Number.isNaN(d.getTime())) return v
    return d.toLocaleString()
  } catch {
    return v
  }
}

onMounted(refresh)
</script>

<template>
  <div class="research-candidate-pool">
    <div class="toolbar">
      <n-text depth="3" class="hint">
        研究候选池 · 只读浏览与标注 · 不会自动下单
      </n-text>
      <n-space align="center">
        <n-select
          v-model:value="statusFilter"
          :options="filterOptions"
          size="small"
          style="width: 140px"
          @update:value="refresh"
        />
        <n-button type="primary" secondary :loading="loading" @click="refresh">刷新</n-button>
      </n-space>
    </div>

    <n-space v-if="!loading" class="meta" align="center" :wrap="true" vertical :size="4">
      <n-text>{{ poolSummaryLine }}</n-text>
      <n-space align="center" :wrap="true" :size="[12, 4]">
        <n-text depth="3">扫描时间：{{ formatTime(asOf) }}</n-text>
        <n-text depth="3">入选评分 &gt; {{ threshold }}</n-text>
        <n-text depth="3">筛选：{{ statusFilterLabel }}</n-text>
        <n-text v-if="hintMessage" depth="3">{{ hintMessage }}</n-text>
      </n-space>
    </n-space>

    <div class="body">
      <n-skeleton v-if="loading" text :repeat="6" />
      <n-empty v-else-if="!hasData" :description="emptyState.description">
        <template #extra>
          <n-space vertical align="center" :size="8">
            <n-text
              v-for="(r, i) in emptyState.reasons || []"
              :key="'er-' + i"
              depth="3"
              style="font-size: 12px"
            >
              · {{ r }}
            </n-text>
            <n-space v-if="emptyState.showRefresh || emptyState.showClearFilter" justify="center">
              <n-button
                v-if="emptyState.showRefresh"
                type="primary"
                secondary
                :loading="loading"
                @click="refresh"
              >
                刷新
              </n-button>
              <n-button v-if="emptyState.showClearFilter" secondary @click="clearStatusFilter">
                清除筛选
              </n-button>
            </n-space>
          </n-space>
        </template>
      </n-empty>
      <n-data-table
        v-else
        class="candidate-table"
        size="small"
        :columns="columns"
        :data="rows"
        :bordered="false"
        :single-line="true"
        :scroll-x="900"
        flex-height
      />
    </div>

    <n-drawer v-model:show="detailOpen" :width="440" placement="right">
      <n-drawer-content title="研究候选详情" closable>
        <n-skeleton v-if="detailLoading" text :repeat="8" />
        <template v-else-if="detail">
          <n-space vertical :size="10">
            <n-text>ID：{{ detail.candidate.id }}</n-text>
            <n-space align="center" :size="8">
              <n-text>股票：</n-text>
              <stock-link
                v-if="adaptResearchCandidate(detail.candidate).klineKey"
                :model="adaptResearchCandidate(detail.candidate)"
                @open="openStockKline"
              />
              <n-text v-else>
                {{ detail.candidate.stock_code }} {{ detail.candidate.stock_name }}
              </n-text>
              <n-button
                size="tiny"
                secondary
                :disabled="!adaptResearchCandidate(detail.candidate).klineKey"
                @click="openRowKline(detail.candidate)"
              >
                多周期K线
              </n-button>
            </n-space>
            <n-text>来源：{{ detail.candidate.source }}（{{ detail.candidate.source_ref || '—' }}）</n-text>
            <n-text>
              信号分：{{ detail.candidate.signal_score ?? '—' }} / 标签：{{ detail.candidate.signal_tag || '—' }}
            </n-text>
            <n-text depth="3">explain_ref：{{ detail.candidate.explain_ref || detail.explanation.explain_ref || '—' }}</n-text>
            <n-text depth="3">解释摘要：{{ detail.explanation.summary || detail.candidate.explain_summary || '—' }}</n-text>

            <n-form label-placement="top" size="small">
              <n-form-item label="研究状态">
                <n-select v-model:value="editForm.status" :options="statusOptions" />
              </n-form-item>
              <n-form-item label="研究备注">
                <n-input
                  v-model:value="editForm.note"
                  type="textarea"
                  :rows="3"
                  placeholder="仅研究侧备注，不写入交易池"
                />
              </n-form-item>
              <n-form-item label="标签（逗号分隔）">
                <n-input v-model:value="editForm.tagsText" placeholder="例如：银行, 观察" />
              </n-form-item>
            </n-form>

            <n-space>
              <n-button type="primary" :loading="saving" @click="saveAnnotation">保存研究标注</n-button>
              <n-button secondary @click="openExplain(detail.candidate.id)">查看研究解释</n-button>
            </n-space>

            <n-divider style="margin: 12px 0">策略意图</n-divider>
            <n-skeleton v-if="intentLoading" text :repeat="4" />
            <template v-else>
              <n-space v-if="intent" align="center">
                <n-tag size="small" :type="intentTagType(intent.status)" :bordered="false">
                  {{ intentStatusLabel(intent.status) }}
                </n-tag>
                <n-text depth="3">ID：{{ intent.id }}</n-text>
              </n-space>
              <n-empty v-else description="暂无策略意图" size="small" />

              <n-form v-if="intentEditable || canCreateIntent" label-placement="top" size="small">
                <n-form-item label="摘要 summary">
                  <n-input v-model:value="intentForm.summary" placeholder="一句话意图摘要" />
                </n-form-item>
                <n-form-item label="action.verb">
                  <n-select v-model:value="intentForm.verb" :options="verbOptions" />
                </n-form-item>
                <n-form-item label="action.text">
                  <n-input v-model:value="intentForm.actionText" type="textarea" :rows="2" />
                </n-form-item>
                <n-form-item label="Schema 绑定">
                  <n-select
                    v-model:value="intentForm.unbound"
                    :options="[
                      { label: 'unbound（未绑定）', value: true },
                      { label: '钉住 revision', value: false },
                    ]"
                  />
                </n-form-item>
                <template v-if="!intentForm.unbound">
                  <n-form-item label="strategy_id">
                    <n-input v-model:value="intentForm.strategyId" placeholder="sdef:..." />
                  </n-form-item>
                  <n-form-item label="revision（禁 current/latest）">
                    <n-input v-model:value="intentForm.schemaRevision" placeholder="v1" />
                  </n-form-item>
                </template>
              </n-form>

              <n-space wrap>
                <n-button
                  v-if="canCreateIntent"
                  type="primary"
                  :loading="intentSaving"
                  @click="createIntent"
                >
                  创建意图草稿
                </n-button>
                <n-button
                  v-if="intentEditable"
                  secondary
                  :loading="intentSaving"
                  @click="saveIntentDraft"
                >
                  保存草稿
                </n-button>
                <n-button
                  v-if="intentEditable"
                  type="warning"
                  secondary
                  :loading="intentSubmitting"
                  @click="submitIntent"
                >
                  提交审核
                </n-button>
                <n-button
                  v-if="intent?.status === 'reviewing'"
                  type="primary"
                  :loading="intentApproving"
                  @click="approveIntent"
                >
                  Approve
                </n-button>
              </n-space>

              <n-text v-if="intent && !intentEditable" depth="3" style="font-size: 12px">
                当前状态只读；已批准后如需修改请新建意图（本阶段未开放）。
              </n-text>
            </template>

            <n-text depth="3">交易池/计划链接仍为空（无 Promote）。</n-text>
          </n-space>
        </template>
      </n-drawer-content>
    </n-drawer>

    <n-drawer v-model:show="explainOpen" :width="480" placement="right">
      <n-drawer-content title="研究解释 ResearchExplain" closable>
        <n-skeleton v-if="explainLoading" text :repeat="10" />
        <template v-else-if="explain">
          <n-space vertical :size="8">
            <n-text>ID：{{ explain.id }}</n-text>
            <n-text>类型：{{ explain.explain_type }} · schema {{ explain.schema_version }}</n-text>
            <n-text>可用：{{ explain.available ? '是' : '否' }} {{ explain.missing_reason || '' }}</n-text>
            <n-text depth="3">intent_ref：{{ explain.strategy_intent_ref ?? 'null' }}</n-text>

            <n-text strong>证据 evidence</n-text>
            <n-text depth="3">tag={{ explain.evidence?.signal_tag || '—' }} score={{ explain.evidence?.signal_score ?? '—' }}</n-text>
            <n-text depth="3">hash={{ explain.evidence?.evidence_hash || '—' }}</n-text>
            <n-text depth="3">steps：{{ (explain.evidence?.score_steps || []).join('；') || '—' }}</n-text>

            <n-form label-placement="top" size="small">
              <n-form-item label="摘要 summary">
                <n-input v-model:value="explainForm.summary" type="textarea" :rows="2" />
              </n-form-item>
              <n-form-item label="研究理由 research_reason">
                <n-input v-model:value="explainForm.reasonText" type="textarea" :rows="2" />
              </n-form-item>
              <n-form-item label="风险提示 severity">
                <n-select v-model:value="explainForm.riskSeverity" :options="riskOptions" />
              </n-form-item>
              <n-form-item label="风险提示文本">
                <n-input v-model:value="explainForm.riskText" type="textarea" :rows="2" />
              </n-form-item>
            </n-form>

            <n-button type="primary" :loading="explainSaving" @click="saveExplain">保存人工解释层</n-button>
            <n-text depth="3">更新：{{ formatTime(explain.updated_at) }} · 证据仍为信号派生，非 AI</n-text>
          </n-space>
        </template>
      </n-drawer-content>
    </n-drawer>

    <stock-kline-modal
      v-model:show="klineModal.visible"
      :title="klineModal.title"
      :chart-key="'research-candidate-kline-' + klineModal.chartCode"
      :code="klineModal.chartCode"
      :stock-name="klineModal.stockName"
    />
  </div>
</template>

<style scoped>
.research-candidate-pool {
  box-sizing: border-box;
  flex: 1 1 auto;
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow: hidden;
}
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-shrink: 0;
}
.hint {
  font-size: 12px;
}
.meta {
  flex-shrink: 0;
}
/*
  Phase16.21-C: 表格占满剩余视口。
  根因：Phase16.15-E 的 min-height:280px 在 flex 链未撑开时变成实际高度（约 5 行）。
  现用 calc(100vh - 顶部) + min 400px，表头 sticky 随表体滚动固定。
*/
.body {
  flex: 1 1 0;
  min-height: max(400px, calc(100vh - 280px));
  height: calc(100vh - 260px);
  max-height: calc(100vh - 220px);
  position: relative;
  overflow: hidden;
}
.body :deep(.candidate-table) {
  position: absolute;
  inset: 0;
  height: 100% !important;
  max-height: 100%;
}
.body :deep(.candidate-table .n-data-table-wrapper) {
  height: 100%;
}
.body :deep(.candidate-table .n-data-table-base-table) {
  height: 100%;
}
/* sticky header while scrolling table body */
.body :deep(.candidate-table .n-data-table-thead) {
  position: sticky;
  top: 0;
  z-index: 3;
}
.research-kline-code {
  display: inline;
  margin: 0;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--n-primary-color, #18a058);
  cursor: pointer;
  font: inherit;
  text-align: left;
}
.research-kline-code:hover {
  text-decoration: underline;
}
</style>
