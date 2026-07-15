<script setup>
import { NButton, NText, useMessage } from 'naive-ui'
import { onBeforeMount, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { Follow, GetConfig, GetFollowList, GetMoneyRankSina } from '../../wailsjs/go/main/App'
import StockKlineModal from './StockKlineModal.vue'
import { toEastMoneyCode } from '../utils/stockCode'
import { followWithDateGroup, formatFollowGroupMessage } from '../utils/followDateGroup'
import '../assets/rank-list-table.css'

const props = defineProps({
  headerTitle: {
    type: String,
    default: '净流入额排名',
  },
  sort: {
    type: String,
    default: 'netamount',
  },
})

const message = useMessage()
const dataList = ref([])
const sort = ref(props.sort)
const interval = ref(null)
const darkTheme = ref(false)
const followCodes = ref(new Set())
const modalDataRef = reactive({
  visible: false,
  title: '',
  stockCode: '',
  stockName: '',
})

onBeforeMount(() => {
  GetConfig().then((result) => {
    if (result.darkTheme) {
      darkTheme.value = true
    }
  })
  loadFollowSet()
})

onMounted(() => {
  sort.value = props.sort
  fetchData(false)
  interval.value = setInterval(() => fetchData(true), 1000 * 60)
})

onBeforeUnmount(() => {
  clearInterval(interval.value)
})

async function loadFollowSet() {
  try {
    const list = await GetFollowList(0)
    followCodes.value = new Set(
      (list || []).map((item) => String(item.StockCode || '').trim().toLowerCase()).filter(Boolean),
    )
  } catch {
    followCodes.value = new Set()
  }
}

function fetchData(silent = false) {
  let loadingMsg = null
  if (!silent) {
    loadingMsg = message.loading('正在刷新数据...', { duration: 0 })
  }
  GetMoneyRankSina(sort.value)
    .then((result) => {
      if (result?.length > 0) {
        dataList.value = result
      }
    })
    .finally(() => loadingMsg?.destroy())
}

function openKline(item) {
  const em = toEastMoneyCode(item.symbol)
  if (!em) {
    message.warning('无法识别股票代码')
    return
  }
  modalDataRef.stockCode = em
  modalDataRef.stockName = item.name
  modalDataRef.title = `${item.name} ${em} — 日K`
  modalDataRef.visible = true
}

function isFollowed(item) {
  const code = String(item?.symbol || '').trim().toLowerCase()
  return code && followCodes.value.has(code)
}

async function handleFollow(item) {
  const code = String(item?.symbol || '').trim().toLowerCase()
  if (!code) {
    message.warning('无法识别股票代码')
    return
  }
  if (isFollowed(item)) {
    message.info('已经关注了')
    return
  }
  const { followResult, groupInfo } = await followWithDateGroup(code, Follow)
  if (followResult === '关注成功') {
    followCodes.value.add(code)
    message.success(formatFollowGroupMessage(followResult, groupInfo))
  } else if (followResult === '已经关注了') {
    followCodes.value.add(code)
    message.info(followResult)
  } else {
    message.warning(followResult || '关注失败')
  }
}
</script>

<template>
  <div class="rank-list-wrap">
    <n-table class="rank-list-table" striped :single-line="false" size="small">
      <n-thead>
        <n-tr>
          <n-th class="col-name">代码</n-th>
          <n-th class="col-name">名称</n-th>
          <n-th>所属行业</n-th>
          <n-th class="col-num">最新价</n-th>
          <n-th class="col-num">涨跌幅</n-th>
          <n-th class="col-num">换手率</n-th>
          <n-th class="col-num">成交额/万</n-th>
          <n-th class="col-num">流出/万</n-th>
          <n-th class="col-num">流入/万</n-th>
          <n-th class="col-num">净流入/万</n-th>
          <n-th class="col-num">净流入率</n-th>
          <n-th v-if="sort === 'r0_net' || sort === 'r0_out'" class="col-num">主力流出/万</n-th>
          <n-th v-if="sort === 'r0_net'" class="col-num">主力流入/万</n-th>
          <n-th v-if="sort === 'r0_net'" class="col-num">主力净流入/万</n-th>
          <n-th class="col-num">主力净流入率</n-th>
          <n-th v-if="sort === 'r3_net' || sort === 'r3_out'" class="col-num">散户流出/万</n-th>
          <n-th v-if="sort === 'r3_net'" class="col-num">散户流入/万</n-th>
          <n-th v-if="sort === 'r3_net'" class="col-num">散户净流入/万</n-th>
          <n-th class="col-num">散户净流入率</n-th>
          <n-th class="col-action">操作</n-th>
        </n-tr>
      </n-thead>
      <n-tbody>
        <n-tr v-for="item in dataList" :key="item.symbol">
          <n-td class="col-name">
            <n-tag :bordered="false" type="info">{{ item.symbol }}</n-tag>
          </n-td>
          <n-td class="col-name">
            <span class="stock-link" @click="openKline(item)">
              <span
                class="stock-name"
                :class="item.changeratio > 0 ? 'up' : item.changeratio < 0 ? 'down' : ''"
              >
                {{ item.name }}
              </span>
            </span>
          </n-td>
          <n-td>
            <n-text depth="2">{{ item.industry || '—' }}</n-text>
          </n-td>
          <n-td class="col-num">
            <n-text :type="item.changeratio > 0 ? 'error' : item.changeratio < 0 ? 'success' : undefined">
              {{ item.trade }}
            </n-text>
          </n-td>
          <n-td class="col-num">
            <n-text :type="item.changeratio > 0 ? 'error' : item.changeratio < 0 ? 'success' : undefined">
              {{ (item.changeratio * 100).toFixed(2) }}%
            </n-text>
          </n-td>
          <n-td class="col-num">
            <n-text :type="item.turnover > 500 ? 'error' : undefined">
              {{ (item.turnover / 100).toFixed(2) }}%
            </n-text>
          </n-td>
          <n-td class="col-num">
            <n-text depth="2">{{ (item.amount / 10000).toFixed(2) }}</n-text>
          </n-td>
          <n-td class="col-num">
            <n-text depth="2">{{ (item.outamount / 10000).toFixed(2) }}</n-text>
          </n-td>
          <n-td class="col-num">
            <n-text depth="2">{{ (item.inamount / 10000).toFixed(2) }}</n-text>
          </n-td>
          <n-td class="col-num">
            <n-text :type="item.netamount > 0 ? 'error' : item.netamount < 0 ? 'success' : undefined">
              {{ (item.netamount / 10000).toFixed(2) }}
            </n-text>
          </n-td>
          <n-td class="col-num">
            <n-text :type="item.ratioamount > 0 ? 'error' : item.ratioamount < 0 ? 'success' : undefined">
              {{ (item.ratioamount * 100).toFixed(2) }}%
            </n-text>
          </n-td>
          <n-td v-if="sort === 'r0_net' || sort === 'r0_out'" class="col-num">
            <n-text type="success">{{ (item.r0_out / 10000).toFixed(2) }}</n-text>
          </n-td>
          <n-td v-if="sort === 'r0_net'" class="col-num">
            <n-text type="error">{{ (item.r0_in / 10000).toFixed(2) }}</n-text>
          </n-td>
          <n-td v-if="sort === 'r0_net'" class="col-num">
            <n-text :type="item.r0_net > 0 ? 'error' : 'success'">
              {{ (item.r0_net / 10000).toFixed(2) }}
            </n-text>
          </n-td>
          <n-td class="col-num">
            <n-text :type="item.r0_ratio > 0 ? 'error' : item.r0_ratio < 0 ? 'success' : undefined">
              {{ (item.r0_ratio * 100).toFixed(2) }}%
            </n-text>
          </n-td>
          <n-td v-if="sort === 'r3_net' || sort === 'r3_out'" class="col-num">
            <n-text type="success">{{ (item.r3_out / 10000).toFixed(2) }}</n-text>
          </n-td>
          <n-td v-if="sort === 'r3_net'" class="col-num">
            <n-text type="error">{{ (item.r3_in / 10000).toFixed(2) }}</n-text>
          </n-td>
          <n-td v-if="sort === 'r3_net'" class="col-num">
            <n-text :type="item.r3_net > 0 ? 'error' : 'success'">
              {{ (item.r3_net / 10000).toFixed(2) }}
            </n-text>
          </n-td>
          <n-td class="col-num">
            <n-text :type="item.r3_ratio > 0 ? 'error' : item.r3_ratio < 0 ? 'success' : undefined">
              {{ (item.r3_ratio * 100).toFixed(2) }}%
            </n-text>
          </n-td>
          <n-td class="col-action">
            <n-button
              size="tiny"
              :type="isFollowed(item) ? 'default' : 'warning'"
              :tertiary="isFollowed(item)"
              :disabled="isFollowed(item)"
              @click="handleFollow(item)"
            >
              {{ isFollowed(item) ? '已关注' : '关注' }}
            </n-button>
          </n-td>
        </n-tr>
      </n-tbody>
    </n-table>
  </div>

  <stock-kline-modal
    v-model:show="modalDataRef.visible"
    :title="modalDataRef.title"
    :chart-key="'money-rank-kline-' + modalDataRef.stockCode"
    :code="modalDataRef.stockCode"
    :stock-name="modalDataRef.stockName"
    :dark-theme="darkTheme"
    :strategy-signals="true"
  />
</template>

<style scoped>
.stock-name.up {
  color: var(--n-error-color);
}

.stock-name.down {
  color: var(--n-success-color);
}

.rank-list-table :deep(.col-action),
.rank-list-table :deep(th.col-action) {
  text-align: center;
  white-space: nowrap;
  width: 72px;
}
</style>
