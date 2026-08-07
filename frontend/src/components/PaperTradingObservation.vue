<script setup>
import { computed, h, onMounted, ref } from 'vue'
import {
  NButton,
  NDataTable,
  NEmpty,
  NInput,
  NSpace,
  NSpin,
  NStatistic,
  NTabPane,
  NTabs,
  NTag,
  NText,
  NTooltip,
  useMessage,
} from 'naive-ui'
import {
  getPaperDailyReports,
  getPaperDashboardPositions,
  getPaperDashboardRuns,
  getPaperDashboardToday,
  getPaperObservationMetrics,
} from '../api/paperObservation'

const message = useMessage()
const loading = ref(false)
const errorMessage = ref('')
const tradeDate = ref('')
const activeTab = ref('overview')
const today = ref(null)
const positions = ref(null)
const runs = ref(null)
const dailyReports = ref(null)
const metrics = ref(null)
const metricsUnavailable = ref(false)

const dataSourceNote = computed(
  () =>
    today.value?.dataSourceNote ||
    positions.value?.dataSourceNote ||
    runs.value?.dataSourceNote ||
    'Phase6.6 Paper Trading MVP · paper_sim_* · 仅用于策略观察和模拟分析，不代表真实交易账户',
)

const hasFullyLockedT1 = computed(() =>
  (positions.value?.positions || []).some((row) => {
    const available = Number(row?.availableVolume || 0)
    const locked = Number(row?.lockedVolume || 0)
    return !!row?.t1Locked && available <= 0 && locked > 0
  }),
)

function formatMoney(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function formatPct(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return `${(n * 100).toFixed(2)}%`
}

function formatTime(v) {
  if (!v) return '—'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return String(v)
  return d.toLocaleString()
}

function runStatusType(s) {
  const v = String(s || '')
  if (v === 'completed') return 'success'
  if (v === 'completed_with_rejects') return 'warning'
  if (v === 'failed') return 'error'
  if (v.startsWith('skipped_')) return 'default'
  return 'info'
}

function quoteSourceLabel(source) {
  const v = String(source || '').trim()
  if (v === 'live') return '实时行情'
  if (v === 'open_fallback') return '开盘价/备用价格'
  return '历史成交价'
}

const positionColumns = [
  { title: '代码', key: 'stockCode', width: 110 },
  {
    title: '名称',
    key: 'stockName',
    width: 120,
    ellipsis: { tooltip: true },
    render(row) {
      const name = String(row?.stockName || '').trim()
      const code = String(row?.stockCode || '').trim()
      return name || code || '—'
    },
  },
  { title: '持仓数量', key: 'totalVolume', width: 90 },
  { title: '可卖数量', key: 'availableVolume', width: 90 },
  {
    title: 'T+1冻结',
    key: 'lockedVolume',
    width: 160,
    render(row) {
      if (!row.t1Locked) return h(NText, { depth: 3 }, { default: () => String(row.lockedVolume || 0) })
      const available = Number(row?.availableVolume || 0)
      const locked = Number(row?.lockedVolume || 0)
      const isFullyLocked = available <= 0 && locked > 0
      const tagText = isFullyLocked ? 'T+1锁定' : '部分T+1锁定'
      const tip = isFullyLocked
        ? 'T+1锁定，下一交易日可卖'
        : '存在T+1冻结数量，下一交易日逐步可卖'
      return h(
        NSpace,
        { size: 4, align: 'center' },
        {
          default: () => [
            h(NText, null, { default: () => String(row.lockedVolume) }),
            h(
              NTooltip,
              null,
              {
                trigger: () =>
                  h(NTag, { size: 'tiny', type: 'warning', bordered: false }, { default: () => tagText }),
                default: () => tip,
              },
            ),
          ],
        },
      )
    },
  },
  {
    title: '成本价',
    key: 'avgCost',
    width: 90,
    render(row) {
      return formatMoney(row.avgCost)
    },
  },
  {
    title: '当前价',
    key: 'markPrice',
    width: 180,
    render(row) {
      const sourceLabel = quoteSourceLabel(row?.quoteSource)
      const displayPrice =
        row?.displayPrice != null && Number.isFinite(Number(row.displayPrice))
          ? Number(row.displayPrice)
          : Number(row?.markPrice)
      const persistedPrice =
        row?.persistedMarkPrice != null && Number.isFinite(Number(row.persistedMarkPrice))
          ? Number(row.persistedMarkPrice)
          : null
      const updatedAt = row?.quoteUpdatedAt ? formatTime(row.quoteUpdatedAt) : '—'
      return h(
        NSpace,
        { size: 4, align: 'center' },
        {
          default: () => [
            h(NText, null, { default: () => formatMoney(row.markPrice) }),
            h(
              NTooltip,
              null,
              {
                trigger: () =>
                  h(NTag, { size: 'tiny', bordered: false, type: row?.quoteSource === 'live' ? 'success' : 'default' }, {
                    default: () => sourceLabel,
                  }),
                default: () =>
                  `展示价格：${formatMoney(displayPrice)}\n历史成交价：${persistedPrice == null ? '—' : formatMoney(persistedPrice)}\n来源：${sourceLabel}\n更新时间：${updatedAt}`,
              },
            ),
          ],
        },
      )
    },
  },
  {
    title: '市值',
    key: 'marketValue',
    width: 110,
    render(row) {
      return formatMoney(row.marketValue)
    },
  },
  {
    title: '浮盈亏',
    key: 'unrealizedPnl',
    width: 110,
    render(row) {
      return formatMoney(row.unrealizedPnl)
    },
  },
  {
    title: '收益率',
    key: 'returnRate',
    width: 90,
    render(row) {
      return formatPct(row.returnRate)
    },
  },
]

const runColumns = [
  {
    title: '时间',
    key: 'startedAt',
    width: 170,
    render(row) {
      return formatTime(row.startedAt)
    },
  },
  { title: '交易日', key: 'tradeDate', width: 110 },
  { title: 'trigger', key: 'trigger', width: 80 },
  { title: 'actor', key: 'actor', width: 90 },
  {
    title: 'result',
    key: 'status',
    width: 180,
    render(row) {
      return h(
        NTag,
        { size: 'small', type: runStatusType(row.status), bordered: false },
        { default: () => row.status || '—' },
      )
    },
  },
  {
    title: '订单/成交/拒绝',
    key: 'counts',
    width: 130,
    render(row) {
      return `${row.ordersTotal ?? 0} / ${row.filledCount ?? 0} / ${row.rejectCount ?? 0}`
    },
  },
  {
    title: 'message',
    key: 'message',
    ellipsis: { tooltip: true },
  },
]

const dailyReportColumns = [
  { title: '日期', key: 'reportDate', width: 110 },
  {
    title: '权益',
    key: 'equity',
    width: 110,
    render(row) {
      return formatMoney(row.equity)
    },
  },
  {
    title: '现金',
    key: 'cash',
    width: 110,
    render(row) {
      return formatMoney(row.cash)
    },
  },
  {
    title: '持仓市值',
    key: 'marketValue',
    width: 110,
    render(row) {
      return formatMoney(row.marketValue)
    },
  },
  {
    title: '浮盈亏',
    key: 'floatingPnl',
    width: 110,
    render(row) {
      return formatMoney(row.floatingPnl)
    },
  },
  { title: '成交数量', key: 'filledCount', width: 90 },
  { title: '拒绝数量', key: 'rejectedCount', width: 90 },
  {
    title: '风险暴露',
    key: 'exposure',
    width: 200,
    render(row) {
      const single = formatPct(row.maxSinglePositionPct)
      const gross = formatPct(row.maxGrossExposurePct)
      const code = row.maxSingleStockCode ? `（${row.maxSingleStockCode}）` : ''
      return `单票 ${single}${code}｜总敞口 ${gross}`
    },
  },
]

function formatCompliance(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return `${(n * 100).toFixed(1)}%`
}

async function refresh() {
  loading.value = true
  errorMessage.value = ''
  const date = tradeDate.value.trim() || undefined

  // Metrics must not block / clear the existing observation panels on failure.
  const metricsPromise = getPaperObservationMetrics(date)
    .then((m) => {
      metrics.value = m
      metricsUnavailable.value = false
    })
    .catch(() => {
      metrics.value = null
      metricsUnavailable.value = true
    })

  try {
    const [t, p, r, d] = await Promise.all([
      getPaperDashboardToday(date),
      getPaperDashboardPositions(),
      getPaperDashboardRuns({ tradeDate: date, limit: 50 }),
      getPaperDailyReports({ limit: 60 }),
    ])
    today.value = t
    positions.value = p
    runs.value = r
    dailyReports.value = d
    if (!tradeDate.value && t.tradeDate) {
      tradeDate.value = t.tradeDate
    }
  } catch (e) {
    today.value = null
    positions.value = null
    runs.value = null
    dailyReports.value = null
    errorMessage.value = e?.message || String(e)
    message.error(errorMessage.value)
  } finally {
    await metricsPromise
    loading.value = false
  }
}

onMounted(refresh)
</script>

<template>
  <div class="paper-observation">
    <n-space justify="space-between" align="center" style="margin-bottom: 12px">
      <n-space align="center">
        <n-text strong>模拟盘观察</n-text>
        <n-tag size="small" type="info" :bordered="false">只读</n-tag>
      </n-space>
      <n-space>
        <n-input
          v-model:value="tradeDate"
          placeholder="交易日 YYYY-MM-DD"
          style="width: 160px"
          clearable
          @keyup.enter="refresh"
        />
        <n-button :loading="loading" @click="refresh">刷新</n-button>
      </n-space>
    </n-space>

    <div class="source-banner">
      <n-text strong>数据来源：</n-text>
      <n-text>Phase6.6 Paper Trading MVP</n-text>
      <n-text depth="3"> · 数据表：</n-text>
      <n-text code>paper_sim_*</n-text>
      <div class="source-desc">
        仅用于策略观察和模拟分析。不代表真实交易账户。与「量化交易 / 研究中心模拟盘」的生产
        <n-text code>paper_*</n-text>
        账本相互独立。
      </div>
      <n-text depth="3" style="font-size: 12px">{{ dataSourceNote }}</n-text>
    </div>

    <n-spin :show="loading">
      <n-tabs v-model:value="activeTab" type="line" style="margin-top: 8px">
        <n-tab-pane name="overview" tab="今日观察">
          <template v-if="today">
            <n-text strong style="display: block; margin: 8px 0">今日模拟交易</n-text>
            <n-space align="center" :wrap="true" style="margin-bottom: 10px">
              <n-text>交易日期：{{ today.tradeDate || '—' }}</n-text>
              <n-tag size="small" :type="today.enabled ? 'success' : 'default'" :bordered="false">
                {{ today.enabled ? 'enabled' : 'disabled' }}
              </n-tag>
              <n-tag size="small" :type="runStatusType(today.runStatus)" :bordered="false">
                {{ today.runStatus || '—' }}
              </n-tag>
              <n-text v-if="today.message" depth="3">{{ today.message }}</n-text>
            </n-space>

            <n-space :wrap="true" :size="24" style="margin-bottom: 18px">
              <n-statistic label="计划数量" :value="today.planCount" />
              <n-statistic label="订单数量" :value="today.ordersTotal" />
              <n-statistic label="成交数量" :value="today.filledCount" />
              <n-statistic label="拒绝数量" :value="today.rejectCount" />
              <n-statistic label="成交金额" :value="formatMoney(today.filledAmount)" />
              <n-statistic label="现金余额" :value="formatMoney(today.cash)" />
              <n-statistic label="浮盈亏" :value="formatMoney(today.unrealizedPnl)" />
              <n-statistic label="权益" :value="formatMoney(today.equity)" />
            </n-space>
          </template>

          <div class="exec-observation">
            <n-space align="center" style="margin: 4px 0 10px">
              <n-text strong>Execution Observation</n-text>
              <n-tag size="small" type="info" :bordered="false">合规观察</n-tag>
              <n-tag size="small" :bordered="false">只读 · metrics API</n-tag>
            </n-space>

            <template v-if="metrics">
              <n-text strong style="display: block; margin-bottom: 6px">执行窗口运行次数</n-text>
              <n-text depth="3" style="display: block; margin-bottom: 8px; font-size: 12px">
                按 run.started_at 派生 Session；此处为运行次数，不是成交数量。
              </n-text>
              <n-space :wrap="true" :size="24" style="margin-bottom: 16px">
                <n-statistic label="A窗口运行次数" :value="metrics.sessionDistribution.sessionA" />
                <n-statistic label="B窗口运行次数" :value="metrics.sessionDistribution.sessionB" />
                <n-statistic label="C窗口运行次数" :value="metrics.sessionDistribution.sessionC" />
                <n-statistic label="关闭窗口运行次数" :value="metrics.sessionDistribution.sessionClosed" />
                <n-statistic label="总运行次数" :value="metrics.totalRuns" />
              </n-space>

              <n-text strong style="display: block; margin-bottom: 6px">B窗口成交价格策略</n-text>
              <n-space :wrap="true" :size="24" style="margin-bottom: 16px">
                <n-statistic label="符合：B窗口 Close 成交" :value="metrics.fillPolicy.bWindowCloseFills" />
                <n-statistic label="异常：B窗口 Open 成交" :value="metrics.fillPolicy.bWindowOpenFills" />
              </n-space>

              <n-text strong style="display: block; margin-bottom: 6px">历史基线（已排除）</n-text>
              <n-text depth="3" style="display: block; margin-bottom: 8px; font-size: 12px">
                C.2-C修复前历史成交，不参与当前合规统计
              </n-text>
              <n-space :wrap="true" :size="24" style="margin-bottom: 16px">
                <n-statistic label="历史基线笔数" :value="metrics.legacy.legacyBaselineCount" />
                <n-statistic label="已排除笔数" :value="metrics.legacy.excludedFillCount" />
              </n-space>

              <n-text strong style="display: block; margin-bottom: 6px">Quality</n-text>
              <n-space :wrap="true" :size="24" style="margin-bottom: 16px">
                <n-statistic label="OK" :value="metrics.quality.okCount" />
                <n-statistic label="ANOMALY" :value="metrics.quality.anomalyCount" />
                <n-statistic label="LEGACY_BASELINE" :value="metrics.quality.legacyCount" />
              </n-space>

              <n-text strong style="display: block; margin-bottom: 6px">Compliance</n-text>
              <n-space :wrap="true" :size="24" style="margin-bottom: 8px">
                <n-statistic
                  label="执行价格策略合规率"
                  :value="formatCompliance(metrics.pricePolicyCompliance)"
                />
              </n-space>
            </template>

            <n-empty
              v-else-if="metricsUnavailable"
              description="执行观察数据暂不可用"
              style="margin: 8px 0 12px"
            />
            <n-text v-else depth="3" style="display: block; margin-bottom: 8px">
              执行观察加载中…
            </n-text>
          </div>

          <template v-if="positions">
            <n-space align="center" style="margin: 8px 0">
              <n-text strong>当前模拟持仓</n-text>
              <n-tag
                size="small"
                :type="positions.quoteOverlay ? 'success' : 'default'"
                :bordered="false"
              >
                {{ positions.quoteOverlay ? '实时行情覆盖中' : '使用历史成交价' }}
              </n-tag>
            </n-space>
            <n-text depth="3" style="display: block; margin-bottom: 8px">
              A股 T+1：当日买入计入冻结数量，次一交易日才可卖。
            </n-text>
            <n-tag
              v-if="hasFullyLockedT1"
              type="warning"
              size="small"
              :bordered="false"
              style="margin-bottom: 8px"
            >
              T+1锁定，下一交易日可卖
            </n-tag>
            <n-data-table
              v-if="(positions.positions || []).length"
              size="small"
              :columns="positionColumns"
              :data="positions.positions"
              :bordered="false"
              :single-line="false"
            />
            <n-empty v-else description="暂无模拟持仓" style="margin: 12px 0" />
          </template>

          <template v-if="runs">
            <n-text strong style="display: block; margin: 18px 0 8px">运行台账</n-text>
            <n-data-table
              v-if="(runs.runs || []).length"
              size="small"
              :columns="runColumns"
              :data="runs.runs"
              :bordered="false"
              :single-line="false"
            />
            <n-empty v-else description="暂无运行记录" style="margin: 12px 0" />
          </template>
        </n-tab-pane>

        <n-tab-pane name="daily" tab="历史日报">
          <n-text depth="3" style="display: block; margin-bottom: 10px">
            日终不可变快照（paper_sim_daily_reports）。15:05 Settlement 后生成；非实时持仓。
          </n-text>
          <n-data-table
            v-if="dailyReports && (dailyReports.reports || []).length"
            size="small"
            :columns="dailyReportColumns"
            :data="dailyReports.reports"
            :bordered="false"
            :single-line="false"
          />
          <n-empty v-else description="暂无历史日报（需 enablePaperTrading 且当日 Settlement 成功后生成）" />
        </n-tab-pane>
      </n-tabs>

      <n-empty
        v-if="errorMessage && !today && !positions && !runs && !dailyReports"
        :description="errorMessage"
        style="margin-top: 24px"
      />
    </n-spin>
  </div>
</template>

<style scoped>
.paper-observation {
  padding: 12px 16px;
}
.source-banner {
  padding: 10px 12px;
  margin-bottom: 8px;
  border-left: 3px solid #2080f0;
  background: rgba(32, 128, 240, 0.06);
}
.source-desc {
  margin: 6px 0 4px;
  font-size: 13px;
  line-height: 1.5;
}
.exec-observation {
  margin: 8px 0 18px;
  padding: 12px 14px;
  border: 1px solid rgba(32, 128, 240, 0.25);
  border-radius: 6px;
  background: rgba(32, 128, 240, 0.04);
}
</style>
