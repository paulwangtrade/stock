<script setup>
import { computed, h, onMounted, ref } from 'vue'
import { NDataTable, NEmpty, NSpace, NStatistic, NTag, NText } from 'naive-ui'
import { getPaperPortfolioObservation } from '../api/portfolioObservation'
import { getObservationPerformance } from '../api/observationPerformance'

const portfolioObs = ref(null)
const unavailable = ref(false)
const perf = ref(null)
const perfUnavailable = ref(false)

function formatMoney(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return n.toLocaleString('zh-CN', { maximumFractionDigits: 2 })
}

function formatPct(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return `${(n * 100).toFixed(1)}%`
}

function formatName(name, code) {
  const n = String(name || '').trim()
  const c = String(code || '').trim()
  if (n && n !== '未知名称' && n !== c) return n
  if (c) return `未知名称(${c})`
  return '未知名称'
}

const decisionColumns = [
  { title: '代码', key: 'symbol', width: 100 },
  {
    title: '名称',
    key: 'stockName',
    width: 110,
    ellipsis: { tooltip: true },
    render(row) {
      return formatName(row?.stockName, row?.symbol)
    },
  },
  {
    title: '当前权重',
    key: 'currentWeight',
    width: 90,
    render(row) {
      return formatPct(row?.currentWeight)
    },
  },
  {
    title: '盈亏',
    key: 'pnl',
    width: 110,
    render(row) {
      const n = Number(row?.pnl)
      return Number.isFinite(n) ? formatMoney(n) : '—'
    },
  },
  {
    title: '收益率',
    key: 'returnRate',
    width: 90,
    render(row) {
      return formatPct(row?.returnRate)
    },
  },
  {
    title: '持仓天数',
    key: 'holdingDays',
    width: 80,
  },
  {
    title: 'Aging',
    key: 'holdingPeriodBucket',
    width: 90,
    render(row) {
      const s = String(row?.holdingPeriodBucket || '').trim() || '—'
      const type = s === 'LONG' ? 'warning' : s === 'UNKNOWN' ? 'default' : 'info'
      return h(NTag, { size: 'small', type, bordered: false }, { default: () => s })
    },
  },
  {
    title: 'Health',
    key: 'healthLevel',
    width: 100,
    render(row) {
      const s = String(row?.healthLevel || '').trim() || '—'
      const type = s === 'RISK' ? 'error' : s === 'WATCH' ? 'warning' : s === 'HEALTHY' ? 'success' : 'default'
      return h(NTag, { size: 'small', type, bordered: false }, { default: () => s })
    },
  },
  {
    title: 'Decision',
    key: 'decisionState',
    width: 130,
    render(row) {
      const s = String(row?.decisionState || '').trim() || '—'
      const type = s === 'HOLD_REVIEW' || s === 'EXIT_CANDIDATE' ? 'warning' : s === 'HOLD_WATCH' ? 'info' : 'default'
      return h(NTag, { size: 'small', type, bordered: false }, { default: () => s })
    },
  },
  {
    title: '原因',
    key: 'decisionReason',
    minWidth: 140,
    ellipsis: { tooltip: true },
  },
]

const agingColumns = [
  { title: '代码', key: 'symbol', width: 100 },
  {
    title: '名称',
    key: 'stockName',
    width: 110,
    ellipsis: { tooltip: true },
    render(row) {
      return formatName(row?.stockName, row?.symbol)
    },
  },
  { title: '买入日', key: 'firstBuyDate', width: 110 },
  { title: '持仓天数', key: 'holdingDays', width: 90 },
  {
    title: 'Aging',
    key: 'holdingPeriodBucket',
    width: 90,
    render(row) {
      return h(NTag, { size: 'small', type: 'warning', bordered: false }, { default: () => row?.holdingPeriodBucket || 'LONG' })
    },
  },
  {
    title: 'Health',
    key: 'healthLevel',
    width: 100,
    render(row) {
      const s = String(row?.healthLevel || '').trim() || '—'
      return h(NTag, { size: 'small', bordered: false }, { default: () => s })
    },
  },
  {
    title: 'Decision',
    key: 'decisionState',
    width: 130,
    render(row) {
      return String(row?.decisionState || '').trim() || '—'
    },
  },
]

const historyColumns = [
  { title: '代码', key: 'symbol', width: 100 },
  {
    title: '名称',
    key: 'stockName',
    width: 110,
    ellipsis: { tooltip: true },
    render(row) {
      return formatName(row?.stockName, row?.symbol)
    },
  },
  {
    title: '当前 Decision',
    key: 'decisionState',
    width: 130,
    render(row) {
      const s = String(row?.decisionState || '').trim() || '—'
      const type = s === 'HOLD_REVIEW' || s === 'EXIT_CANDIDATE' ? 'warning' : s === 'HOLD_WATCH' ? 'info' : 'default'
      return h(NTag, { size: 'small', type, bordered: false }, { default: () => s })
    },
  },
  {
    title: '变化观察',
    key: 'historyText',
    minWidth: 220,
    ellipsis: { tooltip: true },
    render(row) {
      const pts = Array.isArray(row?.decisionHistory) ? row.decisionHistory : []
      if (!pts.length) return row?.decisionHistoryNote || '仅当前观察点'
      return pts.map((p) => `${p.asOf || '?'} ${p.state || ''}`).join(' → ')
    },
  },
]

const diffColumns = [
  { title: '代码', key: 'symbol', width: 100 },
  {
    title: '名称',
    key: 'stockName',
    width: 110,
    ellipsis: { tooltip: true },
    render(row) {
      return formatName(row?.stockName, row?.symbol)
    },
  },
  {
    title: '当前',
    key: 'currentWeight',
    width: 90,
    render(row) {
      return formatPct(row?.currentWeight)
    },
  },
  {
    title: '目标',
    key: 'targetWeight',
    width: 90,
    render(row) {
      return formatPct(row?.targetWeight)
    },
  },
  {
    title: 'Diff',
    key: 'rebalanceAction',
    width: 110,
    render(row) {
      const s = String(row?.rebalanceAction || '').trim() || '—'
      const type = s === 'REMOVE' || s === 'DECREASE' ? 'warning' : s === 'ADD' || s === 'INCREASE' ? 'info' : 'default'
      return h(NTag, { size: 'small', type, bordered: false }, { default: () => s })
    },
  },
  {
    title: '原因',
    key: 'rebalanceReason',
    minWidth: 160,
    ellipsis: { tooltip: true },
  },
]

const diffRows = computed(() => {
  const rows = portfolioObs.value?.positions || []
  return rows.filter((r) => {
    const a = String(r?.rebalanceAction || '').toUpperCase()
    return a && a !== 'KEEP'
  })
})

const agingRows = computed(() => {
  const rows = portfolioObs.value?.positions || []
  return rows.filter((r) => !!r?.isAging)
})

function formatSignedPct(v) {
  if (v === null || v === undefined) return '—'
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  const sign = n > 0 ? '+' : ''
  return `${sign}${(n * 100).toFixed(1)}%`
}

async function load() {
  try {
    portfolioObs.value = await getPaperPortfolioObservation({ target: 'identity' })
    unavailable.value = false
  } catch {
    portfolioObs.value = null
    unavailable.value = true
  }
  try {
    perf.value = await getObservationPerformance({ horizon: 5, benchmark: 'csi300' })
    perfUnavailable.value = false
  } catch {
    perf.value = null
    perfUnavailable.value = true
  }
}

onMounted(load)
</script>

<template>
  <div class="portfolio-observation">
    <n-space align="center" style="margin: 4px 0 10px">
      <n-text strong>组合观察</n-text>
      <n-tag size="small" type="info" :bordered="false">Portfolio Observation · 只读</n-tag>
      <n-tag size="small" :bordered="false">非交易建议</n-tag>
      <n-tag size="small" type="warning" :bordered="false">不会自动卖出</n-tag>
    </n-space>
    <n-text strong type="warning" style="display: block; margin-bottom: 10px">
      观察结果不是交易建议。不会自动卖出。
    </n-text>
    <n-text v-if="unavailable" depth="3" type="warning" style="display: block; margin-bottom: 8px">
      组合观察 API 暂不可用（不影响买入模拟盘）。
    </n-text>
    <template v-else-if="portfolioObs">
      <n-text depth="3" style="display: block; margin-bottom: 10px; font-size: 12px">
        {{ portfolioObs.disclaimer }}
      </n-text>

      <n-text strong style="display: block; margin: 8px 0 6px">1. 账户状态</n-text>
      <n-space :wrap="true" :size="24" style="margin-bottom: 16px">
        <n-statistic label="权益" :value="formatMoney(portfolioObs.account.totalEquity)" />
        <n-statistic label="现金" :value="formatMoney(portfolioObs.account.cash)" />
        <n-statistic label="市值" :value="formatMoney(portfolioObs.account.marketValue)" />
        <n-statistic label="敞口" :value="formatMoney(portfolioObs.account.exposure)" />
        <n-statistic label="持仓数" :value="portfolioObs.account.positionCount" />
      </n-space>
      <n-text depth="3" style="display: block; margin-bottom: 12px; font-size: 12px">
        账户数字来自 Snapshot 落库 mark，与评价 overlay 现价可能不一致。
      </n-text>

      <n-text v-if="(portfolioObs.warnings || []).length" depth="3" type="warning" style="display: block; margin-bottom: 10px; font-size: 12px">
        {{ portfolioObs.warnings.join('；') }}
      </n-text>

      <n-text strong style="display: block; margin: 8px 0 6px">2. 组合健康度</n-text>
      <n-space :wrap="true" :size="24" style="margin-bottom: 8px">
        <n-statistic label="组合健康" :value="portfolioObs.health?.portfolioHealth || 'UNKNOWN'" />
        <n-statistic label="HEALTHY" :value="portfolioObs.health?.healthyPositions ?? 0" />
        <n-statistic label="WATCH(决策)" :value="portfolioObs.health?.watchPositions ?? 0" />
        <n-statistic label="REVIEW" :value="portfolioObs.health?.reviewPositions ?? 0" />
        <n-statistic label="AGING" :value="portfolioObs.health?.agingPositions ?? 0" />
        <n-statistic label="RISK" :value="portfolioObs.health?.riskPositions ?? 0" />
      </n-space>
      <n-text depth="3" style="display: block; margin-bottom: 12px; font-size: 12px">
        观察健康分，不是 Strategy Score，不是卖出指令。机会成本模型未启用。
      </n-text>

      <n-text strong style="display: block; margin: 8px 0 6px">3. 组合风险 / 决策分布</n-text>
      <n-space :wrap="true" :size="24" style="margin-bottom: 16px">
        <n-statistic label="HOLD_NORMAL" :value="portfolioObs.decision.normalCount" />
        <n-statistic label="NORMAL权重" :value="formatPct(portfolioObs.decision.normalWeight)" />
        <n-statistic label="HOLD_WATCH" :value="portfolioObs.decision.watchCount" />
        <n-statistic label="WATCH权重" :value="formatPct(portfolioObs.decision.watchWeight)" />
        <n-statistic label="HOLD_REVIEW" :value="portfolioObs.decision.reviewCount" />
        <n-statistic label="REVIEW权重" :value="formatPct(portfolioObs.decision.reviewWeight)" />
        <n-statistic label="EXIT_CANDIDATE" :value="portfolioObs.decision.exitCandidateCount" />
        <n-statistic label="EXIT权重" :value="formatPct(portfolioObs.decision.exitCandidateWeight)" />
      </n-space>
      <n-text depth="3" style="display: block; margin-bottom: 12px; font-size: 12px">
        决策分布为观察标签，不是卖出建议。EXIT_CANDIDATE 默认关闭。
      </n-text>

      <n-text strong style="display: block; margin: 8px 0 6px">4. 持仓决策列表</n-text>
      <n-data-table
        v-if="(portfolioObs.positions || []).length"
        size="small"
        :columns="decisionColumns"
        :data="portfolioObs.positions"
        :row-key="(row) => row.symbol"
        :bordered="false"
        :single-line="false"
        style="margin-bottom: 12px"
      />
      <n-empty v-else description="暂无持仓" style="margin: 8px 0 12px" />

      <n-text strong style="display: block; margin: 8px 0 6px">5. 长持仓观察</n-text>
      <n-text depth="3" style="display: block; margin-bottom: 8px; font-size: 12px">
        holding_days &gt; 30 → AGING。只显示关注，不会自动卖出。
      </n-text>
      <n-data-table
        v-if="agingRows.length"
        size="small"
        :columns="agingColumns"
        :data="agingRows"
        :row-key="(row) => row.symbol + '-aging'"
        :bordered="false"
        :single-line="false"
        style="margin-bottom: 12px"
      />
      <n-empty v-else description="无长持仓（AGING）" style="margin: 8px 0 12px" />

      <n-text strong style="display: block; margin: 8px 0 6px">6. Decision 变化</n-text>
      <n-text depth="3" style="display: block; margin-bottom: 8px; font-size: 12px">
        运行时仅当前观察点；无持久化决策快照。不是交易信号。
      </n-text>
      <n-data-table
        v-if="(portfolioObs.positions || []).length"
        size="small"
        :columns="historyColumns"
        :data="portfolioObs.positions"
        :row-key="(row) => row.symbol + '-hist'"
        :bordered="false"
        :single-line="false"
        style="margin-bottom: 12px"
      />
      <n-empty v-else description="暂无 Decision 观察" style="margin: 8px 0 12px" />

      <n-text strong style="display: block; margin: 8px 0 6px">7. 持仓判断效果（观察）</n-text>
      <n-text strong type="warning" style="display: block; margin-bottom: 8px; font-size: 13px">
        历史判断统计，不代表未来收益
      </n-text>
      <n-text v-if="perfUnavailable" depth="3" type="warning" style="display: block; margin-bottom: 8px; font-size: 12px">
        判断效果 API 暂不可用（不影响买入模拟盘）。
      </n-text>
      <template v-else-if="perf">
        <n-space :wrap="true" :size="24" style="margin-bottom: 10px">
          <n-statistic label="样本" :value="perf.samples" />
          <n-statistic label="已评价(T+5)" :value="perf.evaluated" />
          <n-statistic label="5日胜率" :value="formatSignedPct(perf.winRate).replace('+','')" />
          <n-statistic label="平均收益" :value="formatSignedPct(perf.avgReturn)" />
          <n-statistic label="平均超额" :value="formatSignedPct(perf.avgAlpha)" />
          <n-statistic label="判断准确率" :value="formatSignedPct(perf.decisionAccuracy).replace('+','')" />
        </n-space>
        <n-space :wrap="true" :size="16" style="margin-bottom: 8px">
          <n-tag size="small" :bordered="false">
            HOLD_NORMAL 样本 {{ perf.byState.HOLD_NORMAL?.samples ?? 0 }} · 5日胜率
            {{ formatSignedPct(perf.byState.HOLD_NORMAL?.winRate).replace('+','') }}
          </n-tag>
          <n-tag size="small" type="info" :bordered="false">
            HOLD_WATCH 样本 {{ perf.byState.HOLD_WATCH?.samples ?? 0 }} · 风险捕获
            {{ formatSignedPct(perf.byState.HOLD_WATCH?.riskCaptureRate).replace('+','') }}
          </n-tag>
          <n-tag size="small" type="warning" :bordered="false">
            HOLD_REVIEW 样本 {{ perf.byState.HOLD_REVIEW?.samples ?? 0 }} · 平均
            {{ formatSignedPct(perf.byState.HOLD_REVIEW?.avgReturn) }}
          </n-tag>
        </n-space>
        <n-text depth="3" style="display: block; margin-bottom: 12px; font-size: 12px">
          {{ perf.disclaimer }} 基准默认沪深300。行业指数未启用。不是买卖建议。
        </n-text>
      </template>

      <n-text strong style="display: block; margin: 8px 0 6px">8. 模拟调仓观察</n-text>
      <n-space :wrap="true" :size="16" style="margin-bottom: 8px">
        <n-tag size="small" :bordered="false">KEEP {{ portfolioObs.rebalance.keepCount }}</n-tag>
        <n-tag size="small" type="info" :bordered="false">ADD {{ portfolioObs.rebalance.addCount }}</n-tag>
        <n-tag size="small" type="info" :bordered="false">INCREASE {{ portfolioObs.rebalance.increaseCount }}</n-tag>
        <n-tag size="small" type="warning" :bordered="false">DECREASE {{ portfolioObs.rebalance.decreaseCount }}</n-tag>
        <n-tag size="small" type="warning" :bordered="false">REMOVE {{ portfolioObs.rebalance.removeCount }}</n-tag>
      </n-space>
      <n-text depth="3" style="display: block; margin-bottom: 8px; font-size: 12px">
        仅观察 Current vs Target 差异，不执行。例如：当前 10% / 目标 0% → REMOVE。
      </n-text>
      <n-data-table
        v-if="diffRows.length"
        size="small"
        :columns="diffColumns"
        :data="diffRows"
        :row-key="(row) => row.symbol + '-' + row.rebalanceAction"
        :bordered="false"
        :single-line="false"
      />
      <n-empty v-else description="无调仓差异（或 identity 目标下全 KEEP）" style="margin: 8px 0" />
    </template>
  </div>
</template>

<style scoped>
.portfolio-observation {
  margin: 8px 0 18px;
  padding: 12px 14px;
  border: 1px solid rgba(64, 128, 192, 0.28);
  border-radius: 6px;
  background: rgba(64, 128, 192, 0.04);
}
</style>
