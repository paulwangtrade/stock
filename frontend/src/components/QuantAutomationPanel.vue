<script setup>
import { computed } from 'vue'
import {
  NButton,
  NCollapse,
  NCollapseItem,
  NGrid,
  NGi,
  NInputNumber,
  NSpace,
  NSwitch,
  NText,
} from 'naive-ui'
import {
  DEFAULT_QUANT_AUTOMATION,
  QUANT_AUTOMATION_FIELDS,
  mergeQuantAutomation,
} from '../utils/quantAutomationSettings'
import { mergeSignalSettings } from '../utils/signalSettings'

const model = defineModel({ type: Object, required: true })

const automation = computed({
  get: () => mergeQuantAutomation(model.value?.automation),
  set: (v) => {
    model.value = mergeSignalSettings({ ...model.value, automation: v })
  },
})

const alertLabels = {
  signalNew: '新买点信号',
  zoneTouch: '区间触达',
  zoneLeave: '离开区间',
  sellSignal: '止/减风控',
  sellDraft: '止/减写交易草稿',
  checklistReady: '清单就绪',
  positionPlan: '仓位建议',
}

function updateField(key, val) {
  automation.value = { ...automation.value, [key]: val }
}

function updateAlert(key, val) {
  automation.value = {
    ...automation.value,
    alerts: { ...automation.value.alerts, [key]: val },
  }
}

function updateConfidence(tag, val) {
  automation.value = {
    ...automation.value,
    tagConfidence: { ...automation.value.tagConfidence, [tag]: val },
  }
}

function restoreDefault() {
  automation.value = JSON.parse(JSON.stringify(DEFAULT_QUANT_AUTOMATION))
}
</script>

<template>
  <div class="quant-auto-panel">
    <div class="quant-auto-panel__head">
      <n-text depth="3">
        等级口径：1级最防守、5级最进攻 · 固定比例风险仓位 · 信号/区间/清单自动推送。修改后请保存设置。
      </n-text>
      <n-button size="small" tertiary type="warning" @click="restoreDefault">恢复默认</n-button>
    </div>

    <n-collapse :default-expanded-names="[]">
      <n-collapse-item title="总开关与账户风控" name="core">
        <n-grid :cols="24" :x-gap="16" :y-gap="12">
          <n-gi :span="12">
            <div class="qa-field qa-field--bool">
              <span>启用量化自动化</span>
              <n-switch :value="automation.enabled" @update:value="(v) => updateField('enabled', v)" />
            </div>
          </n-gi>
          <n-gi v-for="field in QUANT_AUTOMATION_FIELDS.filter((f) => f.key !== 'enabled')" :key="field.key" :span="8">
            <div v-if="field.type === 'bool'" class="qa-field qa-field--bool">
              <span>{{ field.label }}</span>
              <n-switch :value="!!automation[field.key]" @update:value="(v) => updateField(field.key, v)" />
            </div>
            <div v-else class="qa-field">
              <span class="qa-label">{{ field.label }}</span>
              <n-input-number
                size="small"
                class="qa-input"
                :value="field.scale ? automation[field.key] * field.scale : automation[field.key]"
                :min="field.scale ? field.min * field.scale : field.min"
                :max="field.scale ? field.max * field.scale : field.max"
                :step="field.scale ? field.step * field.scale : field.step"
                @update:value="(v) => updateField(field.key, field.scale ? v / field.scale : v)"
              >
                <template v-if="field.suffix" #suffix>{{ field.suffix }}</template>
              </n-input-number>
            </div>
          </n-gi>
        </n-grid>
      </n-collapse-item>

      <n-collapse-item title="信号置信度 → 仓位系数" name="conf">
        <n-grid :cols="24" :x-gap="16" :y-gap="12">
          <n-gi v-for="(val, tag) in automation.tagConfidence" :key="tag" :span="6">
            <div class="qa-field">
              <span class="qa-label">{{ tag }}</span>
              <n-input-number
                size="small"
                class="qa-input"
                :value="val"
                :min="0.2"
                :max="1.2"
                :step="0.05"
                @update:value="(v) => updateConfidence(tag, v)"
              />
            </div>
          </n-gi>
        </n-grid>
      </n-collapse-item>

      <n-collapse-item title="自动推送" name="alerts">
        <n-grid :cols="24" :x-gap="16" :y-gap="8">
          <n-gi v-for="(label, key) in alertLabels" :key="key" :span="8">
            <div class="qa-field qa-field--bool">
              <span>{{ label }}</span>
              <n-switch :value="automation.alerts[key]" @update:value="(v) => updateAlert(key, v)" />
            </div>
          </n-gi>
        </n-grid>
      </n-collapse-item>
    </n-collapse>
  </div>
</template>

<style scoped>
.quant-auto-panel__head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  font-size: 13px;
}

.qa-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-height: 52px;
}

.qa-field--bool {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  min-height: 36px;
  padding: 8px 12px;
  border-radius: 6px;
  background: var(--n-action-color);
}

.qa-label {
  font-size: 12px;
  color: var(--n-text-color-2);
}

.qa-input {
  width: 100%;
}
</style>
