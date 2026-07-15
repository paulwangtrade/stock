<script setup lang="ts">
import { onBeforeMount, onUnmounted, reactive, ref } from 'vue'
import { GetConfig, HotStock } from '../../wailsjs/go/main/App'
import StockKlineModal from './StockKlineModal.vue'
import { toEastMoneyCode } from '../utils/stockCode'
import { ArrowDown, ArrowUp } from '@vicons/ionicons5'
import { formatPercent2 } from '../utils/formatNumber'
import { useMessage } from 'naive-ui'
import '../assets/rank-list-table.css'

const { marketType } = defineProps({
  marketType: {
    type: String,
    default: '10',
  },
})

const message = useMessage()
const task = ref()
const list = ref([])
const darkTheme = ref(false)
const modalDataRef = reactive({
  visible: false,
  title: '',
  stockCode: '',
  stockName: '',
})

onBeforeMount(async () => {
  GetConfig().then((result) => {
    if (result?.darkTheme) darkTheme.value = true
  })
  list.value = await HotStock(marketType)
  task.value = setInterval(async () => {
    list.value = await HotStock(marketType)
  }, 5000)
})

onUnmounted(() => {
  clearInterval(task.value)
})

function getMarketCode(item) {
  if (item.exchange === 'SZ' || item.exchange === 'SH') {
    return item.code.toLowerCase()
  }
  if (item.exchange === 'HK') {
    return (item.exchange + item.code).toLowerCase()
  }
  return ('gb_' + item.code).toLowerCase()
}

function displayCode(item) {
  if (item.exchange === 'SH' || item.exchange === 'SZ' || item.exchange === 'HK') {
    return `${item.exchange}${item.code}`
  }
  return item.code || ''
}

function resolveEastMoneyCode(item) {
  const candidates = [
    getMarketCode(item),
    item.exchange && item.code ? `${item.exchange}${item.code}` : '',
    item.code,
  ].filter(Boolean)
  for (const raw of candidates) {
    const em = toEastMoneyCode(raw)
    if (em) return em
  }
  return ''
}

function openKline(item) {
  const em = resolveEastMoneyCode(item)
  if (!em) {
    message.warning('该市场暂不支持 K 线弹窗')
    return
  }
  modalDataRef.stockCode = em
  modalDataRef.stockName = item.name
  modalDataRef.title = `${item.name} ${em} — 日K`
  modalDataRef.visible = true
}

function deltaType(value) {
  if (value > 0) return 'error'
  if (value < 0) return 'success'
  return undefined
}
</script>

<template>
  <div class="rank-list-wrap">
    <n-table v-if="list.length" class="rank-list-table" striped :single-line="false">
      <n-thead>
        <n-tr>
          <n-th class="col-name">股票名称</n-th>
          <n-th class="col-num">涨跌幅</n-th>
          <n-th class="col-num">当前价格</n-th>
          <n-th class="col-num">热度</n-th>
          <n-th class="col-num">热度变化</n-th>
          <n-th class="col-num">排名变化</n-th>
        </n-tr>
      </n-thead>
      <n-tbody>
        <n-tr v-for="item in list" :key="item.code + item.exchange">
          <n-td class="col-name">
            <span class="stock-link" @click="openKline(item)">
              <span class="stock-name">{{ item.name }}</span>
              <span class="stock-code">{{ displayCode(item) }}</span>
            </span>
          </n-td>
          <n-td class="col-num">
            <n-text :type="item.percent > 0 ? 'error' : item.percent < 0 ? 'success' : undefined">
              {{ formatPercent2(item.percent) }}%
            </n-text>
          </n-td>
          <n-td class="col-num">
            <n-text>{{ item.current }}</n-text>
          </n-td>
          <n-td class="col-num">
            <n-text depth="2">{{ item.value }}</n-text>
          </n-td>
          <n-td class="col-num">
            <span class="delta-cell">
              <n-text :type="deltaType(item.increment)">
                {{ item.increment > 0 ? '+' : '' }}{{ item.increment }}
              </n-text>
              <n-icon v-if="item.increment > 0" :component="ArrowUp" />
              <n-icon v-else-if="item.increment < 0" :component="ArrowDown" />
            </span>
          </n-td>
          <n-td class="col-num">
            <span class="delta-cell">
              <n-text :type="deltaType(item.rank_change)">
                {{ item.rank_change > 0 ? '+' : '' }}{{ item.rank_change }}
              </n-text>
              <n-icon v-if="item.rank_change > 0" :component="ArrowUp" />
              <n-icon v-else-if="item.rank_change < 0" :component="ArrowDown" />
            </span>
          </n-td>
        </n-tr>
      </n-tbody>
    </n-table>
    <div v-else class="rank-list-empty">暂无热门数据</div>
  </div>

  <stock-kline-modal
    v-model:show="modalDataRef.visible"
    :title="modalDataRef.title"
    :chart-key="'hot-stock-kline-' + modalDataRef.stockCode"
    :code="modalDataRef.stockCode"
    :stock-name="modalDataRef.stockName"
    :dark-theme="darkTheme"
    :strategy-signals="true"
  />
</template>
