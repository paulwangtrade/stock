<script setup>
import { computed, onMounted, ref } from 'vue'
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
  NThing,
} from 'naive-ui'
import { getPortfolioDecisionDashboard } from '../api/portfolioDecisionDashboard'

const loading = ref(false)
const unavailable = ref(false)
const errorMsg = ref('')
const view = ref(null)

function formatMoney(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return n.toLocaleString('zh-CN', { maximumFractionDigits: 2 })
}

function formatPct(v) {
  if (v === null || v === undefined) return '—'
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return `${(n * 100).toFixed(1)}%`
}

function formatInt(v) {
  if (v === null || v === undefined) return '—'
  const n = Number(v)
  if (!Number.isFinite(n)) return '—'
  return String(Math.round(n))
}

const topSector = computed(() => {
  const buckets = view.value?.currentPortfolio?.sectorBuckets || []
  if (!buckets.length) return null
  return [...buckets].sort((a, b) => Number(b.weight) - Number(a.weight))[0]
})

const sectorStatusLabel = computed(() => {
  const s = view.value?.sectorCoverage?.status
  return s === 'available' ? 'available' : 'unavailable'
})

const sectorStatusType = computed(() =>
  sectorStatusLabel.value === 'available' ? 'success' : 'warning',
)

async function load() {
  loading.value = true
  errorMsg.value = ''
  try {
    view.value = await getPortfolioDecisionDashboard()
    unavailable.value = false
  } catch (e) {
    view.value = null
    unavailable.value = true
    errorMsg.value = e?.message || '加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="portfolio-decision-dashboard">
    <n-space align="center" style="margin: 4px 0 10px" :wrap="true">
      <n-text strong>组合决策差异分析</n-text>
      <n-tag size="small" type="info" :bordered="false">Portfolio Decision Dashboard · 只读</n-tag>
      <n-tag size="small" type="warning" :bordered="false">非交易界面</n-tag>
      <n-tag size="small" :bordered="false">不自动买卖</n-tag>
      <n-tag size="small" :bordered="false">不生成 TradePlan</n-tag>
      <n-button size="tiny" quaternary :loading="loading" @click="load">刷新</n-button>
    </n-space>

    <n-alert type="warning" :bordered="false" style="margin-bottom: 12px">
      本页为<strong>分析工具</strong>：展示组合观察与决策过程说明。
      <strong>不构成投资建议</strong>，<strong>不会自动买卖</strong>，
      <strong>不会创建或执行 TradePlan</strong>，<strong>不会切换 Provider</strong>。
    </n-alert>

    <n-spin :show="loading">
      <n-text v-if="unavailable" depth="3" type="warning" style="display: block; margin-bottom: 8px">
        决策看板 API 暂不可用：{{ errorMsg || '请稍后重试' }}（不影响交易主流程）。
      </n-text>

      <template v-else-if="view">
        <n-text depth="3" style="display: block; margin-bottom: 10px; font-size: 12px">
          {{ view.disclaimer }}
        </n-text>
        <n-space :wrap="true" size="small" style="margin-bottom: 14px">
          <n-tag v-if="view.flags.notAutoTrade" size="small" type="success" :bordered="false">
            not_auto_trade
          </n-tag>
          <n-tag v-if="view.flags.notATradePlan" size="small" :bordered="false">not_a_trade_plan</n-tag>
          <n-tag v-if="view.flags.notExecution" size="small" :bordered="false">not_execution</n-tag>
          <n-tag v-if="view.flags.notProviderSwitch" size="small" :bordered="false">
            not_provider_switch
          </n-tag>
          <n-tag v-if="view.tradeDate" size="small" :bordered="false">交易日 {{ view.tradeDate }}</n-tag>
          <n-tag v-if="view.accountId" size="small" :bordered="false">账户 {{ view.accountId }}</n-tag>
        </n-space>

        <!-- 当前组合概览 -->
        <n-text strong style="display: block; margin: 8px 0 6px">当前组合概览</n-text>
        <n-text
          v-if="!view.currentPortfolio.available"
          depth="3"
          type="warning"
          style="display: block; margin-bottom: 8px; font-size: 12px"
        >
          组合账本暂不可用{{ view.currentPortfolio.note ? `（${view.currentPortfolio.note}）` : '' }}
        </n-text>
        <n-space :wrap="true" :size="24" style="margin-bottom: 16px">
          <n-statistic label="总资产" :value="formatMoney(view.currentPortfolio.equity)" />
          <n-statistic label="股票数量" :value="formatInt(view.currentPortfolio.nameCount)" />
          <n-statistic label="现金比例" :value="formatPct(view.currentPortfolio.cashRatio)" />
          <n-statistic label="最大持仓" :value="formatPct(view.currentPortfolio.top1Weight)" />
          <n-statistic
            label="行业集中"
            :value="
              topSector
                ? `${topSector.sector} ${formatPct(topSector.weight)}`
                : view.currentPortfolio.sectorAvailable
                  ? '—'
                  : '暂不可用'
            "
          />
          <n-statistic
            v-if="view.currentPortfolio.grossExposure != null"
            label="毛敞口"
            :value="formatPct(view.currentPortfolio.grossExposure)"
          />
        </n-space>

        <!-- Sector Coverage -->
        <n-text strong style="display: block; margin: 8px 0 6px">Sector Coverage 状态</n-text>
        <n-space align="center" :wrap="true" style="margin-bottom: 8px">
          <n-tag size="small" :type="sectorStatusType" :bordered="false">
            {{ sectorStatusLabel }}
          </n-tag>
          <n-tag
            size="small"
            :type="view.sectorCoverage.allowSectorConstraint ? 'success' : 'default'"
            :bordered="false"
          >
            allow_sector_constraint={{ view.sectorCoverage.allowSectorConstraint }}
          </n-tag>
          <n-text depth="3" style="font-size: 12px">
            持仓覆盖 {{ formatPct(view.sectorCoverage.holdingsCoverage) }} · 缺失
            {{ view.sectorCoverage.missingCount }} · unknown {{ view.sectorCoverage.unknownCount }}
          </n-text>
        </n-space>
        <n-text depth="3" style="display: block; margin-bottom: 14px; font-size: 12px">
          {{ view.sectorCoverage.note || '覆盖不足时不得开启行业约束（fail-closed）。' }}
          <template v-if="!view.currentPortfolio.sectorAvailable">
            行业暴露：unavailable
            <template v-if="view.currentPortfolio.sectorNote">
              （{{ view.currentPortfolio.sectorNote }}）
            </template>
          </template>
        </n-text>

        <!-- Portfolio Insight 卡片 -->
        <n-text strong style="display: block; margin: 8px 0 6px">Portfolio Insight</n-text>
        <div class="insight-card">
          <n-space align="center" :wrap="true" style="margin-bottom: 8px">
            <n-tag
              size="small"
              :type="view.insight.present ? 'info' : 'default'"
              :bordered="false"
            >
              {{ view.insight.present ? 'insight 已附' : 'insight 部分缺失' }}
            </n-tag>
            <n-tag v-if="view.insight.riskLevel" size="small" :bordered="false">
              风险等级 {{ view.insight.riskLevelLabel || view.insight.riskLevel }}
            </n-tag>
          </n-space>
          <n-space :wrap="true" :size="20" style="margin-bottom: 8px">
            <n-statistic label="持仓数（观察）" :value="formatInt(view.insight.nameCount)" />
            <n-statistic label="现金比例（观察）" :value="formatPct(view.insight.cashRatio)" />
          </n-space>
          <n-list v-if="view.insight.riskReasons.length" size="small">
            <n-list-item v-for="(r, i) in view.insight.riskReasons" :key="'rr-' + i">
              <n-thing :title="r" description="风险等级说明（观察）" />
            </n-list-item>
          </n-list>
          <n-text v-else depth="3" style="font-size: 12px">暂无风险等级说明条目。</n-text>
        </div>

        <!-- 风险解释 -->
        <n-text strong style="display: block; margin: 16px 0 6px">风险解释（观察）</n-text>
        <n-text depth="3" style="display: block; margin-bottom: 8px; font-size: 12px">
          下列为决策过程说明，不是下单指令，不会自动减仓或买入。
        </n-text>

        <n-text strong style="display: block; margin: 10px 0 4px; font-size: 13px">
          why_buy_less · 为什么买少
        </n-text>
        <n-list
          v-if="view.decisionExplain.whyBuyLess.length"
          size="small"
          style="margin-bottom: 10px"
        >
          <n-list-item v-for="(f, i) in view.decisionExplain.whyBuyLess" :key="'bl-' + i">
            <n-thing :title="f.plainText || f.code">
              <template #description>
                <n-space size="small">
                  <n-tag size="tiny" :bordered="false">{{ f.code || '—' }}</n-tag>
                  <n-tag size="tiny" :bordered="false">{{ f.source || '—' }}</n-tag>
                  <n-tag
                    size="tiny"
                    :type="f.available ? 'success' : 'warning'"
                    :bordered="false"
                  >
                    {{ f.available ? 'available' : 'partial' }}
                  </n-tag>
                </n-space>
              </template>
            </n-thing>
          </n-list-item>
        </n-list>
        <n-empty v-else size="small" description="暂无「买少」说明" style="margin-bottom: 10px" />

        <n-text strong style="display: block; margin: 10px 0 4px; font-size: 13px">
          why_position_limited · 为什么限制仓位
        </n-text>
        <n-list
          v-if="view.decisionExplain.whyPositionLimited.length"
          size="small"
          style="margin-bottom: 10px"
        >
          <n-list-item v-for="(f, i) in view.decisionExplain.whyPositionLimited" :key="'pl-' + i">
            <n-thing :title="f.plainText || f.code">
              <template #description>
                <n-space size="small">
                  <n-tag size="tiny" :bordered="false">{{ f.code || '—' }}</n-tag>
                  <n-tag size="tiny" :bordered="false">{{ f.source || '—' }}</n-tag>
                </n-space>
              </template>
            </n-thing>
          </n-list-item>
        </n-list>
        <n-empty
          v-else
          size="small"
          description="暂无「限制仓位」说明"
          style="margin-bottom: 10px"
        />

        <n-text strong style="display: block; margin: 10px 0 4px; font-size: 13px">
          why_suggest_reduce · 为什么建议减少
        </n-text>
        <n-list
          v-if="view.decisionExplain.whySuggestReduce.length"
          size="small"
          style="margin-bottom: 10px"
        >
          <n-list-item v-for="(f, i) in view.decisionExplain.whySuggestReduce" :key="'sr-' + i">
            <n-thing :title="f.plainText || f.code">
              <template #description>
                <n-space size="small">
                  <n-tag size="tiny" :bordered="false">{{ f.code || '—' }}</n-tag>
                  <n-tag size="tiny" :bordered="false">{{ f.source || '—' }}</n-tag>
                  <n-tag size="tiny" type="warning" :bordered="false">观察建议 · 非订单</n-tag>
                </n-space>
              </template>
            </n-thing>
          </n-list-item>
        </n-list>
        <n-empty
          v-else
          size="small"
          description="暂无「建议减少」说明（卖建议引擎默认关闭）"
          style="margin-bottom: 10px"
        />

        <n-text
          v-if="(view.dataGaps || []).length"
          depth="3"
          type="warning"
          style="display: block; margin-top: 12px; font-size: 12px"
        >
          数据缺口：{{ view.dataGaps.join('；') }}
        </n-text>
        <n-text
          v-if="(view.warnings || []).length"
          depth="3"
          type="warning"
          style="display: block; margin-top: 6px; font-size: 12px"
        >
          警告：{{ view.warnings.join('；') }}
        </n-text>
        <n-text depth="3" style="display: block; margin-top: 10px; font-size: 11px">
          {{ view.dataSourceNote }}
        </n-text>
      </template>
    </n-spin>
  </div>
</template>

<style scoped>
.portfolio-decision-dashboard {
  padding: 4px 0 12px;
}
.insight-card {
  padding: 12px 14px;
  margin-bottom: 8px;
  border: 1px solid rgba(128, 128, 128, 0.28);
  border-radius: 6px;
  background: rgba(128, 128, 128, 0.04);
}
</style>
