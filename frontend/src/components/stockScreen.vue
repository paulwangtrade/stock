<script setup>
import { onMounted } from 'vue'
import AllStockList from './allStockList.vue'
import { EventsEmit } from '../../wailsjs/runtime'

onMounted(() => {
  EventsEmit('allStockListRefresh')
})
</script>

<template>
  <div class="stock-screen-shell">
    <AllStockList />
  </div>
</template>

<style scoped>
.stock-screen-shell {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: calc(92vh - 52px);
  min-height: calc(92vh - 52px);
  max-height: calc(92vh - 52px);
  overflow: hidden;
  box-sizing: border-box;
  padding: 4px 8px 8px;
}

.stock-screen-shell :deep(.stock-screen-page) {
  flex: 1;
  min-height: 0;
  width: 100%;
  height: auto;
}
</style>
