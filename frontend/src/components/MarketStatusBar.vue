<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { EventsEmit } from '../../wailsjs/runtime'
import { GetFollowList } from '../../wailsjs/go/main/App'
import {
  fetchMarketStatusSnapshot,
  getMarketStatusRefreshIntervalMs,
  resolveMarketSessionLabel,
} from '../utils/marketStatusBar'
import { STOCK_MARKET_SEGMENTS, STOCK_MARKET_SEGMENT_ORDER } from '../utils/stockMarketSegment'
import { buildAccountOverview } from '../utils/accountOverview'
import { toDisplayTradingLevel } from '../utils/tradingLevelRules'
import { withTimeout } from '../utils/withTimeout'

defineProps({
  darkTheme: { type: Boolean, default: false },
})

/** 顶栏首刷兜底：避免 Wails/chromedp 长时间不返回导致永久 spinner */
const MARKET_STATUS_UI_TIMEOUT_MS = 15_000

const router = useRouter()
const snapshot = ref(null)
const accountOverview = ref(null)
const loading = ref(false)
const loadError = ref('')
/** 顶栏账户概览最近刷新时间（独立于行情 snapshot.updatedAt） */
const accountUpdatedAt = ref(0)
let timer = null
let sessionTimer = null

const sessionLabel = computed(() => snapshot.value?.session?.label || resolveMarketSessionLabel().label)

const dataUpdatedAtLabel = computed(() => {
  const ts = accountUpdatedAt.value || snapshot.value?.updatedAt || 0
  if (!ts) return ''
  try {
    return new Date(ts).toLocaleString('zh-CN', {
      timeZone: 'Asia/Shanghai',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    })
  } catch {
    return ''
  }
})

const riskStatusLabel = computed(() => {
  if (!accountOverview.value) return ''
  return accountOverview.value.overCap ? '警告' : '正常'
})

const riskStatusType = computed(() => (accountOverview.value?.overCap ? 'error' : 'success'))

const sessionTagType = computed(() => {
  const key = snapshot.value?.session?.key
  if (key === 'trading') return 'success'
  if (key === 'pre' || key === 'post') return 'info'
  if (key === 'lunch') return 'warning'
  return 'default'
})

const modeTagType = computed(() => {
  const key = snapshot.value?.mode?.key
  if (key === 'level5') return 'success'
  if (key === 'level4') return 'info'
  if (key === 'level3') return 'warning'
  if (key === 'level1' || key === 'level2') return 'error'
  return 'default'
})

function levelTagType(level) {
  if (level === 5) return 'success'
  if (level === 4) return 'info'
  if (level === 3) return 'warning'
  if (level === 1 || level === 2) return 'error'
  return 'default'
}

const segmentTags = computed(() => STOCK_MARKET_SEGMENT_ORDER.map((key) => {
  const segment = STOCK_MARKET_SEGMENTS[key]
  const market = snapshot.value?.markets?.[key]
  return {
    key,
    label: `${segment.short}${toDisplayTradingLevel(market?.mode?.level) ?? '—'}`,
    type: levelTagType(market?.mode?.level),
    title: `${segment.name} · ${market?.mode?.label || '数据不足'}\n${market?.mode?.reason || '暂无指数数据'}`,
  }
}))

const positionCapDisplay = computed(() => {
  const cap = snapshot.value?.positionCap
  if (!cap?.rangeLabel) return ''
  return `仓位 ${cap.rangeLabel}`
})

const barTitle = computed(() => {
  const cap = snapshot.value?.positionCap
  const parts = ['市场级别依据（仅信息；点「重大指数」跳转）']
  const reason = snapshot.value?.mode?.reason
  if (reason) parts.push(`级别依据：${reason}`)
  for (const tag of segmentTags.value) parts.push(tag.title.replace('\n', '：'))
  if (cap?.hint) parts.push(cap.hint)
  if (accountOverview.value?.hint) parts.push(accountOverview.value.hint)
  return parts.join('\n')
})

function goMarket(e) {
  e?.stopPropagation?.()
  router.push({ name: 'market', query: { name: '重大指数', indexTab: '上证指数' } })
  EventsEmit('changeActiveMenuKey', 'market')
  EventsEmit('changeMarketTab', { ID: 0, name: '重大指数', indexTab: '上证指数' })
}

function scheduleRefresh() {
  clearTimeout(timer)
  const ms = getMarketStatusRefreshIntervalMs()
  timer = setTimeout(async () => {
    if (!document.hidden) {
      await refresh(false)
    }
    scheduleRefresh()
  }, ms)
}

async function refresh(showLoading = true) {
  if (showLoading && !snapshot.value) loading.value = true
  loadError.value = ''
  try {
    const next = await withTimeout(
      fetchMarketStatusSnapshot(),
      MARKET_STATUS_UI_TIMEOUT_MS,
      'marketStatusSnapshot',
    )
    snapshot.value = {
      ...next,
      session: next.session || resolveMarketSessionLabel(),
    }
    try {
      const follows = (await withTimeout(GetFollowList(0), 8_000, 'GetFollowList')) || []
      const rows = (Array.isArray(follows) ? follows : []).map((f) => ({
        costVolume: f.Volume ?? f.volume ?? 0,
        costPrice: f.CostPrice ?? f.costPrice ?? 0,
        '当前价格': f.Price ?? f.price ?? f.CostPrice ?? 0,
      }))
      accountOverview.value = buildAccountOverview(rows, {
        marketModeKey: next?.mode?.key,
      })
      accountUpdatedAt.value = Date.now()
    } catch {
      accountOverview.value = null
    }
  } catch (e) {
    loadError.value = e?.message || String(e)
    if (!snapshot.value) {
      snapshot.value = {
        ok: false,
        session: resolveMarketSessionLabel(),
        markets: {},
        mode: { key: 'unknown', label: '数据不足', reason: loadError.value },
        positionCap: null,
        error: loadError.value,
        updatedAt: Date.now(),
      }
    }
  } finally {
    loading.value = false
  }
}

function onVisibilityChange() {
  if (!document.hidden) refresh(false)
}

function tickSessionLabel() {
  if (!snapshot.value) return
  snapshot.value = {
    ...snapshot.value,
    session: resolveMarketSessionLabel(),
  }
}

onMounted(async () => {
  await refresh()
  scheduleRefresh()
  sessionTimer = setInterval(tickSessionLabel, 30_000)
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', onVisibilityChange)
  clearTimeout(timer)
  clearInterval(sessionTimer)
})
</script>

<template>
  <div
    class="market-status-bar"
    :class="{ 'market-status-bar--dark': darkTheme }"
    :title="barTitle"
  >
    <n-flex align="center" justify="center" :size="6" :wrap="true" class="msb-inner">
      <n-button size="tiny" secondary type="primary" class="msb-action" @click="goMarket">
        重大指数
      </n-button>
      <n-tag size="small" :type="sessionTagType" :bordered="false" class="msb-static">
        {{ sessionLabel }}
      </n-tag>
      <span class="msb-divider">|</span>
      <n-tag size="small" :type="modeTagType" :bordered="false" class="msb-static">
        {{ snapshot?.mode?.label || '—' }}
      </n-tag>
      <n-tooltip v-for="item in segmentTags" :key="item.key" trigger="hover">
        <template #trigger>
          <n-tag size="small" :type="item.type" :bordered="true" class="msb-static">
            {{ item.label }}
          </n-tag>
        </template>
        <span style="white-space: pre-wrap">{{ item.title }}</span>
      </n-tooltip>
      <template v-if="positionCapDisplay">
        <span class="msb-divider">|</span>
        <n-tag size="small" type="info" :bordered="false" class="msb-static">
          {{ positionCapDisplay }}
        </n-tag>
      </template>
      <template v-if="accountOverview?.exposurePctDisplay != null">
        <span class="msb-divider">|</span>
        <n-tag
          size="small"
          :type="accountOverview.overCap ? 'error' : 'default'"
          :bordered="false"
          class="msb-static"
          :title="accountOverview.hint"
        >
          股票仓位 {{ accountOverview.exposurePctDisplay }}% · 仓位余量 {{ accountOverview.roomPctDisplay }}%
        </n-tag>
        <n-tag size="small" :type="riskStatusType" :bordered="false" class="msb-static">
          风险 {{ riskStatusLabel }}
        </n-tag>
      </template>
      <template v-if="dataUpdatedAtLabel">
        <span class="msb-divider">|</span>
        <n-text depth="3" class="msb-updated">更新 {{ dataUpdatedAtLabel }}</n-text>
      </template>
      <n-spin v-if="loading" size="small" class="msb-spin" />
      <n-button
        v-else-if="loadError"
        size="tiny"
        secondary
        type="warning"
        class="msb-action"
        :title="loadError"
        @click.stop="refresh(true)"
      >
        重试
      </n-button>
    </n-flex>
  </div>
</template>

<style scoped>
.market-status-bar {
  position: sticky;
  top: 0;
  z-index: 21;
  width: 100%;
  box-sizing: border-box;
  padding: 5px 12px;
  border-bottom: 1px solid rgba(128, 128, 128, 0.25);
  background: rgba(255, 255, 255, 0.92);
  backdrop-filter: blur(6px);
  --wails-draggable: no-drag;
  cursor: default;
}

.msb-inner {
  font-size: 13px;
  line-height: 1.4;
  min-height: 28px;
}

.msb-divider {
  opacity: 0.35;
  user-select: none;
  pointer-events: none;
}

.msb-spin {
  margin-left: 4px;
}

.msb-action {
  cursor: pointer;
}

.msb-static {
  cursor: default;
}

.msb-updated {
  font-size: 11px;
  white-space: nowrap;
  pointer-events: none;
  user-select: none;
}

.market-status-bar--dark {
  background: rgba(24, 24, 28, 0.92);
}
</style>
