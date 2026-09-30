<script setup>
/**
 * 决策时间轴：单只股票的观察证据，不是买卖指令。
 * 入场 / 持有 / 离场分列过滤，点击有日期的事件只打开已有 K 线。
 */
import { computed, ref, watch } from 'vue'
import { NAlert, NButton, NEmpty, NSpin, NTag, NText } from 'naive-ui'
import { fetchDecisionTimeline } from '../api/decisionTimeline.ts'
import {
  DECISION_TIMELINE_DISCLAIMER,
  LANE_FILTERS,
  canOpenKline,
  emptyTimelineMessage,
  filterTimelineEvents,
  kindLabel,
  laneLabel,
  sourceLabel,
  sourceStatusLabel,
  sourceTagType,
} from '../utils/decisionTimelineDisplay.js'

const props = defineProps({
  code: { type: String, default: '' },
  active: { type: Boolean, default: true },
})

const emit = defineEmits(['open-kline'])

const loading = ref(false)
const errorText = ref('')
const timeline = ref(null)
const lane = ref('all')

const visibleEvents = computed(() => filterTimelineEvents(timeline.value?.events, lane.value))
const emptyText = computed(() => {
  if (errorText.value || loading.value) return ''
  if (!String(props.code || '').trim()) return '先打开一只股票，再回顾它的决策记录。'
  if (lane.value !== 'all' && timeline.value?.events?.length && !visibleEvents.value.length) {
    return '这一栏没有记录。其他栏的入场、持有、离场不会混到这里。'
  }
  return emptyTimelineMessage(timeline.value)
})

async function load() {
  const code = String(props.code || '').trim()
  if (!props.active) return
  if (!code) {
    timeline.value = null
    errorText.value = ''
    return
  }
  loading.value = true
  errorText.value = ''
  try {
    timeline.value = await fetchDecisionTimeline(code)
  } catch (e) {
    timeline.value = null
    errorText.value = String(e?.message || e || '读取决策时间轴失败')
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.code, props.active],
  () => {
    lane.value = 'all'
    load()
  },
  { immediate: true },
)

function openKline(event) {
  if (!canOpenKline(event)) return
  emit('open-kline', event.kline_date)
}
</script>

<template>
  <div class="decision-timeline">
    <n-alert type="info" :bordered="false" class="decision-timeline__note">
      {{ timeline?.disclaimer || DECISION_TIMELINE_DISCLAIMER }}
      <template #header>观察记录</template>
      信号、关注、模拟盘触及和结果备注按时间摊开，用来核对「当时到底看到了什么」。不会生成交易计划，也不会自动下单。
    </n-alert>

    <div v-if="timeline?.sources?.length" class="decision-timeline__sources">
      <n-tag
        v-for="src in timeline.sources"
        :key="src.source"
        size="small"
        :bordered="false"
        :type="sourceTagType(src.status)"
      >
        {{ sourceLabel(src.source) }} · {{ sourceStatusLabel(src.status) }}
        <template v-if="src.count"> {{ src.count }}</template>
      </n-tag>
    </div>

    <n-text v-if="timeline?.sources?.some((s) => s.status !== 'ok' && s.message)" depth="3" class="decision-timeline__source-notes">
      <div v-for="src in timeline.sources.filter((s) => s.status !== 'ok' && s.message)" :key="src.source + '-msg'">
        {{ sourceLabel(src.source) }}：{{ src.message }}
      </div>
    </n-text>

    <div class="decision-timeline__filters">
      <n-button
        v-for="item in LANE_FILTERS"
        :key="item.id"
        size="tiny"
        :type="lane === item.id ? 'primary' : 'default'"
        :secondary="lane !== item.id"
        @click="lane = item.id"
      >
        {{ item.label }}
      </n-button>
    </div>

    <n-spin :show="loading">
      <n-alert v-if="errorText" type="error" :bordered="false" style="margin-bottom: 8px">
        {{ errorText }}
        <template #footer>
          <n-button size="tiny" @click="load">重试</n-button>
        </template>
      </n-alert>

      <n-empty v-else-if="emptyText" :description="emptyText" />

      <ol v-else class="decision-timeline__list">
        <li v-for="event in visibleEvents" :key="event.id" class="decision-timeline__item">
          <div class="decision-timeline__rail" />
          <div class="decision-timeline__body">
            <div class="decision-timeline__meta">
              <n-text strong>{{ event.occurred_on || '日期未记录' }}</n-text>
              <n-tag size="small" :bordered="false">{{ laneLabel(event.lane) }}</n-tag>
              <n-tag size="small" :bordered="false" type="info">{{ kindLabel(event.kind) }}</n-tag>
              <n-tag v-if="event.tag" size="small">{{ event.tag }}</n-tag>
            </div>
            <div class="decision-timeline__title">{{ event.title }}</div>
            <n-text depth="3" class="decision-timeline__detail">{{ event.detail }}</n-text>
            <div class="decision-timeline__actions">
              <n-button
                v-if="canOpenKline(event)"
                size="tiny"
                quaternary
                type="primary"
                @click="openKline(event)"
              >
                在K线查看该日
              </n-button>
              <n-text v-else depth="3">这一条没有可定位的交易日</n-text>
            </div>
          </div>
        </li>
      </ol>
    </n-spin>

    <n-text depth="3" class="decision-timeline__hint">
      若该日早于当前已加载的日K窗口，图上可能仍停在默认区间。
    </n-text>
  </div>
</template>

<style scoped>
.decision-timeline {
  text-align: left;
  padding-top: 4px;
}
.decision-timeline__note {
  margin-bottom: 10px;
}
.decision-timeline__sources {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 6px;
}
.decision-timeline__source-notes {
  display: block;
  margin-bottom: 8px;
  font-size: 12px;
  line-height: 1.5;
}
.decision-timeline__filters {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 8px 0 12px;
}
.decision-timeline__list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.decision-timeline__item {
  display: flex;
  gap: 10px;
  position: relative;
}
.decision-timeline__rail {
  width: 10px;
  position: relative;
  flex: none;
}
.decision-timeline__rail::before {
  content: '';
  position: absolute;
  left: 4px;
  top: 4px;
  bottom: -4px;
  width: 2px;
  background: rgba(148, 163, 184, 0.7);
}
.decision-timeline__rail::after {
  content: '';
  position: absolute;
  left: 1px;
  top: 8px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #64748b;
}
.decision-timeline__item:last-child .decision-timeline__rail::before {
  bottom: auto;
  height: 12px;
}
.decision-timeline__body {
  padding-bottom: 14px;
  min-width: 0;
}
.decision-timeline__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}
.decision-timeline__title {
  margin-top: 4px;
  font-weight: 600;
}
.decision-timeline__detail {
  display: block;
  margin-top: 4px;
  line-height: 1.5;
}
.decision-timeline__actions {
  margin-top: 4px;
}
.decision-timeline__hint {
  display: block;
  margin-top: 8px;
  font-size: 12px;
}
</style>
