# PHASE17 Menu Capability Visibility Audit

**日期：** 2026-09-07  
**性质：** 发布前菜单 / 入口可达性审计（随后最小修复）  
**壳：** `App.vue` → `createProductMenuOptions`（`productMenu.js`）+ `router.js`

---

## 0. 总判（修复前）

| 维度 | 结论 |
| --- | --- |
| 主菜单 → router → component | **基本完整**（产品壳已接线） |
| Phase17 组合内能力（Health / 做T适宜性 / Origin / Data Truth） | **可从「我的组合」表格到达** |
| 独立「持仓做T」页 (`HoldingTPanel`) | **路由在、菜单 `show: false` → 用户不可见** |
| 独立「风险 / 选股」一级菜单 | **无**；能力分别落在组合风险标签 / 「机会列表」 |

---

## 1. 主菜单 → Router → Component

| 菜单 | key / name | path | component | 可见 |
| --- | --- | --- | --- | --- |
| 投资驾驶舱 | investmentHome | `/investment-home` | InvestmentHome.vue | ✅ |
| 我的组合 | portfolioDashboard | `/portfolio` | PortfolioDashboard.vue | ✅ |
| 机会 → 机会列表 | stockScreen | `/stock-screen` | stockScreen.vue | ✅ |
| 机会 → 跟踪股票 | watchedOpportunities | `/opportunities/watched` | WatchedOpportunities.vue | ✅ |
| 交易计划 | tradePlanUpcoming | `/trade-plan-upcoming` | TradePlanUpcoming.vue | ✅ |
| 事件监控 | research (eventMonitor) | `/research` | researchIndex.vue | ✅ |
| 自选股 | stock | `/stock` | stock.vue | ✅ |
| 设置 / 关于 | settings / about | `/settings` `/about` | settings / about | ✅ |
| 持仓做T | holdingT | `/holding-t` | HoldingTPanel.vue | ❌ `show: false` |
| 执行记录等 | paperObservation… | 有路由 | 有 | ❌ 故意隐藏（调试） |

**「选股」：** 无同名一级项；等价入口 = **机会 → 机会列表**。  
**「风险」：** 无一级项；组合页 risk 标签 + 驾驶舱风险摘要。

---

## 2. 重点能力

### 2.1 持仓做 T

| 入口 | 状态 |
| --- | --- |
| Portfolio「做 T」列 + `HoldingTSuitabilityDrawer` | ✅ 代码在；数据经 exit-evaluation |
| 菜单「持仓做T」→ HoldingTPanel（Follow/5m 观察台） | ❌ 隐藏 |
| 路由 `#/holding-t` | ✅ 直链可用 |

**缺口：** 独立观察台入口丢失（适宜性评价不依赖该页，但用户找不到「持仓做T」页）。

### 2.2 Holding Health

| 健康列 → Drawer | ✅ `HoldingHealthDrawer` |

### 2.3 Position Explanation / Provenance

| 「为什么买入」/ 来源 chip → `PositionOriginDrawer` | ✅ |
| 「完整溯源」→ `PortfolioProvenanceDrawer` | ✅ |

### 2.4 Portfolio Data Truth

| 估值价 / 行情价+时间 / 持仓今日浮盈 | ✅ 列与 tooltip 已实现 |

### 2.5 TradePlan Origin

| 交易计划页 Origin Panel / Explanation Drawer | ✅（页内，非独立菜单） |

### 2.6 Opportunity / Signal

| 机会列表扫描 / 信号标签 / OpportunityProjectionDrawer | ✅ |

---

## 3. Capability Matrix（修复前）

| 功能 | 后端 | 前端 | 菜单 | 状态 |
| --- | --- | --- | --- | --- |
| Holding Health | OK | OK | 经「我的组合」 | PASS |
| 持仓做T 适宜性 | OK | OK | 经「我的组合」列 | PASS |
| 持仓做T 观察台 HoldingT | OK | OK | **缺（隐藏）** | **FIX** |
| Provenance / Origin | OK | OK | 经「我的组合」 | PASS |
| Portfolio Data Truth | OK | OK | 经「我的组合」 | PASS |
| TradePlan Origin | OK | OK | 经「交易计划」 | PASS |
| Opportunity / Signal | OK | OK | 经「机会」 | PASS |
| 风险一级菜单 | N/A | 页内 | 无一级 | N/A（不扩功能） |

---

## 4. 修复计划（最小）→ 已执行

1. ✅ 「我的组合」子项：组合总览 + **持仓做T**（移出 settings 隐藏区）。  
2. ✅ `App.vue` `activeKey` 同步 `holdingT`。  
3. ✅ 组合页脚补充 Health / 做T / Origin 可达提示。  
4. ✅ 未改 Broker / Settlement / DB / 交易逻辑。

构建见：`PHASE17_RELEASE_BUILD_REPORT.md` · exe：`D:\stock\build\bin\go-stock.exe`

---

*审计 + 修复 + 发布构建完成。*
