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
  ExpandOutline, Flag,
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
const loadingMsg = ref("鍔犺浇鏁版嵁涓?..")
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
const activeKey = ref('stock')
const containerRef = ref({})
const realtimeProfit = ref(0)
const telegraph = ref([])
const groupList = ref([])
const officialStatement= ref("")
const menuOptions = ref([
  {
    label: () =>
        h(
            RouterLink,
            {
              to: {
                name: 'stock',
                query: {
                  groupName: '鍏ㄩ儴',
                  groupId: 0,
                },
                params: {},
              },
              onClick: () => {
                activeKey.value = 'stock'
              },
            },
            {default: () => '鑷€夎偂',}
        ),
    key: 'stock',
    icon: renderIcon(StarOutline),
    children: [
      {
        label: () =>
            h(
                'a',
                {
                  href: '#',
                  type: 'info',
                  onClick: () => {
                    activeKey.value = 'stock'
                    //console.log("push",item)
                    router.push({
                      name: 'stock',
                      query: {
                        groupName: '鍏ㄩ儴',
                        groupId: 0,
                      },
                    })
                    EventsEmit("changeTab", {ID: 0, name: '鍏ㄩ儴'})
                  },
                  to: {
                    name: 'stock',
                    query: {
                      groupName: '鍏ㄩ儴',
                      groupId: 0,
                    },
                  }
                },
                {default: () => '鍏ㄩ儴',}
            ),
        key: 0,
      }
    ],
  },
  {
    label: () =>
        h(
            RouterLink,
            {
              to: { name: 'holdings' },
              onClick: () => {
                activeKey.value = 'holdings'
              },
            },
            {default: () => '鎸佷粨鑲?}
        ),
    key: 'holdings',
    icon: renderIcon(Wallet),
  },
  {
    label: () =>
        h(
            RouterLink,
            {
              to: { name: 'holdingT' },
              onClick: () => {
                activeKey.value = 'holdingT'
              },
            },
            {default: () => '鎸佷粨鍋歍'}
        ),
    key: 'holdingT',
    icon: renderIcon(Pulse),
  },
  {
    label: () =>
        h(
            RouterLink,
            {
              to: { name: 'quantTrading' },
              onClick: () => {
                activeKey.value = 'quantTrading'
              },
            },
            {default: () => '閲忓寲浜ゆ槗'}
        ),
    key: 'quantTrading',
    icon: renderIcon(AnalyticsOutline),
  },
  {
    label: () =>
        h(
            RouterLink,
            {
              to: { name: 'tradePlanUpcoming' },
              onClick: () => {
                activeKey.value = 'tradePlanUpcoming'
              },
            },
            { default: () => '鏄庢棩浜ゆ槗璁″垝' },
        ),
    key: 'tradePlanUpcoming',
    icon: renderIcon(TimeOutline),
  },
  {
    label: () =>
        h(
            RouterLink,
            {
              to: { name: 'paperObservation' },
              onClick: () => {
                activeKey.value = 'paperObservation'
              },
            },
            { default: () => '妯℃嫙鐩樿瀵? },
        ),
    key: 'paperObservation',
    icon: renderIcon(AnalyticsOutline),
  },
  {
    label: () =>
        h(
            RouterLink,
            {
              to: { name: 'commercialDemo' },
              onClick: () => {
                activeKey.value = 'commercialDemo'
              },
            },
            { default: () => '套餐演示' },
        ),
    key: 'commercialDemo',
    icon: renderIcon(DiamondOutline),
  },
  {
    label: () =>
        h(
            RouterLink,
            {
              to: {
                name: 'stockScreen',
                params: {},
              },
              onClick: () => {
                activeKey.value = 'stockScreen'
                EventsEmit('allStockListRefresh')
              },
            },
            { default: () => '鑲＄エ绛涢€? },
        ),
    key: 'stockScreen',
    icon: renderIcon(AppsList20Regular),
  },
  {
    label: () =>
        h(
            RouterLink,
            {
              href: '#',
              to: {
                name: 'market',
                params: {}
              },
              onClick: () => {
                activeKey.value = 'market'
                EventsEmit("changeMarketTab", {ID: 0, name: '甯傚満蹇'})
              },
            },
            {default: () => '甯傚満琛屾儏'}
        ),
    key: 'market',
    icon: renderIcon(NewspaperOutline),
    children: [
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "甯傚満蹇",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '甯傚満蹇'})
                  },
                },
                {default: () => '甯傚満蹇',}
            ),
        key: 'market1',
        icon: renderIcon(NewspaperSharp),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "鍏ㄧ悆鑲℃寚",
                    },
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '鍏ㄧ悆鑲℃寚'})
                  },
                },
                {default: () => '鍏ㄧ悆鑲℃寚',}
            ),
        key: 'market2',
        icon: renderIcon(BarChartSharp),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "閲嶅ぇ鎸囨暟",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '閲嶅ぇ鎸囨暟'})
                  },
                },
                {default: () => '閲嶅ぇ鎸囨暟',}
            ),
        key: 'market3',
        icon: renderIcon(AnalyticsOutline),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "琛屼笟鎺掑悕",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '琛屼笟鎺掑悕'})
                  },
                },
                {default: () => '琛屼笟鎺掑悕',}
            ),
        key: 'market4',
        icon: renderIcon(Flag),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "涓偂璧勯噾娴佸悜",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '涓偂璧勯噾娴佸悜'})
                  },
                },
                {default: () => '涓偂璧勯噾娴佸悜',}
            ),
        key: 'market5',
        icon: renderIcon(Pulse),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "榫欒檸姒?,
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '榫欒檸姒?})
                  },
                },
                {default: () => '榫欒檸姒?,}
            ),
        key: 'market6',
        icon: renderIcon(Dragon),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "涓偂鐮旀姤",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '涓偂鐮旀姤'})
                  },
                },
                {default: () => '涓偂鐮旀姤',}
            ),
        key: 'market7',
        icon: renderIcon(StockOutlined),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "鍏徃鍏憡",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '鍏徃鍏憡'})
                  },
                },
                {default: () => '鍏徃鍏憡',}
            ),
        key: 'market8',
        icon: renderIcon(NotificationFilled),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "琛屼笟鐮旂┒",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '琛屼笟鐮旂┒'})
                  },
                },
                {default: () => '琛屼笟鐮旂┒',}
            ),
        key: 'market9',
        icon: renderIcon(ReportSearch),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "褰撳墠鐑棬",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '褰撳墠鐑棬'})
                  },
                },
                {default: () => '褰撳墠鐑棬',}
            ),
        key: 'market10',
        icon: renderIcon(Gripfire),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "鎸囨爣閫夎偂",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '鎸囨爣閫夎偂'})
                  },
                },
                {default: () => '鎸囨爣閫夎偂',}
            ),
        key: 'market11',
        icon: renderIcon(BoxSearch20Regular),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "鍚嶇珯浼橀€?,
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '鍚嶇珯浼橀€?})
                  },
                },
                {default: () => '鍚嶇珯浼橀€?,}
            ),
        key: 'market12',
        icon: renderIcon(FirefoxBrowser),
      },
    ]
  },
  {
    label: () =>
        h(
            RouterLink,
            {
              to: {
                name: 'fund',
                query: {
                  name: '鍩洪噾鑷€?,
                },
              },
              onClick: () => {
                activeKey.value = 'fund'
              },
            },
            {default: () => '鍩洪噾鑷€?,}
        ),
    show: enableFund.value,
    key: 'fund',
    icon: renderIcon(SparklesOutline),
    children: [
      {
        label: () => h(NText, {type: realtimeProfit.value > 0 ? 'error' : 'success'}, {default: () => '鍔熻兘瀹屽杽涓紒'}),
        key: 'realtimeProfit',
        show: realtimeProfit.value,
        icon: renderIcon(AlarmOutline),
      },
    ]
  },
  {
    label: () =>
        h(
            RouterLink,
            {
              to: {
                name: 'agent',
                query: {
                  name:"Ai鏅鸿兘浣?,
                },
                onClick: () => {
                  activeKey.value = 'agent'
                },
              }
            },
            {default: () => 'Ai鏅鸿兘浣?}
        ),
    key: 'agent',
    show:enableAgent.value,
    icon: renderIcon(Robot),
  },
    {
      label: () =>
          h(
              RouterLink,
              {
                to: {
                  name: 'research',
                  query: {
                    name:"鐮旂┒涓績",
                  },
                },
                onClick: () => {
                  activeKey.value = 'research'
                  setTimeout(() => {
                    EventsEmit("changeResearchTab", {ID: 0, name: 'AI鍒嗘瀽鎶ュ憡'})
                  }, 100)
                },
              },
              {default: () => '鐮旂┒涓績'}
          ),
      key: 'research',
      icon: renderIcon(FlaskOutline),
      children:[
          {
            label: () =>
                h(
                    RouterLink,
                    {
                      to: {
                        name: 'research',
                        query: {
                          name:"AI鍒嗘瀽鎶ュ憡",
                        },
                      },
                      onClick: () => {
                        activeKey.value = 'research'
                        setTimeout(() => {
                          EventsEmit("changeResearchTab", {ID: 0, name: 'AI鍒嗘瀽鎶ュ憡'})
                        }, 100)
                      },
                    },
                    {default: () => 'AI鍒嗘瀽鎶ュ憡'}
                ),
            key: 'research1',
            icon: renderIcon(ReportAnalytics),
          },
        {
          label: () =>
              h(
                  RouterLink,
                  {
                    to: {
                      name: 'research',
                      query: {
                        name:"鑲＄エ鎺ㄨ崘璁板綍",
                      },
                    },
                    onClick: () => {
                      activeKey.value = 'research'
                      setTimeout(() => {
                        EventsEmit("changeResearchTab", {ID: 1, name: '鑲＄エ鎺ㄨ崘璁板綍'})
                      }, 100)
                    },
                  },
                  {default: () => '鑲＄エ鎺ㄨ崘璁板綍'}
              ),
          key: 'research2',
          icon: renderIcon(DiamondOutline),
        },
        {
          label: () =>
              h(
                  RouterLink,
                  {
                    to: {
                      name: 'research',
                      query: {
                        name:"寮傚姩鐩戞帶",
                      },
                    },
                    onClick: () => {
                      activeKey.value = 'research'
                      setTimeout(() => {
                        EventsEmit("changeResearchTab", {ID: 2, name: '寮傚姩鐩戞帶'})
                      }, 100)
                    },
                  },
                  {default: () => '寮傚姩鐩戞帶'}
              ),
          key: 'stockChanges',
          icon: renderIcon(TrendingUp),
        },
        {
          label: () =>
              h(
                  RouterLink,
                  {
                    to: {
                      name: 'research',
                      query: {
                        name:"鎻愮ず璇嶆ā鏉?,
                      },
                    },
                    onClick: () => {
                      activeKey.value = 'research'
                      setTimeout(() => {
                        EventsEmit("changeResearchTab", {ID: 3, name: '鎻愮ず璇嶆ā鏉?})
                      }, 100)
                    },
                  },
                  {default: () => '鎻愮ず璇嶆ā鏉?}
              ),
          key: 'research3',
          icon: renderIcon(Prompt),
        },
        {
          label: () =>
              h(
                  RouterLink,
                  {
                    to: {
                      name: 'research',
                      query: {
                        name:"鎴戠殑绛栫暐",
                      },
                    },
                    onClick: () => {
                      activeKey.value = 'research'
                      setTimeout(() => {
                        EventsEmit("changeResearchTab", {ID: 0, name: '鎴戠殑绛栫暐'})
                      }, 100)
                    },
                  },
                  {default: () => '鎴戠殑绛栫暐'}
              ),
          key: 'research_strategy',
          icon: renderIcon(FlaskOutline),
        },
        {
          label: () =>
              h(
                  RouterLink,
                  {
                    to: {
                      name: 'cronTasks',
                      query: {
                        name:"瀹氭椂浠诲姟",
                      },
                    },
                    onClick: () => {
                      activeKey.value = 'research'
                      setTimeout(() => {
                        EventsEmit("changeResearchTab", {ID: 5, name: '瀹氭椂浠诲姟'})
                      }, 100)
                    },
                  },
                  {default: () => '瀹氭椂浠诲姟'}
              ),
          key: 'research5',
          icon: renderIcon(TimeOutline),
        },
        {
          label: () =>
              h(
                  RouterLink,
                  {
                    to: {
                      name: 'research',
                      query: {
                        name:"浜ゆ槗鏃ュ織",
                      },
                    },
                    onClick: () => {
                      activeKey.value = 'research'
                      setTimeout(() => {
                        EventsEmit("changeResearchTab", {ID: 6, name: '浜ゆ槗鏃ュ織'})
                      }, 100)
                    },
                  },
                  {default: () => '浜ゆ槗鏃ュ織(beta)'}
              ),
          key: 'research6',
          icon: renderIcon(MoneyCollectOutlined),
        },
      ],
    },
  {
    label: () =>
        h(
            RouterLink,
            {
              to: {
                name: 'settings',
                query: {
                  name:"璁剧疆",
                },
                onClick: () => {
                  activeKey.value = 'settings'
                },
              }
            },
            {default: () => '璁剧疆'}
        ),
    key: 'settings',
    icon: renderIcon(SettingsOutline),
  },
  {
    show:false,
    label: () => h("a", {
      href: '#',
      onClick: toggleFullscreen,
      title: '鍏ㄥ睆 Ctrl+F 閫€鍑哄叏灞?Esc',
    }, {default: () => isFullscreen.value ? '鍙栨秷鍏ㄥ睆' : '鍏ㄥ睆'}),
    key: 'full',
    icon: renderIcon(ExpandOutline),
  },
  // {
  //   label: ()=> h("a", {
  //     href: 'javascript:void(0)',
  //     style: 'cursor: move;',
  //     onClick: toggleStartMoveWindow,
  //   }, { default: () => '绉诲姩' }),
  //   key: 'move',
  //   icon: renderIcon(MoveOutline),
  // },
  {
    show: false,
    label: () => h("a", {
      href: '#',
      onClick: Quit,
    }, {default: () => '閫€鍑虹▼搴?}),
    key: 'exit',
    icon: renderIcon(PowerOutline),
  },
])

watch(
  () => router.currentRoute.value.name,
  (name) => {
    const directKeys = new Set(['stock', 'holdings', 'holdingT', 'quantTrading', 'tradePlanUpcoming', 'paperObservation', 'commercialDemo', 'stockScreen', 'market', 'fund', 'agent', 'research', 'settings'])
    if (directKeys.has(String(name))) activeKey.value = String(name)
    if (name === 'cronTasks') activeKey.value = 'research'
  },
  { immediate: true },
)

function renderIcon(icon) {
  return () => h(NIcon, null, {default: () => h(icon)})
}

function toggleFullscreen(e) {
  activeKey.value = 'full'
  //console.log(e)
  if (isFullscreen.value) {
    WindowUnfullscreen()
    //e.target.innerHTML = '鍏ㄥ睆'
  } else {
    WindowFullscreen()
    // e.target.innerHTML = '鍙栨秷鍏ㄥ睆'
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
    loadingMsg.value = "鍔犺浇瀹屾垚..."
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
  // 灏嗛敊璇俊鎭彂閫佺粰鍚庣
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
    groupList.value = result
    menuOptions.value.map((item) => {
      //console.log(item)
      if (item.key === 'stock') {
        item.children.push(...groupList.value.map(item => {
          return {
            label: () =>
                h(
                    'a',
                    {
                      href: '#',
                      type: 'info',
                      onClick: () => {
                        //console.log("push",item)
                        router.push({
                          name: 'stock',
                          query: {
                            groupName: item.name,
                            groupId: item.ID,
                          },
                        })
                        setTimeout(() => {
                          EventsEmit("changeTab", item)
                        }, 100)
                      },
                      to: {
                        name: 'stock',
                        query: {
                          groupName: item.name,
                          groupId: item.ID,
                        },
                      }
                    },
                    {default: () => item.name,}
                ),
            key: item.ID,
          }
        }))
      }
    })
  })


  GetConfig().then((res) => {
    applySettingsToShell(res)
  })
})

function openStockStrategySave(payload) {
  activeKey.value = 'research'
  router.push({ name: 'research', query: { name: '鎴戠殑绛栫暐' } })
  setTimeout(() => {
    EventsEmit('changeResearchTab', { name: '鎴戠殑绛栫暐' })
    setTimeout(() => EventsEmit('openSaveStockStrategy', payload), 400)
  }, 150)
}

function applySettingsToShell(cfg) {
  if (!cfg) return
  enableNews.value = !!cfg.enableNews
  enableFund.value = !!cfg.enableFund
  enableAgent.value = !!cfg.enableAgent
  enableDarkTheme.value = cfg.darkTheme ? darkTheme : null
  menuOptions.value.forEach((item) => {
    if (item.key === 'fund') item.show = !!cfg.enableFund
    if (item.key === 'agent') item.show = !!cfg.enableAgent
  })
}

onMounted(() => {
  initSignalSettingsSync()
  EventsOn('requestSaveStockStrategy', openStockStrategySave)
  // 璁剧疆鐑洿鏂帮細閬垮厤浠呮敼鍒锋柊闂撮殧/鏆楅粦涓婚涔熸暣椤?reload 鏃跺３灞備笉鍚屾
  EventsOn('updateSettings', (cfg) => {
    applySettingsToShell(cfg || {})
  })
  WindowSetTitle("鑲＄エ鍒嗘瀽")
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
    /** 鏄惁鍦ㄥ彸渚у脊鍑烘柊闂婚€氱煡锛堟殏鍏抽棴锛屾柊闂讳粛鍙湪銆屽競鍦鸿祫璁€嶉〉鏌ョ湅锛?*/
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
        const isRisk = !!data.isRed || alert?.actionLabel === '鍏堥鎺?
        notification.create({
          type: isRisk ? 'warning' : 'success',
          title: data.title || '鑷€夋搷浣滄彁绀?,
          content: () => h(WatchlistActionNotification, { alert }),
          duration: 1000 * 20,
          keepAliveOnHover: true,
        })
        return
      }

      notification.create({
        title: data.title || '閲忓寲鎻愰啋',
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
            { default: () => '閲忓寲鑷姩鍖? },
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
