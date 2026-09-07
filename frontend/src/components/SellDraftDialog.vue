<script setup>
/**
 * Paper sim T-sell draft dialog (Phase14-A-R1-B).
 * Creates POST /api/tradeplans/t-sell/draft then navigates to trade plan page.
 */
import { computed, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  NAlert,
  NButton,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NSpace,
  NText,
  useMessage,
} from 'naive-ui'
import { createTSellDraft, mapTSellDraftErrorMessage, T_SELL_SOURCE_SESSION } from '../api/tradePlansTSell'
import {
  maxSellQuantity,
  sellRowStockCode,
  sellRowStockName,
  validateSellQuantity,
} from '../utils/portfolioSellEntry.js'

const props = defineProps({
  show: { type: Boolean, default: false },
  row: { type: Object, default: null },
  tradeDate: { type: String, default: '' },
  actor: { type: String, default: 'ui:portfolio-sell' },
  /** t_sell (default) | exit_review (Phase14-M1). */
  sourceSession: { type: String, default: T_SELL_SOURCE_SESSION },
  /** Override source display; e.g. 退出复评 */
  sourceLabel: { type: String, default: '' },
  /** Pre-fill reason when dialog opens. */
  defaultReason: { type: String, default: '' },
})

const emit = defineEmits(['update:show', 'created'])

const router = useRouter()
const message = useMessage()
const submitting = ref(false)

const form = reactive({
  quantity: 100,
  reason: 'manual sell',
})

const stockCode = computed(() => sellRowStockCode(props.row))
const stockName = computed(() => sellRowStockName(props.row))
const maxQty = computed(() => maxSellQuantity(props.row))
const positionStateLabel = computed(() => {
  const raw =
    props.row?.positionState ??
    props.row?.position_state ??
    props.row?.state ??
    ''
  return String(raw || '').trim() || '—'
})
const displaySourceLabel = computed(() => {
  const custom = String(props.sourceLabel || '').trim()
  if (custom) return custom
  if (props.sourceSession === 'exit_review') return '退出复评'
  return ''
})

const quantityValid = computed(() => validateSellQuantity(form.quantity, maxQty.value) != null)

watch(
  () => props.show,
  (visible) => {
    if (!visible) return
    const max = maxSellQuantity(props.row)
    form.quantity = max >= 100 ? 100 : max > 0 ? max : 1
    form.reason = String(props.defaultReason || '').trim() || 'manual sell'
  },
)

function close() {
  emit('update:show', false)
}

async function submit() {
  const qty = validateSellQuantity(form.quantity, maxQty.value)
  if (qty == null) {
    message.warning(`数量须大于 0 且不超过可卖数量（最多 ${maxQty.value} 股）`)
    return
  }
  if (!stockCode.value) {
    message.warning('股票代码无效')
    return
  }
  submitting.value = true
  try {
    const res = await createTSellDraft({
      tradeDate: props.tradeDate || undefined,
      stockCode: stockCode.value,
      quantity: qty,
      actor: props.actor,
      reason: form.reason || 'manual sell',
      sourceSession: props.sourceSession,
    })
    message.success(`卖出计划草稿已创建（#${res.planId}）`)
    emit('created', { planId: res.planId })
    close()
    await router.push({
      name: 'tradePlanUpcoming',
      query: { plan_id: String(res.planId), flow: 'sell' },
    })
  } catch (e) {
    const code = e?.code || ''
    const text =
      e?.userMessage ||
      (code ? mapTSellDraftErrorMessage(code, e?.httpStatus) : e?.message || String(e))
    message.error(text)
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    title="创建卖出计划"
    style="width: min(440px, 92vw)"
    :mask-closable="!submitting"
    @update:show="(v) => { if (!v) close() }"
  >
    <n-alert type="info" :bordered="false" style="margin-bottom: 12px">
      <template v-if="displaySourceLabel">
        来源：{{ displaySourceLabel }} · Paper 模拟 · 创建后请在交易计划页批准并冻结
      </template>
      <template v-else>
        Paper 模拟账户 · T-sell 人工卖出 · 创建后请在交易计划页批准并冻结
      </template>
    </n-alert>

    <n-form label-placement="left" label-width="72">
      <n-form-item v-if="displaySourceLabel" label="来源">
        <n-text>{{ displaySourceLabel }}</n-text>
      </n-form-item>
      <n-form-item label="代码">
        <n-text>{{ stockCode || '—' }}</n-text>
      </n-form-item>
      <n-form-item label="名称">
        <n-text>{{ stockName || '—' }}</n-text>
      </n-form-item>
      <n-form-item label="可卖数量">
        <n-text>{{ maxQty.toLocaleString('zh-CN') }} 股</n-text>
      </n-form-item>
      <n-form-item v-if="positionStateLabel !== '—'" label="持仓状态">
        <n-text>{{ positionStateLabel }}</n-text>
      </n-form-item>
      <n-form-item label="卖出数量">
        <n-input-number
          v-model:value="form.quantity"
          :min="1"
          :max="maxQty"
          :step="100"
          :disabled="maxQty <= 0"
          style="width: 100%"
        />
      </n-form-item>
      <n-form-item label="原因">
        <n-input
          v-model:value="form.reason"
          type="textarea"
          placeholder="可选，默认 manual sell"
          :autosize="{ minRows: 2, maxRows: 4 }"
        />
      </n-form-item>
    </n-form>

    <template #footer>
      <n-space justify="end">
        <n-button :disabled="submitting" @click="close">取消</n-button>
        <n-button
          type="warning"
          :loading="submitting"
          :disabled="!quantityValid || maxQty <= 0"
          @click="submit"
        >
          创建卖出草稿
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>
