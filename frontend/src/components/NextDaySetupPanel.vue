<script setup>
import { computed } from 'vue'
import {
  NEXT_DAY_SETUP_DISCLAIMER,
  NEXT_DAY_SETUP_ENGINE_LABEL,
  NEXT_DAY_SETUP_TAG_SUPPORT,
  NEXT_DAY_SETUP_UNAVAILABLE,
  formatSetupDistance,
  formatSetupTrigger,
  priceModeLabel,
} from '../utils/nextDaySetupWatch.js'

const props = defineProps({
  setups: { type: Array, default: () => [] },
  asOfDate: { type: String, default: '' },
  /** 旧快照的 result 里没有 nextDaySetups 字段 */
  legacySnapshot: { type: Boolean, default: false },
})

const columns = [
  { title: '股票', key: 'name', width: 140, ellipsis: { tooltip: true } },
  { title: '规则', key: 'rule', width: 120 },
  { title: '状态', key: 'statusText', width: 140 },
  { title: '参考价', key: 'trigger', width: 110 },
  { title: '距 T 收盘', key: 'distance', width: 150 },
  { title: '条件缺口', key: 'gapText', ellipsis: { tooltip: true } },
]

const rows = computed(() =>
  (props.setups || []).map((row, index) => ({
    key: `${row.SECUCODE || ''}-${row.engine || ''}-${index}`,
    name: `${row.SECURITY_NAME_ABBR || '—'} ${row.SECUCODE || ''}`.trim(),
    rule: `${row.tag || '—'} · ${NEXT_DAY_SETUP_ENGINE_LABEL[row.engine] || row.engine || ''}`,
    statusText: row.statusText || '未确认 · 次日观察',
    trigger: formatSetupTrigger(row),
    distance: formatSetupDistance(row),
    gapText: row.gapText || NEXT_DAY_SETUP_UNAVAILABLE,
  })),
)

const emptyText = computed(() => {
  if (props.legacySnapshot) {
    return '这份收盘快照没有次日观察列表。观察项与已确认信号分开保存；重新生成盘后快照后才会出现。'
  }
  return 'T 收盘时没有处于一步之遥的突破前高或收回 MA20。'
})
</script>

<template>
  <n-card size="small" class="next-day-setup" :bordered="true">
    <template #header>
      <div class="next-day-setup__title">
        <span>次日观察 / setup</span>
        <n-tag size="small" :bordered="false" type="warning">未确认</n-tag>
        <n-text v-if="asOfDate" depth="3">观察日 {{ asOfDate }}（T 收盘）</n-text>
      </div>
    </template>
    <n-text class="next-day-setup__disclaimer">{{ NEXT_DAY_SETUP_DISCLAIMER }}</n-text>
    <n-text depth="3" class="next-day-setup__note">
      与已确认收盘信号分开。不是买卖指令，不会写入交易计划或委托。
    </n-text>
    <n-data-table
      v-if="rows.length"
      size="small"
      :columns="columns"
      :data="rows"
      :pagination="rows.length > 8 ? { pageSize: 8 } : false"
      :max-height="240"
    />
    <n-empty v-else size="small" :description="emptyText" />
    <div class="next-day-setup__support">
      <n-text depth="3">标签对照（本期只计算可给出参考价的「突」「弹」）</n-text>
      <ul>
        <li v-for="item in NEXT_DAY_SETUP_TAG_SUPPORT" :key="item.tag">
          <n-text>「{{ item.tag }}」{{ priceModeLabel(item.priceMode) }}{{ item.active ? '' : '（本期不按股输出）' }}。</n-text>
          <n-text depth="3">{{ item.note }}</n-text>
        </li>
      </ul>
    </div>
  </n-card>
</template>

<style scoped>
.next-day-setup {
  margin-top: 8px;
}
.next-day-setup__title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
}
.next-day-setup__disclaimer {
  display: block;
  color: #b45309;
  font-weight: 600;
  margin-bottom: 4px;
}
.next-day-setup__note {
  display: block;
  margin-bottom: 8px;
}
.next-day-setup__support {
  margin-top: 8px;
}
.next-day-setup__support ul {
  margin: 4px 0 0;
  padding-left: 18px;
}
.next-day-setup__support li {
  margin: 2px 0;
}
</style>
