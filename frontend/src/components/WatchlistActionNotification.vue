<script setup>
import { computed } from 'vue'
import { formatSignalTagLabel } from '../utils/signalBuyGuide'

const props = defineProps({
  alert: { type: Object, default: () => ({}) },
})

const ACTION_THEME = {
  先风控: { accent: '#dc2626', soft: '#fef2f2' },
  早减: { accent: '#ea580c', soft: '#fff7ed' },
  等回踩: { accent: '#d97706', soft: '#fffbeb' },
  可买: { accent: '#16a34a', soft: '#f0fdf4' },
  可加仓: { accent: '#059669', soft: '#ecfdf5' },
}

const batchItems = computed(() => {
  const batch = props.alert?.batch
  return Array.isArray(batch) && batch.length ? batch : null
})

const theme = computed(() => {
  const label = props.alert?.actionLabel || (batchItems.value?.some((a) => a.actionLabel === '先风控') ? '先风控' : '可买')
  return ACTION_THEME[label] || ACTION_THEME.可买
})

const tagLabel = computed(() => {
  const a = props.alert || {}
  return formatSignalTagLabel(a.tag, a.actionHint?.sellPositionPct ?? a.sellPositionPct ?? a.addPositionPct ?? a.rushReducePct)
})

const detailLines = computed(() => {
  const lines = props.alert?.actionHint?.lines || []
  if (lines.length) return lines
  const parsed = String(props.alert?.content || '').split('\n').filter(Boolean)
  if (parsed.length > 1 && parsed[0].includes('(')) return parsed.slice(1)
  return parsed
})

const headline = computed(() => {
  const lines = detailLines.value
  const main = lines.find((l) => /建议|早减|可买|等回踩|加仓|止盈|减仓/.test(String(l)))
  return main || lines[0] || ''
})

const importantLine = computed(() => {
  const line = detailLines.value.find((item) => item !== headline.value && !/非投资建议|参考/.test(String(item)))
  return line || ''
})

function itemTagLabel(item) {
  return formatSignalTagLabel(item.tag, item.actionHint?.sellPositionPct ?? item.sellPositionPct ?? item.addPositionPct ?? item.rushReducePct)
}
</script>

<template>
  <div class="wa-notify" :style="{ '--wa-accent': theme.accent, '--wa-soft': theme.soft }">
    <template v-if="batchItems">
      <div class="wa-notify__summary">
        <strong>{{ batchItems.length }}</strong> 只自选有操作提示
      </div>
      <ul class="wa-notify__batch">
        <li v-for="item in batchItems.slice(0, 4)" :key="`${item.code}-${item.tag}-${item.actionLabel}`">
          <span class="wa-notify__batch-name">{{ item.name }}</span>
          <span class="wa-notify__mini-tag">{{ itemTagLabel(item) }}</span>
          <span class="wa-notify__batch-action">{{ item.actionLabel }}</span>
        </li>
      </ul>
      <div v-if="batchItems.length > 4" class="wa-notify__more">还有 {{ batchItems.length - 4 }} 只</div>
    </template>

    <template v-else>
      <div class="wa-notify__head">
        <div class="wa-notify__name">{{ alert.name || '—' }}</div>
        <span class="wa-notify__action-pill">{{ alert.actionLabel || '提示' }}</span>
      </div>
      <div class="wa-notify__meta">
        <span>{{ alert.code || '' }}</span>
        <span v-if="tagLabel">{{ tagLabel }}</span>
      </div>
      <div v-if="headline" class="wa-notify__headline">{{ headline }}</div>
      <div v-if="importantLine" class="wa-notify__reason">{{ importantLine }}</div>
    </template>
  </div>
</template>

<style scoped>
.wa-notify {
  text-align: left;
  font-size: 13px;
  line-height: 1.35;
  color: #334155;
  background: var(--wa-soft);
  border-left: 3px solid var(--wa-accent);
  border-radius: 6px;
  padding: 8px 10px;
  margin-top: -2px;
}

.wa-notify__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.wa-notify__name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  font-weight: 600;
  color: #0f172a;
}

.wa-notify__meta {
  display: flex;
  gap: 8px;
  margin-top: 3px;
  font-size: 12px;
  color: #64748b;
}

.wa-notify__action-pill {
  flex-shrink: 0;
  padding: 2px 7px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  color: var(--wa-accent);
  background: rgba(255, 255, 255, 0.72);
}

.wa-notify__headline {
  margin-top: 7px;
  font-size: 13px;
  font-weight: 600;
  color: var(--wa-accent);
}

.wa-notify__reason {
  margin-top: 4px;
  color: #475569;
  font-size: 12px;
}

.wa-notify__summary {
  font-size: 13px;
  color: #475569;
}

.wa-notify__summary strong {
  color: var(--wa-accent);
  font-weight: 700;
}

.wa-notify__batch {
  list-style: none;
  margin: 6px 0 0;
  padding: 0;
}

.wa-notify__batch li {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 3px 0;
}

.wa-notify__batch-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
  color: #0f172a;
}

.wa-notify__mini-tag {
  flex-shrink: 0;
  color: #64748b;
  font-size: 12px;
}

.wa-notify__batch-action {
  flex-shrink: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--wa-accent);
}

.wa-notify__more {
  margin-top: 4px;
  font-size: 12px;
  color: #94a3b8;
}
</style>
