<script setup>
import { computed, h, onMounted, ref, watch } from 'vue'
import {
  NButton,
  NCheckbox,
  NCheckboxGroup,
  NDataTable,
  NInput,
  NSelect,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import { GetStockEastMoneyKLine, GetStockList, GetStockStrategyList } from '../../wailsjs/go/main/App'
import { buildIndexMa20ByDay, normalizeDayKey } from '../utils/icePointSignals'
import {
  COMPARE_FOOTER_TEXT,
  buildStrategyCatalog,
  defaultSelectedStrategyIds,
  evaluateObservationCompare,
  klineRowsToScanBars,
  sliceBarsToIndex,
} from '../utils/multiStrategyCompare'
import { resolveStockIdentity } from '../utils/signalBacktestIdentity'
import { STRATEGY_ROLE_FOOTER, resolveStrategyRole } from '../utils/multiStrategyRole'
import { getSignalOptions, loadSignalSettingsFromBackend, signalSettingsState } from '../utils/signalSettingsStore'
import { eastMoneyCodeVariants } from '../utils/stockCode'
import { resolveSignalLastBarIndex } from '../utils/tradingSession'
import { isScannableWatchlistCode, resolveScanKlineBarCount } from '../utils/watchlistSignalScan'

const props = defineProps({
  /** 有代码时按该股票自动对照，并收起手动输入。研究页不传，仍用手输。 */
  stockCode: { type: String, default: '' },
  stockName: { type: String, default: '' },
})

const message = useMessage()
const queryText = ref('')
const catalogReady = ref(false)
const catalog = ref([])
const selectedIds = ref([])
const picks = ref([])
const pickedKey = ref(null)
const loading = ref(false)
const rows = ref([])
const subjectLabel = ref('')
const ran = ref(false)

const wiredEngines = computed(() => catalog.value.filter((item) => item.wired && item.kind === 'scan_family'))
const presetEngines = computed(() => catalog.value.filter((item) => item.wired && item.kind === 'scan_preset'))
const unwiredEngines = computed(() => catalog.value.filter((item) => !item.wired))
const embeddedCode = computed(() => String(props.stockCode || '').trim())
const embeddedName = computed(() => String(props.stockName || '').trim())
const embedded = computed(() => embeddedCode.value.length > 0)

const pickOptions = computed(() =>
  picks.value.map((item) => ({
    label: item.label,
    value: item.code || item.label,
  })),
)

function roleForStrategy(strategyId) {
  const engine = catalog.value.find((item) => item.strategyId === strategyId)
  return resolveStrategyRole(strategyId, engine || {})
}

function renderRoleTag(role) {
  return h(
    NTag,
    { size: 'small', bordered: false, type: role.tagType },
    { default: () => role.label },
  )
}

const columns = [
  {
    title: '策略名',
    key: 'strategyName',
    minWidth: 180,
    render(row) {
      return `${row.strategyName}（${row.strategyId}）`
    },
  },
  {
    title: '角色',
    key: 'role',
    width: 120,
    render(row) {
      return renderRoleTag(roleForStrategy(row.strategyId))
    },
  },
  {
    title: '结论',
    key: 'verdict',
    width: 110,
    render(row) {
      return row.verdict
    },
  },
  {
    title: '倾向',
    key: 'bias',
    width: 90,
  },
  {
    title: '理由',
    key: 'reason',
    minWidth: 240,
  },
  {
    title: '数据日',
    key: 'dataDay',
    width: 120,
    render(row) {
      return row.dataDay || '—'
    },
  },
]

watch(queryText, () => {
  picks.value = []
  pickedKey.value = null
})

onMounted(async () => {
  await loadSignalSettingsFromBackend()
  let saved = []
  try {
    const res = await GetStockStrategyList({ page: 1, pageSize: 100, name: '', queryType: '' })
    saved = Array.isArray(res?.data) ? res.data : []
  } catch {
    saved = []
  }
  catalog.value = buildStrategyCatalog({
    settings: signalSettingsState.value,
    stockStrategies: saved,
  })
  selectedIds.value = defaultSelectedStrategyIds(catalog.value)
  catalogReady.value = true
})

function activePresetName() {
  const current = presetEngines.value.find((item) => item.strategyId === `preset:${signalSettingsState.value?.activeScreenStrategyId}`)
  return current?.name || presetEngines.value[0]?.name || '默认参数预设'
}

async function loadIndexMa20() {
  try {
    const raw = await GetStockEastMoneyKLine('000001.SH', '上证指数', '101', 120)
    const bars = klineRowsToScanBars(raw)
    if (!bars) return null
    const closeByDay = new Map()
    bars.dayKeys.forEach((day, index) => {
      const key = normalizeDayKey(day)
      if (key) closeByDay.set(key, bars.closes[index])
    })
    return buildIndexMa20ByDay(closeByDay, 20)
  } catch {
    return null
  }
}

async function loadDailyBars(code, name) {
  const barCount = resolveScanKlineBarCount()
  for (const emCode of eastMoneyCodeVariants(code)) {
    try {
      const raw = await GetStockEastMoneyKLine(emCode, name || '', '101', barCount)
      const bars = klineRowsToScanBars(raw)
      if (bars && bars.closes.length) return bars
    } catch {
      /* 换一个代码写法再试 */
    }
  }
  return null
}

function selectedEngines() {
  const picked = new Set(selectedIds.value)
  return catalog.value.filter((item) => picked.has(item.strategyId))
}

let compareSeq = 0

async function runCompare(identity) {
  const seq = ++compareSeq
  const engines = selectedEngines()
  if (!engines.length) {
    message.warning('请至少勾选一个策略')
    return
  }
  loading.value = true
  ran.value = true
  subjectLabel.value = identity.name ? `${identity.name}（${identity.code}）` : `${identity.code}（本地库无名称，仅按代码对照）`
  try {
    if (!isScannableWatchlistCode(identity.code)) {
      if (seq !== compareSeq) return
      rows.value = evaluateObservationCompare({
        bars: null,
        engines,
        activeSignalOptions: getSignalOptions(),
      }).map((row) =>
        row.verdict === '数据不足' ? { ...row, reason: '仅支持沪深京日 K 对照' } : row,
      )
      return
    }
    const [daily, indexMa20ByDay] = await Promise.all([
      loadDailyBars(identity.code, identity.name),
      loadIndexMa20(),
    ])
    if (seq !== compareSeq) return
    let bars = daily
    if (bars) {
      const lastIdx = resolveSignalLastBarIndex(bars.dayKeys)
      bars = sliceBarsToIndex({ ...bars, indexMa20ByDay }, lastIdx)
    }
    rows.value = evaluateObservationCompare({
      bars,
      engines,
      activeSignalOptions: getSignalOptions(),
    })
  } catch (error) {
    if (seq !== compareSeq) return
    rows.value = []
    message.error(error?.message || '对照失败')
  } finally {
    if (seq === compareSeq) loading.value = false
  }
}

function rerunEmbedded() {
  if (!embedded.value) return
  runCompare({ code: embeddedCode.value, name: embeddedName.value })
}

watch(
  () => [embeddedCode.value, embeddedName.value, catalogReady.value],
  () => {
    if (!catalogReady.value || !embedded.value) return
    runCompare({ code: embeddedCode.value, name: embeddedName.value })
  },
)

async function startCompare() {
  const text = queryText.value.trim()
  if (!text) {
    message.warning('请输入股票代码或名称')
    return
  }
  if (picks.value.length && pickedKey.value) {
    const chosen = picks.value.find((item) => (item.code || item.label) === pickedKey.value)
    if (!chosen?.code) {
      message.warning('请先从重名结果里选择一只股票')
      return
    }
    picks.value = []
    pickedKey.value = null
    await runCompare({ code: chosen.code, name: chosen.name || '' })
    return
  }
  loading.value = true
  try {
    let list = []
    try {
      list = await GetStockList(text)
    } catch {
      list = []
    }
    const identity = resolveStockIdentity(text, Array.isArray(list) ? list : [])
    if (identity.status === 'missing') {
      message.warning('本地库没有这只股票，请改用 6 位代码')
      return
    }
    if (identity.status === 'ambiguous') {
      picks.value = identity.candidates
      pickedKey.value = null
      rows.value = []
      subjectLabel.value = ''
      message.warning('名称对应多只股票，请先选择')
      return
    }
    picks.value = []
    pickedKey.value = null
    await runCompare({ code: identity.code, name: identity.name || '' })
  } finally {
    loading.value = false
  }
}

function onPick(value) {
  pickedKey.value = value
}
</script>

<template>
  <div class="compare-panel">
    <div class="compare-title">多策略对照</div>
    <n-text depth="3" class="hint">
      {{ embedded ? '按当前股票' : '输入一只股票' }}，把已接入的日 K 观察策略并排看读数和理由。内置引擎使用当前参数预设「{{ activePresetName() }}」。每条预设只写自己的规则结论；尚未独立求值的预设标为未实现独立求值，不会照搬其他策略的命中理由。倾向只表示这行读数的多空观感，不是委托方向。
    </n-text>

    <div v-if="!embedded" class="toolbar">
      <n-input
        v-model:value="queryText"
        placeholder="代码或名称，如 600519 / 贵州茅台"
        style="width: 280px"
        @keyup.enter="startCompare"
      />
      <n-button type="primary" :loading="loading" @click="startCompare">开始对照</n-button>
    </div>
    <div v-else class="toolbar">
      <n-text>{{ embeddedName ? `${embeddedName}（${embeddedCode}）` : embeddedCode }}</n-text>
      <n-button type="primary" :loading="loading" @click="rerunEmbedded">重新对照</n-button>
    </div>

    <div v-if="picks.length" class="pick-row">
      <n-text>名称不唯一，请选择：</n-text>
      <n-select
        :value="pickedKey"
        :options="pickOptions"
        placeholder="选择股票"
        style="width: 280px"
        @update:value="onPick"
      />
      <n-button type="primary" secondary :loading="loading" @click="startCompare">用所选股票对照</n-button>
    </div>

    <n-checkbox-group v-model:value="selectedIds" class="strategy-groups">
      <div class="group">
        <div class="group-title">日 K 扫描引擎</div>
        <n-checkbox v-for="item in wiredEngines" :key="item.strategyId" :value="item.strategyId">
          <span class="strategy-check">
            {{ item.name }}
            <n-tag size="small" :bordered="false" :type="roleForStrategy(item.strategyId).tagType">
              {{ roleForStrategy(item.strategyId).label }}
            </n-tag>
          </span>
        </n-checkbox>
      </div>
      <div v-if="presetEngines.length" class="group">
        <div class="group-title">参数预设</div>
        <n-checkbox v-for="item in presetEngines" :key="item.strategyId" :value="item.strategyId">
          <span class="strategy-check">
            {{ item.name }}
            <n-tag size="small" :bordered="false" :type="roleForStrategy(item.strategyId).tagType">
              {{ roleForStrategy(item.strategyId).label }}
            </n-tag>
          </span>
        </n-checkbox>
      </div>
      <div v-if="unwiredEngines.length" class="group">
        <div class="group-title">选股策略包</div>
        <n-checkbox v-for="item in unwiredEngines" :key="item.strategyId" :value="item.strategyId">
          <span class="strategy-check">
            {{ item.name }}（未接入）
            <n-tag size="small" :bordered="false" :type="roleForStrategy(item.strategyId).tagType">
              {{ roleForStrategy(item.strategyId).label }}
            </n-tag>
          </span>
        </n-checkbox>
      </div>
    </n-checkbox-group>

    <div v-if="subjectLabel" class="subject">对照标的：{{ subjectLabel }}</div>

    <n-data-table
      v-if="ran"
      class="result-table"
      :columns="columns"
      :data="rows"
      :loading="loading"
      :bordered="false"
      size="small"
      :pagination="false"
    />

    <div class="footer">
      <div>{{ COMPARE_FOOTER_TEXT }}</div>
      <div>{{ STRATEGY_ROLE_FOOTER }}</div>
    </div>
  </div>
</template>

<style scoped>
.compare-panel {
  height: 100%;
  overflow: auto;
  padding: 8px 4px 28px;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.compare-title {
  font-size: 16px;
  font-weight: 600;
}
.hint {
  font-size: 13px;
  line-height: 1.5;
}
.toolbar,
.pick-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}
.strategy-groups {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 14px;
  align-items: center;
}
.group-title {
  width: 100%;
  font-size: 13px;
  opacity: 0.75;
}
.strategy-check {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.subject {
  font-size: 13px;
}
.result-table {
  flex: 1;
}
.footer {
  position: sticky;
  bottom: 0;
  padding: 8px 0;
  font-size: 12px;
  opacity: 0.8;
  background: var(--n-color, transparent);
}
</style>
