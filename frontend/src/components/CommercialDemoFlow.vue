<script setup>
/**
 * Phase13-H: Commercial Demo Flow — Plan Overview / Comparison / Upgrade CTA.
 * No payment, no account system, no cloud.
 */
import { computed, h, onMounted, ref } from 'vue'
import {
  NButton,
  NCard,
  NDataTable,
  NSpace,
  NTag,
  NText,
  NAlert,
  useMessage,
} from 'naive-ui'
import {
  getProductPlans,
  getFeatureComparison,
  postUpgradeDemo,
  readProductTier,
  writeProductTier,
  recordProductUsage,
} from '../api/productCapabilities'

const message = useMessage()
const loading = ref(false)
const tier = ref(readProductTier())
const plans = ref([])
const upgradeNote = ref('')
const compareRows = ref([])
const upgrading = ref(false)

const tierLabel = computed(() => {
  if (tier.value === 'pro') return 'Pro'
  if (tier.value === 'enterprise') return 'Enterprise'
  return 'Free'
})

function cellLabel(v) {
  if (v === 'included') return '✓'
  if (v === 'locked') return 'Locked'
  if (v === '—') return '—'
  return String(v || '—')
}

function cellType(v) {
  if (v === 'included') return 'success'
  if (v === 'locked') return 'warning'
  return 'default'
}

const compareColumns = [
  { title: '能力', key: 'capability', width: 160 },
  {
    title: 'Free',
    key: 'free',
    width: 100,
    render(row) {
      return h(NTag, { size: 'small', type: cellType(row.free), bordered: false }, { default: () => cellLabel(row.free) })
    },
  },
  {
    title: 'Pro',
    key: 'pro',
    width: 100,
    render(row) {
      return h(NTag, { size: 'small', type: cellType(row.pro), bordered: false }, { default: () => cellLabel(row.pro) })
    },
  },
  {
    title: 'Enterprise',
    key: 'enterprise',
    width: 110,
    render(row) {
      return h(NTag, { size: 'small', type: cellType(row.enterprise), bordered: false }, { default: () => cellLabel(row.enterprise) })
    },
  },
  {
    title: '价值说明',
    key: 'value_hint',
    ellipsis: { tooltip: true },
  },
  {
    title: '当前',
    key: 'allowed_now',
    width: 90,
    render(row) {
      if (!row.feature) return h(NTag, { size: 'small', type: 'success', bordered: false }, { default: () => '可用' })
      return h(
        NTag,
        { size: 'small', type: row.allowed_now ? 'success' : 'warning', bordered: false },
        { default: () => (row.allowed_now ? '已解锁' : '已锁定') },
      )
    },
  },
]

async function refresh() {
  loading.value = true
  try {
    tier.value = readProductTier()
    const [p, c] = await Promise.all([
      getProductPlans(tier.value),
      getFeatureComparison(tier.value),
    ])
    plans.value = p?.plans || []
    upgradeNote.value = p?.upgrade_note || ''
    compareRows.value = c?.rows || []
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    loading.value = false
  }
}

async function onUpgrade(target) {
  if (target === 'enterprise') {
    message.info('Enterprise 为预留档位，演示流暂不激活。可先体验 Pro。')
    await postUpgradeDemo('enterprise').catch(() => null)
    return
  }
  upgrading.value = true
  try {
    const resp = await postUpgradeDemo(target)
    if (!resp?.ok || resp.accepted === false) {
      message.warning(resp?.message || '无法演示升级')
      return
    }
    writeProductTier(target === 'pro' ? 'pro' : 'free')
    tier.value = readProductTier()
    await recordProductUsage({
      feature: 'AIAnalysis',
      event: 'opened',
      usageKey: target === 'pro' ? 'upgrade_demo_pro' : 'upgrade_demo_free',
      scene: 'commercial_demo',
      tier: tier.value,
    }).catch(() => null)
    message.success(target === 'pro' ? '已切换演示档位为 Pro（无支付）' : '已切回 Free 演示档位')
    await refresh()
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    upgrading.value = false
  }
}

onMounted(refresh)
</script>

<template>
  <div class="commercial-demo">
    <n-space justify="space-between" align="center" :wrap="true" style="margin-bottom: 12px">
      <div>
        <n-text strong style="font-size: 18px">商业套餐演示</n-text>
        <n-text depth="3" style="display: block; margin-top: 4px; font-size: 12px">
          Commercial Demo Flow · 无支付 / 无账号 / 无云服务 · 当前演示档位：
          <n-tag size="small" type="info" :bordered="false">{{ tierLabel }}</n-tag>
        </n-text>
      </div>
      <n-button :loading="loading" secondary @click="refresh">刷新</n-button>
    </n-space>

    <n-alert type="info" :bordered="false" style="margin-bottom: 14px">
      {{ upgradeNote || '演示升级仅切换本地档位，不接支付。' }}
      交易核心（计划 / 执行 / Broker）不受套餐阻断。
    </n-alert>

    <!-- 1. Plan Overview -->
    <n-text strong style="display: block; margin-bottom: 8px">1. Plan Overview</n-text>
    <div class="plan-grid">
      <n-card
        v-for="plan in plans"
        :key="plan.code"
        size="small"
        :class="['plan-card', { current: plan.current, reserved: plan.reserved }]"
      >
        <n-space align="center" :wrap="true" style="margin-bottom: 6px">
          <n-text strong>{{ plan.display_name }}</n-text>
          <n-tag v-if="plan.current" size="small" type="success" :bordered="false">当前</n-tag>
          <n-tag v-if="plan.reserved" size="small" type="warning" :bordered="false">预留</n-tag>
        </n-space>
        <n-text depth="3" style="display: block; font-size: 13px; margin-bottom: 8px">{{ plan.tagline }}</n-text>
        <ul class="plan-list">
          <li v-for="(h, i) in plan.highlights || []" :key="i">{{ h }}</li>
        </ul>
        <n-space style="margin-top: 10px">
          <n-button
            v-if="plan.code === 'pro'"
            size="small"
            type="primary"
            :loading="upgrading"
            :disabled="tier === 'pro'"
            @click="onUpgrade('pro')"
          >
            {{ tier === 'pro' ? '已是 Pro 演示' : 'Upgrade to Pro（演示）' }}
          </n-button>
          <n-button
            v-else-if="plan.code === 'free'"
            size="small"
            secondary
            :loading="upgrading"
            :disabled="tier === 'free'"
            @click="onUpgrade('free')"
          >
            切回 Free 演示
          </n-button>
          <n-button v-else size="small" secondary disabled>
            Enterprise 预留
          </n-button>
        </n-space>
      </n-card>
    </div>

    <!-- 3. Upgrade CTA strip -->
    <div class="upgrade-cta">
      <n-space justify="space-between" align="center" :wrap="true">
        <div>
          <n-text strong>Upgrade</n-text>
          <n-text depth="3" style="display: block; font-size: 12px">
            解锁策略解释、高级风险与 AI 分析。本按钮不发起支付。
          </n-text>
        </div>
        <n-button type="primary" :loading="upgrading" :disabled="tier === 'pro'" @click="onUpgrade('pro')">
          升级到 Pro（演示）
        </n-button>
      </n-space>
    </div>

    <!-- 2. Feature Comparison -->
    <n-text strong style="display: block; margin: 16px 0 8px">2. Feature Comparison</n-text>
    <n-data-table
      size="small"
      :loading="loading"
      :columns="compareColumns"
      :data="compareRows"
      :bordered="false"
      :single-line="false"
      :row-key="(row) => row.capability"
    />
  </div>
</template>

<style scoped>
.commercial-demo {
  padding: 16px 18px;
  height: 100%;
  overflow: auto;
}
.plan-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 12px;
  margin-bottom: 14px;
}
.plan-card.current {
  border-color: rgba(24, 160, 88, 0.45);
}
.plan-card.reserved {
  opacity: 0.92;
}
.plan-list {
  margin: 0;
  padding-left: 18px;
  font-size: 13px;
  line-height: 1.55;
  color: var(--n-text-color-3);
}
.upgrade-cta {
  margin: 8px 0 4px;
  padding: 12px 14px;
  border-radius: 8px;
  border: 1px dashed rgba(32, 128, 240, 0.45);
  background: rgba(32, 128, 240, 0.06);
}
</style>
