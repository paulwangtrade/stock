<script setup>
import { computed, h, onMounted, ref } from 'vue'
import {
  NButton,
  NDataTable,
  NEmpty,
  NInput,
  NSpace,
  NSpin,
  NTag,
  NText,
  NTooltip,
  useDialog,
  useMessage,
} from 'naive-ui'
import {
  TRADE_PLAN_CODE_NO_PLAN,
  TRADE_PLAN_CODE_NO_UPCOMING,
  TRADE_PLAN_CODE_OK,
  approveTradePlan,
  canShowMorningMaterializeButton,
  formatMaterializeMorningDisplay,
  freezeTradePlan,
  generateNextTradePlan,
  getTradePlanReadiness,
  getUpcomingTradePlan,
  inferPricingStageForMaterializeUI,
  isPreferredGeneratedPlan,
  materializeMorningTradePlan,
  resolveUpcomingQueryTradeDateAfterGenerate,
} from '../api/tradePlans'
import { describeReadinessFinding } from '../utils/readinessExplain'

const UI_ACTOR = 'ui:trade-plan-upcoming'
const UI_SOURCE = 'ui'

const REF_AMOUNT_TIP =
  '该金额为观察阶段参考配置，不代表真实仓位计算结果。真实仓位需要结合账户资金、风险预算和价格模型。'

const message = useMessage()
const dialog = useDialog()
const loading = ref(false)
const generating = ref(false)
const approving = ref(false)
const freezing = ref(false)
const materializing = ref(false)
const tradeDateInput = ref('')
const requestTradeDate = ref('')
const nextTradingDay = ref('')
const emptyMessage = ref('')
const plan = ref(null)
/** Focus plan produced by the latest successful generate-next (UI-only). */
const preferredGeneratedPlanId = ref(0)
const preferredGeneratedTradeDate = ref('')
/** Last known pricing_stage from materialize response (UI-only until upcoming exposes it). */
const knownPricingStage = ref('')
const materializeResult = ref(null)
const materializeDisplay = ref(null)

const tradeDatePlaceholder = computed(() =>
  nextTradingDay.value
    ? `trade_date（默认 ${nextTradingDay.value}）`
    : 'trade_date YYYY-MM-DD（可选）',
)

const readinessLoading = ref(false)
const readinessError = ref('')
const readinessData = ref(null)

const hasPlan = computed(() => !!plan.value)
const hasReadiness = computed(() => !!readinessData.value)

const sourceLabel = computed(() => {
  const s = String(plan.value?.source_session || '').trim()
  if (s === 'after_close') return '盘后计划'
  if (s === 'morning_rebuild') return '早盘重建'
  return s ? s : '手动生成'
})

const triggerLabel = computed(() => {
  const s = String(plan.value?.source_session || '').trim()
  if (s === 'after_close') return 'after_close'
  if (s === 'morning_rebuild') return 'morning_rebuild'
  return 'manual'
})

function pad2(n) {
  return String(n).padStart(2, '0')
}

const generatedAtLabel = computed(() => {
  const raw = String(plan.value?.generated_at || '').trim()
  if (!raw) return '—'
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return raw
  return (
    `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} ` +
    `${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}`
  )
})

const planVersionLabel = computed(() => {
  const v = Number(plan.value?.plan_version)
  if (!Number.isFinite(v) || v <= 0) return '—'
  return `v${v}`
})

const readinessReady = computed(() => {
  const blockers = readinessData.value?.blockers || []
  return blockers.length === 0
})

const blockerCount = computed(() => (readinessData.value?.blockers || []).length)
const warningCount = computed(() => (readinessData.value?.warnings || []).length)

const isDraft = computed(() => String(plan.value?.status || '') === 'draft')
const isReady = computed(() => String(plan.value?.status || '') === 'ready')
const isApproved = computed(() => !!String(plan.value?.freeze?.approved_at || '').trim())
const isFrozen = computed(() => !!plan.value?.freeze?.is_frozen)

const lifecycleLabel = computed(() => {
  if (isFrozen.value || isReady.value) return 'Ready'
  if (isDraft.value) return 'Draft'
  return String(plan.value?.status || '—')
})

const approvalLabel = computed(() => (isApproved.value ? '已审批' : '未审批'))
const freezeLabel = computed(() => (isFrozen.value ? '已冻结' : '未冻结'))

const approveDisabledReason = computed(() => {
  if (!hasPlan.value) return '当前无交易计划'
  if (isFrozen.value) return '计划已冻结，无法再次审批'
  if (!isDraft.value) return `当前状态为 ${plan.value?.status || '—'}，仅 Draft 可审批`
  if (isApproved.value) return '计划已审批'
  if (readinessLoading.value) return 'Readiness 评估中'
  if (!hasReadiness.value) return readinessError.value || '暂无 Readiness，无法审批'
  if (!readinessReady.value) return 'Readiness 存在阻断，暂不可审批'
  return ''
})

const freezeDisabledReason = computed(() => {
  if (!hasPlan.value) return '当前无交易计划'
  if (isFrozen.value) return '计划已冻结'
  if (!isDraft.value) return `当前状态为 ${plan.value?.status || '—'}，仅 Draft 可冻结`
  if (!isApproved.value) return '请先完成计划审批'
  if (readinessLoading.value) return 'Readiness 评估中'
  if (!hasReadiness.value) return readinessError.value || '暂无 Readiness，无法冻结'
  if (!readinessReady.value) return 'Risk / Readiness 存在阻断，暂不可冻结'
  return ''
})

const canApprove = computed(() => !approveDisabledReason.value)
const canFreeze = computed(() => !freezeDisabledReason.value)

const inferredPricingStage = computed(() =>
  inferPricingStageForMaterializeUI({
    explicitPricingStage: knownPricingStage.value,
    lifecycleStage: readinessData.value?.lifecycle_stage,
    blockers: readinessData.value?.blockers || [],
  }),
)

const showMorningMaterialize = computed(() =>
  canShowMorningMaterializeButton({
    status: plan.value?.status,
    isFrozen: isFrozen.value,
    pricingStage: inferredPricingStage.value,
  }),
)

const itemColumns = [
  { title: '代码', key: 'stock_code', width: 110 },
  {
    title: '名称',
    key: 'stock_name',
    width: 120,
    ellipsis: { tooltip: true },
    render(row) {
      return row.stock_name || '—'
    },
  },
  {
    title: '方向',
    key: 'side',
    width: 80,
    render(row) {
      const side = String(row.side || '')
      const type = side === 'buy' ? 'error' : side === 'sell' ? 'success' : 'default'
      return h(NTag, { size: 'small', type, bordered: false }, { default: () => side || '—' })
    },
  },
  { title: '优先级', key: 'priority', width: 80 },
  {
    title: () =>
      h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', { class: 'col-tip' }, '参考计划金额'),
          default: () => REF_AMOUNT_TIP,
        },
      ),
    key: 'target_amount',
    width: 130,
    render(row) {
      const n = Number(row.target_amount) || 0
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h('span', n.toLocaleString()),
          default: () => REF_AMOUNT_TIP,
        },
      )
    },
  },
  { title: '状态', key: 'status', width: 90 },
  {
    title: '得分',
    key: 'score',
    width: 80,
    render(row) {
      return Number(row.score || 0).toFixed(2)
    },
  },
  {
    title: '风控',
    key: 'risk',
    ellipsis: { tooltip: true },
    render(row) {
      const code = String(row.risk_code || '').trim()
      const msg = String(row.risk_message || '').trim()
      if (!code && !msg) return '—'
      return code && msg ? `${code}: ${msg}` : code || msg
    },
  },
  {
    title: '策略',
    key: 'strategy_name',
    ellipsis: { tooltip: true },
    render(row) {
      return row.strategy_name || '—'
    },
  },
  {
    title: '观察',
    key: 'observation',
    width: 90,
    render() {
      return h(NTag, { size: 'small', bordered: false }, { default: () => '待观察' })
    },
  },
]

function clearReadiness() {
  readinessData.value = null
  readinessError.value = ''
  readinessLoading.value = false
}

function clearMaterializeResult() {
  materializeResult.value = null
  materializeDisplay.value = null
}

async function loadReadiness(planId) {
  readinessLoading.value = true
  readinessError.value = ''
  readinessData.value = null
  try {
    const res = await getTradePlanReadiness({ planId })
    if (res.code === TRADE_PLAN_CODE_NO_PLAN) {
      readinessError.value = res.message || '无 Readiness（40401）'
      return
    }
    if (res.code !== TRADE_PLAN_CODE_OK || !res.readiness) {
      readinessError.value = res.message || `Readiness 加载失败 (code=${res.code})`
      message.warning(readinessError.value)
      return
    }
    readinessData.value = res.readiness
  } catch (e) {
    readinessError.value = e?.message || String(e)
    message.warning(readinessError.value)
  } finally {
    readinessLoading.value = false
  }
}

async function refresh() {
  loading.value = true
  emptyMessage.value = ''
  clearReadiness()
  try {
    const queryDate = resolveUpcomingQueryTradeDateAfterGenerate({
      planId: preferredGeneratedPlanId.value,
      generatedTradeDate: preferredGeneratedTradeDate.value,
      inputTradeDate: tradeDateInput.value,
    })
    let res = await getUpcomingTradePlan(queryDate)
    // Prefer generated plan: if focus id set but first hit differs, retry once on generated trade_date.
    if (
      preferredGeneratedPlanId.value > 0
      && preferredGeneratedTradeDate.value
      && res.plan
      && !isPreferredGeneratedPlan(preferredGeneratedPlanId.value, res.plan.id)
    ) {
      res = await getUpcomingTradePlan(preferredGeneratedTradeDate.value)
    }
    requestTradeDate.value = res.trade_date || queryDate || tradeDateInput.value || ''
    nextTradingDay.value = res.next_trading_day || ''
    if (res.code === TRADE_PLAN_CODE_NO_UPCOMING) {
      plan.value = null
      knownPricingStage.value = ''
      clearMaterializeResult()
      emptyMessage.value = res.message || '暂无即将交易的计划'
      return
    }
    if (res.code !== TRADE_PLAN_CODE_OK) {
      plan.value = null
      knownPricingStage.value = ''
      clearMaterializeResult()
      emptyMessage.value = res.message || `加载失败 (code=${res.code})`
      message.error(emptyMessage.value)
      return
    }
    const prevId = Number(plan.value?.id) || 0
    plan.value = res.plan
    if (!res.plan) {
      knownPricingStage.value = ''
      clearMaterializeResult()
      emptyMessage.value = res.message || '暂无即将交易的计划'
      return
    }
    if (prevId && prevId !== Number(res.plan.id)) {
      knownPricingStage.value = ''
      clearMaterializeResult()
    }
    if (
      preferredGeneratedPlanId.value > 0
      && isPreferredGeneratedPlan(preferredGeneratedPlanId.value, res.plan.id)
      && res.plan.trade_date
    ) {
      tradeDateInput.value = res.plan.trade_date
    }
    loading.value = false
    // Readiness by plan id keeps focus on the generated plan when preferred.
    const readinessPlanId =
      preferredGeneratedPlanId.value > 0
      && isPreferredGeneratedPlan(preferredGeneratedPlanId.value, res.plan.id)
        ? preferredGeneratedPlanId.value
        : res.plan.id
    await loadReadiness(readinessPlanId)
    return
  } catch (e) {
    plan.value = null
    knownPricingStage.value = ''
    clearMaterializeResult()
    emptyMessage.value = e?.message || String(e)
    message.error(emptyMessage.value)
  } finally {
    loading.value = false
  }
}

function confirmGenerateNext() {
  if (generating.value) return
  const targetDay = nextTradingDay.value || '下一交易日'
  dialog.warning({
    title: '生成交易计划',
    content: `将按盘后 Candidate → Draft → Risk 链路写入 ${targetDay} 的交易计划。不会自动批准、冻结或执行，也不会写入模拟盘观察（paper_sim）；重复生成会创建新的 PlanVersion。`,
    positiveText: '确认生成',
    negativeText: '取消',
    onPositiveClick: () => {
      // Enter generating immediately so the primary button disables before the async call.
      if (generating.value) return false
      generating.value = true
      return (async () => {
        try {
          const res = await generateNextTradePlan({ actor: UI_ACTOR })
          if (res.code !== TRADE_PLAN_CODE_OK) {
            message.error(res.message || `生成失败 (code=${res.code})`)
            return
          }
          if (!res.ok && res.failed_step === 'risk') {
            message.warning(
              `计划 v${res.plan_version || '—'} 已生成，但 Risk 未通过；未批准、未冻结、未执行`,
            )
          } else if (!res.ok) {
            message.error(res.message || `生成失败（${res.failed_step || 'unknown'}）`)
            return
          } else {
            message.success(`已生成 ${res.trade_date} 交易计划 v${res.plan_version}`)
          }
          // Focus the newly generated plan on refresh (do not change upcoming API semantics).
          const newPlanId = Number(res.plan_id) || 0
          if (newPlanId > 0) {
            preferredGeneratedPlanId.value = newPlanId
            preferredGeneratedTradeDate.value = String(res.trade_date || '').trim()
            if (preferredGeneratedTradeDate.value) {
              tradeDateInput.value = preferredGeneratedTradeDate.value
            }
          } else {
            preferredGeneratedPlanId.value = 0
            preferredGeneratedTradeDate.value = ''
            tradeDateInput.value = ''
          }
          await refresh()
        } catch (e) {
          message.error(e?.message || String(e))
        } finally {
          generating.value = false
        }
      })()
    },
  })
}

function confirmApprove() {
  if (!canApprove.value || !plan.value) {
    message.warning(approveDisabledReason.value || '当前不可批准')
    return
  }
  const p = plan.value
  dialog.warning({
    title: '批准交易计划',
    content: () =>
      h('div', { style: 'line-height: 1.7' }, [
        h('div', `目标交易日：${p.trade_date || '—'}`),
        h('div', `计划数量：${(p.items || []).length}`),
        h('div', `计划状态：${p.status || '—'}`),
        h('div', { style: 'margin-top: 8px' }, '批准后：'),
        h('ul', { style: 'margin: 4px 0 0; padding-left: 18px' }, [
          h('li', '状态保持 Draft（不写入 approved status）'),
          h('li', '仅记录 approved_at / approved_by / approved_source'),
          h('li', '不会冻结、不会下单、不会进入 Execution'),
        ]),
      ]),
    positiveText: '确认批准',
    negativeText: '取消',
    onPositiveClick: async () => {
      approving.value = true
      try {
        const res = await approveTradePlan({
          planId: p.id,
          actor: UI_ACTOR,
          source: UI_SOURCE,
        })
        if (res.code !== TRADE_PLAN_CODE_OK || !res.ok) {
          message.error(res.message || `批准失败 (code=${res.code})`)
          return
        }
        message.success('计划已批准（仍为 Draft）')
        await refresh()
      } catch (e) {
        message.error(e?.message || String(e))
      } finally {
        approving.value = false
      }
    },
  })
}

function confirmFreeze() {
  if (!canFreeze.value || !plan.value) {
    message.warning(freezeDisabledReason.value || '当前不可冻结')
    return
  }
  const p = plan.value
  dialog.warning({
    title: '冻结交易计划',
    content: () =>
      h('div', { style: 'line-height: 1.7' }, [
        h('div', `目标交易日：${p.trade_date || '—'}`),
        h('div', `计划数量：${(p.items || []).length}`),
        h('div', `计划状态：${p.status || '—'}`),
        h('div', { style: 'margin-top: 8px' }, '冻结后说明：'),
        h('ul', { style: 'margin: 4px 0 0; padding-left: 18px' }, [
          h('li', '状态将变为 READY'),
          h('li', 'Execution 可以读取该计划'),
          h('li', '后续修改需要生成新版本'),
          h('li', '本操作不会立即下单或进入执行'),
        ]),
      ]),
    positiveText: '确认冻结',
    negativeText: '取消',
    onPositiveClick: async () => {
      freezing.value = true
      try {
        const res = await freezeTradePlan({
          planId: p.id,
          actor: UI_ACTOR,
          reason: 'ui freeze',
          source: UI_SOURCE,
        })
        if (res.code !== TRADE_PLAN_CODE_OK || !res.ok) {
          message.error(res.message || `冻结失败 (code=${res.code})`)
          return
        }
        message.success(res.already_frozen ? '计划已处于冻结状态' : '计划已冻结为 Ready')
        await refresh()
      } catch (e) {
        message.error(e?.message || String(e))
      } finally {
        freezing.value = false
      }
    },
  })
}

function confirmMaterializeMorning() {
  if (!showMorningMaterialize.value || !plan.value) {
    message.warning('当前计划不满足早盘物化条件（需 Draft + after_close_intent + 未冻结）')
    return
  }
  const p = plan.value
  dialog.warning({
    title: '早盘物化',
    content: () =>
      h('div', { style: 'line-height: 1.7' }, [
        h('div', `计划 ID：${p.id}`),
        h('div', `目标交易日：${p.trade_date || '—'}`),
        h('div', { style: 'margin-top: 8px' }, '将执行：'),
        h('ul', { style: 'margin: 4px 0 0; padding-left: 18px' }, [
          h('li', 'Limit Price 早盘物化'),
          h('li', 'Target Volume 早盘物化'),
          h('li', '重新检查 Intent Readiness'),
          h('li', '不会自动批准 / 冻结 / 下单'),
        ]),
      ]),
    positiveText: '确认物化',
    negativeText: '取消',
    onPositiveClick: async () => {
      materializing.value = true
      clearMaterializeResult()
      try {
        const res = await materializeMorningTradePlan({ planId: p.id })
        materializeResult.value = res
        materializeDisplay.value = formatMaterializeMorningDisplay(res)
        if (res.pricing_stage) {
          knownPricingStage.value = res.pricing_stage
        }
        if (!res.success) {
          message.error(materializeDisplay.value.title + '：' + (res.message || res.failed_step || '失败'))
          return
        }
        message.success(
          `早盘物化完成：物化 ${res.materialized_items} 项；Readiness ${res.readiness_ready ? 'Ready' : 'Blocked'}`,
        )
        await refresh()
      } catch (e) {
        const errMsg = e?.message || String(e)
        materializeDisplay.value = {
          ok: false,
          title: '早盘物化失败',
          detailLines: [errMsg],
        }
        message.error(errMsg)
      } finally {
        materializing.value = false
      }
    },
  })
}

onMounted(refresh)
</script>

<template>
  <div class="trade-plan-upcoming">
    <n-space justify="space-between" align="center" style="margin-bottom: 8px">
      <n-space align="center" :wrap="true">
        <n-text strong>交易计划</n-text>
        <n-tag size="small" type="info" :bordered="false">观察阶段</n-tag>
        <n-text>下一交易日：{{ nextTradingDay || '—' }}</n-text>
      </n-space>
      <n-space align="center">
        <n-input
          v-model:value="tradeDateInput"
          :placeholder="tradeDatePlaceholder"
          clearable
          :disabled="generating"
          style="width: 240px"
        />
        <n-button
          type="primary"
          :loading="generating"
          :disabled="loading || readinessLoading || approving || freezing || materializing || generating"
          @click="confirmGenerateNext"
        >
          {{ generating ? '生成中，请等待。' : '生成明日计划' }}
        </n-button>
        <n-button
          :loading="loading || readinessLoading"
          :disabled="generating || materializing"
          @click="refresh"
        >
          刷新
        </n-button>
      </n-space>
    </n-space>

    <n-text v-if="generating" depth="3" class="generating-hint">
      生成中，请等待。
    </n-text>

    <n-text depth="3" class="observation-note">
      当前计划用于策略观察与验证，不代表实际成交价格或执行订单。生成结果为 Draft，不会自动进入模拟盘观察（paper_sim）。
    </n-text>

    <n-spin :show="loading">
      <template v-if="hasPlan">
        <n-space vertical :size="10" style="margin-bottom: 14px">
          <div class="meta-grid">
            <div class="meta-item">
              <span class="meta-k">交易计划日期</span>
              <span class="meta-v">{{ plan.trade_date || '—' }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-k">计划版本</span>
              <span class="meta-v">{{ planVersionLabel }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-k">生成时间</span>
              <span class="meta-v">{{ generatedAtLabel }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-k">生成来源</span>
              <span class="meta-v">{{ sourceLabel }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-k">触发方式</span>
              <span class="meta-v">{{ triggerLabel }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-k">当前状态</span>
              <n-tag size="small" :type="isReady || isFrozen ? 'success' : 'default'" :bordered="false">
                {{ lifecycleLabel }}
              </n-tag>
            </div>
          </div>

          <div v-if="isDraft" class="draft-status-hint">
            <n-text strong>Draft</n-text>
            <n-text depth="3">· {{ isApproved ? '已批准（仍为 Draft）' : '未批准' }}</n-text>
            <n-text depth="3">· 不会进入模拟盘观察</n-text>
            <n-text depth="3" class="draft-status-detail">
              生成结果不会自动写入 paper_sim_*；需人工批准并冻结后，且由 Paper Trading Job 执行，才会进入模拟盘观察。
            </n-text>
          </div>

          <n-space align="center" :wrap="true">
            <n-text>审批状态：</n-text>
            <n-tag
              size="small"
              :type="isApproved ? 'success' : 'warning'"
              :bordered="false"
            >
              {{ approvalLabel }}
            </n-tag>
            <n-text depth="3">|</n-text>
            <n-text>冻结状态：</n-text>
            <n-tag
              size="small"
              :type="isFrozen ? 'success' : 'default'"
              :bordered="false"
            >
              {{ freezeLabel }}
            </n-tag>
            <template v-if="plan.freeze?.approved_at">
              <n-text depth="3">|</n-text>
              <n-text depth="3">ApprovedAt：{{ plan.freeze.approved_at }}</n-text>
            </template>
            <template v-if="plan.freeze?.freeze_at">
              <n-text depth="3">|</n-text>
              <n-text>FreezeAt：{{ plan.freeze.freeze_at }}</n-text>
            </template>
          </n-space>

          <n-space align="center" :wrap="true" class="action-row">
            <n-button
              v-if="showMorningMaterialize"
              type="info"
              :loading="materializing"
              :disabled="loading || approving || freezing || generating || readinessLoading"
              @click="confirmMaterializeMorning"
            >
              早盘物化
            </n-button>
            <n-button
              type="primary"
              :loading="approving"
              :disabled="!canApprove || loading || freezing || materializing"
              @click="confirmApprove"
            >
              批准计划
            </n-button>
            <n-button
              type="warning"
              :loading="freezing"
              :disabled="!canFreeze || loading || approving || materializing"
              @click="confirmFreeze"
            >
              冻结计划
            </n-button>
            <n-text v-if="!canApprove && !isApproved" depth="3">
              {{ approveDisabledReason }}
            </n-text>
            <n-text v-else-if="!canFreeze && !isFrozen" depth="3">
              {{ freezeDisabledReason }}
            </n-text>
          </n-space>

          <div v-if="materializeDisplay" class="materialize-result-block">
            <n-space align="center" :wrap="true" style="margin-bottom: 4px">
              <n-text strong>早盘物化结果</n-text>
              <n-tag
                size="small"
                :type="materializeDisplay.ok ? 'success' : 'error'"
                :bordered="false"
              >
                {{ materializeDisplay.ok ? '成功' : '失败' }}
              </n-tag>
            </n-space>
            <ul class="materialize-result-list">
              <li v-for="(line, i) in materializeDisplay.detailLines" :key="'m-' + i">
                {{ line }}
              </li>
            </ul>
            <template v-if="materializeResult?.success">
              <n-text depth="3" class="section-note">
                materialized_items={{ materializeResult.materialized_items }}
                · readiness_ready={{ materializeResult.readiness_ready ? 'true' : 'false' }}
                · blockers={{ (materializeResult.blockers || []).length }}
              </n-text>
            </template>
          </div>

          <div class="risk-block">
            <n-space align="center" :wrap="true" style="margin-bottom: 4px">
              <n-text strong>Risk结果：</n-text>
              <n-tag
                size="small"
                :type="plan.risk?.passed ? 'success' : 'warning'"
                :bordered="false"
              >
                {{ plan.risk?.passed ? '通过' : '未通过' }}
              </n-tag>
            </n-space>
            <n-text depth="3" class="section-note">
              Risk通过表示风险规则检查通过，不代表计划一定盈利。
            </n-text>
            <div v-if="(plan.risk?.reasons || []).length" class="risk-reasons">
              <n-text depth="3">明细：</n-text>
              <ul>
                <li v-for="(r, i) in plan.risk.reasons" :key="i">{{ r }}</li>
              </ul>
            </div>
          </div>

          <div class="readiness-block">
            <n-space align="center" :wrap="true" style="margin-bottom: 6px">
              <n-text strong>Intent Readiness</n-text>
              <n-tag size="small" type="info" :bordered="false">门禁观测</n-tag>
              <n-tag v-if="readinessLoading" size="small" :bordered="false">评估中</n-tag>
            </n-space>

            <n-spin :show="readinessLoading" size="small">
              <template v-if="hasReadiness">
                <n-space align="center" :wrap="true">
                  <n-text>Status：</n-text>
                  <n-tag
                    size="small"
                    :type="readinessReady ? 'success' : 'error'"
                    :bordered="false"
                  >
                    {{ readinessReady ? 'Ready' : 'Blocked' }}
                  </n-tag>
                  <n-text depth="3">|</n-text>
                  <n-text>Stage：{{ readinessData.lifecycle_stage || '—' }}</n-text>
                </n-space>

                <div v-if="!readinessReady" class="why-blocked">
                  <n-text strong>为什么 Blocked</n-text>
                  <div class="why-line">阻断数量：{{ blockerCount }}</div>
                  <div class="why-line">影响：当前计划不可Approve / Freeze</div>
                  <n-text depth="3" class="section-note">
                    该计划可以用于策略观察，但尚未满足执行准备条件。
                  </n-text>
                </div>
                <n-text v-else depth="3" class="section-note" style="display: block; margin-top: 6px">
                  Readiness Ready：无阻断项；WARN 不阻断 Ready。
                </n-text>

                <div v-if="blockerCount" class="readiness-list">
                  <n-text depth="3">Blockers（{{ blockerCount }}）：</n-text>
                  <ul>
                    <li v-for="(b, i) in readinessData.blockers" :key="'b-' + i">
                      <div class="finding-card">
                        <div class="finding-row">
                          <span class="finding-k">问题：</span>
                          <span class="finding-v">{{ describeReadinessFinding(b).title }}</span>
                        </div>
                        <div class="finding-row finding-code">
                          <span class="finding-k">代码：</span>
                          <span class="finding-v">[{{ b.rule_code || '—' }}] {{ b.code || '—' }}</span>
                        </div>
                        <div class="finding-row">
                          <span class="finding-k">影响：</span>
                          <span class="finding-v">{{ describeReadinessFinding(b).impact }}</span>
                        </div>
                        <div class="finding-row">
                          <span class="finding-k">原因：</span>
                          <span class="finding-v">{{ b.message || describeReadinessFinding(b).reason }}</span>
                        </div>
                      </div>
                    </li>
                  </ul>
                </div>
                <n-text v-else depth="3" style="display: block; margin-top: 4px">Blockers：无</n-text>

                <div v-if="warningCount" class="readiness-list">
                  <n-text depth="3">Warnings（{{ warningCount }}，不影响 Ready）：</n-text>
                  <ul>
                    <li v-for="(w, i) in readinessData.warnings" :key="'w-' + i">
                      <div class="finding-card">
                        <div class="finding-row">
                          <span class="finding-k">问题：</span>
                          <span class="finding-v">{{ describeReadinessFinding(w).title }}</span>
                        </div>
                        <div class="finding-row finding-code">
                          <span class="finding-k">代码：</span>
                          <span class="finding-v">[{{ w.rule_code || '—' }}] {{ w.code || '—' }}</span>
                        </div>
                        <div class="finding-row">
                          <span class="finding-k">影响：</span>
                          <span class="finding-v">{{ describeReadinessFinding(w).impact }}</span>
                        </div>
                        <div class="finding-row">
                          <span class="finding-k">原因：</span>
                          <span class="finding-v">{{ w.message || describeReadinessFinding(w).reason }}</span>
                        </div>
                      </div>
                    </li>
                  </ul>
                </div>
                <n-text v-else depth="3" style="display: block; margin-top: 4px">Warnings：无</n-text>

                <n-text depth="3" style="display: block; margin-top: 6px">
                  WARN 不阻断 Ready；Approve / Freeze 需两步确认；本页不提供下单。
                </n-text>
              </template>
              <n-empty
                v-else
                size="small"
                :description="readinessError || '暂无 Readiness'"
              />
            </n-spin>
          </div>

          <n-text v-if="!isFrozen" depth="3" style="display: block">
            当前计划未冻结；批准后仍为 Draft，冻结后才会变为 Ready 供 Execution 读取。
          </n-text>
        </n-space>

        <n-data-table
          v-if="(plan.items || []).length"
          size="small"
          :columns="itemColumns"
          :data="plan.items"
          :row-key="(row) => `${row.stock_code}-${row.priority}`"
        />
        <n-empty v-else description="计划无明细 items" />

        <div class="review-block">
          <n-text strong>交易后复盘</n-text>
          <n-text depth="3" class="section-note" style="display: block; margin: 4px 0 6px">
            用于后续评估策略有效性。本次仅展示待记录项，不自动统计。
          </n-text>
          <ul class="review-list">
            <li>待记录：开盘价</li>
            <li>待记录：最高价</li>
            <li>待记录：最低价</li>
            <li>待记录：收盘价</li>
            <li>待记录：次日收益</li>
          </ul>
        </div>
      </template>

      <n-empty
        v-else
        :description="emptyMessage || (requestTradeDate ? `无 upcoming（${requestTradeDate}）` : '暂无即将交易的计划')"
      />
    </n-spin>
  </div>
</template>

<style scoped>
.trade-plan-upcoming {
  height: 100%;
  overflow: auto;
  padding: 4px 2px 12px;
  box-sizing: border-box;
}
.meta-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 8px 16px;
  padding: 8px 0;
}
.meta-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.meta-k {
  font-size: 12px;
  opacity: 0.65;
}
.meta-v {
  font-size: 14px;
}
.risk-reasons ul,
.readiness-list ul,
.review-list {
  margin: 4px 0 0;
  padding-left: 18px;
}
.readiness-list li {
  margin-bottom: 10px;
}
.finding-card {
  line-height: 1.55;
}
.finding-row {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}
.finding-k {
  opacity: 0.65;
  font-size: 12px;
  min-width: 36px;
}
.finding-v {
  font-size: 13px;
}
.finding-code .finding-v {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
}
.risk-block,
.readiness-block,
.review-block,
.materialize-result-block {
  padding: 8px 0 2px;
  border-top: 1px solid rgba(128, 128, 128, 0.2);
}
.materialize-result-list {
  margin: 4px 0 0;
  padding-left: 18px;
  font-size: 13px;
  line-height: 1.55;
}
.why-blocked {
  margin-top: 8px;
  padding: 8px 10px;
  background: rgba(208, 48, 80, 0.06);
  border-radius: 4px;
}
.why-line {
  margin-top: 4px;
  font-size: 13px;
}
.section-note {
  display: block;
  font-size: 12px;
  margin-top: 2px;
}
.observation-note {
  display: block;
  margin-bottom: 12px;
  font-size: 12px;
}
.generating-hint {
  display: block;
  margin: -4px 0 10px;
  font-size: 13px;
  color: var(--n-primary-color, #2080f0);
}
.draft-status-hint {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 6px 10px;
  padding: 8px 10px;
  margin: 2px 0 4px;
  background: rgba(240, 160, 32, 0.08);
  border-radius: 4px;
  border-left: 3px solid rgba(240, 160, 32, 0.65);
  font-size: 13px;
}
.draft-status-detail {
  flex-basis: 100%;
  font-size: 12px;
  margin-top: 2px;
}
.action-row {
  padding: 6px 0;
  border-top: 1px solid rgba(128, 128, 128, 0.15);
  border-bottom: 1px solid rgba(128, 128, 128, 0.15);
}
.col-tip {
  border-bottom: 1px dashed rgba(128, 128, 128, 0.55);
  cursor: help;
}
.review-list li {
  margin-bottom: 2px;
  font-size: 13px;
}
</style>
