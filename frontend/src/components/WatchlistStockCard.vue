<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { formatPercent2 } from '../utils/formatNumber'
import { formatBuyPriceRangeText, buyPriceRangeLabel, formatPriceTick } from '../utils/buyPriceRange'
import { formatPositionPlanText } from '../utils/buyPositionSizing'
import { formatChecklistScore } from '../utils/buyChecklist'
import { formatSignalTagLabel, getSignalTagColor } from '../utils/signalBuyGuide'
import { resolveSignalActionHint, formatSignalActionTooltip } from '../utils/signalActionHint'
import { signalSettingsState } from '../utils/signalSettingsStore'
import StockSparkLine from './stockSparkLine.vue'

const props = defineProps({
  result: { type: Object, required: true },
  signal: { type: Object, default: null },
  buyPriceRange: { type: Object, default: null },
  entryTag: { type: String, default: '' },
  quantPlan: { type: Object, default: null },
  quantChecklist: { type: Object, default: null },
  showSparkline: { type: Boolean, default: true },
  /** 父级渲染窗口内的卡片才允许挂载迷你分时（再叠加可视区/hover） */
  sparklineEligible: { type: Boolean, default: true },
  openAiEnable: { type: Boolean, default: false },
  groupList: { type: Array, default: () => [] },
  groupMode: { type: Boolean, default: false },
  groupId: { type: Number, default: null },
  compact: { type: Boolean, default: false },
})

const cardRootRef = ref(null)
const sparkInView = ref(false)
const sparkHovered = ref(false)
let sparkObserver = null

/**
 * 前 N*cols 窗口内可直接挂载 spark 宿主；窗口外仅在进入可视区或 hover 时挂载。
 * API 请求再由 sparkActive（可视/hover）二次门禁。
 */
const sparkMountAllowed = computed(
  () => props.showSparkline
    && (props.sparklineEligible || sparkInView.value || sparkHovered.value),
)

/** 仅可视区或 hover 时激活 GetStockMinutePriceLineData；离开可视区可停止 */
const sparkActive = computed(
  () => props.showSparkline && (sparkInView.value || sparkHovered.value),
)

function onSparkEnter() {
  sparkHovered.value = true
}

function onSparkLeave() {
  sparkHovered.value = false
}

onMounted(() => {
  if (typeof IntersectionObserver === 'undefined' || !cardRootRef.value) {
    sparkInView.value = true
    return
  }
  sparkObserver = new IntersectionObserver(
    (entries) => {
      const entry = entries[0]
      sparkInView.value = !!entry?.isIntersecting
    },
    { root: null, rootMargin: '80px 0px', threshold: 0.01 },
  )
  sparkObserver.observe(cardRootRef.value)
})

onBeforeUnmount(() => {
  if (sparkObserver) {
    sparkObserver.disconnect()
    sparkObserver = null
  }
})

const emit = defineEmits([
  'unfollow',
  'signal-click',
  'ai',
  'remove-group',
  'cost',
  'lw-kline',
  'money',
  'detail',
  'notice',
  'report',
  'set-group',
  'create-draft',
])

const r = computed(() => props.result)
const hasQuote = computed(() => Number(r.value['买一报价']) > 0)

const signalLabel = computed(() => {
  const s = props.signal
  if (!s?.tag) return ''
  return formatSignalTagLabel(s.tag, s.sellPositionPct ?? s.addPositionPct ?? s.rushReducePct)
})

const signalActionHint = computed(() =>
  resolveSignalActionHint({
    tag: props.signal?.tag,
    buyPriceRange: props.buyPriceRange,
    sellPositionPct: props.signal?.sellPositionPct,
    addPositionPct: props.signal?.addPositionPct,
    rushReducePct: props.signal?.rushReducePct,
    sellVolume: props.signal?.sellVolume,
    costPrice: props.signal?.costPrice,
    sourceTag: props.signal?.sourceTag,
    daysAgo: props.signal?.recentSignalDaysAgo ?? props.signal?.daysAgo,
    checklistReady: props.quantChecklist?.ready,
    holdingAdvice: props.signal?.holdingAdvice,
  }),
)

const signalTooltip = computed(() => {
  const parts = [`${r.value['股票名称'] || '未知股票'} (${r.value['股票代码'] || '--'})`]
  if (props.signal?.statusText) parts.push(props.signal.statusText)
  const advice = props.signal?.holdingAdvice
  if (advice?.factors?.length) {
    const lines = advice.factors.map((f) => `${f.impact >= 0 ? '+' : ''}${f.impact} ${f.label}：${f.detail}`)
    parts.push(`持仓辅助（参考）\n${lines.join('\n')}`)
  }
  const action = formatSignalActionTooltip(signalActionHint.value)
  if (action) parts.push(action)
  return parts.filter(Boolean).join('\n\n')
})

const hasHolding = computed(() => Number(r.value.costVolume) > 0 && Number(r.value.costPrice) > 0)

const holdingAdviceLabel = computed(() => {
  const advice = props.signal?.holdingAdvice
  if (!advice || !hasHolding.value) return ''
  const pct = advice.suggestPctDisplay
  if (pct > 0 && !props.signal?.tag) {
    return `${advice.actionLabel} ${pct}%`
  }
  return advice.actionLabel
})

const holdingAdviceType = computed(() => {
  const action = props.signal?.holdingAdvice?.action
  if (action === 'add') return 'success'
  if (action === 'reduce') return 'error'
  return 'default'
})

const buyPriceDisplay = computed(() => formatBuyPriceRangeText(props.buyPriceRange))

const buyRangeLabel = computed(() => buyPriceRangeLabel(props.buyPriceRange))

const buyPriceTooltip = computed(() => {
  const bp = props.buyPriceRange
  if (!bp) return ''
  const parts = [bp.note]
  if (bp.instantText) parts.push(`出信号价 ${bp.instantText}`)
  if (bp.deferMode === 'wait') parts.push('今日偏高，明日等回踩，不必当日追入')
  else if (bp.deferMode === 'todayOrTomorrow') {
    parts.push('明日仍可买，开盘或盘中落入区间即可')
    if (bp.rangeHigh > bp.instantPrice) {
      parts.push(`出信号价 ${formatPriceTick(bp.instantPrice)}，区间上沿含小幅高开容忍`)
    }
  }
  parts.push('参考区间，非投资建议')
  return parts.filter(Boolean).join(' · ')
})

const planText = computed(() => formatPositionPlanText(props.quantPlan))
const checklistText = computed(() => formatChecklistScore(props.quantChecklist))

const canCreateDraft = computed(() => {
  if (props.quantPlan?.ok && (props.quantPlan.suggestedAddShares > 0 || props.quantPlan.suggestedShares > 0)) {
    return true
  }
  const tag = props.signal?.tag
  const sellPct = props.signal?.sellPositionPct
  if (tag && Number(sellPct) > 0 && hasHolding.value) return true
  return false
})

const draftButtonLabel = computed(() => {
  if (props.quantPlan?.ok) return '生成买入草稿'
  if (props.signal?.sellPositionPct > 0) return '生成卖出草稿'
  return '生成交易草稿'
})

const checklistTooltip = computed(() => {
  const ck = props.quantChecklist
  if (!ck?.items?.length) return ''
  return ck.items.map((i) => `${i.passed ? '✓' : '✗'} ${i.label}`).join('\n')
})

function parseFollowDate(raw) {
  if (!raw) return null
  const s = String(raw).trim()
  const m = s.match(/^(\d{4})-(\d{2})-(\d{2})/)
  if (m) return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]))
  const d = new Date(s.replace(' ', 'T'))
  return Number.isNaN(d.getTime()) ? null : d
}

const followTimeText = computed(() => {
  const raw = r.value['关注时间']
  if (!raw) return ''
  const d = parseFollowDate(raw)
  if (!d) return `关注 ${raw}`
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const fd = new Date(d)
  fd.setHours(0, 0, 0, 0)
  const days = Math.round((today - fd) / 86400000)
  const dateStr = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  if (days <= 0) return `关注 ${dateStr} · 今日`
  return `关注 ${dateStr} · ${days}天`
})

const tintTag = computed(() => props.signal?.tag || props.entryTag || props.buyPriceRange?.tag || '')

const cardSignalTintOn = computed(
  () => !!signalSettingsState.value.display?.watchlistCardSignalTint && !!tintTag.value,
)

const cardTintStyle = computed(() => {
  if (!cardSignalTintOn.value) return undefined
  const c = getSignalTagColor(tintTag.value)
  return {
    '--wl-card-tint-bg': c.color,
    '--wl-card-tint-border': c.borderColor,
  }
})

const followBasePrice = computed(() => {
  const p = Number(r.value['关注价格'] ?? r.value.FollowPrice)
  return Number.isFinite(p) && p > 0 ? p : null
})

function resolveCurrentPrice(row) {
  const cur = Number(row['当前价格'])
  if (Number.isFinite(cur) && cur > 0) return cur
  const bid = Number(row['买一报价'])
  if (Number.isFinite(bid) && bid > 0) return bid
  const pre = Number(row['昨日收盘价'])
  if (Number.isFinite(pre) && pre > 0) return pre
  return null
}

const followCurrentPrice = computed(() => resolveCurrentPrice(r.value))

const followProfitPct = computed(() => {
  const fp = Number(r.value.followProfit)
  if (Number.isFinite(fp)) return fp
  const base = followBasePrice.value
  const cur = followCurrentPrice.value
  if (base && cur != null) {
    return ((cur - base) / base) * 100
  }
  return null
})

const followProfitDiff = computed(() => {
  const base = followBasePrice.value
  const cur = followCurrentPrice.value
  if (base == null || cur == null) return null
  return cur - base
})

const followProfitType = computed(() => {
  const p = followProfitPct.value
  if (p == null) return 'default'
  if (p > 0) return 'error'
  if (p < 0) return 'success'
  return 'default'
})

const followProfitTooltip = computed(() => {
  if (!followBasePrice.value) return '暂无关注基准价，刷新后会自动回填'
  const cur = followCurrentPrice.value
  const pct = followProfitPct.value
  const diff = followProfitDiff.value
  const pctText = pct == null ? '—' : `${formatPercent2(pct)}%`
  const diffText = diff == null ? '—' : `${diff >= 0 ? '+' : ''}${formatPriceTick(diff)}`
  return `自关注以来 · 关注价 ${formatPriceTick(followBasePrice.value)} → 现价 ${formatPriceTick(cur)} · ${diffText} (${pctText})`
})
</script>

<template>
  <div
    ref="cardRootRef"
    class="watchlist-card-host"
    :data-code="r['股票代码']"
    @mouseenter="onSparkEnter"
    @mouseleave="onSparkLeave"
  >
  <n-card
    class="watchlist-card"
    :class="{
      'watchlist-card--compact': compact,
      'watchlist-card--signal-tint': cardSignalTintOn,
    }"
    :style="cardTintStyle"
    :data-sort="r.sort"
    :id="r['股票代码']"
    :data-code="r['股票代码']"
    size="small"
    :bordered="true"
    :content-style="compact ? 'padding: 10px 12px 8px' : 'padding: 12px 14px 10px'"
  >
    <template #header>
      <div class="wl-card__head">
        <div class="wl-card__title-row">
          <n-text strong class="wl-card__name">{{ r['股票名称'] }}</n-text>
          <n-tooltip v-if="signal?.tag || holdingAdviceLabel" trigger="hover">
            <template #trigger>
              <n-flex :size="4" align="center" :wrap="false" style="display: inline-flex">
                <n-tag
                  v-if="signal?.tag"
                  class="wl-card__signal"
                  size="small"
                  round
                  :bordered="true"
                  :color="getSignalTagColor(signal.tag)"
                  style="cursor: pointer"
                  @click="emit('signal-click', signal)"
                >
                  {{ signalLabel }}
                </n-tag>
                <n-tag
                  v-if="signalActionHint"
                  size="tiny"
                  :type="signalActionHint.type"
                  :bordered="true"
                  round
                  style="cursor: help"
                >
                  {{ signalActionHint.label }}
                </n-tag>
                <n-tag
                  v-if="holdingAdviceLabel && !signal?.tag"
                  size="tiny"
                  :type="holdingAdviceType"
                  :bordered="true"
                  round
                  style="cursor: help"
                >
                  {{ holdingAdviceLabel }}
                </n-tag>
              </n-flex>
            </template>
            <span style="white-space: pre-wrap">{{ signalTooltip }}</span>
          </n-tooltip>
        </div>
        <div class="wl-card__head-actions">
          <n-text depth="3" class="wl-card__code">{{ r['股票代码'] }}</n-text>
          <n-tag
            v-if="r['所属行业']"
            size="tiny"
            type="primary"
            :bordered="false"
            class="wl-card__industry-tag"
          >
            {{ r['所属行业'] }}
          </n-tag>
          <n-text v-if="followTimeText" depth="3" class="wl-card__follow-time">{{ followTimeText }}</n-text>
          <n-button size="tiny" quaternary type="error" @click="emit('unfollow', r)">取消关注</n-button>
          <n-button
            v-if="openAiEnable"
            size="tiny"
            quaternary
            type="warning"
            @click="emit('ai', r)"
          >
            AI
          </n-button>
          <n-button
            v-if="groupMode && groupId != null"
            size="tiny"
            quaternary
            type="error"
            @click="emit('remove-group', { result: r, groupId })"
          >
            移出分组
          </n-button>
        </div>
      </div>
    </template>

    <div class="wl-card__price-row">
      <div class="wl-card__price-main">
        <n-text :type="r.type" class="wl-card__price">
          <n-number-animation
            :duration="800"
            :precision="2"
            :from="r['上次当前价格']"
            :to="Number(r['当前价格'])"
          />
        </n-text>
        <n-text :type="r.type" class="wl-card__pct">
          <n-number-animation :duration="800" :precision="2" :from="0" :to="r.changePercent" />
          <span class="wl-card__pct-unit">%</span>
        </n-text>
        <n-tag
          v-if="r['盘前盘后'] > 0"
          size="tiny"
          :type="r.type"
          :bordered="false"
          class="wl-card__pre"
        >
          {{ r['盘前盘后'] }} {{ formatPercent2(r['盘前盘后涨跌幅']) }}%
        </n-tag>
        <n-text v-if="r.costVolume > 0" size="small" :type="r.type" class="wl-card__today-pnl">
          今盈亏
          <n-number-animation :duration="800" :precision="2" :from="0" :to="r.profitAmountToday" />
        </n-text>
        <n-tooltip v-if="followProfitPct != null" trigger="hover">
          <template #trigger>
            <n-text :type="followProfitType" size="small" class="wl-card__follow-pnl">
              盈亏比
              <n-number-animation
                :duration="800"
                :precision="2"
                :from="0"
                :to="followProfitPct"
              />
              <span class="wl-card__pct-unit">%</span>
            </n-text>
          </template>
          {{ followProfitTooltip }}
        </n-tooltip>
      </div>
      <div v-if="sparkMountAllowed" class="wl-card__spark" data-sparkline-lazy="1">
        <stock-spark-line
          :active="sparkActive"
          :last-price="Number(r['当前价格'])"
          :open-price="Number(r['昨日收盘价'])"
          :stock-code="r['股票代码']"
          :stock-name="r['股票名称']"
        />
      </div>
    </div>

    <div class="wl-card__stats">
      <div v-if="buyPriceRange" class="wl-card__stat wl-card__stat--buy-range">
        <span class="wl-card__stat-label">{{ buyRangeLabel }}</span>
        <n-tooltip trigger="hover">
          <template #trigger>
            <span
              class="wl-card__stat-val wl-card__buy-range"
              :class="{ 'wl-card__buy-range--extended': buyPriceRange.extended }"
            >
              {{ buyPriceDisplay }}
            </span>
          </template>
          {{ buyPriceTooltip }}
        </n-tooltip>
      </div>
      <div v-if="quantPlan?.ok || quantChecklist" class="wl-card__stat wl-card__stat--quant">
        <span class="wl-card__stat-label">量化</span>
        <n-tooltip v-if="quantChecklist" trigger="hover">
          <template #trigger>
            <n-tag
              size="tiny"
              :type="quantChecklist.ready ? 'success' : 'default'"
              :bordered="false"
              class="wl-card__quant-tag"
            >
              清单 {{ checklistText }}
            </n-tag>
          </template>
          <span style="white-space: pre-wrap">{{ checklistTooltip }}</span>
        </n-tooltip>
        <n-tooltip v-if="quantPlan" trigger="hover">
          <template #trigger>
            <span class="wl-card__stat-val wl-card__quant-plan">{{ planText }}</span>
          </template>
          {{ quantPlan.reason }}
          <template v-if="quantPlan.stopPrice"> · 止损≈{{ formatPriceTick(quantPlan.stopPrice) }}</template>
        </n-tooltip>
      </div>
      <div class="wl-card__stat">
        <span class="wl-card__stat-label">最高</span>
        <span class="wl-card__stat-val">{{ r['今日最高价'] }}</span>
        <span :class="['wl-card__stat-sub', r.highRate >= 0 ? 'up' : 'down']">
          {{ formatPercent2(r.highRate) }}%
        </span>
      </div>
      <div class="wl-card__stat">
        <span class="wl-card__stat-label">最低</span>
        <span class="wl-card__stat-val">{{ r['今日最低价'] }}</span>
        <span :class="['wl-card__stat-sub', r.lowRate >= 0 ? 'up' : 'down']">
          {{ formatPercent2(r.lowRate) }}%
        </span>
      </div>
      <div class="wl-card__stat">
        <span class="wl-card__stat-label">昨收</span>
        <span class="wl-card__stat-val">{{ r['昨日收盘价'] }}</span>
      </div>
      <div class="wl-card__stat">
        <span class="wl-card__stat-label">今开</span>
        <span class="wl-card__stat-val">{{ r['今日开盘价'] }}</span>
      </div>
    </div>

    <div class="wl-card__meta">
      <n-tag v-if="r.volume > 0" size="tiny" :type="r.profitType" :bordered="false">{{ r.volume }}股</n-tag>
      <n-tag v-if="r.costPrice > 0" size="tiny" :type="r.profitType" :bordered="false">
        成本 {{ r.costPrice }}×{{ r.costVolume }} · {{ formatPercent2(r.profit) }}% ({{ r.profitAmount }}¥)
      </n-tag>
    </div>

    <div class="wl-card__primary-actions">
      <n-button size="small" type="primary" @click="emit('lw-kline', r)">多周期 K 线</n-button>
      <n-button
        v-if="canCreateDraft"
        size="small"
        type="warning"
        secondary
        @click="emit('create-draft', r)"
      >
        {{ draftButtonLabel }}
      </n-button>
    </div>

    <div class="wl-card__toolbar">
      <n-button size="tiny" quaternary @click="emit('cost', r)">成本</n-button>
      <n-button v-if="hasQuote" size="tiny" quaternary @click="emit('money', r)">资金</n-button>
      <n-button size="tiny" quaternary @click="emit('detail', r)">详情</n-button>
      <n-button v-if="hasQuote" size="tiny" quaternary @click="emit('notice', r)">公告</n-button>
      <n-button v-if="hasQuote" size="tiny" quaternary @click="emit('report', r)">研报</n-button>
      <n-dropdown
        trigger="click"
        :options="groupList"
        key-field="ID"
        label-field="name"
        @select="(groupId) => emit('set-group', { groupId, result: r })"
      >
        <n-button size="tiny" quaternary>分组</n-button>
      </n-dropdown>
    </div>
  </n-card>
  </div>
</template>

<style scoped>
.watchlist-card-host {
  width: 100%;
  height: 100%;
}

.watchlist-card {
  border-radius: 10px;
  transition: box-shadow 0.2s ease, border-color 0.2s ease, background-color 0.2s ease;
}

.watchlist-card--signal-tint {
  background-color: var(--wl-card-tint-bg, transparent) !important;
  border-color: var(--wl-card-tint-border, var(--n-border-color)) !important;
}

.watchlist-card--signal-tint :deep(.n-card-header),
.watchlist-card--signal-tint :deep(.n-card__content) {
  background-color: transparent;
}

.watchlist-card:hover {
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.06);
}

.wl-card__head {
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: 100%;
}

.wl-card__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.wl-card__name {
  font-size: 15px;
  line-height: 1.3;
}

.wl-card__signal {
  font-weight: 600;
  min-width: 3.2em;
  justify-content: center;
}

.wl-card__code {
  font-size: 12px;
  font-family: ui-monospace, monospace;
}

.wl-card__industry-tag {
  max-width: 120px;
}

.wl-card__industry-tag :deep(.n-tag__content) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.wl-card__follow-time {
  font-size: 11px;
  margin-right: auto;
  white-space: nowrap;
}

.wl-card__head-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}

.wl-card__head-actions .wl-card__code {
  margin-right: 0;
}

.wl-card__price-row {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 10px;
}

.wl-card__price-main {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 6px 10px;
  min-width: 0;
}

.wl-card__price {
  font-size: 26px;
  font-weight: 600;
  line-height: 1.1;
  font-variant-numeric: tabular-nums;
}

.wl-card__pct {
  font-size: 20px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.wl-card__pct-unit {
  font-size: 14px;
  margin-left: 1px;
}

.wl-card__pre {
  vertical-align: middle;
}

.wl-card__today-pnl {
  opacity: 0.9;
}

.wl-card__follow-pnl {
  opacity: 0.95;
  font-variant-numeric: tabular-nums;
  font-weight: 600;
}

.wl-card__spark {
  flex: 0 0 88px;
  height: 36px;
  min-width: 72px;
}

.wl-card__stats {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px 12px;
  padding: 8px 10px;
  border-radius: 8px;
  background: var(--n-color-modal, rgba(128, 128, 128, 0.06));
  margin-bottom: 8px;
}

.wl-card__stat {
  display: flex;
  align-items: baseline;
  gap: 6px;
  font-size: 12px;
  line-height: 1.4;
}

.wl-card__stat-label {
  color: var(--n-text-color-3);
  flex-shrink: 0;
}

.wl-card__stat-val {
  font-variant-numeric: tabular-nums;
}

.wl-card__stat-sub {
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.wl-card__stat-sub.up {
  color: #e88080;
}

.wl-card__stat-sub.down {
  color: #63e2b7;
}

.wl-card__stat--buy-range {
  grid-column: 1 / -1;
}

.wl-card__stat--quant {
  grid-column: 1 / -1;
  flex-wrap: wrap;
  gap: 8px;
}

.wl-card__quant-tag {
  font-weight: 600;
}

.wl-card__quant-plan {
  font-size: 12px;
  font-weight: 600;
}

.wl-card__buy-range {
  font-weight: 600;
  color: var(--n-text-color);
}

.wl-card__buy-range--extended {
  color: #e88080;
}

.wl-card__quote {
  margin-bottom: 6px;
}

.wl-card__quote :deep(.n-collapse-item__header) {
  padding: 4px 0 !important;
  font-size: 12px;
}

.wl-card__book-preview {
  font-size: 11px;
  color: var(--n-text-color-3);
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.wl-card__book-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4px 12px;
  font-size: 12px;
}

.wl-card__meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin-bottom: 10px;
}

.wl-card__time {
  font-size: 11px;
}

.wl-card__primary-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 8px;
}

.wl-card__toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 2px 4px;
  padding-top: 8px;
  border-top: 1px solid var(--n-divider-color);
}

.watchlist-card--compact .wl-card__name {
  font-size: 14px;
}

.watchlist-card--compact .wl-card__price {
  font-size: 21px;
}

.watchlist-card--compact .wl-card__pct {
  font-size: 16px;
}

.watchlist-card--compact .wl-card__stats {
  padding: 6px 8px;
  gap: 4px 8px;
}

.watchlist-card--compact .wl-card__stat {
  font-size: 11px;
}

.watchlist-card--compact .wl-card__primary-actions :deep(.n-button) {
  font-size: 12px;
  padding: 0 8px;
}

.watchlist-card--compact .wl-card__head-actions :deep(.n-button) {
  padding: 0 6px;
}
</style>
