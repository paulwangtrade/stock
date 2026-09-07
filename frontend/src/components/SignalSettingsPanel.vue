<script setup>
import { ref } from 'vue'
import {
  NButton,
  NCollapse,
  NCollapseItem,
  NInputNumber,
  NSpace,
  NSwitch,
  NTag,
  NText,
} from 'naive-ui'
import { getSignalTagColor } from '../utils/signalBuyGuide'
import {
  cloneDefaultSignalSettings,
  cloneDefaultSignalStrategySettings,
  mergeSignalSettings,
  restoreSignalSection,
  SIGNAL_PARAM_SECTIONS,
} from '../utils/signalSettings'

const model = defineModel({ type: Object, required: true })
const props = defineProps({
  showDisplay: { type: Boolean, default: true },
  showSignalRules: { type: Boolean, default: true },
})
const expanded = ref([])

const SECTION_META = {
  display: { tag: '展示', desc: '自选卡片等界面表现' },
  common: { tag: '通用', desc: '' },
  sell: { tag: '止/减', desc: '减 = 主卖；止 = 有浮盈时的辅助止盈' },
  strong: { tag: '强', desc: '强化买：放量、指数环境、确认站稳' },
  trend: { tag: '趋', desc: '上升趋势中回踩均线；可选次日收阳确认' },
  reversal: { tag: '转', desc: '收盘上穿 MA60 等转势启动' },
  breakout: { tag: '突', desc: '箱体整理后放量突破' },
  rebound: { tag: '弹', desc: '止/减后收复 MA20 的反弹确认' },
  buy: { tag: '买', desc: '出冰点后的基础买点过滤' },
}

function sectionMeta(key) {
  return SECTION_META[key] || { tag: key, desc: '' }
}

function sectionTagStyle(key) {
  const tag = sectionMeta(key).tag
  const colorKey = tag === '通用' ? '冰' : tag === '止/减' ? '减' : tag
  const c = getSignalTagColor(colorKey)
  return {
    color: c.textColor,
    backgroundColor: c.color,
    borderColor: c.borderColor,
  }
}

function roundToStep(value, step) {
  if (!Number.isFinite(value) || !Number.isFinite(step) || step <= 0) return value
  return Math.round(value / step) * step
}

function fieldUiPrecision(field) {
  if (field.uiPrecision != null) return field.uiPrecision
  if (field.scale && field.suffix === '%') {
    const uiStep = field.step * field.scale
    if (uiStep >= 1) return 0
    if (uiStep >= 0.1) return 1
    return 2
  }
  return undefined
}

function uiValue(sectionKey, field, stored) {
  if (stored == null) return null
  if (field.scale) {
    const raw = stored * field.scale
    const p = fieldUiPrecision(field)
    return p != null ? roundToStep(raw, field.step * field.scale) : raw
  }
  return stored
}

function onFieldUpdate(sectionKey, field, val) {
  const next = mergeSignalSettings(model.value)
  let stored = val
  if (field.scale && val != null) {
    stored = val / field.scale
    if (field.step) stored = roundToStep(stored, field.step)
  }
  next[sectionKey][field.key] = stored
  model.value = next
}

function restoreSection(sectionKey) {
  model.value = restoreSignalSection(model.value, sectionKey)
}

function restoreAll() {
  model.value = props.showSignalRules ? cloneDefaultSignalStrategySettings() : cloneDefaultSignalSettings()
}

function onDisplayUpdate(key, val) {
  const next = mergeSignalSettings(model.value)
  next.display = { ...next.display, [key]: val }
  model.value = next
}

function sectionFieldGroups(section) {
  if (!section.groups?.length) {
    return [{ id: '_all', title: '', desc: '', fields: section.fields }]
  }
  const grouped = section.groups
    .map((g) => ({
      ...g,
      fields: section.fields.filter((f) => f.group === g.id),
    }))
    .filter((g) => g.fields.length)
  const used = new Set(grouped.flatMap((g) => g.fields.map((f) => f.key)))
  const rest = section.fields.filter((f) => !used.has(f.key))
  if (rest.length) grouped.push({ id: '_rest', title: '其他', desc: '', fields: rest })
  return grouped
}

/** 每组字段铺满整行；字段少时均分，多时自动换行 */
function gridStyle(group) {
  const n = group.fields?.length || 1
  if (group.id === 'protect') {
    return { gridTemplateColumns: `repeat(${n}, minmax(72px, 1fr))` }
  }
  if (n <= 6) {
    return { gridTemplateColumns: `repeat(${n}, minmax(0, 1fr))` }
  }
  return {}
}
</script>

<template>
  <div class="signal-settings">
    <div class="signal-settings__head">
      <n-text depth="3" class="signal-settings__hint">
        <template v-if="showSignalRules">
          调整 K 线信号识别规则；列表主标签优先级：止/减 &gt; 强 &gt; 趋 &gt; 转 &gt; 突 &gt; 弹 &gt; 买。修改后请点击页面底部「保存设置」生效。
          <br />
          参数调整只影响未来扫描结果，不会修改历史交易计划和已保存策略解释。
          <br />
          Beta 提示：建议不要频繁修改默认参数；试用请优先使用默认配置。
        </template>
        <template v-else>
          调整自选卡片等全局界面表现。修改后请点击页面底部「保存设置」生效。
        </template>
      </n-text>
      <n-button size="small" tertiary type="warning" @click="restoreAll">全部恢复默认</n-button>
    </div>

    <n-collapse v-model:expanded-names="expanded" class="signal-settings__collapse">
      <n-collapse-item v-if="showDisplay" name="display">
        <template #header>
          <n-space align="center" :size="8" class="signal-section-header">
            <n-tag size="small" :bordered="true" :style="sectionTagStyle('display')">
              {{ sectionMeta('display').tag }}
            </n-tag>
            <span class="signal-section-title">自选卡片</span>
          </n-space>
        </template>
        <template #header-extra>
          <n-button size="tiny" quaternary @click.stop="restoreSection('display')">恢复默认</n-button>
        </template>
        <p class="signal-section-desc">{{ sectionMeta('display').desc }}</p>
        <div class="signal-param-grid" :style="{ gridTemplateColumns: 'repeat(4, minmax(0, 1fr))' }">
          <div class="signal-param-cell signal-param-cell--bool">
            <span class="signal-param-label">信号标签色铺卡片底色</span>
            <n-switch
              :value="model.display?.watchlistCardSignalTint"
              @update:value="(v) => onDisplayUpdate('watchlistCardSignalTint', v)"
            />
          </div>
          <div class="signal-param-cell signal-param-cell--bool">
            <span class="signal-param-label">有操作提示时弹窗</span>
            <n-switch
              :value="model.display?.watchlistActionPopup !== false"
              @update:value="(v) => onDisplayUpdate('watchlistActionPopup', v)"
            />
          </div>
          <div class="signal-param-cell signal-param-cell--bool">
            <span class="signal-param-label">关注时按日期分组</span>
            <n-switch
              :value="model.display?.followDateGroupEnabled !== false"
              @update:value="(v) => onDisplayUpdate('followDateGroupEnabled', v)"
            />
          </div>
          <div class="signal-param-cell">
            <span class="signal-param-label">日期分组保留天数</span>
            <n-input-number
              :value="model.display?.dateGroupRetainDays ?? 30"
              :min="7"
              :max="180"
              :step="1"
              size="small"
              class="signal-param-input"
              :disabled="model.display?.followDateGroupEnabled === false"
              @update:value="(v) => onDisplayUpdate('dateGroupRetainDays', v)"
            />
          </div>
          <div class="signal-param-cell">
            <span class="signal-param-label">买点回溯</span>
            <n-input-number
              :value="model.display?.recentBuyDays ?? 15"
              :min="1"
              :max="30"
              :step="1"
              size="small"
              class="signal-param-input"
              @update:value="(v) => onDisplayUpdate('recentBuyDays', v)"
            >
              <template #suffix>日</template>
            </n-input-number>
          </div>
          <div class="signal-param-cell">
            <span class="signal-param-label">止/减回溯</span>
            <n-input-number
              :value="model.display?.recentSellDays ?? 5"
              :min="1"
              :max="15"
              :step="1"
              size="small"
              class="signal-param-input"
              @update:value="(v) => onDisplayUpdate('recentSellDays', v)"
            >
              <template #suffix>日</template>
            </n-input-number>
          </div>
          <div class="signal-param-cell">
            <span class="signal-param-label">自选数量上限</span>
            <n-input-number
              :value="model.display?.maxFollowCount ?? 100"
              :min="1"
              :max="500"
              :step="1"
              size="small"
              class="signal-param-input"
              @update:value="(v) => onDisplayUpdate('maxFollowCount', v)"
            />
          </div>
        </div>
      </n-collapse-item>

      <n-collapse-item
        v-if="showSignalRules"
        v-for="section in SIGNAL_PARAM_SECTIONS"
        :key="section.key"
        :name="section.key"
      >
        <template #header>
          <n-space align="center" :size="8" class="signal-section-header">
            <n-tag size="small" :bordered="true" :style="sectionTagStyle(section.key)">
              {{ sectionMeta(section.key).tag }}
            </n-tag>
            <span class="signal-section-title">{{ section.title }}</span>
          </n-space>
        </template>
        <template #header-extra>
          <n-button size="tiny" quaternary @click.stop="restoreSection(section.key)">
            恢复默认
          </n-button>
        </template>

        <p v-if="sectionMeta(section.key).desc" class="signal-section-desc">
          {{ sectionMeta(section.key).desc }}
        </p>

        <div
          v-for="group in sectionFieldGroups(section)"
          :key="group.id"
          class="signal-param-group"
          :class="{ 'signal-param-group--flat': !group.title }"
        >
          <div v-if="group.title" class="signal-param-group__head">
            <span class="signal-param-group__title">{{ group.title }}</span>
            <span v-if="group.desc" class="signal-param-group__desc">{{ group.desc }}</span>
          </div>

          <div
            class="signal-param-grid"
            :style="gridStyle(group)"
          >
            <div
              v-for="field in group.fields"
              :key="field.key"
              class="signal-param-cell"
              :class="{ 'signal-param-cell--bool': field.type === 'bool' }"
            >
              <span class="signal-param-label">{{ field.label }}</span>
              <n-switch
                v-if="field.type === 'bool'"
                :value="model[section.key][field.key]"
                @update:value="(v) => onFieldUpdate(section.key, field, v)"
              />
              <n-input-number
                v-else
                size="small"
                class="signal-param-input"
                :show-button="field.type === 'int' || !field.scale"
                :value="uiValue(section.key, field, model[section.key][field.key])"
                :min="field.scale ? field.min * field.scale : field.min"
                :max="field.scale ? field.max * field.scale : field.max"
                :step="field.scale ? field.step * field.scale : field.step"
                :precision="fieldUiPrecision(field)"
                @update:value="(v) => onFieldUpdate(section.key, field, v)"
              >
                <template v-if="field.suffix" #suffix>{{ field.suffix }}</template>
              </n-input-number>
            </div>
          </div>
        </div>
      </n-collapse-item>
    </n-collapse>
  </div>
</template>

<style scoped>
.signal-settings {
  width: 100%;
}

.signal-settings__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.signal-settings__hint {
  flex: 1;
  font-size: 13px;
  line-height: 1.5;
  margin: 0;
}

.signal-settings__collapse :deep(.n-collapse-item__header) {
  padding-top: 10px;
  padding-bottom: 10px;
}

.signal-settings__collapse :deep(.n-collapse-item__content-inner) {
  padding-top: 4px;
  padding-bottom: 14px;
}

.signal-section-header {
  min-width: 0;
}

.signal-section-title {
  font-size: 14px;
  font-weight: 500;
}

.signal-section-desc {
  margin: 0 0 10px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--n-text-color-3);
}

.signal-param-group {
  margin-bottom: 10px;
  padding: 8px 10px 6px;
  border-radius: 8px;
  border: 1px solid var(--n-border-color);
  background: var(--n-color-modal);
}

.signal-param-group:last-child {
  margin-bottom: 0;
}

.signal-param-group--flat {
  padding: 0;
  border: none;
  background: transparent;
}

.signal-param-group__head {
  margin-bottom: 6px;
}

.signal-param-group__title {
  display: block;
  font-size: 13px;
  font-weight: 600;
  line-height: 1.4;
  color: var(--n-text-color-1);
}

.signal-param-group__desc {
  display: block;
  margin-top: 2px;
  font-size: 12px;
  line-height: 1.45;
  color: var(--n-text-color-3);
}

.signal-param-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(156px, 1fr));
  gap: 12px 20px;
  align-items: start;
  width: 100%;
}

.signal-param-cell {
  display: flex;
  flex-direction: column;
  gap: 5px;
  min-width: 0;
}

.signal-param-cell--bool {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  border-radius: 6px;
  background: var(--n-action-color);
}

.signal-param-cell--bool .signal-param-label {
  flex: 1;
  min-width: 0;
}

.signal-param-label {
  font-size: 12px;
  line-height: 1.35;
  color: var(--n-text-color-2);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.signal-param-input {
  width: 100%;
}

.signal-param-input :deep(.n-input-wrapper) {
  padding-left: 8px;
  padding-right: 4px;
}

@media (max-width: 900px) {
  .signal-settings__head {
    flex-direction: column;
    align-items: stretch;
  }

  .signal-param-grid {
    grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)) !important;
  }
}
</style>
