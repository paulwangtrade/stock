<script setup>
/**
 * Phase15-B-A 首页展示重构（只读）。
 * 主数据：GET /api/investment/home（资产来自后端 Snapshot 投影，前端不重算）。
 * 计划：GET /api/tradeplans/upcoming（只读）。
 * 开盘开关：Wails GetPaperOpenBuyConfig（只读，失败则按未开启）。
 * 旧运维区块保留在模板中，SHOW_LEGACY_HOME_BLOCKS=false 隐藏，不删数据映射。
 */
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  NAlert,
  NButton,
  NEmpty,
  NList,
  NListItem,
  NSpace,
  NSpin,
  NStatistic,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import {
  attentionLabel,
  autoExecuteSentence,
  getInvestmentHome,
  itemTypeLabel,
  planUserStatus,
  sanitizeInternalCopy,
  tradingStatusLabel,
  tradingStepLabel,
} from '../api/investmentHome'
import { TRADE_PLAN_CODE_NO_UPCOMING, getUpcomingTradePlan } from '../api/tradePlans'
import { positionStateTagType } from '../utils/positionStateDisplay.js'
import { applyStockClickAction, toStockDisplay } from '../utils/stockDisplay.js'
import { GetPaperOpenBuyConfig } from '../../wailsjs/go/main/App'
import StockKlineModal from './StockKlineModal.vue'
import StockLink from './StockLink.vue'

/** Hidden, not deleted. Flip to true only for developer inspection. */
const SHOW_LEGACY_HOME_BLOCKS = false

const router = useRouter()
const message = useMessage()
const loading = ref(false)
const errorMessage = ref('')
const view = ref(null)
const upcomingPlan = ref(null)
const enableOpenBuy = ref(false)

const portfolio = computed(() => view.value?.portfolio || null)
const decision = computed(() => view.value?.decision || null)
const daily = computed(() => view.value?.daily || null)
const trading = computed(() => view.value?.trading || null)
const dailyAttention = computed(() => view.value?.dailyAttention || null)
const attentionRows = computed(() => dailyAttention.value?.items || [])
const positionStates = computed(() => view.value?.positionStates || [])

const opportunityCards = computed(() =>
  Array.isArray(view.value?.opportunityCards) ? view.value.opportunityCards : [],
)

function formatScore(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return String(Math.round(n))
}

function formatScoreGap(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  const r = Math.round(n)
  return r > 0 ? `+${r}` : String(r)
}

const planCard = computed(() => planUserStatus(upcomingPlan.value))
const planItemCount = computed(() => {
  const items = upcomingPlan.value?.items
  return Array.isArray(items) ? items.length : 0
})
const planStocks = computed(() => {
  const items = upcomingPlan.value?.items
  if (!Array.isArray(items)) return []
  const out = []
  for (const it of items) {
    const model = toStockDisplay({
      stock_code: it?.stock_code,
      stock_name: it?.stock_name,
    })
    if (model) out.push(model)
    if (out.length >= 5) break
  }
  return out
})

const klineModal = reactive({
  visible: false,
  title: '',
  chartCode: '',
  stockName: '',
})

function openStockKline(model) {
  applyStockClickAction(model, klineModal)
}

const isTodayPlan = computed(() => {
  const planDate = String(upcomingPlan.value?.trade_date || '').trim()
  const homeDate = String(view.value?.tradeDate || '').trim()
  if (!planDate || !homeDate) return !!upcomingPlan.value
  return planDate === homeDate
})
const currentStatusText = computed(() =>
  autoExecuteSentence({
    enableOpenBuy: !!enableOpenBuy.value,
    hasPlan: !!upcomingPlan.value,
    isFrozen: planCard.value.key === 'locked',
    isTodayPlan: !!isTodayPlan.value,
  }),
)

const pipelineSteps = computed(() => {
  const t = trading.value
  if (!t) return []
  return [
    { key: 'materialize', status: t.materializeStatus },
    { key: 'approve', status: t.approveStatus },
    { key: 'freeze', status: t.freezeStatus },
    { key: 'execution', status: t.executionStatus },
    { key: 'settlement', status: t.settlementStatus },
  ]
})

function formatMoney(v) {
  if (v === null || v === undefined) return '—'
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return n.toLocaleString('zh-CN', { maximumFractionDigits: 2 })
}

function pnlColor(v) {
  const n = Number(v)
  if (!Number.isFinite(n) || n === 0) return undefined
  return n > 0 ? '#18a058' : '#d03050'
}

function attentionTagType(level) {
  const s = String(level || '').toUpperCase()
  if (s === 'REVIEW') return 'error'
  if (s === 'WATCH') return 'warning'
  if (s === 'HOLD') return 'success'
  return 'default'
}

function statusTagType(status) {
  const s = String(status || '').toUpperCase()
  if (s === 'PASS') return 'success'
  if (s === 'FAIL') return 'error'
  if (s === 'SKIP') return 'warning'
  if (s === 'PENDING') return 'info'
  return 'default'
}

function planTagType(key) {
  if (key === 'locked') return 'success'
  if (key === 'prepared') return 'info'
  if (key === 'pending_confirm') return 'warning'
  return 'default'
}

function opportunityTagType(label) {
  if (label === '建议研究') return 'warning'
  if (label === '等待确认') return 'info'
  return 'default'
}

function goDiscover() {
  router.push({ name: 'stockScreen' })
}

function goPlan() {
  router.push({ name: 'tradePlanUpcoming' })
}

function goPortfolio() {
  router.push({ name: 'portfolioDashboard' })
}

async function loadOpenBuySwitch() {
  try {
    const cfg = await GetPaperOpenBuyConfig()
    enableOpenBuy.value = !!cfg?.enablePaperOpenBuy
  } catch {
    enableOpenBuy.value = false
  }
}

async function loadUpcoming(tradeDate) {
  try {
    const res = await getUpcomingTradePlan(tradeDate || undefined)
    if (!res?.ok || res.code === TRADE_PLAN_CODE_NO_UPCOMING || !res.plan) {
      upcomingPlan.value = null
      return
    }
    upcomingPlan.value = res.plan
  } catch {
    upcomingPlan.value = null
  }
}

async function refresh() {
  loading.value = true
  errorMessage.value = ''
  try {
    const homePromise = getInvestmentHome()
    const switchPromise = loadOpenBuySwitch()
    view.value = await homePromise
    await Promise.all([switchPromise, loadUpcoming(view.value?.tradeDate)])
  } catch (e) {
    view.value = null
    upcomingPlan.value = null
    errorMessage.value = e?.message || String(e)
    message.error(errorMessage.value)
  } finally {
    loading.value = false
  }
}

onMounted(refresh)
</script>

<template>
  <div class="investment-home">
    <n-space justify="space-between" align="center" style="margin-bottom: 12px">
      <n-space align="center" :wrap="true">
        <n-text strong style="font-size: 18px">首页</n-text>
        <n-tag size="small" type="info" :bordered="false">模拟账户 · 只读</n-tag>
        <n-text v-if="view?.tradeDate" depth="3">{{ view.tradeDate }}</n-text>
      </n-space>
      <n-button :loading="loading" @click="refresh">刷新</n-button>
    </n-space>

    <n-alert type="info" :bordered="false" style="margin-bottom: 14px">
      模拟账户，不是券商资金。非投资建议，不生成买卖指令。
    </n-alert>

    <n-space :wrap="true" style="margin-bottom: 16px">
      <n-button @click="goDiscover">发现机会</n-button>
      <n-button @click="goPlan">查看计划</n-button>
      <n-button type="primary" @click="goPortfolio">查看组合</n-button>
    </n-space>

    <n-spin :show="loading">
      <template v-if="errorMessage && !view">
        <n-empty :description="errorMessage" />
      </template>

      <template v-else-if="view">
        <section class="block">
          <n-text strong class="block-title">我的资产</n-text>
          <n-text depth="3" class="block-sub">模拟账户摘要。查看明细请到我的组合。</n-text>
          <n-space :wrap="true" :size="28" style="margin-top: 10px">
            <n-statistic label="总资产">
              <template #default>{{ formatMoney(portfolio?.equity) }}</template>
            </n-statistic>
            <n-statistic label="现金">
              <template #default>{{ formatMoney(portfolio?.cash) }}</template>
            </n-statistic>
            <n-statistic label="股票市值">
              <template #default>{{ formatMoney(portfolio?.marketValue) }}</template>
            </n-statistic>
            <n-statistic label="持仓数量">
              <template #default>{{ portfolio?.positionCount ?? '—' }}</template>
            </n-statistic>
          </n-space>
          <n-button text type="primary" style="margin-top: 8px" @click="goPortfolio">
            查看组合
          </n-button>
        </section>

        <section class="block">
          <n-text strong class="block-title">今日机会</n-text>
          <n-list v-if="opportunityCards.length" bordered>
            <n-list-item
              v-for="(card, idx) in opportunityCards"
              :key="(card.candidate_stock?.code || '') + '-' + (card.holding_stock?.code || '') + '-' + idx"
            >
              <n-space vertical :size="6" style="width: 100%">
                <n-space align="center" :wrap="true">
                  <n-tag
                    size="tiny"
                    :type="opportunityTagType(card.user_label)"
                    :bordered="false"
                  >
                    {{ card.user_label }}
                  </n-tag>
                  <n-text strong>机会对比</n-text>
                  <stock-link
                    v-if="card.candidate_stock?.display"
                    :model="card.candidate_stock.display"
                    @open="openStockKline"
                  />
                </n-space>
                <n-text depth="3">候选评分：{{ formatScore(card.candidate_score) }}</n-text>
                <n-text depth="3">超过当前持仓：</n-text>
                <stock-link
                  v-if="card.holding_stock?.display"
                  :model="card.holding_stock.display"
                  @open="openStockKline"
                />
                <n-text depth="3">持仓评分：{{ formatScore(card.holding_score) }}</n-text>
                <n-text>机会优势：{{ formatScoreGap(card.score_gap) }}</n-text>
              </n-space>
            </n-list-item>
          </n-list>
          <n-empty v-else description="今天没有单独机会提示" size="small" />
          <n-button text type="primary" style="margin-top: 8px" @click="goDiscover">
            发现机会
          </n-button>
        </section>

        <section class="block">
          <n-text strong class="block-title">我的计划</n-text>
          <template v-if="upcomingPlan && planCard.key !== 'none'">
            <n-space align="center" :wrap="true" style="margin: 8px 0 6px">
              <n-tag size="medium" :type="planTagType(planCard.key)" :bordered="false">
                {{ planCard.label }}
              </n-tag>
              <n-text v-if="planItemCount > 0" depth="3">共 {{ planItemCount }} 只</n-text>
              <n-text v-if="!isTodayPlan && upcomingPlan.trade_date" depth="3">
                下一交易日 {{ upcomingPlan.trade_date }}
              </n-text>
            </n-space>
            <n-space v-if="planStocks.length" :wrap="true" :size="[12, 8]" style="margin-top: 4px">
              <stock-link
                v-for="row in planStocks"
                :key="row.stock_code"
                :model="row"
                @open="openStockKline"
              />
            </n-space>
          </template>
          <n-empty v-else description="暂无交易计划" size="small" />
          <n-button text type="primary" style="margin-top: 8px" @click="goPlan">
            查看计划
          </n-button>
        </section>

        <section class="block">
          <n-text strong class="block-title">当前状态</n-text>
          <n-text style="display: block; margin-top: 8px; line-height: 1.6">
            {{ currentStatusText }}
          </n-text>
        </section>

        <!-- Legacy ops blocks: hidden, not deleted. Data still loaded via getInvestmentHome. -->
        <template v-if="SHOW_LEGACY_HOME_BLOCKS">
          <section class="block">
            <n-text strong class="block-title">账户总览</n-text>
            <n-text v-if="portfolio?.narrative" depth="3" class="block-sub">
              {{ sanitizeInternalCopy(portfolio.narrative) }}
            </n-text>
            <n-space :wrap="true" :size="28" style="margin-top: 10px">
              <n-statistic label="总资产">
                <template #default>{{ formatMoney(portfolio?.equity) }}</template>
              </n-statistic>
              <n-statistic label="今日收益">
                <template #default>
                  <span :style="{ color: pnlColor(portfolio?.dailyPnl) }">
                    {{
                      portfolio?.dailyPnl === null || portfolio?.dailyPnl === undefined
                        ? '—'
                        : formatMoney(portfolio.dailyPnl)
                    }}
                  </span>
                </template>
              </n-statistic>
              <n-statistic label="持仓数量">
                <template #default>{{ portfolio?.positionCount ?? '—' }}</template>
              </n-statistic>
              <n-statistic label="股票市值">
                <template #default>{{ formatMoney(portfolio?.marketValue) }}</template>
              </n-statistic>
              <n-statistic label="现金">
                <template #default>{{ formatMoney(portfolio?.cash) }}</template>
              </n-statistic>
            </n-space>
          </section>

          <section class="block">
            <n-text strong class="block-title">今日关注建议</n-text>
            <template v-if="decision">
              <n-space align="center" :wrap="true" style="margin: 8px 0 6px">
                <n-tag
                  size="medium"
                  :type="attentionTagType(decision.overallAttention)"
                  :bordered="false"
                >
                  {{ attentionLabel(decision.overallAttention) }}
                </n-tag>
                <n-text depth="3">组合状态：{{ attentionLabel(decision.healthStatus) }}</n-text>
              </n-space>
              <n-text v-if="decision.explanation" style="display: block; line-height: 1.6">
                {{ sanitizeInternalCopy(decision.explanation) }}
              </n-text>
              <n-space v-if="decision.healthNotes?.length" :wrap="true" style="margin-top: 8px">
                <n-tag
                  v-for="(note, i) in decision.healthNotes"
                  :key="i"
                  size="small"
                  :bordered="false"
                >
                  {{ sanitizeInternalCopy(note) }}
                </n-tag>
              </n-space>
            </template>
            <n-empty v-else description="关注建议暂不可用" size="small" />
          </section>

          <section class="block">
            <n-text strong class="block-title">今日摘要</n-text>
            <template v-if="daily">
              <div class="summary-row">
                <n-text depth="3" class="row-label">今日交易</n-text>
                <n-text>{{ sanitizeInternalCopy(daily.tradingNarrative) || '—' }}</n-text>
              </div>
              <div class="summary-row">
                <n-text depth="3" class="row-label">风险摘要</n-text>
                <n-text>{{ sanitizeInternalCopy(daily.riskNarrative) || '—' }}</n-text>
              </div>
              <div class="summary-row">
                <n-text depth="3" class="row-label">明日关注</n-text>
                <n-space v-if="daily.tomorrowFocus?.length" :wrap="true">
                  <n-tag
                    v-for="(f, i) in daily.tomorrowFocus"
                    :key="i"
                    size="small"
                    type="info"
                    :bordered="false"
                  >
                    {{ sanitizeInternalCopy(f) }}
                  </n-tag>
                </n-space>
                <n-text v-else>—</n-text>
              </div>
            </template>
            <n-empty v-else description="今日摘要暂不可用" size="small" />
          </section>

          <section class="block">
            <n-text strong class="block-title">今日流程</n-text>
            <n-text v-if="trading?.narrative" depth="3" class="block-sub">
              {{ sanitizeInternalCopy(trading.narrative) }}
            </n-text>
            <div class="pipeline">
              <div v-for="step in pipelineSteps" :key="step.key" class="pipeline-item">
                <n-text class="pipeline-label">{{ tradingStepLabel(step.key) }}</n-text>
                <n-tag size="small" :type="statusTagType(step.status)" :bordered="false">
                  {{ tradingStatusLabel(step.status) }}
                </n-tag>
              </div>
            </div>
            <n-text
              v-if="trading?.executionStatus === 'FAIL' && trading?.executionReason"
              type="error"
              style="display: block; margin-top: 8px"
            >
              说明：{{ sanitizeInternalCopy(trading.executionReason) }}
            </n-text>
          </section>

          <section class="block">
            <n-space align="center" :wrap="true" style="margin-bottom: 6px">
              <n-text strong class="block-title" style="margin-bottom: 0">今日关注</n-text>
              <n-tag
                v-if="dailyAttention"
                size="small"
                :type="attentionTagType(dailyAttention.overallAction)"
                :bordered="false"
              >
                {{ attentionLabel(dailyAttention.overallAction) }}
              </n-tag>
            </n-space>
            <n-text v-if="dailyAttention?.headline" depth="3" class="block-sub">
              {{ sanitizeInternalCopy(dailyAttention.headline) }}
            </n-text>
            <n-list v-if="attentionRows.length" bordered>
              <n-list-item v-for="(item, idx) in attentionRows" :key="idx">
                <n-space vertical :size="4" style="width: 100%">
                  <n-space align="center" :wrap="true">
                    <n-tag
                      size="tiny"
                      :type="attentionTagType(item.suggestedAction)"
                      :bordered="false"
                    >
                      {{ attentionLabel(item.suggestedAction) }}
                    </n-tag>
                    <n-text strong>{{ sanitizeInternalCopy(item.title) }}</n-text>
                    <n-text v-if="item.stockCode" depth="3">{{ item.stockCode }}</n-text>
                  </n-space>
                  <n-text v-if="item.reason" depth="3">{{ sanitizeInternalCopy(item.reason) }}</n-text>
                </n-space>
              </n-list-item>
            </n-list>
            <n-empty v-else description="暂无今日关注事项" size="small" />
          </section>

          <section class="block">
            <n-text strong class="block-title">持仓状态</n-text>
            <n-text depth="3" class="block-sub">
              持仓数量来自组合快照；可卖与锁定分列。
            </n-text>
            <div v-if="positionStates.length" class="ps-table-wrap">
              <table class="ps-table">
                <thead>
                  <tr>
                    <th>代码</th>
                    <th>状态</th>
                    <th>新建仓</th>
                    <th>持仓数量</th>
                    <th>可卖数量</th>
                    <th>T+1锁定</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="row in positionStates" :key="row.symbol">
                    <td class="code">{{ row.symbol }}</td>
                    <td>
                      <n-tag
                        size="tiny"
                        :type="positionStateTagType(row.positionStatus)"
                        :bordered="false"
                      >
                        {{ row.positionStatusLabel }}
                      </n-tag>
                    </td>
                    <td>{{ row.isNewPosition ? '是' : '否' }}</td>
                    <td>{{ row.totalQty }}</td>
                    <td>{{ row.availableQty }}</td>
                    <td>{{ row.lockedQty }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <n-empty v-else description="暂无持仓状态" size="small" />
          </section>
        </template>
      </template>
    </n-spin>

    <stock-kline-modal
      v-model:show="klineModal.visible"
      :title="klineModal.title"
      :chart-key="'home-plan-kline-' + klineModal.chartCode"
      :code="klineModal.chartCode"
      :stock-name="klineModal.stockName"
    />
  </div>
</template>

<style scoped>
.investment-home {
  padding: 8px 4px 24px;
  max-width: 960px;
}
.block {
  margin-bottom: 22px;
  padding: 14px 16px;
  border: 1px solid rgba(128, 128, 128, 0.18);
  border-radius: 10px;
  background: rgba(128, 128, 128, 0.04);
}
.block-title {
  display: block;
  font-size: 15px;
  margin-bottom: 4px;
}
.block-sub {
  display: block;
  margin-bottom: 6px;
  line-height: 1.5;
}
.summary-row {
  display: flex;
  gap: 12px;
  margin-top: 10px;
  align-items: flex-start;
}
.row-label {
  flex: 0 0 72px;
  padding-top: 2px;
}
.pipeline {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 18px;
  margin-top: 10px;
}
.pipeline-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 96px;
}
.pipeline-label {
  font-size: 13px;
  opacity: 0.85;
}
.ps-table-wrap {
  overflow-x: auto;
  margin-top: 8px;
}
.ps-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.ps-table th,
.ps-table td {
  padding: 8px 10px;
  text-align: right;
  border-bottom: 1px solid rgba(128, 128, 128, 0.14);
  white-space: nowrap;
}
.ps-table th:nth-child(-n + 2),
.ps-table td:nth-child(-n + 2) {
  text-align: left;
}
.ps-table .code {
  font-family: Consolas, monospace;
}
</style>
