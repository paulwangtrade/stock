import { createRouter, createWebHashHistory } from 'vue-router'
import { recordPerformanceMetric } from '../services/performanceMetrics'
import { pageTitleForRoute } from '../navigation/productMenu'

const routes = [
    // J.6: default home → 投资驾驶舱; stock kept at /stock for compatibility
    { path: '/', redirect: { name: 'investmentHome' } },
    { path: '/stock', component: () => import('../components/stock.vue'), name: 'stock' },
    { path: '/holdings', component: () => import('../components/HoldingPositions.vue'), name: 'holdings' },
    { path: '/holding-t', component: () => import('../components/HoldingTPanel.vue'), name: 'holdingT' },
    { path: '/quant-trading', component: () => import('../components/QuantTradingDashboard.vue'), name: 'quantTrading' },
    { path: '/trade-plan-upcoming', component: () => import('../components/TradePlanUpcoming.vue'), name: 'tradePlanUpcoming' },
    { path: '/production-readiness', component: () => import('../components/ProductionReadiness.vue'), name: 'productionReadiness' },
    { path: '/recovery-readiness', component: () => import('../components/RecoveryReadiness.vue'), name: 'recoveryReadiness' },
    { path: '/broker-reconcile', component: () => import('../components/BrokerReconcile.vue'), name: 'brokerReconcile' },
    { path: '/paper-observation', component: () => import('../components/PaperTradingObservation.vue'), name: 'paperObservation' },
    { path: '/portfolio-decision-dashboard', component: () => import('../components/PortfolioDecisionDashboard.vue'), name: 'portfolioDecisionDashboard' },
    { path: '/investment-home', component: () => import('../components/InvestmentHome.vue'), name: 'investmentHome' },
    { path: '/portfolio', component: () => import('../components/PortfolioDashboard.vue'), name: 'portfolioDashboard' },
    { path: '/trading-monitor', component: () => import('../components/TradingDayMonitor.vue'), name: 'tradingDayMonitor' },
    { path: '/commercial-demo', component: () => import('../components/CommercialDemoFlow.vue'), name: 'commercialDemo' },
    { path: '/phase9-c3-observation', component: () => import('../components/Phase9C3ObservationPanel.vue'), name: 'phase9C3Observation' },
    { path: '/stock-screen', component: () => import('../components/stockScreen.vue'), name: 'stockScreen' },
    { path: '/opportunities/watched', component: () => import('../components/WatchedOpportunities.vue'), name: 'watchedOpportunities' },
    { path: '/fund', component: () => import('../components/fund.vue'), name: 'fund' },
    { path: '/settings', component: () => import('../components/settings.vue'), name: 'settings' },
    { path: '/about', component: () => import('../components/about.vue'), name: 'about' },
    { path: '/market', component: () => import('../components/market.vue'), name: 'market' },
    { path: '/agent', component: () => import('../components/agent-chat.vue'), name: 'agent' },
    { path: '/research', component: () => import('../components/researchIndex.vue'), name: 'research' },
    { path: '/cron-tasks', component: () => import('../components/cron-task-manager.vue'), name: 'cronTasks' },
    { path: '/strategy-schemas', component: () => import('../components/StrategySchemaList.vue'), name: 'strategySchemas' },
    { path: '/strategy-schemas/detail', component: () => import('../components/StrategySchemaDetail.vue'), name: 'strategySchemaDetail' },
    { path: '/strategy-schemas/revision', component: () => import('../components/StrategyRevisionView.vue'), name: 'strategyRevisionView' },

]

const router = createRouter({
    //history: createWebHistory(),
    history: createWebHashHistory(),
    routes,
})

let navigationStartedAt = 0
router.beforeEach(() => {
    navigationStartedAt = performance.now()
})
router.afterEach((to) => {
    recordPerformanceMetric('route.navigation', performance.now() - navigationStartedAt, {
        route: String(to.name || to.path),
    })
    const title = pageTitleForRoute(to.name)
    if (typeof document !== 'undefined') {
        document.title = title
    }
})

export default router
