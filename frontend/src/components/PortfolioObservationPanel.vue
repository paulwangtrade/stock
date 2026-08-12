<script setup>
import { computed, h, onMounted, ref } from 'vue'
import { NDataTable, NEmpty, NSpace, NStatistic, NTag, NText } from 'naive-ui'
import { getPaperPortfolioObservation } from '../api/portfolioObservation'

const portfolioObs = ref(null)
const unavailable = ref(false)

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

async function load() {
  try {
    portfolioObs.value = await getPaperPortfolioObservation({ target: 'identity' })
    unavailable.value = false
  } catch {
    portfolioObs.value = null
    unavailable.value = true
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
    </n-space>
    <n-text strong type="warning" style="display: block; margin-bottom: 10px">
      观察结果不是交易建议。不生成买卖单，不执行调仓。
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

      <n-text strong style="display: block; margin: 8px 0 6px">2. 组合风险 / 决策分布</n-text>
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

      <n-text strong style="display: block; margin: 8px 0 6px">3. 持仓决策列表</n-text>
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

      <n-text strong style="display: block; margin: 8px 0 6px">4. 模拟调仓观察</n-text>
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
