<script setup lang="ts">
import { onBeforeMount, reactive, ref } from 'vue'
import { GetConfig, LongTigerRank } from '../../wailsjs/go/main/App'
import { ArrowDownOutline } from '@vicons/ionicons5'
import _ from 'lodash'
import StockKlineModal from './StockKlineModal.vue'
import { resolveStrategyRowCode, resolveStrategyRowName } from '../utils/stockCode'
import { useMessage } from 'naive-ui'
import '../assets/rank-list-table.css'

const message = useMessage()

const lhbList = ref([])
const EXPLANATIONs = ref([])
const darkTheme = ref(false)
const modalDataRef = reactive({
  visible: false,
  title: '',
  stockCode: '',
  stockName: '',
})

const today = new Date()
const year = today.getFullYear()
const month = String(today.getMonth() + 1).padStart(2, '0')
const day = String(today.getDate()).padStart(2, '0')
const formattedDate = `${year}-${month}-${day}`

const SearchForm = ref({
  dateValue: formattedDate,
  EXPLANATION: null,
})

onBeforeMount(() => {
  GetConfig().then((result) => {
    if (result?.darkTheme) darkTheme.value = true
  })
  longTiger(formattedDate)
})

function displayCode(item) {
  const secu = item?.SECUCODE || ''
  const parts = secu.split('.')
  if (parts.length === 2) return parts[1].toLowerCase() + parts[0]
  return item?.SECURITY_CODE || secu
}

function openKline(item) {
  const em = resolveStrategyRowCode(item)
  if (!em) {
    message.warning('无法识别股票代码')
    return
  }
  modalDataRef.stockCode = em
  modalDataRef.stockName = resolveStrategyRowName(item)
  modalDataRef.title = `${modalDataRef.stockName} ${em} — 日K`
  modalDataRef.visible = true
}

function longTiger(date) {
  if (date) {
    SearchForm.value.dateValue = date
  }

  const loading1 = message.loading('正在获取龙虎榜数据...', {
    duration: 0,
  })

  const fetchDate = (currentDate, retryCount = 0) => {
    if (retryCount > 7) {
      lhbList.value = []
      EXPLANATIONs.value = []
      loading1.destroy()
      message.info('暂无历史数据')
      return
    }

    LongTigerRank(currentDate)
      .then((res) => {
        if (res.length === 0) {
          const previousDate = new Date(currentDate)
          previousDate.setDate(previousDate.getDate() - 1)

          const y = previousDate.getFullYear()
          const m = String(previousDate.getMonth() + 1).padStart(2, '0')
          const d = String(previousDate.getDate()).padStart(2, '0')
          const prevFormattedDate = `${y}-${m}-${d}`

          message.info(`当前日期 ${currentDate} 暂无数据，尝试查询前一日：${prevFormattedDate}`)

          SearchForm.value.dateValue = prevFormattedDate
          fetchDate(prevFormattedDate, retryCount + 1)
        } else {
          lhbList.value = res
          loading1.destroy()
          EXPLANATIONs.value = _.uniqBy(
            _.map(lhbList.value, (item) => ({
              label: item['EXPLANATION'],
              value: item['EXPLANATION'],
            })),
            'label',
          )
        }
      })
      .catch((err) => {
        loading1.destroy()
        message.error('获取数据失败，请重试')
        console.error(err)
      })
  }

  fetchDate(date || formattedDate)
}

function handleEXPLANATION(value) {
  SearchForm.value.EXPLANATION = value
  if (value) {
    LongTigerRank(SearchForm.value.dateValue).then((res) => {
      lhbList.value = _.filter(res, (o) => o['EXPLANATION'] === value)
      if (res.length === 0) {
        message.info('暂无数据,请切换日期')
      }
    })
  } else {
    longTiger(SearchForm.value.dateValue)
  }
}

function fmtNum(v, digits = 2) {
  const n = Number(v)
  return Number.isFinite(n) ? n.toFixed(digits) : '—'
}
</script>

<template>
  <n-form :model="SearchForm" class="lhb-search">
    <n-grid :cols="24" :x-gap="24">
      <n-form-item-gi :span="4" label="日期" path="dateValue" label-placement="left">
        <n-date-picker
          v-model:formatted-value="SearchForm.dateValue"
          value-format="yyyy-MM-dd"
          type="date"
          :on-update:value="(v, v2) => longTiger(v2)"
        />
      </n-form-item-gi>
      <n-form-item-gi :span="8" label="上榜原因" path="EXPLANATION" label-placement="left">
        <n-select
          clearable
          placeholder="上榜原因过滤"
          v-model:value="SearchForm.EXPLANATION"
          :options="EXPLANATIONs"
          :on-update:value="handleEXPLANATION"
        />
      </n-form-item-gi>
      <n-form-item-gi :span="10" label="" label-placement="left">
        <n-text depth="3">* 当天龙虎榜数据通常在收盘后约 1 小时更新</n-text>
      </n-form-item-gi>
    </n-grid>
  </n-form>

  <div class="rank-list-wrap">
    <n-table v-if="lhbList.length" class="rank-list-table" :single-line="false" striped>
      <n-thead>
        <n-tr>
          <n-th class="col-name">名称</n-th>
          <n-th class="col-num">收盘价</n-th>
          <n-th class="col-num">涨跌幅</n-th>
          <n-th class="col-num">净买额(万)</n-th>
          <n-th class="col-num">买入额(万)</n-th>
          <n-th class="col-num">卖出额(万)</n-th>
          <n-th class="col-num">成交额(万)</n-th>
          <n-th class="col-num">
            换手率
            <n-icon :component="ArrowDownOutline" />
          </n-th>
          <n-th class="col-num">流通市值(亿)</n-th>
          <n-th>上榜原因</n-th>
        </n-tr>
      </n-thead>
      <n-tbody>
        <n-tr v-for="(item, index) in lhbList" :key="index">
          <n-td class="col-name">
            <span class="stock-link" @click="openKline(item)">
              <span
                class="stock-name"
                :class="item.CHANGE_RATE > 0 ? 'up' : item.CHANGE_RATE < 0 ? 'down' : ''"
              >
                {{ item.SECURITY_NAME_ABBR }}
              </span>
              <span class="stock-code">{{ displayCode(item) }}</span>
            </span>
          </n-td>
          <n-td class="col-num">
            <n-text :type="item.CHANGE_RATE > 0 ? 'error' : item.CHANGE_RATE < 0 ? 'success' : undefined">
              {{ fmtNum(item.CLOSE_PRICE) }}
            </n-text>
          </n-td>
          <n-td class="col-num">
            <n-text :type="item.CHANGE_RATE > 0 ? 'error' : item.CHANGE_RATE < 0 ? 'success' : undefined">
              {{ fmtNum(item.CHANGE_RATE) }}%
            </n-text>
          </n-td>
          <n-td class="col-num">
            <n-text :type="item.BILLBOARD_NET_AMT > 0 ? 'error' : item.BILLBOARD_NET_AMT < 0 ? 'success' : undefined">
              {{ fmtNum(item.BILLBOARD_NET_AMT / 10000) }}
            </n-text>
          </n-td>
          <n-td class="col-num">
            <n-text type="error">{{ fmtNum(item.BILLBOARD_BUY_AMT / 10000) }}</n-text>
          </n-td>
          <n-td class="col-num">
            <n-text type="success">{{ fmtNum(item.BILLBOARD_SELL_AMT / 10000) }}</n-text>
          </n-td>
          <n-td class="col-num">
            <n-text depth="2">{{ fmtNum(item.BILLBOARD_DEAL_AMT / 10000) }}</n-text>
          </n-td>
          <n-td class="col-num">
            <n-text depth="2">{{ fmtNum(item.TURNOVERRATE) }}%</n-text>
          </n-td>
          <n-td class="col-num">
            <n-text depth="2">{{ fmtNum(item.FREE_MARKET_CAP / 100000000) }}</n-text>
          </n-td>
          <n-td>
            <n-text depth="2" class="lhb-reason">{{ item.EXPLANATION }}</n-text>
          </n-td>
        </n-tr>
      </n-tbody>
    </n-table>
    <div v-else class="rank-list-empty">暂无龙虎榜数据</div>
  </div>

  <stock-kline-modal
    v-model:show="modalDataRef.visible"
    :title="modalDataRef.title"
    :chart-key="'lhb-kline-' + modalDataRef.stockCode"
    :code="modalDataRef.stockCode"
    :stock-name="modalDataRef.stockName"
    :dark-theme="darkTheme"
    :strategy-signals="true"
  />
</template>

<style scoped>
.lhb-search {
  margin-bottom: 12px;
}

.stock-name.up {
  color: var(--n-error-color);
}

.stock-name.down {
  color: var(--n-success-color);
}

.lhb-reason {
  display: inline-block;
  max-width: 280px;
  line-height: 1.45;
  word-break: break-all;
}
</style>
