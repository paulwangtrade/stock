<script setup>
/**
 * Phase13-D/H + V2 polish: Locked Pro capability panel + Upgrade CTA.
 * Frontend-only; does not change FeatureGate / UsageMetrics / trading chain.
 */
import { computed, h, onMounted, ref, watch } from 'vue'
import { NButton, NSpace, NTag, NText, NSelect, NIcon, NEmpty } from 'naive-ui'
import { LockClosedOutline, LockOpenOutline, SparklesOutline } from '@vicons/ionicons5'
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

function renderSparklesIcon() {
  return h(NIcon, { size: 16, component: SparklesOutline })
}

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
        <n-text strong class="panel-title">{{ title }}</n-text>
        <n-tag
          size="small"
          :type="allowed ? 'success' : 'warning'"
          :bordered="false"
          :class="allowed ? 'status-tag' : 'status-tag status-tag--locked'"
        >
          <n-space :size="4" align="center">
            <n-icon v-if="!allowed" :component="LockClosedOutline" :size="14" class="lock-icon" />
            <n-icon v-else :component="LockOpenOutline" :size="14" class="lock-icon lock-icon--open" />
            <span>{{ allowed ? 'Pro 可用' : 'Locked' }}</span>
          </n-space>
        </n-tag>
        <n-tag size="small" :bordered="false" class="feature-tag">{{ feature }}</n-tag>
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
          class="upgrade-cta"
          size="medium"
          type="warning"
          strong
          :loading="upgradeLoading"
          :render-icon="renderSparklesIcon"
          @click="onUpgrade"
        >
          升级到 Pro（演示）
        </n-button>
      </n-space>
    </n-space>

    <template v-if="!allowed">
      <div class="locked-box">
        <div class="locked-box__head">
          <n-icon :component="LockClosedOutline" :size="20" class="locked-box__icon" />
          <n-text strong class="locked-box__title">Pro 功能已锁定</n-text>
        </div>
        <n-text class="locked-box__label">功能说明</n-text>
        <n-text class="locked-box__body">
          {{ defaultDescription }}
        </n-text>
        <n-text class="locked-box__label">价值说明</n-text>
        <n-text class="locked-box__body">
          {{ defaultValueHint }}
        </n-text>
        <n-text class="locked-box__label">升级提示</n-text>
        <n-text class="locked-box__body locked-box__body--last">
          此功能属于 Pro。使用下方主按钮切换本地演示档位即可体验（无支付 / 无账号）。
          <template v-if="gateReason && gateReason !== 'OK'">
            · Gate: {{ gateReason }}
          </template>
        </n-text>
        <n-button
          class="upgrade-cta upgrade-cta--in-box"
          block
          type="warning"
          strong
          :loading="upgradeLoading"
          :render-icon="renderSparklesIcon"
          @click="onUpgrade"
        >
          升级到 Pro（演示）
        </n-button>
      </div>

      <div class="locked-empty">
        <n-empty size="small" description="内容已锁定，升级 Pro 后可查看">
          <template #icon>
            <n-icon :component="LockClosedOutline" :size="36" class="locked-empty__icon" />
          </template>
        </n-empty>
      </div>
    </template>

    <div v-show="allowed" class="panel-slot">
      <slot :allowed="allowed" :tier="tier" />
    </div>
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
  border-style: solid;
  border-width: 1px;
  border-color: #c47a12;
  background: linear-gradient(180deg, #fff8eb 0%, #fff3dc 100%);
}
.panel-title {
  color: #1f1f1f;
}
.status-tag--locked {
  font-weight: 600;
}
.lock-icon {
  color: #b45309;
  opacity: 1;
}
.lock-icon--open {
  color: #15803d;
}
.feature-tag {
  opacity: 0.9;
}

.locked-box {
  padding: 12px 14px;
  margin-bottom: 10px;
  border-radius: 8px;
  border: 1px solid rgba(180, 83, 9, 0.35);
  background: #fffaf0;
  color: #292524;
}
.locked-box__head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}
.locked-box__icon {
  color: #b45309;
  flex-shrink: 0;
}
.locked-box__title {
  color: #9a3412;
  font-size: 14px;
}
.locked-box__label {
  display: block;
  margin-bottom: 2px;
  font-size: 12px;
  font-weight: 650;
  color: #7c2d12;
}
.locked-box__body {
  display: block;
  margin-bottom: 10px;
  font-size: 12.5px;
  line-height: 1.55;
  color: #44403c;
}
.locked-box__body--last {
  margin-bottom: 12px;
}

.upgrade-cta {
  font-weight: 650;
  box-shadow: 0 1px 0 rgba(180, 83, 9, 0.18);
}
.upgrade-cta--in-box {
  margin-top: 2px;
}

.locked-empty {
  padding: 6px 0 2px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.55);
}
.locked-empty__icon {
  color: #b45309;
  opacity: 1;
}
.locked-empty :deep(.n-empty__icon) {
  opacity: 1;
  color: #b45309;
}
.locked-empty :deep(.n-empty__description) {
  color: #57534e;
  opacity: 1;
}
.panel-slot {
  min-height: 0;
}
</style>
