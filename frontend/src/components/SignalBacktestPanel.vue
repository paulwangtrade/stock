<script setup>
import { computed, ref } from 'vue'
import {
  NButton,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NSelect,
  NSpace,
  NStatistic,
  NSwitch,
  NText,
  useMessage,
} from 'naive-ui'
import { GetStockEastMoneyKLine } from '../../wailsjs/go/main/App'
import { eastMoneyKLinesToBars, runDailySignalBacktest } from '../utils/backtestEngine'
import { resolveMarketMode } from '../utils/tradingLevelRules'

const message = useMessage()
const stockCode = ref('600519')
const limit = ref(250)
const initialCash = ref(1000000)
const useDiscipline = ref(true)
const loading = ref(false)
const result = ref(null)
const compare = ref(null)

const ma = (closes, i, n) => {
  if (i + 1 < n) return null
  let s = 0
  for (let j = i - n + 1; j <= i; j++) s += closes[j]
  return s / n
}

async function run() {
  loading.value = true
  result.value = null
  compare.value = null
  try {
    const raw = await GetStockEastMoneyKLine(stockCode.value.trim(), '101', String(limit.value), '', '1')
    const bars = eastMoneyKLinesToBars(raw)
    if (bars.length < 60) {
      message.warning('K 线不足，请换代码或加大条数')
      return
    }
    const closes = bars.map((b) => b.close)
    const volumes = bars.map((b) => b.volume || 0)

    const signalAt = (i) => {
      if (i < 60) return null
      const ma5 = ma(closes, i, 5)
      const ma10 = ma(closes, i, 10)
      const ma20 = ma(closes, i, 20)
      const prevMa5 = ma(closes, i - 1, 5)
      const prevMa10 = ma(closes, i - 1, 10)
      if (ma5 && ma10 && prevMa5 && prevMa10 && prevMa5 < prevMa10 && ma5 >= ma10 && closes[i] > ma20) {
        return { action: 'buy', strength: 0.7, tag: '趋' }
      }
      if (ma5 && ma10 && prevMa5 && prevMa10 && prevMa5 > prevMa10 && ma5 <= ma10) {
        return { action: 'sell', sellPct: 1, tag: '止' }
      }
      return null
    }

    const marketModeAt = (i) => {
      const ma5 = ma(closes, i, 5)
      const ma10 = ma(closes, i, 10)
      const ma20 = ma(closes, i, 20)
      const ma60 = ma(closes, i, 60)
      let volSum = 0
      let cnt = 0
      for (let j = Math.max(0, i - 5); j < i; j++) {
        if (volumes[j] > 0) {
          volSum += volumes[j]
          cnt++
        }
      }
      const avg = cnt ? volSum / cnt : 0
      const volumeRatio = avg > 0 ? volumes[i] / avg : 1
      const ma20Prev = ma(closes, i - 1, 20)
      return resolveMarketMode({
        close: closes[i],
        ma5,
        ma10,
        ma20,
        ma60,
        volumeRatio,
        volumeExpanding: volumeRatio >= 1.05,
        ma20Rising: ma20Prev != null && ma20 != null && ma20 >= ma20Prev,
      }).key
    }

    const withDisc = runDailySignalBacktest({
      bars,
      initialCash: initialCash.value,
      usePositionDiscipline: useDiscipline.value,
      signalAt,
      marketModeAt,
    })
    const noDisc = runDailySignalBacktest({
      bars,
      initialCash: initialCash.value,
      usePositionDiscipline: false,
      signalAt,
      marketModeAt,
    })
    result.value = withDisc
    compare.value = noDisc
    message.success('回测完成')
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    loading.value = false
  }
}

const summaryText = computed(() => {
  if (!result.value) return ''
  const a = result.value
  const b = compare.value
  return [
    `纪律模型：收益 ${a.totalReturnPct}% · 回撤 ${a.maxDrawdownPct}% · 夏普 ${a.sharpe} · 交易 ${a.tradeCount}`,
    b ? `无纪律对照：收益 ${b.totalReturnPct}% · 回撤 ${b.maxDrawdownPct}% · 夏普 ${b.sharpe} · 交易 ${b.tradeCount}` : '',
  ]
    .filter(Boolean)
    .join('\n')
})
</script>

<template>
  <div class="backtest-panel">
    <n-text depth="3" class="hint">
      日频 MVP：MA 金叉/死叉示意信号 + 可选 5 级仓位纪律，含佣金/印花税/滑点/T+1。仅供研究，非投资建议。
    </n-text>
    <n-form inline label-placement="left" :show-feedback="false" class="form">
      <n-form-item label="代码">
        <n-input v-model:value="stockCode" style="width: 120px" placeholder="600519" />
      </n-form-item>
      <n-form-item label="K线条数">
        <n-input-number v-model:value="limit" :min="120" :max="1000" :step="10" />
      </n-form-item>
      <n-form-item label="初始资金">
        <n-input-number v-model:value="initialCash" :min="100000" :step="100000" />
      </n-form-item>
      <n-form-item label="5级纪律">
        <n-switch v-model:value="useDiscipline" />
      </n-form-item>
      <n-form-item>
        <n-button type="primary" :loading="loading" @click="run">运行回测</n-button>
      </n-form-item>
    </n-form>

    <n-space v-if="result" :size="24" style="margin-top: 16px">
      <n-statistic label="最终权益" :value="result.finalEquity" />
      <n-statistic label="总收益%" :value="result.totalReturnPct" />
      <n-statistic label="最大回撤%" :value="result.maxDrawdownPct" />
      <n-statistic label="夏普(简)" :value="result.sharpe" />
      <n-statistic label="交易次数" :value="result.tradeCount" />
    </n-space>
    <pre v-if="summaryText" class="summary">{{ summaryText }}</pre>
  </div>
</template>

<style scoped>
.backtest-panel {
  padding: 12px 8px;
  height: 100%;
  overflow: auto;
}
.hint {
  display: block;
  margin-bottom: 12px;
  font-size: 13px;
}
.form {
  flex-wrap: wrap;
}
.summary {
  margin-top: 16px;
  white-space: pre-wrap;
  font-size: 13px;
  line-height: 1.6;
  opacity: 0.85;
}
</style>
