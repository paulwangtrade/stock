<script setup>
/**
 * Exit Review detail drawer (Phase14-D2 + D3 Outcome).
 * Read-only sections ①–⑤; §6 records user decision via POST /api/exit-review/outcome.
 */
import { computed, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NCollapse,
  NCollapseItem,
  NDataTable,
  NDrawer,
  NDrawerContent,
  NDivider,
  NInput,
  NModal,
  NSpace,
  NSpin,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import { postExitReviewOutcome, getPaperHoldingsEvaluation } from '../api/paperObservation'
import { getPortfolioPositionState } from '../api/portfolioPositionState'
import {
  EXIT_REVIEW_DECISION,
  buildExitReviewRiskFactors,
  exitReasonLabel,
  exitReviewDecisionLabel,
  findHoldingEvalRow,
  formatOutcomeReviewTime,
} from '../utils/exitReviewDisplay.js'
import { lookupPositionState, positionStateLabel } from '../utils/positionStateDisplay.js'
import SellDraftDialog from './SellDraftDialog.vue'
import { EXIT_REVIEW_SOURCE_SESSION } from '../api/tradePlansTSell'
import {
  buildExitReviewSellDraftRow,
  buildExitReviewSellReason,
  EXIT_REVIEW_SELL_ACTOR,
} from '../utils/exitReviewSellDraft.js'

const props = defineProps({
  show: { type: Boolean, default: false },
  exitRow: { type: Object, default: null },
  exitMeta: { type: Object, default: null },
  cachedHoldingEval: { type: Object, default: null },
})

const emit = defineEmits(['update:show', 'outcome-saved'])

const message = useMessage()
const loading = ref(false)
const saving = ref(false)
const holdingEvalRow = ref(null)
const positionStateRow = ref(null)
const loadError = ref('')
const reasonModalVisible = ref(false)
const pendingDecision = ref('')
const reasonInput = ref('')
const savedOutcome = ref(null)
const sellDraftVisible = ref(false)

const stockCode = computed(() => String(props.exitRow?.stockCode || '').trim())
const stockName = computed(() => String(props.exitRow?.stockName || '').trim())
const evaluation = computed(() => props.exitRow?.evaluation || {})
const policy = computed(() => props.exitMeta?.policy || null)

const drawerTitle = computed(() => {
  const name = stockName.value || stockCode.value || '—'
  const code = stockCode.value
  return code ? `${name}（${code}）· 退出复评` : `${name} · 退出复评`
})

const asOfLabel = computed(() => {
  const raw = props.exitMeta?.asOf
  if (!raw) return '—'
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return String(raw)
  return d.toLocaleString('zh-CN', { hour12: false })
})

const accountLabel = computed(() => {
  const id = props.exitMeta?.accountId
  return id ? `Paper 模拟 · #${id}` : 'Paper 模拟'
})

const riskPack = computed(() =>
  buildExitReviewRiskFactors({
    holdingDays: holdingEvalRow.value?.holdingDays ?? props.exitRow?.lots?.[0]?.holdingDays ?? 0,
    unrealizedReturn:
      holdingEvalRow.value?.unrealizedReturn ??
      (props.exitRow?.lots?.length === 1 ? props.exitRow.lots[0]?.unrealizedReturn : null),
    reasonCodes: evaluation.value?.reasonCodes || [],
    state: evaluation.value?.state || 'NORMAL',
    policy: policy.value || {},
    lots: props.exitRow?.lots || [],
  }),
)

const displayOutcome = computed(
  () => savedOutcome.value || props.exitRow?.latestOutcome || null,
)

const sellDraftRow = computed(() =>
  buildExitReviewSellDraftRow({
    stockCode: stockCode.value,
    stockName: stockName.value,
    availableQty: positionStateRow.value?.availableQty ?? 0,
    positionState: positionStateLabel(
      positionStateRow.value?.state || evaluation.value?.state || '',
    ),
    canSell: positionStateRow.value?.canSell !== false,
  }),
)

const sellDefaultReason = computed(() =>
  buildExitReviewSellReason({
    exitState: evaluation.value?.state,
    reasonCodes: evaluation.value?.reasonCodes || [],
  }),
)

const reasonModalTitle = computed(() => {
  const label = exitReviewDecisionLabel(pendingDecision.value)
  return label ? `记录结论：${label}` : '记录复评结论'
})

function openReasonModal(decision) {
  pendingDecision.value = decision
  reasonInput.value = ''
  reasonModalVisible.value = true
}

async function submitOutcome(decision, reason) {
  if (!stockCode.value) return
  saving.value = true
  try {
    const res = await postExitReviewOutcome({
      stockCode: stockCode.value,
      decision,
      reason: reason || '',
      exitStateSnapshot: String(evaluation.value?.state || 'NORMAL'),
      reasonCodesSnapshot: evaluation.value?.reasonCodes || [],
      evaluationSummarySnapshot: evaluation.value?.summary || '',
      accountId: props.exitMeta?.accountId,
    })
    savedOutcome.value = {
      id: res.id,
      decision: res.decision,
      reviewTime: res.reviewTime,
      reason: res.reason,
      createdBy: res.createdBy,
    }
    reasonModalVisible.value = false
    message.success(`已记录：${exitReviewDecisionLabel(res.decision)}`)
    emit('outcome-saved', { stockCode: stockCode.value, outcome: savedOutcome.value })
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    saving.value = false
  }
}

function confirmReasonModal() {
  const d = pendingDecision.value
  if (!d) return
  submitOutcome(d, reasonInput.value.trim())
}

function onCreateSellPlanClick() {
  if (!stockCode.value) {
    message.warning('股票代码无效')
    return
  }
  const avail = Math.trunc(Number(positionStateRow.value?.availableQty) || 0)
  if (avail <= 0) {
    message.warning('当前无可卖数量，无法创建卖出计划')
    return
  }
  sellDraftVisible.value = true
}

async function onSellDraftCreated({ planId }) {
  const pid = Math.trunc(Number(planId) || 0)
  if (!stockCode.value || pid <= 0) return
  saving.value = true
  try {
    const res = await postExitReviewOutcome({
      stockCode: stockCode.value,
      decision: EXIT_REVIEW_DECISION.CREATE_SELL_PLAN,
      reason: '',
      exitStateSnapshot: String(evaluation.value?.state || 'NORMAL'),
      reasonCodesSnapshot: evaluation.value?.reasonCodes || [],
      evaluationSummarySnapshot: evaluation.value?.summary || '',
      relatedTradePlanId: pid,
      accountId: props.exitMeta?.accountId,
    })
    savedOutcome.value = {
      id: res.id,
      decision: res.decision,
      reviewTime: res.reviewTime,
      reason: res.reason,
      createdBy: res.createdBy,
      relatedTradePlanId: res.relatedTradePlanId ?? pid,
    }
    message.success(`已记录：${exitReviewDecisionLabel(res.decision)}（计划 #${pid}）`)
    emit('outcome-saved', { stockCode: stockCode.value, outcome: savedOutcome.value })
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    saving.value = false
  }
}

const lotColumns = [
  { title: 'fill_id', key: 'fillId', width: 72 },
  { title: 'plan_id', key: 'planId', width: 72 },
  { title: '持有天数', key: 'holdingDays', width: 80 },
  {
    title: '收益率',
    key: 'unrealizedReturn',
    width: 88,
    render(row) {
      return formatPct(row?.unrealizedReturn)
    },
  },
]

const entryLots = computed(() => {
  const lots = Array.isArray(props.exitRow?.lots) ? props.exitRow.lots : []
  return lots.filter((lot) => lot?.fillId)
})

function formatMoney(v) {
  if (v == null || v === '' || !Number.isFinite(Number(v))) return '—'
  return Number(v).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function formatPct(v) {
  if (v == null || v === '' || !Number.isFinite(Number(v))) return '—'
  return `${(Number(v) * 100).toFixed(2)}%`
}

function pnlClass(v) {
  const n = Number(v)
  if (!Number.isFinite(n) || n === 0) return ''
  return n > 0 ? 'pnl-up' : 'pnl-down'
}

function exitStateTagType(state) {
  const v = String(state || '')
  if (v === 'REVIEW_REQUIRED') return 'error'
  if (v === 'WATCH') return 'warning'
  return 'success'
}

function close() {
  emit('update:show', false)
}

async function loadSupplemental() {
  loadError.value = ''
  holdingEvalRow.value = findHoldingEvalRow(props.cachedHoldingEval, stockCode.value)
  positionStateRow.value = null

  if (!stockCode.value) return

  loading.value = true
  try {
    const tasks = []
    if (!holdingEvalRow.value) {
      tasks.push(
        getPaperHoldingsEvaluation(stockCode.value)
          .then((view) => {
            holdingEvalRow.value = findHoldingEvalRow(view, stockCode.value)
          })
          .catch(() => {
            /* optional enrichment */
          }),
      )
    }
    tasks.push(
      getPortfolioPositionState()
        .then((bundle) => {
          const map = Object.create(null)
          for (const p of bundle?.positions || []) {
            if (p?.symbol) map[String(p.symbol).toLowerCase()] = p
          }
          positionStateRow.value = lookupPositionState(map, stockCode.value)
        })
        .catch(() => {
          /* position-state optional */
        }),
    )
    await Promise.all(tasks)
  } catch (e) {
    loadError.value = e?.message || String(e)
  } finally {
    loading.value = false
  }
}

watch(
  () => props.show,
  (visible) => {
    if (!visible) {
      holdingEvalRow.value = null
      positionStateRow.value = null
      loadError.value = ''
      savedOutcome.value = null
      reasonModalVisible.value = false
      pendingDecision.value = ''
      sellDraftVisible.value = false
      return
    }
    savedOutcome.value = props.exitRow?.latestOutcome || null
    loadSupplemental()
  },
)

watch(
  () => props.exitRow?.latestOutcome,
  (o) => {
    if (props.show && o && !savedOutcome.value) {
      savedOutcome.value = o
    }
  },
)
</script>

<template>
  <n-drawer
    :show="show"
    :width="520"
    placement="right"
    @update:show="(v) => { if (!v) close() }"
  >
    <n-drawer-content :title="drawerTitle" closable>
      <n-spin :show="loading">
        <n-text v-if="loadError" type="warning" depth="3" style="display: block; margin-bottom: 8px">
          {{ loadError }}
        </n-text>

        <n-text strong style="display: block; margin-bottom: 6px">① 股票信息</n-text>
        <n-space vertical :size="4" style="margin-bottom: 14px">
          <n-text depth="3">代码：{{ stockCode || '—' }}</n-text>
          <n-text depth="3">名称：{{ stockName || '—' }}</n-text>
          <n-text depth="3">账户：{{ accountLabel }}</n-text>
          <n-text depth="3">评价时间：截至 {{ asOfLabel }}</n-text>
        </n-space>

        <n-divider style="margin: 12px 0" />

        <n-text strong style="display: block; margin-bottom: 6px">② 持仓信息</n-text>
        <n-space vertical :size="4" style="margin-bottom: 14px">
          <n-text depth="3">
            持仓数量：{{ holdingEvalRow?.totalVolume != null ? `共 ${holdingEvalRow.totalVolume} 股` : '—' }}
          </n-text>
          <n-text depth="3">
            可卖 / 锁定：
            {{
              positionStateRow
                ? `${positionStateRow.availableQty ?? 0} / ${positionStateRow.lockedQty ?? 0}`
                : '—'
            }}
          </n-text>
          <n-text depth="3">
            成本 / 现价：
            {{ formatMoney(holdingEvalRow?.avgCost) }} / {{ formatMoney(holdingEvalRow?.currentPrice ?? holdingEvalRow?.marketPrice) }}
          </n-text>
          <n-text depth="3" :class="pnlClass(holdingEvalRow?.unrealizedPnl)">
            浮动盈亏：
            {{ formatMoney(holdingEvalRow?.unrealizedPnl) }}
            （{{ formatPct(holdingEvalRow?.unrealizedReturn) }}）
          </n-text>
          <n-text depth="3">
            持有天数：{{ holdingEvalRow?.holdingDays != null ? `已持有 ${holdingEvalRow.holdingDays} 天` : '—' }}
          </n-text>
        </n-space>
        <n-alert
          v-if="positionStateRow && (positionStateRow.canSell === false || (positionStateRow.availableQty ?? 0) <= 0)"
          type="info"
          :bordered="false"
          style="margin-bottom: 12px"
        >
          今日无可卖数量（T+1 规则）。本页仅展示复评信息，不会自动卖出。
        </n-alert>
        <n-collapse v-if="entryLots.length" style="margin-bottom: 14px">
          <n-collapse-item title="Lot 明细" name="lots">
            <n-data-table
              size="small"
              :columns="lotColumns"
              :data="entryLots"
              :bordered="false"
              :row-key="(lot) => String(lot.fillId)"
            />
          </n-collapse-item>
        </n-collapse>

        <n-divider style="margin: 12px 0" />

        <n-text strong style="display: block; margin-bottom: 6px">③ 买入依据</n-text>
        <template v-if="entryLots.length">
          <div v-for="lot in entryLots" :key="lot.fillId" class="entry-lot-block">
            <n-text depth="3" style="display: block">
              计划 #{{ lot.context?.plan?.planId || lot.planId || '—' }}
              · {{ lot.context?.plan?.tradeDate || '—' }}
              · 状态 {{ lot.context?.plan?.planStatus || '—' }}
            </n-text>
            <n-text depth="3">策略：{{ lot.context?.entry?.strategyName || '—' }}</n-text>
            <n-text depth="3" style="white-space: pre-wrap">
              入场原因：{{ lot.context?.entry?.entryReason || '—' }}
            </n-text>
            <n-text v-if="lot.context?.entry?.entryRule" depth="3" style="white-space: pre-wrap">
              入场规则：{{ lot.context.entry.entryRule }}
            </n-text>
          </div>
        </template>
        <n-text v-else depth="3">暂无归因 Lot</n-text>
        <n-text depth="3" style="display: block; margin-top: 8px; font-size: 12px">
          买入信号溯源（SignalEvent）将在后续版本关联。
        </n-text>

        <n-divider style="margin: 12px 0" />

        <n-text strong style="display: block; margin-bottom: 6px">④ 当前状态</n-text>
        <n-space align="center" :wrap="true" style="margin-bottom: 8px">
          <n-tag :type="exitStateTagType(evaluation.state)" :bordered="false">
            {{ evaluation.state || 'NORMAL' }}
          </n-tag>
          <n-tag
            v-for="code in evaluation.reasonCodes || []"
            :key="code"
            size="small"
            :bordered="false"
          >
            {{ exitReasonLabel(code) }}
          </n-tag>
        </n-space>
        <n-text style="display: block; margin-bottom: 8px">
          {{ evaluation.summary || '—' }}
        </n-text>
        <n-text v-if="policy" depth="3" style="font-size: 12px">
          策略阈值：观察周期 {{ policy.maxHoldingDays }} 天 · 关注线
          {{ formatPct(policy.lossWatchThreshold) }} · 复评线
          {{ formatPct(policy.lossReviewThreshold) }}
        </n-text>

        <n-divider style="margin: 12px 0" />

        <n-text strong style="display: block; margin-bottom: 6px">⑤ 风险因素</n-text>
        <n-text depth="3" style="display: block; margin-bottom: 8px; font-size: 12px">
          {{ riskPack.summaryLine }}
        </n-text>
        <div v-for="factor in riskPack.factors" :key="factor.code" class="risk-factor-row">
          <n-tag :type="factor.triggered ? 'warning' : 'success'" size="small" :bordered="false">
            {{ factor.triggered ? '触线' : '未触发' }}
          </n-tag>
          <n-text strong style="margin-left: 6px">{{ factor.label }}</n-text>
          <n-text depth="3" style="display: block; margin-left: 4px; font-size: 12px">
            {{ factor.fact }} — {{ factor.detail }}
          </n-text>
        </div>

        <n-divider style="margin: 12px 0" />

        <n-text strong style="display: block; margin-bottom: 6px">⑥ 复评结论</n-text>
        <n-alert type="info" :bordered="false" style="margin-bottom: 12px">
          系统展示复评信号供你判断，<strong>不会自动卖出</strong>。
          「生成卖出计划」将打开卖出草稿对话框；草稿创建成功后会记录复评结论并关联计划 ID，不会自动批准、冻结或执行。
        </n-alert>

        <n-alert
          v-if="displayOutcome?.decision"
          type="success"
          :bordered="false"
          style="margin-bottom: 12px"
        >
          已复评 · 最近决定：{{ exitReviewDecisionLabel(displayOutcome.decision) }}
          <template v-if="displayOutcome.reviewTime">
            （{{ formatOutcomeReviewTime(displayOutcome.reviewTime) }}）
          </template>
          <n-text v-if="displayOutcome.reason" depth="3" style="display: block; margin-top: 4px">
            备注：{{ displayOutcome.reason }}
          </n-text>
        </n-alert>

        <n-space :wrap="true">
          <n-button
            type="primary"
            :loading="saving"
            :disabled="saving"
            @click="openReasonModal(EXIT_REVIEW_DECISION.HOLD)"
          >
            继续持有
          </n-button>
          <n-button
            :loading="saving"
            :disabled="saving"
            @click="openReasonModal(EXIT_REVIEW_DECISION.WATCH)"
          >
            加入观察
          </n-button>
          <n-button
            type="warning"
            secondary
            :loading="saving"
            :disabled="saving || (positionStateRow && (positionStateRow.availableQty ?? 0) <= 0)"
            @click="onCreateSellPlanClick"
          >
            生成卖出计划
          </n-button>
        </n-space>
      </n-spin>

      <n-modal
        v-model:show="reasonModalVisible"
        preset="dialog"
        :title="reasonModalTitle"
        positive-text="确认记录"
        negative-text="取消"
        :loading="saving"
        @positive-click="() => { confirmReasonModal(); return false }"
      >
        <n-text depth="3" style="display: block; margin-bottom: 8px">
          可选填写备注，便于日后追溯（不影响持仓与系统复评状态）。
        </n-text>
        <n-input
          v-model:value="reasonInput"
          type="textarea"
          placeholder="例如：逻辑仍成立，等待下一季财报"
          :autosize="{ minRows: 2, maxRows: 4 }"
        />
      </n-modal>

      <template #footer>
        <n-text depth="3" style="font-size: 12px">
          {{ exitMeta?.dataSourceNote || 'Exit Evaluation · read-only' }}
        </n-text>
      </template>
    </n-drawer-content>
  </n-drawer>

  <SellDraftDialog
    v-model:show="sellDraftVisible"
    :row="sellDraftRow"
    :actor="EXIT_REVIEW_SELL_ACTOR"
    :source-session="EXIT_REVIEW_SOURCE_SESSION"
    source-label="退出复评"
    :default-reason="sellDefaultReason"
    @created="onSellDraftCreated"
  />
</template>

<style scoped>
.entry-lot-block {
  margin-bottom: 10px;
  padding-bottom: 8px;
  border-bottom: 1px dashed rgba(128, 128, 128, 0.2);
}
.entry-lot-block:last-child {
  border-bottom: none;
}
.risk-factor-row {
  margin-bottom: 10px;
}
.pnl-up {
  color: #d03050;
}
.pnl-down {
  color: #18a058;
}
</style>
