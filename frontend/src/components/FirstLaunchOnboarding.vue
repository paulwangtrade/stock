<script setup>
/**
 * Phase13 Onboarding MVP — first-launch wizard (5 steps).
 * Reuses FirstLaunchOnboarding + FirstLaunchState.
 * Local only; no buy/sell CTAs, no trading write APIs, no strategy mutation.
 */
import { computed, ref, watch } from 'vue'
import { NButton, NCheckbox, NModal, NSpace, NSteps, NStep, NTag, NText, NAlert } from 'naive-ui'
import {
  markFirstLaunchCompleted,
  markFirstLaunchSkipped,
  shouldShowFirstLaunch,
  writeFirstLaunchState,
  readFirstLaunchState,
} from '../utils/firstLaunchState'

const TOTAL_STEPS = 5

const props = defineProps({
  /** When true, evaluate whether to open (typically after app loading done). */
  active: { type: Boolean, default: false },
})

const emit = defineEmits(['done'])

const show = ref(false)
/** 1 产品定位 · 2 数据来源 · 3 模拟交易 · 4 Portfolio Decision · 5 风险免责 */
const step = ref(1)
const disclaimerAccepted = ref(false)

const stepIndex = computed(() => Math.max(0, step.value - 1))
const canFinish = computed(() => step.value === TOTAL_STEPS && disclaimerAccepted.value)

const capabilityTags = [
  '智能选股',
  '组合决策',
  '风险观察',
  '模拟交易',
  '持仓',
  '再平衡观察',
]

watch(
  () => props.active,
  (v) => {
    if (!v) return
    if (shouldShowFirstLaunch()) {
      const st = readFirstLaunchState()
      step.value = st.lastStep > 0 && st.lastStep <= TOTAL_STEPS ? st.lastStep : 1
      disclaimerAccepted.value = Boolean(st.disclaimerAcceptedAt)
      show.value = true
    }
  },
  { immediate: true },
)

function persistStep(n) {
  const st = readFirstLaunchState()
  writeFirstLaunchState({ ...st, lastStep: n })
}

function goNext() {
  if (step.value < TOTAL_STEPS) {
    step.value += 1
    persistStep(step.value)
  }
}

function goPrev() {
  if (step.value > 1) {
    step.value -= 1
    persistStep(step.value)
  }
}

function finish(skipped) {
  if (skipped) {
    markFirstLaunchSkipped(step.value)
    show.value = false
    emit('done', { skipped: true })
    return
  }
  if (!disclaimerAccepted.value) return
  const acceptedAt = new Date().toISOString()
  markFirstLaunchCompleted(step.value, undefined, { disclaimerAcceptedAt: acceptedAt })
  show.value = false
  emit('done', { skipped: false, disclaimerAcceptedAt: acceptedAt })
}
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    title="欢迎使用 go-stock"
    :mask-closable="false"
    :close-on-esc="true"
    style="width: min(560px, 94vw)"
    @close="finish(true)"
  >
    <n-steps :current="stepIndex" size="small" style="margin-bottom: 16px">
      <n-step title="定位" />
      <n-step title="数据" />
      <n-step title="模拟" />
      <n-step title="决策" />
      <n-step title="风险" />
    </n-steps>

    <!-- Step 1: 产品定位 -->
    <div v-if="step === 1" class="fl-body">
      <n-text depth="1" class="fl-title">这是什么产品</n-text>
      <n-text depth="1" style="font-size: 15px; display: block; margin-bottom: 8px">
        go-stock — AI 驱动股票分析与组合决策平台
      </n-text>
      <n-text depth="3" style="display: block; margin-bottom: 10px">
        帮你做研究、选股观察、组合风险解释与纸面模拟；核心是「看懂决策过程」，不是代替你下单，也不是券商交易终端。
      </n-text>
      <n-space :wrap="true" :size="6" style="margin-bottom: 10px">
        <n-tag v-for="t in capabilityTags" :key="t" size="small" type="info" :bordered="false">{{ t }}</n-tag>
      </n-space>
      <n-alert type="info" :bordered="false" style="font-size: 12px">
        非投资建议 · 非实盘券商 · 不自动买卖。数据默认留在本机，无需注册即可开始。
      </n-alert>
    </div>

    <!-- Step 2: 数据来源 -->
    <div v-else-if="step === 2" class="fl-body">
      <n-text depth="1" class="fl-title">数据从哪来</n-text>
      <ul class="fl-list">
        <li>
          <strong>本机账本</strong>：持仓、纸面账户、设置与诊断保存在本机 data（SQLite 等），不是云端券商同步。
        </li>
        <li>
          <strong>行情 / 资讯</strong>：来自设置中已配置的市场数据源（名称以你的配置为准）。
        </li>
        <li>
          <strong>AI 解释</strong>：可选；需自行配置 Key。未配置时解释类能力可能空缺，系统不会编造假数据。
        </li>
        <li>
          <strong>版本信息</strong>：可在「关于 / 诊断」查看 version · channel · build_mode。
        </li>
      </ul>
    </div>

    <!-- Step 3: 模拟交易说明 -->
    <div v-else-if="step === 3" class="fl-body">
      <n-text depth="1" class="fl-title">模拟（纸面）交易</n-text>
      <ul class="fl-list">
        <li>
          <strong>是</strong>：按交易日节奏观察计划、执行记录与持仓模拟，用来理解流程与规则。
        </li>
        <li>
          <strong>不是</strong>：实盘委托、真实资金划转、自动跟单。
        </li>
        <li>
          <strong>入口预告</strong>（本引导仅说明，不触发执行）：侧栏「交易」下的交易计划 / 今日流程 / 执行记录。
        </li>
      </ul>
      <n-alert type="warning" :bordered="false" style="font-size: 12px; margin-top: 8px">
        纸面 ≠ 实盘。本步不会批准计划、不会冻结、不会开仓。
      </n-alert>
    </div>

    <!-- Step 4: Portfolio Decision -->
    <div v-else-if="step === 4" class="fl-body">
      <n-text depth="1" class="fl-title">组合决策</n-text>
      <ul class="fl-list">
        <li>
          <strong>是</strong>：只读分析——组合概览、风险解释、行业覆盖、为何买少 / 限仓 / 建议减少等。
        </li>
        <li>
          <strong>不是</strong>：交易台；打开页面不会自动买卖。
        </li>
        <li>
          <strong>入口</strong>：「决策差异分析」（只读观察）。
        </li>
      </ul>
      <n-text depth="3" style="display: block; margin-top: 8px">
        行业数据不可用时会如实提示，不会假装「行业安全」。请把它当作说明书，而不是下单屏。
      </n-text>
    </div>

    <!-- Step 5: 风险免责声明 -->
    <div v-else class="fl-body">
      <n-text depth="1" class="fl-title">风险免责声明</n-text>
      <ul class="fl-list">
        <li>仅用于研究和投资分析辅助。</li>
        <li>不构成投资建议、收益承诺或买卖指令。</li>
        <li>市场有风险；决策与盈亏由使用者自行承担。</li>
        <li>模拟 / 纸面结果不代表实盘。</li>
        <li>AI / 规则解释可能不完整或滞后；缺失数据 ≠ 安全。</li>
        <li>本引导与观察页不创建或执行交易计划（交易流程中的显式操作不在本引导内）。</li>
      </ul>
      <div style="margin-top: 12px">
        <n-checkbox v-model:checked="disclaimerAccepted">
          我已阅读并理解上述风险提示
        </n-checkbox>
      </div>
      <n-text depth="3" style="display: block; margin-top: 8px; font-size: 12px">
        未勾选时无法完成引导；可选择跳过（稍后仍可在关于页查看免责声明）。
      </n-text>
    </div>

    <template #footer>
      <n-space justify="space-between" style="width: 100%">
        <n-button quaternary @click="finish(true)">跳过</n-button>
        <n-space>
          <n-button v-if="step > 1" @click="goPrev">上一步</n-button>
          <n-button v-if="step < TOTAL_STEPS" type="primary" @click="goNext">下一步</n-button>
          <n-button v-else type="primary" :disabled="!canFinish" @click="finish(false)">完成</n-button>
        </n-space>
      </n-space>
    </template>
  </n-modal>
</template>

<style scoped>
.fl-body {
  min-height: 168px;
  line-height: 1.55;
}
.fl-title {
  font-size: 15px;
  font-weight: 600;
  display: block;
  margin-bottom: 8px;
}
.fl-list {
  margin: 0;
  padding-left: 1.2em;
}
.fl-list li {
  margin: 0.4em 0;
}
</style>
