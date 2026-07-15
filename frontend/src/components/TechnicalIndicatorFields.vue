<script setup>
/** 技术面勾选（与股票信息筛选页字段一致） */
defineProps({
  model: { type: Object, required: true },
})
</script>

<template>
  <n-scrollbar style="max-height: 320px">
    <n-card size="small" :bordered="false" style="text-align: left">
      <n-checkbox v-model:checked="model.MACD_GOLDEN_FORK">MACD金叉</n-checkbox>
      <n-checkbox v-model:checked="model.KDJ_GOLDEN_FORK">KDJ金叉</n-checkbox>
      <n-checkbox v-model:checked="model.BREAK_THROUGH">放量突破</n-checkbox>
      <n-checkbox v-model:checked="model.LOW_FUNDS_INFLOW">低位资金净流入</n-checkbox>
      <n-checkbox v-model:checked="model.HIGH_FUNDS_OUTFLOW">高位资金净流出</n-checkbox>
      <n-checkbox v-model:checked="model.BREAKUP_MA_5DAYS">向上突破5日均线</n-checkbox>
      <n-checkbox v-model:checked="model.LONG_AVG_ARRAY">均线多头排列</n-checkbox>
      <n-checkbox v-model:checked="model.SHORT_AVG_ARRAY">均线空头排列</n-checkbox>
      <n-checkbox v-model:checked="model.UPPER_LARGE_VOLUME">连涨放量</n-checkbox>
      <n-checkbox v-model:checked="model.DOWN_NARROW_VOLUME">下跌无量</n-checkbox>
      <n-checkbox v-model:checked="model.ONE_DAYANG_LINE">一根大阳线</n-checkbox>
      <n-checkbox v-model:checked="model.TWO_DAYANG_LINES">两根大阳线</n-checkbox>
      <n-checkbox v-model:checked="model.RISE_SUN">旭日东升</n-checkbox>
      <n-checkbox v-model:checked="model.POWER_FULGUN">强势多方炮</n-checkbox>
      <n-checkbox v-model:checked="model.RESTORE_JUSTICE">拨云见日</n-checkbox>
      <n-checkbox v-model:checked="model.DOWN_7DAYS">七仙女下凡(七连阴)</n-checkbox>
      <n-checkbox v-model:checked="model.UPPER_8DAYS">八仙过海(八连阳)</n-checkbox>
      <n-checkbox v-model:checked="model.UPPER_9DAYS">九阳神功(九连阳)</n-checkbox>
      <n-checkbox v-model:checked="model.UPPER_4DAYS">四串阳</n-checkbox>
      <n-checkbox v-model:checked="model.HEAVEN_RULE">天量法则</n-checkbox>
      <n-checkbox v-model:checked="model.UPSIDE_VOLUME">放量上攻</n-checkbox>
      <n-checkbox v-model:checked="model.BEARISH_ENGULFING">穿头破脚</n-checkbox>
      <n-checkbox v-model:checked="model.REVERSING_HAMMER">倒转锤头</n-checkbox>
      <n-checkbox v-model:checked="model.SHOOTING_STAR">射击之星</n-checkbox>
      <n-checkbox v-model:checked="model.EVENING_STAR">黄昏之星</n-checkbox>
      <n-checkbox v-model:checked="model.FIRST_DAWN">曙光初现</n-checkbox>
      <n-checkbox v-model:checked="model.PREGNANT">身怀六甲</n-checkbox>
      <n-checkbox v-model:checked="model.BLACK_CLOUD_TOPS">乌云盖顶</n-checkbox>
      <n-checkbox v-model:checked="model.MORNING_STAR">早晨之星</n-checkbox>
      <n-checkbox v-model:checked="model.NARROW_FINISH">窄幅整理</n-checkbox>
    </n-card>
    <n-card size="small" :bordered="false" style="text-align: left; margin-top: 8px">
      <n-radio-group v-model:value="model.UPP_DAYS" name="UPP_DAYS" size="small">
        <n-text depth="3" style="margin-right: 8px">人气连涨</n-text>
        <n-radio :value="0">不限</n-radio>
        <n-radio :value="3">≥3天</n-radio>
        <n-radio :value="5">≥5天</n-radio>
        <n-radio :value="7">≥7天</n-radio>
      </n-radio-group>
      <n-divider style="margin: 8px 0" />
      <n-radio-group v-model:value="model.CONCERN_RANK_7DAYS" name="CONCERN_RANK_7DAYS" size="small">
        <n-text depth="3" style="margin-right: 8px">7日关注</n-text>
        <n-radio :value="0">不限</n-radio>
        <n-radio :value="10">前10</n-radio>
        <n-radio :value="50">前50</n-radio>
        <n-radio :value="100">前100</n-radio>
      </n-radio-group>
      <n-divider style="margin: 8px 0" />
      <n-radio-group v-model:value="model.UPNDAY" name="UPNDAY" size="small">
        <n-text depth="3" style="margin-right: 8px">连涨</n-text>
        <n-radio :value="0">不限</n-radio>
        <n-radio :value="3">≥3天</n-radio>
        <n-radio :value="5">≥5天</n-radio>
        <n-radio :value="8">≥8天</n-radio>
      </n-radio-group>
      <n-divider style="margin: 8px 0" />
      <n-radio-group v-model:value="model.DOWNNDAY" name="DOWNNDAY" size="small">
        <n-text depth="3" style="margin-right: 8px">连跌</n-text>
        <n-radio :value="0">不限</n-radio>
        <n-radio :value="3">≥3天</n-radio>
        <n-radio :value="5">≥5天</n-radio>
        <n-radio :value="8">≥8天</n-radio>
        <n-radio :value="10">≥10天</n-radio>
        <n-radio :value="14">≥14天</n-radio>
      </n-radio-group>
    </n-card>
  </n-scrollbar>
</template>
