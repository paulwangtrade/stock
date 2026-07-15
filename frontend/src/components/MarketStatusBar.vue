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

defineProps({
  darkTheme: { type: Boolean, default: false },
})

const router = useRouter()
const snapshot = ref(null)
const accountOverview = ref(null)
const loading = ref(false)
let timer = null
let sessionTimer = null

const sessionLabel = computed(() => snapshot.value?.session?.label || resolveMarketSessionLabel().label)

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
  const parts = ['点击查看重大指数']
  const reason = snapshot.value?.mode?.reason
  if (reason) parts.push(`级别依据：${reason}`)
  for (const tag of segmentTags.value) parts.push(tag.title.replace('\n', '：'))
  if (cap?.hint) parts.push(cap.hint)
  return parts.join('\n')
})

function goMarket() {
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
  try {
    const next = await fetchMarketStatusSnapshot()
    snapshot.value = {
      ...next,
      session: next.session || resolveMarketSessionLabel(),
    }
    try {
      const follows = (await GetFollowList(0)) || []
      const rows = (Array.isArray(follows) ? follows : []).map((f) => ({
        costVolume: f.Volume ?? f.volume ?? 0,
        costPrice: f.CostPrice ?? f.costPrice ?? 0,
        '当前价格': f.Price ?? f.price ?? f.CostPrice ?? 0,
      }))
      accountOverview.value = buildAccountOverview(rows, {
        marketModeKey: next?.mode?.key,
      })
    } catch {
      accountOverview.value = null
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
    class="market-status-bar msb-clickable"
    :class="{ 'market-status-bar--dark': darkTheme }"
    :title="barTitle"
    @click="goMarket"
  >
    <n-flex align="center" justify="center" :size="6" :wrap="true" class="msb-inner">
      <n-tag size="small" :type="sessionTagType" :bordered="false">
        {{ sessionLabel }}
      </n-tag>
      <span class="msb-divider">|</span>
      <n-tag size="small" :type="modeTagType" :bordered="false">
        {{ snapshot?.mode?.label || '—' }}
      </n-tag>
      <n-tooltip v-for="item in segmentTags" :key="item.key" trigger="hover">
        <template #trigger>
          <n-tag size="small" :type="item.type" :bordered="true">
            {{ item.label }}
          </n-tag>
        </template>
        <span style="white-space: pre-wrap">{{ item.title }}</span>
      </n-tooltip>
      <template v-if="positionCapDisplay">
        <span class="msb-divider">|</span>
        <n-tag size="small" type="info" :bordered="false">
          {{ positionCapDisplay }}
        </n-tag>
      </template>
      <template v-if="accountOverview?.exposurePctDisplay != null">
        <span class="msb-divider">|</span>
        <n-tag
          size="small"
          :type="accountOverview.overCap ? 'error' : 'default'"
          :bordered="false"
          :title="accountOverview.hint"
        >
          持仓 {{ accountOverview.exposurePctDisplay }}% · 可用 {{ accountOverview.roomPctDisplay }}%
        </n-tag>
      </template>
      <n-spin v-if="loading" size="small" class="msb-spin" />
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
}

.msb-inner {
  font-size: 13px;
  line-height: 1.4;
  min-height: 28px;
}

.msb-divider {
  opacity: 0.35;
  user-select: none;
}

.msb-spin {
  margin-left: 4px;
}

.msb-clickable {
  cursor: pointer;
}

.msb-clickable:hover {
  opacity: 0.88;
}

.market-status-bar--dark {
  background: rgba(24, 24, 28, 0.92);
}
</style>
