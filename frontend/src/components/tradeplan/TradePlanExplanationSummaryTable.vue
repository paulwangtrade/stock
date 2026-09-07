<script setup>
/**
 * Phase16.15-A / 16.24 — Level 1 解释摘要表（投研表格风格）.
 * Phase16.19-B1: StockLink → 父级 StockKlineModal
 * Phase16.24: 状态列 + priceDisplay/statusDisplay +「查看解释」
 */
import { computed, h } from 'vue'
import { NButton, NDataTable, NEmpty, NTag, NText, NTooltip } from 'naive-ui'
import { toExplanationSummaryRow } from '../../utils/strategyExplanationDisplay.js'
import { formatFieldTooltip } from '../../utils/statusDisplay.js'
import { priceColumnTitle, PRICE_KIND } from '../../utils/priceDisplay.js'
import { adaptTradePlanItem } from '../../utils/stockDisplayAdapters.js'
import StockLink from '../StockLink.vue'

const props = defineProps({
  /** Array of { itemId, code, name, explanation, planItemStatus?, limitPrice? } */
  items: { type: Array, default: () => [] },
})

const emit = defineEmits(['open-detail', 'open-stock'])

const summaryRows = computed(() =>
  props.items.map((entry) => toExplanationSummaryRow(entry)).filter(Boolean),
)

function riskTagType(code) {
  if (!code) return 'default'
  const c = String(code).toLowerCase()
  if (c.includes('high') || c.includes('danger')) return 'error'
  if (c.includes('mid') || c.includes('warn')) return 'warning'
  return 'success'
}

function tipTitle(label, tip) {
  return () =>
    h(
      NTooltip,
      { trigger: 'hover' },
      {
        trigger: () => h('span', { class: 'col-tip' }, label),
        default: () => tip,
      },
    )
}

const columns = [
  {
    title: '股票',
    key: 'code',
    width: 150,
    ellipsis: { tooltip: true },
    render(row) {
      const code = row.code && row.code !== '—' ? row.code : ''
      const name = row.name && row.name !== '—' ? row.name : ''
      if (!code && !name) return h(NText, { depth: 3 }, { default: () => '—' })
      return h(StockLink, {
        model: adaptTradePlanItem({ stock_code: code, stock_name: name }),
        onOpen: (model) => emit('open-stock', model),
      })
    },
  },
  {
    title: tipTitle('信号', formatFieldTooltip('signal')),
    key: 'signalTag',
    width: 72,
    render(row) {
      return row.signalTag
        ? h(NTag, { size: 'small', bordered: false, type: 'info' }, { default: () => row.signalTag })
        : h(NText, { depth: 3 }, { default: () => '—' })
    },
  },
  {
    title: tipTitle('评分', formatFieldTooltip('score')),
    key: 'signalScoreLabel',
    width: 64,
    render(row) {
      return h(NText, null, { default: () => row.signalScoreLabel || '—' })
    },
  },
  {
    title: tipTitle('风险', formatFieldTooltip('risk')),
    key: 'riskCode',
    width: 100,
    ellipsis: { tooltip: true },
    render(row) {
      if (!row.riskCode && !row.riskMessage) return h(NText, { depth: 3 }, { default: () => '—' })
      const label = row.riskCode || row.riskMessage
      const tip = [row.riskCode, row.riskMessage].filter(Boolean).join(': ')
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () =>
            h(
              NTag,
              { size: 'small', bordered: false, type: riskTagType(row.riskCode) },
              { default: () => label },
            ),
          default: () => tip,
        },
      )
    },
  },
  {
    title: '入场策略',
    key: 'strategyName',
    minWidth: 100,
    ellipsis: { tooltip: true },
    render(row) {
      const v = row.strategyName && row.strategyName !== '—' ? row.strategyName : '—'
      return h(NText, { depth: v === '—' ? 3 : undefined }, { default: () => v })
    },
  },
  {
    title: tipTitle(priceColumnTitle(PRICE_KIND.ref), formatFieldTooltip('ref')),
    key: 'refPriceLabel',
    width: 100,
    render(row) {
      if (!row.refPrice || row.refPriceLabel === '—') {
        return h(NText, { depth: 3 }, { default: () => '—' })
      }
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () => h(NText, null, { default: () => row.refPriceLabel }),
          default: () => row.refPriceTooltip || formatFieldTooltip('ref'),
        },
      )
    },
  },
  {
    title: '状态',
    key: 'execStatusLabel',
    width: 110,
    render(row) {
      const label = row.execStatusLabel || '—'
      return h(
        NTooltip,
        { trigger: 'hover' },
        {
          trigger: () =>
            h(
              NTag,
              {
                size: 'small',
                bordered: false,
                type: row.execStatusType || 'default',
              },
              { default: () => label },
            ),
          default: () =>
            row.execStatusTooltip ||
            row.explainStatusTooltip ||
            (row.isMissingExplain
              ? '该计划创建时未保存完整策略解释，无法恢复历史决策依据。'
              : label),
        },
      )
    },
  },
  {
    title: '操作',
    key: '_action',
    width: 96,
    render(row) {
      return h(
        NButton,
        {
          size: 'tiny',
          secondary: true,
          onClick: () => emit('open-detail', props.items.find((e) => e.itemId === row.itemId)),
        },
        { default: () => '查看解释' },
      )
    },
  },
]
</script>

<template>
  <div class="explanation-summary-table">
    <n-text depth="3" class="summary-hint">
      策略解释摘要（表格）· 点击「查看解释」打开详情 · 不会自动下单
    </n-text>
    <n-data-table
      v-if="summaryRows.length"
      size="small"
      :columns="columns"
      :data="summaryRows"
      :row-key="(row) => row.itemId"
      :pagination="false"
      :single-line="true"
    />
    <n-empty
      v-else
      size="small"
      description="点击主表「查看解释」加载策略解释摘要（按需懒加载，不会一次拉全表）"
    />
  </div>
</template>

<style scoped>
.explanation-summary-table {
  width: 100%;
}
.summary-hint {
  display: block;
  font-size: 12px;
  margin-bottom: 8px;
}
.col-tip {
  cursor: help;
  border-bottom: 1px dotted var(--n-text-color-3, #999);
}
</style>
