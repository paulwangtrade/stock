<script setup>
import {
  EventsEmit,
  EventsOff,
  EventsOn,
  Quit,
  WindowFullscreen,
  WindowUnfullscreen,
  WindowSetTitle
} from '../wailsjs/runtime'
import {h, onBeforeMount, onBeforeUnmount, onMounted, ref, computed, watch} from "vue";
import {RouterLink, useRouter} from 'vue-router'
import {createDiscreteApi,darkTheme,lightTheme , NIcon, NText,NButton,dateZhCN,zhCN} from 'naive-ui'
import {
  AlarmOutline,
  AnalyticsOutline,
  BarChartSharp, Bonfire, BonfireOutline, DiamondOutline, EaselSharp,
  ExpandOutline, Flag, GitCompareOutline,
  Flame, FlameSharp, FlaskOutline,   InformationOutline,
  NewspaperOutline,
  NewspaperSharp, Notifications,
  PowerOutline, Pulse,
  SettingsOutline, Skull, SkullOutline, SkullSharp,
  SparklesOutline,
  StarOutline,
  Wallet, WarningOutline, TimeOutline,
} from '@vicons/ionicons5'
import {AnalyzeSentiment, GetConfig, GetGroupList,GetVersionInfo} from "../wailsjs/go/main/App";
import { initSignalSettingsSync, loadSignalSettingsFromBackend } from "./utils/signalSettingsStore";
import { startQuantAutomation, stopQuantAutomation } from "./services/quantAutomationService";
import { recordPerformanceMetric } from "./services/performanceMetrics";
import {
  createProductMenuOptions,
  findMenuItemByKey,
  forEachMenuItem,
} from "./navigation/productMenu";
import FloatingAiAssistant from "./components/FloatingAiAssistant.vue";
import FloatingAgentAssistant from "./components/FloatingAgentAssistant.vue";
import WatchlistActionNotification from "./components/WatchlistActionNotification.vue";
import MarketStatusBar from "./components/MarketStatusBar.vue";
import {Dragon, Fire, FirefoxBrowser, Gripfire, Robot} from "@vicons/fa";
import {Prompt, ReportAnalytics, ReportMoney, ReportSearch, TrendingUp} from "@vicons/tabler";
import {LocalFireDepartmentRound} from "@vicons/material";
import {AppsList20Regular, BoxSearch20Regular, CommentNote20Filled} from "@vicons/fluent";
import {FireFilled, MoneyCollectOutlined, NotificationFilled, StockOutlined} from "@vicons/antd";




const router = useRouter()
const loading = ref(true)
const loadingMsg = ref("加载数据中...")
const enableNews = ref(false)
const contentStyle = ref("")
const mainScrollbarStyle = computed(() => {
  const base = 'box-sizing: border-box; max-height: 100vh; padding-bottom: 72px;'
  const extra = String(contentStyle.value || '').trim()
  return extra ? `${base} ${extra}` : base
})
const enableFund = ref(false)
const enableAgent = ref(false)
const enableDarkTheme = ref(null)
const content = ref('')
const isFullscreen = ref(false)
const activeKey = ref('investmentHome')
const containerRef = ref({})
const realtimeProfit = ref(0)
const telegraph = ref([])
const groupList = ref([])
const officialStatement= ref("")
const menuOptions = ref([])

function rebuildProductMenu() {
  menuOptions.value = createProductMenuOptions({
    h,
    RouterLink,
    router,
    activeKey,
    renderIcon,
    EventsEmit,
    NText,
    Quit,
    toggleFullscreen,
    isFullscreen,
    enableFund,
    enableAgent,
    realtimeProfit,
    icons: {
      SparklesOutline,
      Wallet,
      Pulse,
      AnalyticsOutline,
      TimeOutline,
      Flag,
      GitCompareOutline,
      FlaskOutline,
      AppsList20Regular,
      NewspaperOutline,
      NewspaperSharp,
      BarChartSharp,
      Dragon,
      StockOutlined,
      NotificationFilled,
      ReportSearch,
      Gripfire,
      BoxSearch20Regular,
      FirefoxBrowser,
      DiamondOutline,
      SettingsOutline,
      StarOutline,
      Robot,
      Prompt,
      ReportAnalytics,
      TrendingUp,
      MoneyCollectOutlined,
      AlarmOutline,
      ExpandOutline,
      PowerOutline,
    },
  })
  const stockItem = findMenuItemByKey(menuOptions.value, 'stock')
  if (stockItem && Array.isArray(groupList.value) && groupList.value.length) {
    if (!Array.isArray(stockItem.children)) stockItem.children = []
    stockItem.children = stockItem.children.filter((c) => c.key === 0)
    stockItem.children.push(
      ...groupList.value.map((g) => ({
        label: () =>
          h(
            'a',
            {
              href: '#',
              type: 'info',
              onClick: () => {
                activeKey.value = 'stock'
                router.push({
                  name: 'stock',
                  query: {
                    groupName: g.name,
                    groupId: g.ID,
                  },
                })
                setTimeout(() => {
                  EventsEmit('changeTab', g)
                }, 100)
              },
            },
            { default: () => g.name },
          ),
        key: g.ID,
      })),
    )
  }
}


watch(
  () => router.currentRoute.value.name,
  (name) => {
    const n = String(name || '')
    const directKeys = new Set([
      'investmentHome',
      'portfolioDashboard',
      'holdingT',
      'stockScreen',
      'watchedOpportunities',
      'tradePlanUpcoming',
      'stock',
      'settings',
      'fund',
      'agent',
      'about',
    ])
    if (directKeys.has(n)) activeKey.value = n
    if (n === 'research') activeKey.value = 'eventMonitor'
    if (n === 'cronTasks') activeKey.value = 'settings'
  },
  { immediate: true },
)

function renderIcon(icon) {
  return () => h(NIcon, null, {default: () => h(icon)})
}

rebuildProductMenu()


function toggleFullscreen(e) {
  activeKey.value = 'full'
  //console.log(e)
  if (isFullscreen.value) {
    WindowUnfullscreen()
    //e.target.innerHTML = '全屏'
  } else {
    WindowFullscreen()
    // e.target.innerHTML = '取消全屏'
  }
  isFullscreen.value = !isFullscreen.value
}

// const drag = ref(false)
// const lastPos= ref({x:0,y:0})
// function toggleStartMoveWindow(e) {
//   drag.value=!drag.value
//   lastPos.value={x:e.clientX,y:e.clientY}
// }
// function dragstart(e) {
//   if (drag.value) {
//     let x=e.clientX-lastPos.value.x
//     let y=e.clientY-lastPos.value.y
//     WindowGetPosition().then((pos) => {
//       WindowSetPosition(pos.x+x,pos.y+y)
//     })
//   }
// }
// window.addEventListener('mousemove', dragstart)

EventsOn("realtime_profit", (data) => {
  realtimeProfit.value = data
})
EventsOn("monitor_perf", (payload) => {
  const name = payload?.name || 'monitor.stock_prices'
  const ms = Number(payload?.durationMs)
  if (Number.isFinite(ms)) recordPerformanceMetric(name, ms, { source: 'backend' })
})
EventsOn("telegraph", (data) => {
  telegraph.value = data
})

EventsOn("loadingMsg", (data) => {
  if(data==="done"){
    loadingMsg.value = "加载完成..."
    EventsEmit("loadingDone", "app")
    loading.value  = false
  }else{
    loading.value  = true
    loadingMsg.value = data
  }
})

EventsOn('changeActiveMenuKey', (key) => {
  if (key) activeKey.value = String(key)
})

onBeforeUnmount(() => {
  EventsOff("realtime_profit")
  EventsOff("loadingMsg")
  EventsOff("telegraph")
  EventsOff("newsPush")
  EventsOff("quantPush")
  EventsOff("changeActiveMenuKey")
  EventsOff("requestSaveStockStrategy")
  EventsOff("updateSettings")
  EventsOff("monitor_perf")
  stopQuantAutomation()
})

window.onerror = function (msg, source, lineno, colno, error) {
  // 将错误信息发送给后端
  EventsEmit("frontendError", {
    page: "App.vue",
    message: msg,
    source: source,
    lineno: lineno,
    colno: colno,
    error: error ? error.stack : null,
  });
  return true;
};

onBeforeMount(() => {
  GetVersionInfo().then(result => {
    if(result.officialStatement){
      content.value = result.officialStatement+"\n\n"+content.value
      officialStatement.value = result.officialStatement
    }
  })

  GetGroupList().then(result => {
    groupList.value = result || []
    rebuildProductMenu()
  })


  GetConfig().then((res) => {
    applySettingsToShell(res)
  })
})

function openStockStrategySave(payload) {
  activeKey.value = 'research'
  router.push({ name: 'research', query: { name: '我的策略' } })
  setTimeout(() => {
    EventsEmit('changeResearchTab', { name: '我的策略' })
    setTimeout(() => EventsEmit('openSaveStockStrategy', payload), 400)
  }, 150)
}

function applySettingsToShell(cfg) {
  if (!cfg) return
  enableNews.value = !!cfg.enableNews
  enableFund.value = !!cfg.enableFund
  enableAgent.value = !!cfg.enableAgent
  enableDarkTheme.value = cfg.darkTheme ? darkTheme : null
  rebuildProductMenu()
  forEachMenuItem(menuOptions.value, (item) => {
    if (item.key === 'fund') item.show = !!cfg.enableFund
    if (item.key === 'agent') item.show = !!cfg.enableAgent
  })
}

onMounted(() => {
  initSignalSettingsSync()
  EventsOn('requestSaveStockStrategy', openStockStrategySave)
  // 设置热更新：避免仅改刷新间隔/暗黑主题也整页 reload 时壳层不同步
  EventsOn('updateSettings', (cfg) => {
    applySettingsToShell(cfg || {})
  })
  WindowSetTitle("股票分析")
  contentStyle.value = "height: calc(92vh); max-height: calc(92vh); overflow: hidden; padding-bottom: 52px;"
  GetConfig().then((res) => {
    applySettingsToShell(res)
    const {notification } =createDiscreteApi(["notification"], {
      configProviderProps: {
        theme: enableDarkTheme.value ? darkTheme : lightTheme ,
        max: 3,
      },
    })
    const recentNewsPushKeys = new Set()
    /** 是否在右侧弹出新闻通知（暂关闭，新闻仍可在「市场资讯」页查看） */
    const enableNewsRightPopup = false
    EventsOn("newsPush", (data) => {
      if (!enableNewsRightPopup) return
      const content = (data.content || '').trim()
      const pushKey = `${data.source || ''}|${data.time || ''}|${content.slice(0, 100)}`
      if (recentNewsPushKeys.has(pushKey)) return
      recentNewsPushKeys.add(pushKey)
      setTimeout(() => recentNewsPushKeys.delete(pushKey), 30 * 60 * 1000)
      if(data.isRed){
        notification.create({
          //type:"error",
         // avatar: () => h(NIcon,{component:Notifications,color:"red"}),
          title: data.time,
          content: () => h('div',{type:"error",style:{
              "text-align":"left",
              "font-size":"14px",
              "color":"#f67979"
            }}, { default: () => data.content }),
          meta: () => h(NText,{type:"warning"}, { default: () => data.source}),
          duration:1000*40,
        })
      }else{
         notification.create({
          //type:"info",
          //avatar: () => h(NIcon,{component:Notifications}),
          title: data.time,
          content: () => h('div',{type:"info",style:{
            "text-align":"left",
              "font-size":"14px",
              "color": data.source==="go-stock"?"#F98C24":"#549EC8"
            }}, { default: () => data.content }),
          meta: () => h(NText,{type:"warning"}, { default: () => data.source}),
          duration:1000*30 ,
        })
      }
    })
    const recentQuantKeys = new Set()
    EventsOn('quantPush', (data) => {
      const content = (data?.content || '').trim()
      if (!content) return
      const pushKey = `${data.type || ''}|${data.code || ''}|${content.slice(0, 80)}`
      if (recentQuantKeys.has(pushKey)) return
      recentQuantKeys.add(pushKey)
      setTimeout(() => recentQuantKeys.delete(pushKey), 15 * 60 * 1000)

      if (data.type === 'watchlistAction') {
        const alert = data.watchlistAlert || data
        const isRisk = !!data.isRed || alert?.actionLabel === '先风控'
        notification.create({
          type: isRisk ? 'warning' : 'success',
          title: data.title || '自选操作提示',
          content: () => h(WatchlistActionNotification, { alert }),
          duration: 1000 * 20,
          keepAliveOnHover: true,
        })
        return
      }

      notification.create({
        title: data.title || '量化提醒',
        content: () =>
          h(
            'div',
            {
              style: {
                'text-align': 'left',
                'font-size': '13px',
                'white-space': 'pre-wrap',
                color: data.isRed ? '#f67979' : '#18a058',
              },
            },
            { default: () => content },
          ),
        meta: () =>
          h(
            NText,
            { depth: 3 },
            { default: () => '量化自动化' },
          ),
        duration: 1000 * 45,
      })
    })
    loadSignalSettingsFromBackend().finally(() => startQuantAutomation())
  })
})
</script>
<template>
  <n-config-provider ref="containerRef" :theme="enableDarkTheme" :locale="zhCN" :date-locale="dateZhCN">
    <n-message-provider>
      <n-notification-provider>
        <n-modal-provider>
          <n-dialog-provider>
            <n-watermark
                :content="''"
                cross
                selectable
                :font-size="16"
                :line-height="16"
                :width="500"
                :height="400"
                :x-offset="50"
                :y-offset="150"
                :rotate="-15"
            >
<!--              <FloatingAiAssistant />-->
              <FloatingAgentAssistant />
              <n-flex>
                <n-grid x-gap="12" :cols="1">
                  <n-gi>
                    <n-spin :show="loading">
                      <template #description>
                        {{ loadingMsg }}
                      </template>
                      <n-marquee :speed="100" style="position: relative;top:0;z-index: 19;width: 100%"
                                 v-if="(telegraph.length>0)&&(enableNews)">
                        <n-tag type="warning" v-for="item in telegraph" style="margin-right: 10px">
                          {{ item }}
                        </n-tag>
                      </n-marquee>
                      <MarketStatusBar :dark-theme="!!enableDarkTheme" />
                      <n-scrollbar :style="mainScrollbarStyle">
                        <n-skeleton v-if="loading" height="calc(100vh)" />
                        <RouterView/>
                      </n-scrollbar>
                    </n-spin>
                  </n-gi>
                  <n-gi style="position: fixed;bottom:0;z-index: 9;width: 100%;">
                    <n-card size="small" style="--wails-draggable:no-drag">
                      <n-menu style="font-size: 18px;"
                              v-model:value="activeKey"
                              mode="horizontal"
                              :options="menuOptions"
                              responsive
                      />
                    </n-card>
                  </n-gi>
                </n-grid>
              </n-flex>
            </n-watermark>
          </n-dialog-provider>
        </n-modal-provider>
      </n-notification-provider>
    </n-message-provider>
  </n-config-provider>
</template>
<style>

</style>
