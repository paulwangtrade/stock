<script setup>
/**
 * Phase13-D/H: shared capability panel with Locked experience + Upgrade CTA.
 */
import { computed, onMounted, ref, watch } from 'vue'
import { NButton, NSpace, NTag, NText, NSelect } from 'naive-ui'
import {
  getProductFeatureGate,
  gateAllowed,
  readProductTier,
  writeProductTier,
  recordProductUsage,
  postUpgradeDemo,
} from '../api/productCapabilities'

const props = defineProps({
  title: { type: String, required: true },
  feature: { type: String, required: true },
  usageOpenedKey: { type: String, default: '' },
  usageViewedKey: { type: String, default: '' },
  scene: { type: String, default: '' },
  /** Feature description for Locked state */
  description: { type: String, default: '' },
  /** Value proposition for Locked state */
  valueHint: { type: String, default: '' },
  showTierSwitch: { type: Boolean, default: true },
})

const emit = defineEmits(['open', 'tier-change', 'upgrade'])

const tier = ref(readProductTier())
const gate = ref(null)
const gateLoading = ref(false)
const openLoading = ref(false)
const upgradeLoading = ref(false)

const allowed = computed(() => gateAllowed(gate.value, props.feature))
const gateReason = computed(() => {
  const hit = (gate.value?.features || []).find((f) => f.feature === props.feature)
  return hit?.reason || ''
})

const defaultDescription = computed(() => {
  if (props.description) return props.description
  const map = {
    AdvancedObservation: '策略解释：用只读事实说明计划条目为何入选。',
    AdvancedRisk: '高级风险报告：组合 / 持仓 / 执行维度的只读风险观察。',
    AIAnalysis: 'AI 分析：本地 Context 归纳与 Mock 解释，不上传数据。',
    Backtest: '回测：研究场景商业入口（预留）。',
  }
  return map[props.feature] || '高级产品能力，受 FeatureGate 控制。'
})

const defaultValueHint = computed(() => {
  if (props.valueHint) return props.valueHint
  return '帮助理解「发生了什么 / 为何如此」，不替代交易决策，不生成买卖指令。'
})

const tierOptions = [
  { label: 'Free（基础）', value: 'free' },
  { label: 'Pro（高级入口）', value: 'pro' },
]

async function refreshGate() {
  gateLoading.value = true
  try {
    gate.value = await getProductFeatureGate(tier.value)
  } catch {
    gate.value = null
  } finally {
    gateLoading.value = false
  }
}

async function onTierChange(v) {
  const next = v === 'pro' ? 'pro' : 'free'
  tier.value = next
  writeProductTier(next)
  emit('tier-change', next)
  await refreshGate()
}

async function onUpgrade() {
  upgradeLoading.value = true
  try {
    await postUpgradeDemo('pro').catch(() => null)
    writeProductTier('pro')
    tier.value = 'pro'
    emit('upgrade', { tier: 'pro' })
    emit('tier-change', 'pro')
    await refreshGate()
  } finally {
    upgradeLoading.value = false
  }
}

async function onOpen() {
  if (!allowed.value) return
  openLoading.value = true
  try {
    if (props.usageOpenedKey) {
      await recordProductUsage({
        feature: props.feature,
        event: 'opened',
        usageKey: props.usageOpenedKey,
        scene: props.scene,
        tier: tier.value,
      }).catch(() => null)
    }
    emit('open', { tier: tier.value, feature: props.feature })
    if (props.usageViewedKey) {
      await recordProductUsage({
        feature: props.feature,
        event: 'viewed',
        usageKey: props.usageViewedKey,
        scene: props.scene,
        tier: tier.value,
      }).catch(() => null)
    }
  } finally {
    openLoading.value = false
  }
}

onMounted(refreshGate)
watch(
  () => props.feature,
  () => {
    refreshGate()
  },
)

defineExpose({ tier, allowed, refreshGate })
</script>

<template>
  <div class="product-cap-panel" :class="{ locked: !allowed }">
    <n-space justify="space-between" align="center" :wrap="true" style="margin-bottom: 8px">
      <n-space align="center" :wrap="true">
        <n-text strong>{{ title }}</n-text>
        <n-tag size="small" :type="allowed ? 'success' : 'warning'" :bordered="false">
          {{ allowed ? 'Pro 可用' : 'Locked' }}
        </n-tag>
        <n-tag size="small" :bordered="false">{{ feature }}</n-tag>
      </n-space>
      <n-space align="center" :wrap="true">
        <n-select
          v-if="showTierSwitch"
          :value="tier"
          :options="tierOptions"
          size="small"
          style="width: 160px"
          :loading="gateLoading"
          @update:value="onTierChange"
        />
        <n-button
          v-if="allowed"
          size="small"
          type="primary"
          :loading="openLoading"
          @click="onOpen"
        >
          打开高级分析
        </n-button>
        <n-button
          v-else
          size="small"
          type="primary"
          :loading="upgradeLoading"
          @click="onUpgrade"
        >
          升级到 Pro（演示）
        </n-button>
      </n-space>
    </n-space>

    <template v-if="!allowed">
      <div class="locked-box">
        <n-text strong style="display: block; margin-bottom: 4px">功能说明</n-text>
        <n-text depth="3" style="display: block; font-size: 12px; margin-bottom: 8px">
          {{ defaultDescription }}
        </n-text>
        <n-text strong style="display: block; margin-bottom: 4px">价值说明</n-text>
        <n-text depth="3" style="display: block; font-size: 12px; margin-bottom: 8px">
          {{ defaultValueHint }}
        </n-text>
        <n-text strong style="display: block; margin-bottom: 4px">升级提示</n-text>
        <n-text depth="3" style="display: block; font-size: 12px">
          此功能属于 Pro。点击「升级到 Pro（演示）」切换本地档位即可体验（无支付 / 无账号）。
          <template v-if="gateReason && gateReason !== 'OK'">
            · Gate: {{ gateReason }}
          </template>
        </n-text>
      </div>
    </template>

    <slot :allowed="allowed" :tier="tier" />
  </div>
</template>

<style scoped>
.product-cap-panel {
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  padding: 12px 14px;
  margin: 10px 0 14px;
  background: var(--n-color);
}
.product-cap-panel.locked {
  border-style: dashed;
  border-color: rgba(240, 160, 32, 0.45);
}
.locked-box {
  padding: 10px 12px;
  margin-bottom: 8px;
  border-radius: 6px;
  background: rgba(240, 160, 32, 0.08);
}
</style>
