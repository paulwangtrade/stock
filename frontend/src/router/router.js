import { createRouter, createWebHashHistory } from 'vue-router'
import { recordPerformanceMetric } from '../services/performanceMetrics'

const routes = [
    { path: '/', component: () => import('../components/stock.vue'), name: 'stock' },
    { path: '/holdings', component: () => import('../components/HoldingPositions.vue'), name: 'holdings' },
    { path: '/holding-t', component: () => import('../components/HoldingTPanel.vue'), name: 'holdingT' },
    { path: '/quant-trading', component: () => import('../components/QuantTradingDashboard.vue'), name: 'quantTrading' },
    { path: '/trade-plan-upcoming', component: () => import('../components/TradePlanUpcoming.vue'), name: 'tradePlanUpcoming' },
    { path: '/paper-observation', component: () => import('../components/PaperTradingObservation.vue'), name: 'paperObservation' },
    { path: '/commercial-demo', component: () => import('../components/CommercialDemoFlow.vue'), name: 'commercialDemo' },
    { path: '/stock-screen', component: () => import('../components/stockScreen.vue'), name: 'stockScreen' },
    { path: '/fund', component: () => import('../components/fund.vue'), name: 'fund' },
    { path: '/settings', component: () => import('../components/settings.vue'), name: 'settings' },
    { path: '/about', component: () => import('../components/about.vue'), name: 'about' },
    { path: '/market', component: () => import('../components/market.vue'), name: 'market' },
    { path: '/agent', component: () => import('../components/agent-chat.vue'), name: 'agent' },
    { path: '/research', component: () => import('../components/researchIndex.vue'), name: 'research' },
    { path: '/cron-tasks', component: () => import('../components/cron-task-manager.vue'), name: 'cronTasks' },

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
})

export default router