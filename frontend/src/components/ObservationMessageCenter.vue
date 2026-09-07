<script setup>
import { computed, ref } from 'vue'
import { NBadge, NButton, NDrawer, NDrawerContent, NEmpty, NText, NSpace, NTag } from 'naive-ui'

const show = ref(false)

/** UI framework only — no persistence / no DB. */
const messages = ref([])

const count = computed(() => messages.value.length)

function open() {
  show.value = true
}

function close() {
  show.value = false
}

defineExpose({ open, close })
</script>

<template>
  <n-badge :value="count" :max="99" :show-zero="false">
    <n-button size="small" tertiary @click="open">
      消息
    </n-button>
  </n-badge>

  <n-drawer v-model:show="show" :width="360" placement="right" :trap-focus="false">
    <n-drawer-content title="消息中心" closable>
      <n-text depth="3" style="display: block; font-size: 12px; margin-bottom: 12px">
        观察阶段消息入口（框架）。现有右侧通知弹窗保留；此处暂不落库。
      </n-text>
      <n-space v-if="count" vertical :size="8">
        <div v-for="(m, i) in messages" :key="i" class="msg-item">
          <n-tag size="tiny" :bordered="false">{{ m.type || 'info' }}</n-tag>
          <n-text>{{ m.title || m.body }}</n-text>
        </div>
      </n-space>
      <n-empty v-else description="暂无消息" style="margin-top: 48px" />
    </n-drawer-content>
  </n-drawer>
</template>

<style scoped>
.msg-item {
  padding: 8px 10px;
  border-radius: 6px;
  border: 1px solid rgba(128, 128, 128, 0.18);
  display: flex;
  gap: 8px;
  align-items: flex-start;
}
</style>
