<script setup>
import { computed, onMounted, onBeforeUnmount, reactive, ref, watch } from 'vue'
import {
  NButton,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NSelect,
  NSpace,
  NStatistic,
  NText,
  useMessage,
} from 'naive-ui'
import * as WailsApp from '../../wailsjs/go/main/App'
import {
  createManualTradeIntent,
  confirmManualTradeIntent,
  executeManualTradeIntent,
} from '../api/manualTrade'
import { evaluateTradeRiskGate } from '../utils/riskGate'
import { getLastMarketModeKey } from '../utils/marketStatusBar'
import {
  MARGIN_ORDER_TYPES,
  createMarginFallback,
  isMarginOrderType,
  marginOrderSide,
  normalizeMarginSnapshot,
  toBackendOrderKind,
} from '../utils/marginRiskModel'
import {
  accountModeText,
  formatMoney,
  formatPercent,
  formatRiskValue,
  orderTypeText,
  positionTypeText,
} from '../utils/tradingFormat'
import { EventsOn, EventsOff } from '../../wailsjs/runtime'

const props = defineProps({
  prefill: { type: Object, default: null },
})

const message = useMessage()
const appMethod = name => WailsApp[name]
const loading = ref(false)
const snap = ref(null)
const form = reactive({
  stockCode: '',
  stockName: '',
  orderType: 'normal_buy',
  price: 0,
  volume: 100,
  reason: '',
})

function applyPrefill(p) {
  if (!p) return
  if (p.stockCode) form.stockCode = String(p.stockCode)
  if (p.stockName) form.stockName = String(p.stockName)
  if (Number(p.price) > 0) form.price = Number(p.price)
  form.orderType = 'normal_buy'
  if (!form.reason) form.reason = '模拟交易'
}

watch(() => props.prefill, (v) => applyPrefill(v), { immediate: true, deep: true })

function onPaperBuyPrefill(payload) {
  applyPrefill(payload)
}
const margin = computed(() => snap.value?.margin || {})
const positions = computed(() => (snap.value?.positions || []).map(item => ({
  ...item,
  positionTypeLabel: positionTypeText(item.positionType),
})))
const orders = computed(() => (snap.value?.orders || []).map(item => ({
  ...item,
  orderTypeLabel: orderTypeText(item.orderType || item.businessType, item.side),
})))
const riskEvents = computed(() => (snap.value?.riskEvents || []).map(item => ({
  ...item,
  currentText: formatRiskValue(item.currentValue, item.unit),
  thresholdText: formatRiskValue(item.threshold, item.unit),
})))

const posColumns = [
  { title: '代码', key: 'stockCode' },
  { title: '名称', key: 'stockName' },
  { title: '持仓类型', key: 'positionTypeLabel' },
  { title: '数量', key: 'volume' },
  { title: '可卖', key: 'sellable' },
  { title: '成本', key: 'avgCost' },
]

const orderColumns = [
  { title: 'ID', key: 'id', width: 60 },
  { title: '代码', key: 'stockCode' },
  { title: '订单种类', key: 'orderTypeLabel' },
  { title: '状态', key: 'status' },
  { title: '价格', key: 'price' },
  { title: '数量', key: 'volume' },
  { title: '成交价', key: 'filledPrice' },
  { title: '费用', key: 'fee' },
]

const riskColumns = [
  { title: '原因码', key: 'reasonCode' },
  { title: '当前值', key: 'currentText' },
  { title: '阈值', key: 'thresholdText' },
  { title: '说明', key: 'message' },
]

async function refresh() {
  loading.value = true
  try {
    const base = await WailsApp.GetPaperAccountSnapshot(0)
    const getMarginSnapshot = appMethod('GetPaperMarginSnapshot')
    if (typeof getMarginSnapshot === 'function') {
      try {
        snap.value = normalizeMarginSnapshot(await getMarginSnapshot(0), base)
      } catch (error) {
        snap.value = createMarginFallback(base, error)
      }
    } else {
      snap.value = createMarginFallback(base)
    }
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    loading.value = false
  }
}

async function resetAcc() {
  await WailsApp.ResetPaperAccount(1000000)
  message.success('模拟账户已重置为 100 万')
  await refresh()
}

async function settle() {
  const id = snap.value?.account?.id || 0
  await WailsApp.SettlePaperSellable(id)
  message.success('已结算 T+1 可卖')
  await refresh()
}

async function submit() {
  const side = marginOrderSide(form.orderType)
  const kind = toBackendOrderKind(form.orderType)
  if (['normal_buy', 'margin_buy', 'buy'].includes(kind)) {
    const gate = evaluateTradeRiskGate({
      marketModeKey: getLastMarketModeKey(),
      direction: '买入',
    })
    if (!gate.ok) {
      message.error(gate.reason)
      return
    }
  }
  if (!form.stockCode.trim()) {
    message.warning('股票代码不能为空')
    return
  }
  if (!(Number(form.volume) > 0) || !(Number(form.price) > 0)) {
    message.warning('请填写有效的价格与数量')
    return
  }
  try {
    if (isMarginOrderType(kind)) {
      const submitMarginOrder = appMethod('SubmitPaperMarginOrder')
      if (typeof submitMarginOrder !== 'function') {
        message.warning('两融下单绑定尚未生成，未提交订单')
        return
      }
      await submitMarginOrder({
        accountId: snap.value?.account?.id || 0,
        stockCode: form.stockCode.trim(),
        stockName: form.stockName.trim() || form.stockCode.trim(),
        kind,
        price: Number(form.price),
        volume: Number(form.volume),
        reason: form.reason || '模拟执行台',
      })
      message.success('模拟成交完成')
      await refresh()
      return
    }

    // 普通单：ManualTrade Intent → ExecutionService（禁止直连纸面会计 Submit）
    const created = await createManualTradeIntent({
      account_id: snap.value?.account?.id || 0,
      symbol: form.stockCode.trim(),
      stock_name: form.stockName.trim() || form.stockCode.trim(),
      side,
      price: Number(form.price),
      volume: Number(form.volume),
      reason: form.reason || '模拟交易',
      order_kind: 'normal',
    })
    await confirmManualTradeIntent(created.id)
    const exec = await executeManualTradeIntent(created.id)
    if (exec.status === 'submitted') {
      message.success(`模拟成交完成（订单 #${exec.order_id || ''}）`)
      await refresh()
      return
    }
    message.error(
      exec.error_message ||
        exec.error_code ||
        `模拟成交失败（${exec.status || 'rejected'}）`,
    )
  } catch (e) {
    message.error(e?.message || String(e))
  }
}

async function confirmBroker() {
  try {
    const res = await WailsApp.ConfirmBrokerOrderPlan(
      form.stockCode.trim(),
      form.stockName.trim() || form.stockCode.trim(),
      marginOrderSide(form.orderType),
      Number(form.price),
      Number(form.volume),
      form.reason || '人工确认送单',
    )
    if (res?.ok) message.info(res.message || '已记录人工确认')
    else message.error(res?.message || '失败')
  } catch (e) {
    message.error(e?.message || String(e))
  }
}

async function accrueInterest() {
  const accrueMarginInterest = appMethod('AccruePaperMarginInterest')
  if (typeof accrueMarginInterest !== 'function') {
    message.warning('计息绑定尚未生成')
    return
  }
  try {
    await accrueMarginInterest(snap.value?.account?.id || 0, '')
    message.success('两融利息已计提')
    await refresh()
  } catch (error) {
    message.error(error?.message || String(error))
  }
}

async function runRiskScan() {
  const scanMarginRisk = appMethod('RunPaperMarginRiskScan')
  if (typeof scanMarginRisk !== 'function') {
    message.warning('风险扫描绑定尚未生成')
    return
  }
  try {
    await scanMarginRisk(snap.value?.account?.id || 0)
    message.success('两融风险扫描完成')
    await refresh()
  } catch (error) {
    message.error(error?.message || String(error))
  }
}

onMounted(() => {
  refresh()
  EventsOn('paperBuyPrefill', onPaperBuyPrefill)
})

onBeforeUnmount(() => {
  EventsOff('paperBuyPrefill')
})
</script>

<template>
  <div class="paper-panel">
    <n-text depth="3" class="hint">
      模拟盘：现金/持仓/委托本地闭环，含佣金、印花税与 T+1。券商送单默认仅记录人工确认，不实盘下单。
    </n-text>
    <n-space style="margin-bottom: 12px">
      <n-button :loading="loading" @click="refresh">刷新</n-button>
      <n-button @click="settle">结算可卖(T+1)</n-button>
      <n-button secondary @click="accrueInterest">计提两融利息</n-button>
      <n-button secondary @click="runRiskScan">扫描两融风险</n-button>
      <n-button type="warning" secondary @click="resetAcc">重置账户</n-button>
    </n-space>

    <div v-if="snap?.account" class="account-grid">
      <n-statistic label="账户模式" :value="accountModeText(margin.accountMode)" />
      <n-statistic label="现金" :value="formatMoney(snap.account.cash)" />
      <n-statistic label="权益" :value="formatMoney(snap.account.equity)" />
      <n-statistic label="融资余额" :value="formatMoney(margin.financingBalance)" />
      <n-statistic label="融券负债" :value="formatMoney(margin.securitiesLiability)" />
      <n-statistic label="可用保证金" :value="formatMoney(margin.availableMargin)" />
      <n-statistic label="维持担保比例" :value="formatPercent(margin.maintenanceRatio)" />
      <n-statistic label="净 / 总暴露" :value="`${formatMoney(margin.netExposure)} / ${formatMoney(margin.grossExposure)}`" />
    </div>
    <n-text v-if="snap && !snap.marginApiAvailable" type="warning" class="fallback">
      两融服务不可用，已回退普通模拟账户快照。
    </n-text>

    <n-form inline label-placement="left" :show-feedback="false" class="form">
      <n-form-item label="代码">
        <n-input v-model:value="form.stockCode" style="width: 110px" />
      </n-form-item>
      <n-form-item label="名称">
        <n-input v-model:value="form.stockName" style="width: 110px" />
      </n-form-item>
      <n-form-item label="订单种类">
        <n-select v-model:value="form.orderType" :options="MARGIN_ORDER_TYPES" style="width: 130px" />
      </n-form-item>
      <n-form-item label="价格">
        <n-input-number v-model:value="form.price" :min="0" :step="0.01" />
      </n-form-item>
      <n-form-item label="数量">
        <n-input-number v-model:value="form.volume" :min="100" :step="100" />
      </n-form-item>
      <n-form-item>
        <n-button type="primary" @click="submit">模拟成交</n-button>
      </n-form-item>
      <n-form-item>
        <n-button secondary @click="confirmBroker">人工确认送单</n-button>
      </n-form-item>
    </n-form>

    <n-text strong>持仓</n-text>
    <n-data-table
      size="small"
      :columns="posColumns"
      :data="positions"
      :bordered="false"
      style="margin: 8px 0 16px"
    />
    <n-text strong>最近委托</n-text>
    <n-data-table
      size="small"
      :columns="orderColumns"
      :data="orders"
      :bordered="false"
      style="margin-top: 8px"
    />
    <n-text strong>两融风险事件</n-text>
    <n-data-table
      size="small"
      :columns="riskColumns"
      :data="riskEvents"
      :bordered="false"
      style="margin-top: 8px"
    />
  </div>
</template>

<style scoped>
.paper-panel {
  padding: 12px 8px;
  height: 100%;
  overflow: auto;
}
.hint {
  display: block;
  margin-bottom: 12px;
  font-size: 13px;
}
.account-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(130px, 1fr));
  gap: 12px;
  margin-bottom: 12px;
}
.account-grid > * {
  padding: 10px 12px;
  border: 1px solid rgba(128, 128, 128, .16);
  border-radius: 8px;
}
.fallback {
  display: block;
  margin-bottom: 12px;
}
.form {
  flex-wrap: wrap;
  margin-bottom: 12px;
}
</style>
