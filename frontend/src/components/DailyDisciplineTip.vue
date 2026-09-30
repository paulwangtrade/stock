<script setup>
/**
 * 每日精进 — slim, dismissible discipline reminder on the home dashboard.
 * One tip per Asia/Shanghai day. 「下一条」only walks today's small pool.
 * Not a modal and not a trade signal.
 */
import { ref } from 'vue'
import { NButton, NTag } from 'naive-ui'
import {
  advanceDailyDiscipline,
  dismissDailyDiscipline,
  presentDailyDiscipline,
} from '../utils/dailyDisciplineTips.js'

const view = ref(presentDailyDiscipline())
const expanded = ref(false)

function onToggle() {
  expanded.value = !expanded.value
}

function onNext() {
  view.value = advanceDailyDiscipline()
  expanded.value = true
}

function onDismiss() {
  view.value = dismissDailyDiscipline()
  expanded.value = false
}
</script>

<template>
  <section
    v-if="view.visible && view.tip"
    class="daily-tip"
    data-testid="daily-discipline-tip"
    :data-tip-id="view.tip.id"
    aria-label="每日精进"
  >
    <div class="daily-tip-row">
      <div class="daily-tip-main">
        <div class="daily-tip-kicker">
          <span class="daily-tip-name">每日精进</span>
          <n-tag size="small" :bordered="false" type="info">{{ view.tip.category }}</n-tag>
          <span v-if="view.positionLabel" class="daily-tip-pos">今日 {{ view.positionLabel }}</span>
        </div>
        <p class="daily-tip-summary" :class="{ 'is-open': expanded }">
          <strong>{{ view.tip.title }}。</strong>{{ view.tip.summary }}
        </p>
      </div>
      <div class="daily-tip-actions">
        <n-button quaternary size="tiny" @click="onToggle">
          {{ expanded ? '收起' : '展开' }}
        </n-button>
        <n-button v-if="view.canAdvance" quaternary size="tiny" @click="onNext">
          下一条
        </n-button>
        <n-button quaternary size="tiny" @click="onDismiss">今日不再显示</n-button>
      </div>
    </div>
    <p v-if="expanded" class="daily-tip-body">{{ view.tip.body }}</p>
    <p class="daily-tip-disclaimer">{{ view.disclaimer }}</p>
  </section>
</template>

<style scoped>
.daily-tip {
  margin: 0 0 14px;
  padding: 8px 12px 7px;
  border: 1px solid rgba(32, 128, 240, 0.28);
  border-radius: 8px;
  background: rgba(32, 128, 240, 0.05);
}
.daily-tip-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px 12px;
}
.daily-tip-main {
  min-width: 0;
  flex: 1 1 auto;
}
.daily-tip-kicker {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 2px;
}
.daily-tip-name {
  font-size: 13px;
  font-weight: 650;
}
.daily-tip-pos {
  font-size: 12px;
  opacity: 0.65;
}
.daily-tip-summary {
  margin: 0;
  font-size: 13px;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.daily-tip-summary.is-open {
  display: block;
  -webkit-line-clamp: unset;
  overflow: visible;
}
.daily-tip-body {
  margin: 6px 0 0;
  font-size: 13px;
  line-height: 1.6;
  opacity: 0.92;
}
.daily-tip-disclaimer {
  margin: 4px 0 0;
  font-size: 12px;
  line-height: 1.4;
  opacity: 0.62;
}
.daily-tip-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 2px;
  flex: 0 0 auto;
}
@media (max-width: 640px) {
  .daily-tip-row {
    flex-direction: column;
  }
  .daily-tip-actions {
    justify-content: flex-start;
  }
}
</style>
