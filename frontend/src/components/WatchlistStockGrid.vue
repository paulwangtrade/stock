<script setup>
import { computed, ref, watch } from 'vue'
import WatchlistStockCard from './WatchlistStockCard.vue'
import {
  chunkWatchlistVirtualRows,
  nextWatchlistVirtualEnabled,
} from '../utils/watchlistVirtualScroll.js'

const props = defineProps({
  items: { type: Array, default: () => [] },
  cols: { type: Number, default: 4 },
  openAiEnable: { type: Boolean, default: false },
  groupList: { type: Array, default: () => [] },
  groupMode: { type: Boolean, default: false },
  groupId: { type: Number, default: null },
  draggable: { type: Boolean, default: true },
  dragCode: { type: String, default: '' },
  dragOverCode: { type: String, default: '' },
  signalFor: { type: Function, default: () => null },
  buyPriceFor: { type: Function, default: () => null },
  entryTagFor: { type: Function, default: () => '' },
  quantPlanFor: { type: Function, default: () => null },
  quantChecklistFor: { type: Function, default: () => null },
  /** Phase1-A：优先传入投影；缺省时卡片由散装 props 组装 */
  projectionFor: { type: Function, default: null },
  sparklineEligible: { type: Function, default: () => true },
  /** (code, row) => string[] observation tags */
  observationTagsFor: { type: Function, default: null },
  /** 虚拟列表可视高度 */
  maxHeight: { type: String, default: 'calc(92vh - 240px)' },
})

const emit = defineEmits([
  'unfollow',
  'signal-click',
  'ai',
  'cost',
  'lw-kline',
  'money',
  'detail',
  'notice',
  'report',
  'set-group',
  'create-draft',
  'remove-group',
  'dragstart',
  'dragover',
  'dragleave',
  'drop',
  'dragend',
])

const VIRTUAL_ON = 80
const VIRTUAL_OFF = 50
const virtualEnabled = ref(false)

watch(
  () => props.items.length,
  (n) => {
    virtualEnabled.value = nextWatchlistVirtualEnabled(
      virtualEnabled.value,
      n,
      VIRTUAL_ON,
      VIRTUAL_OFF,
    )
  },
  { immediate: true },
)

const compact = computed(() => props.cols >= 4)
const showSparkline = computed(() => props.cols < 4)

/** 按列切成虚拟滚动的「行」 */
const virtualRows = computed(() => chunkWatchlistVirtualRows(props.items, props.cols))

/** 行高估算：卡片 + 行间距（item-resizable 会再校正） */
const rowItemSize = computed(() => (compact.value ? 250 : 340))

function codeOf(result) {
  return String(result?.['股票代码'] || '')
}

function projectionOf(result) {
  if (typeof props.projectionFor !== 'function') return null
  return props.projectionFor(codeOf(result), result)
}

function observationTagsOf(result) {
  if (typeof props.observationTagsFor !== 'function') return ['用户关注']
  const tags = props.observationTagsFor(codeOf(result), result)
  return Array.isArray(tags) ? tags : ['用户关注']
}
</script>

<template>
  <n-grid
    v-if="!virtualEnabled"
    class="watchlist-grid"
    :class="'watchlist-grid--cols-' + cols"
    :x-gap="10"
    :cols="cols"
    :y-gap="10"
  >
    <n-gi
      v-for="(result, cardIndex) in items"
      :id="codeOf(result) + '_gi'"
      :key="codeOf(result)"
      class="watchlist-card-draggable"
      :class="{
        'watchlist-card-dragging': dragCode === codeOf(result),
        'watchlist-card-drag-over': dragOverCode === codeOf(result),
      }"
      :draggable="draggable"
      @dragstart="emit('dragstart', $event, result)"
      @dragover="emit('dragover', $event, result)"
      @dragleave="emit('dragleave', result)"
      @drop="emit('drop', $event, result)"
      @dragend="emit('dragend')"
    >
      <watchlist-stock-card
        :result="result"
        :projection="projectionOf(result)"
        :signal="signalFor(codeOf(result))"
        :buy-price-range="buyPriceFor(codeOf(result))"
        :entry-tag="entryTagFor(codeOf(result))"
        :quant-plan="quantPlanFor(codeOf(result))"
        :quant-checklist="quantChecklistFor(codeOf(result))"
        :show-sparkline="showSparkline"
        :sparkline-eligible="sparklineEligible(cardIndex)"
        :compact="compact"
        :open-ai-enable="openAiEnable"
        :group-list="groupList"
        :group-mode="groupMode"
        :group-id="groupId"
        :observation-tags="observationTagsOf(result)"
        @unfollow="emit('unfollow', $event)"
        @signal-click="emit('signal-click', $event)"
        @ai="emit('ai', $event)"
        @cost="emit('cost', $event)"
        @lw-kline="emit('lw-kline', $event)"
        @money="emit('money', $event)"
        @detail="emit('detail', $event)"
        @notice="emit('notice', $event)"
        @report="emit('report', $event)"
        @set-group="emit('set-group', $event)"
        @create-draft="emit('create-draft', $event)"
        @remove-group="emit('remove-group', $event)"
      />
    </n-gi>
  </n-grid>

  <n-virtual-list
    v-else
    class="watchlist-virtual-list"
    :style="{ maxHeight, paddingBottom: '12px' }"
    :items="virtualRows"
    :item-size="rowItemSize"
    item-resizable
    key-field="key"
  >
    <template #default="{ item }">
      <div
        class="watchlist-virtual-row"
        :class="'watchlist-grid--cols-' + cols"
        :style="{
          display: 'grid',
          gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))`,
          gap: '10px',
          paddingBottom: '10px',
          boxSizing: 'border-box',
        }"
      >
        <div
          v-for="cell in item.cells"
          :id="codeOf(cell.result) + '_gi'"
          :key="codeOf(cell.result)"
          class="watchlist-card-draggable"
          :class="{
            'watchlist-card-dragging': dragCode === codeOf(cell.result),
            'watchlist-card-drag-over': dragOverCode === codeOf(cell.result),
          }"
          :draggable="draggable"
          @dragstart="emit('dragstart', $event, cell.result)"
          @dragover="emit('dragover', $event, cell.result)"
          @dragleave="emit('dragleave', cell.result)"
          @drop="emit('drop', $event, cell.result)"
          @dragend="emit('dragend')"
        >
          <watchlist-stock-card
            :result="cell.result"
            :projection="projectionOf(cell.result)"
            :signal="signalFor(codeOf(cell.result))"
            :buy-price-range="buyPriceFor(codeOf(cell.result))"
            :entry-tag="entryTagFor(codeOf(cell.result))"
            :quant-plan="quantPlanFor(codeOf(cell.result))"
            :quant-checklist="quantChecklistFor(codeOf(cell.result))"
            :show-sparkline="showSparkline"
            :sparkline-eligible="true"
            :compact="compact"
            :open-ai-enable="openAiEnable"
            :group-list="groupList"
            :group-mode="groupMode"
            :group-id="groupId"
            :observation-tags="observationTagsOf(cell.result)"
            @unfollow="emit('unfollow', $event)"
            @signal-click="emit('signal-click', $event)"
            @ai="emit('ai', $event)"
            @cost="emit('cost', $event)"
            @lw-kline="emit('lw-kline', $event)"
            @money="emit('money', $event)"
            @detail="emit('detail', $event)"
            @notice="emit('notice', $event)"
            @report="emit('report', $event)"
            @set-group="emit('set-group', $event)"
            @create-draft="emit('create-draft', $event)"
            @remove-group="emit('remove-group', $event)"
          />
        </div>
      </div>
    </template>
  </n-virtual-list>
</template>

<style scoped>
.watchlist-grid {
  padding: 2px 0 12px;
}

.watchlist-card-draggable {
  cursor: grab;
  content-visibility: auto;
  contain-intrinsic-size: auto 320px;
}

.watchlist-card-draggable[draggable='false'] {
  cursor: default;
}

.watchlist-virtual-list {
  width: 100%;
}

.watchlist-virtual-row .watchlist-card-draggable {
  /* 虚拟列表内已裁剪可视区，无需再 content-visibility */
  content-visibility: visible;
  contain-intrinsic-size: none;
}
</style>
