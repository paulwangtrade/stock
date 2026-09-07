/**
 * Phase17 product navigation restore.
 * Flat product shell — old route names/paths remain valid.
 * Hidden items keep `show: false` so hash routes still work for debug.
 */
export function createProductMenuOptions(ctx) {
  const {
    h,
    RouterLink,
    router,
    activeKey,
    renderIcon,
    EventsEmit,
    NText,
    icons,
    enableFund,
    enableAgent,
    realtimeProfit,
  } = ctx

  const {
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
    DiamondOutline,
    SettingsOutline,
    StarOutline,
    Robot,
    TrendingUp,
    AlarmOutline,
    ExpandOutline,
    PowerOutline,
  } = icons

  const setKey = (key) => {
    activeKey.value = key
  }

  const routeLink = (name, label, key, icon, extra = {}) => ({
    label: () =>
      h(
        RouterLink,
        {
          to: extra.to || { name },
          onClick: () => {
            setKey(key || name)
            if (typeof extra.onClick === 'function') extra.onClick()
          },
        },
        { default: () => label },
      ),
    key: key || name,
    icon: icon ? renderIcon(icon) : undefined,
    show: extra.show,
  })

  /** 自选股：独立研究入口（分组子项由 App GetGroupList 注入） */
  const stockNode = {
    label: () =>
      h(
        RouterLink,
        {
          to: {
            name: 'stock',
            query: { groupName: '全部', groupId: 0 },
          },
          onClick: () => setKey('stock'),
        },
        { default: () => '自选股' },
      ),
    key: 'stock',
    icon: renderIcon(StarOutline),
    children: [
      {
        label: () =>
          h(
            RouterLink,
            {
              to: {
                name: 'stock',
                query: { groupName: '全部', groupId: 0 },
              },
              onClick: () => {
                setKey('stock')
                EventsEmit('changeTab', { ID: 0, name: '全部' })
              },
            },
            { default: () => '全部' },
          ),
        key: 0,
      },
    ],
  }

  return [
    routeLink('investmentHome', '投资驾驶舱', 'investmentHome', SparklesOutline),
    {
      // Phase17 release：组合总览 + 持仓做T 观察台（适宜性评价仍在组合表内）
      label: () =>
        h(
          RouterLink,
          {
            to: { name: 'portfolioDashboard' },
            onClick: () => setKey('portfolioDashboard'),
          },
          { default: () => '我的组合' },
        ),
      key: 'portfolio',
      icon: renderIcon(Wallet),
      children: [
        routeLink('portfolioDashboard', '组合总览', 'portfolioDashboard', Wallet),
        routeLink('holdingT', '持仓做T', 'holdingT', Pulse),
      ],
    },
    {
      // Phase17.1：机会子菜单 — 机会列表 + 跟踪股票（原 settings 隐藏项恢复可见）
      label: () =>
        h(
          RouterLink,
          {
            to: { name: 'stockScreen' },
            onClick: () => {
              setKey('stockScreen')
              EventsEmit('allStockListRefresh')
            },
          },
          { default: () => '机会' },
        ),
      key: 'opportunity',
      icon: renderIcon(AppsList20Regular),
      children: [
        routeLink('stockScreen', '机会列表', 'stockScreen', AppsList20Regular, {
          onClick: () => EventsEmit('allStockListRefresh'),
        }),
        routeLink('watchedOpportunities', '跟踪股票', 'watchedOpportunities', AppsList20Regular),
      ],
    },
    routeLink('tradePlanUpcoming', '交易计划', 'tradePlanUpcoming', TimeOutline),
    routeLink('research', '事件监控', 'eventMonitor', TrendingUp, {
      to: { name: 'research', query: { name: '异动监控' } },
      onClick: () => {
        setKey('eventMonitor')
        setTimeout(() => {
          EventsEmit('changeResearchTab', { ID: 2, name: '异动监控' })
        }, 100)
      },
    }),
    stockNode,

    {
      label: () =>
        h(
          RouterLink,
          {
            to: { name: 'settings', query: { name: '设置' } },
            onClick: () => setKey('settings'),
          },
          { default: () => '设置' },
        ),
      key: 'settings',
      icon: renderIcon(SettingsOutline),
      children: [
        routeLink('settings', '系统设置', 'settingsPage', SettingsOutline, {
          to: { name: 'settings', query: { name: '设置' } },
          onClick: () => setKey('settings'),
        }),
        routeLink('about', '关于', 'about', SettingsOutline),
        routeLink('cronTasks', '定时任务', 'cronTasks', TimeOutline, { show: false }),
        routeLink('commercialDemo', '订阅演示', 'commercialDemo', DiamondOutline, { show: false }),
        {
          label: () =>
            h(
              RouterLink,
              {
                to: { name: 'fund', query: { name: '基金自选' } },
                onClick: () => setKey('fund'),
              },
              { default: () => '基金自选' },
            ),
          show: !!enableFund?.value,
          key: 'fund',
          icon: renderIcon(SparklesOutline),
          children: [
            {
              label: () =>
                h(
                  NText,
                  { type: realtimeProfit?.value > 0 ? 'error' : 'success' },
                  { default: () => '功能完善中！' },
                ),
              key: 'realtimeProfit',
              show: !!realtimeProfit?.value,
              icon: renderIcon(AlarmOutline),
            },
          ],
        },
        {
          label: () =>
            h(
              RouterLink,
              {
                to: { name: 'agent', query: { name: 'Ai智能体' } },
                onClick: () => setKey('agent'),
              },
              { default: () => '对话' },
            ),
          key: 'agent',
          show: !!enableAgent?.value,
          icon: renderIcon(Robot),
        },
        // Debug / legacy — hidden from product shell
        routeLink('tradingDayMonitor', '今日流程', 'tradingDayMonitor', AnalyticsOutline, {
          show: false,
        }),
        routeLink('paperObservation', '执行记录', 'paperObservation', AnalyticsOutline, {
          show: false,
        }),
        routeLink('quantTrading', '量化交易', 'quantTrading', AnalyticsOutline, { show: false }),
        routeLink('holdings', '持仓明细', 'holdings', Wallet, { show: false }),
        routeLink('market', '市场资讯', 'market', NewspaperOutline, { show: false }),
        routeLink('strategySchemas', '策略规则', 'strategy_schemas', FlaskOutline, { show: false }),
        routeLink('phase9C3Observation', 'Observation·C.3', 'phase9C3Observation', FlaskOutline, {
          show: false,
        }),
        routeLink('productionReadiness', '生产就绪', 'productionReadiness', Flag, { show: false }),
        routeLink('recoveryReadiness', '恢复就绪', 'recoveryReadiness', Pulse, { show: false }),
        routeLink('brokerReconcile', 'Broker 对账', 'brokerReconcile', GitCompareOutline, {
          show: false,
        }),
        routeLink(
          'portfolioDecisionDashboard',
          '决策差异分析',
          'portfolioDecisionDashboard',
          AnalyticsOutline,
          { show: false },
        ),
      ],
    },

    {
      show: false,
      label: () =>
        h(
          'a',
          { href: '#', onClick: ctx.toggleFullscreen, title: '全屏 Ctrl+F 退出全屏 Esc' },
          { default: () => (ctx.isFullscreen?.value ? '取消全屏' : '全屏') },
        ),
      key: 'full',
      icon: renderIcon(ExpandOutline),
    },
    {
      show: false,
      label: () => h('a', { href: '#', onClick: ctx.Quit }, { default: () => '退出程序' }),
      key: 'exit',
      icon: renderIcon(PowerOutline),
    },
  ]
}

/** Recursively find menu node by key. */
export function findMenuItemByKey(items, key) {
  if (!Array.isArray(items)) return null
  for (const item of items) {
    if (item?.key === key) return item
    const nested = findMenuItemByKey(item?.children, key)
    if (nested) return nested
  }
  return null
}

/** Walk menu tree and apply visitor. */
export function forEachMenuItem(items, fn) {
  if (!Array.isArray(items)) return
  for (const item of items) {
    fn(item)
    if (item?.children) forEachMenuItem(item.children, fn)
  }
}

/** Shell window / document titles for product routes. */
export const ROUTE_PAGE_TITLES = {
  investmentHome: '投资驾驶舱',
  portfolioDashboard: '我的组合',
  holdings: '持仓明细',
  stockScreen: '机会列表',
  watchedOpportunities: '跟踪股票',
  research: '事件监控',
  cronTasks: '定时任务',
  tradePlanUpcoming: '交易计划',
  tradingDayMonitor: '今日流程',
  holdingT: '持仓做T',
  quantTrading: '量化交易',
  paperObservation: '执行记录',
  portfolioDecisionDashboard: '决策差异分析',
  phase9C3Observation: 'Observation',
  settings: '设置',
  about: '关于',
  stock: '自选股',
  market: '市场资讯',
  fund: '基金自选',
  agent: '助手',
  productionReadiness: '生产就绪',
  recoveryReadiness: '恢复就绪',
  brokerReconcile: 'Broker 对账',
  commercialDemo: '订阅演示',
  strategySchemas: '策略规则',
  strategySchemaDetail: '策略规则详情',
  strategyRevisionView: '策略规则版本',
}

export function pageTitleForRoute(name) {
  return ROUTE_PAGE_TITLES[String(name || '')] || 'go-stock'
}

/** Hash paths kept for debug after menu hide (router.js unchanged). */
export const PHASE15_A_HIDDEN_HASH_PATHS = [
  '#/quant-trading',
  '#/phase9-c3-observation',
  '#/production-readiness',
  '#/recovery-readiness',
  '#/broker-reconcile',
  '#/holdings',
  '#/research?name=%E6%A8%A1%E6%8B%9F%E7%9B%98',
  '#/research?name=%E6%A8%A1%E6%8B%9F%E5%88%B8%E5%95%86(RealStub)',
]
