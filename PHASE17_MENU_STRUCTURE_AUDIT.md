# PHASE17 菜单结构恢复审计（只读）

**日期：** 2026-09-07  
**性质：** 只读审计（**未改代码 / 未自动修复 / 未格式化**）  
**背景：** 菜单曾重构为产品一级导航；近期硬盘空间不足后做过 git 恢复 / 拉取 / 构建。怀疑菜单相关页面回退。  
**对照目标（用户给定）：**

```text
投资驾驶舱
├── 我的组合
├── 机会
├── 交易计划
├── 事件监控
├── 自选股
└── 设置
```

---

## 0. 总判

# **FAIL**

| 维度 | 结论 |
| --- | --- |
| 运行时实际菜单（`App.vue` → `n-menu`） | **旧研发平铺菜单**（自选股为首项；无「投资驾驶舱」一级） |
| 产品菜单模块 `productMenu.js` | **磁盘存在但未接入 App**；且结构是 Phase15-A（首页/研究/交易…），**也不是**用户目标扁平树 |
| 路由层 | **工作区已含**驾驶舱 / 组合 / 机会 / monitor 等（相对 HEAD 有增量） |
| 与设计目标对齐 | **偏离** |
| Git | 产品菜单接线 **从未进入 HEAD**；`App.vue` = HEAD 旧壳；`productMenu.js` **未跟踪** |

---

## 1. 路由审计

**文件：** `frontend/src/router/router.js`  
**相对 HEAD：** 有未提交改动（`M`）；默认首页与产品路由已在工作区补齐。

### 1.1 当前实际 route（name）

| name | path | 说明 |
| --- | --- | --- |
| *(redirect)* | `/` → `investmentHome` | J.6：默认进驾驶舱 |
| `stock` | `/stock` | 自选股（兼容；不再占 `/`） |
| `investmentHome` | `/investment-home` | 投资驾驶舱页 |
| `portfolioDashboard` | `/portfolio` | 我的组合 |
| `watchedOpportunities` | `/opportunities/watched` | 跟踪机会 |
| `stockScreen` | `/stock-screen` | 选股 / 机会 |
| `tradePlanUpcoming` | `/trade-plan-upcoming` | 交易计划 |
| `tradingDayMonitor` | `/trading-monitor` | 今日流程 / 事件向监视 |
| `paperObservation` | `/paper-observation` | 执行记录 |
| `settings` | `/settings` | 设置 |
| `holdings` / `holdingT` / `quantTrading` | 各自 path | 旧持仓/做T/量化 |
| `research` / `market` / `agent` / `fund` / `about` / `cronTasks` | 各自 path | 研究/行情/助手等 |
| `strategySchemas*` | `/strategy-schemas*` | 策略规则 |
| `productionReadiness` / `recoveryReadiness` / `brokerReconcile` / `phase9C3Observation` / `portfolioDecisionDashboard` / `commercialDemo` | 各自 path | 运维/演示 |

**HEAD（已提交）路由缺口：** `/` 仍是 `stock`；**无** `investmentHome` / `portfolio` / `tradingDayMonitor` 等。工作区 router 已修复这些缺口，但菜单层未跟上。

### 1.2 Layout / App

| 文件 | 角色 | 现状 |
| --- | --- | --- |
| `frontend/src/App.vue` | **唯一壳菜单**：内联 `menuOptions` → `<n-menu :options="menuOptions">` | **= HEAD**；**无** `createProductMenuOptions` / `rebuildProductMenu` / `FirstLaunchOnboarding` |
| `frontend/src/navigation/productMenu.js` | 设计中的「唯一产品菜单树」 | **`??` 未跟踪**；仅被 `router.js` 引用 `pageTitleForRoute`（标题表），**不驱动侧栏** |
| `frontend/src/layouts/*` | — | **不存在**独立 layout 菜单 |

### 1.3 当前实际菜单树（运行时 · App.vue）

```text
自选股                          → stock（默认 activeKey=stock）
  └── 全部 (+ GetGroupList 动态分组)
持仓股                          → holdings
持仓做T                         → holdingT
量化交易                        → quantTrading
明日交易计划                    → tradePlanUpcoming
模拟盘观察                      → paperObservation
套餐演示                        → commercialDemo
股票筛选                        → stockScreen
市场行情                        → market
  └── 市场快讯 / 全球股指 / … / 名站优选
基金自选                        → fund（feature）
Ai智能体                        → agent（feature）
研究中心                        → research
  ├── AI分析报告
  ├── 股票推荐记录
  ├── 异动监控
  ├── 提示词模板
  ├── 我的策略
  ├── 定时任务
  └── 交易日志(beta)
设置                            → settings
```

**缺失于菜单（但路由已有）：** `investmentHome`、`portfolioDashboard`、`tradingDayMonitor`、`watchedOpportunities` 等。

---

## 2. 菜单组件审计

### 2.1 显示名称（运行时）

| 检查点 | 结果 |
| --- | --- |
| 是否「投资驾驶舱」为首页入口 | **否**（无此项；默认高亮仍是自选股） |
| 是否旧「自选股」作一级首项 | **是** |
| 是否旧分组（持仓股 / 量化 / 研究中心 / 市场行情平铺） | **是** |
| 「我的组合」一级 | **否**（菜单写「持仓股」→ `holdings`，非 `/portfolio`） |
| 「机会」独立 | **否**（仅有「股票筛选」） |
| 「事件监控」独立 | **否**（仅有研究中心下「异动监控」；无独立 `tradingDayMonitor`） |
| 「设置」独立 | **是**（但仍是旧设置入口，未收拢产品子树） |

### 2.2 孤儿模块：`productMenu.js`（未驱动 UI）

磁盘内容（Phase15-A 实现报告口径）一级为：

```text
首页 → investmentHome
研究
  ├── 自选 → stock
  ├── 选股 / 机会 → stockScreen
  ├── 我的跟踪机会 → watchedOpportunities
  ├── 选股任务 / 研究名单 / 异动监控 / 市场资讯 / 策略规则 / 交易日志
交易
  ├── 交易计划 / 今日流程 / 执行记录 / 持仓做T …
我的组合
  └── 持仓列表 → portfolioDashboard
助手
设置
  └── 系统设置 / 关于 / 定时任务 …
```

标题映射里首页仍叫「首页」，不是「投资驾驶舱」。

### 2.3 `frontend/dist` 旁证（2026-09-07 12:06 构建）

| 字符串 | 出现 | 解读 |
| --- | --- | --- |
| `自选股` / `持仓股` / `明日交易计划` / `研究中心` | **有** | 打进包的是 **App.vue 旧菜单** |
| `createProductMenuOptions` | **0** | 菜单工厂未进 UI 图 |
| `投资驾驶舱` / `事件监控` | **0** | 目标文案未进菜单产物 |
| `选股 / 机会` / `今日流程` | **有** | 来自 `pageTitleForRoute` 标题表（router 引用），**不是**侧栏已切换 |

---

## 3. Git 恢复影响检查

### 3.1 状态摘要

| 路径 | Git | 说明 |
| --- | --- | --- |
| `frontend/src/App.vue` | **干净（= HEAD）** | hash 与 `HEAD:frontend/src/App.vue` 一致；内容为旧内联菜单 |
| `frontend/src/router/router.js` | **`M` 未提交** | 相对 HEAD：默认改驾驶舱 + 大量产品路由 + `pageTitleForRoute` |
| `frontend/src/navigation/` | **`??` 未跟踪** | `productMenu.js` **不在 HEAD**；`git cat-file` 确认不存在于提交树 |
| HEAD 最近相关提交 | `19cd8e6` / `441bd97`（2026-08-09）等 | 商业演示 / 观察 UI；**无**「菜单重构」正式合入记录 |

### 3.2 是否「被回滚」？

更准确的结论：

1. **产品菜单接线从未进入已提交历史。** Phase11-J.6 / Phase15-A 报告写过改 `App.vue` + `productMenu.js`，但当前仓库 **HEAD 仍是旧 App 菜单**；`productMenu.js` 仅残留为本地未跟踪文件。  
2. 硬盘恢复 / git 拉取之后，**权威源变成「已提交的旧 App.vue」**；磁盘上的 `productMenu.js` **没有挂回 App**，表现为「菜单回到旧版」。  
3. **不是**「router 被整文件 reset 掉」：工作区 router **仍保留**驾驶舱默认与产品路由（相对 HEAD 是前进态）。  
4. **App.vue 相对 HEAD 无 diff** → 本次并非「刚把 App 从产品版 checkout 回旧版」的痕迹；而是 **产品版 App 接线本就不在版本库中**。

### 3.3 历史设计对照（文档，非当前代码）

| 阶段 | 文档中的一级结构 |
| --- | --- |
| Phase11-J.6（实施报告） | 投资驾驶舱 · 我的组合 · 投资机会 · 策略中心 · 模拟交易 · 设置 |
| Phase15-A（实施报告 / 现 `productMenu.js`） | 首页 · 研究 · 交易 · 我的组合 · 助手 · 设置 |
| **用户本次目标** | 投资驾驶舱 · 我的组合 · 机会 · 交易计划 · 事件监控 · 自选股 · 设置 |
| **当前运行时** | 自选股 / 持仓股 / 量化 / 明日计划 / 研究中心…（J.2 审计时的研发平铺） |

三者与用户目标均不完全一致；**运行时离目标最远**。

---

## 4. 与设计目标对比

### 应该结构（用户给定）

```text
投资驾驶舱
├── 我的组合
├── 机会
├── 交易计划
├── 事件监控
├── 自选股
└── 设置
```

### 当前结构（运行时实际）

```text
自选股
├── 全部 (+ 分组)
持仓股
持仓做T
量化交易
明日交易计划
模拟盘观察
套餐演示
股票筛选
市场行情（多子项）
基金自选
Ai智能体
研究中心（多子项，含异动监控）
设置
```

### 逐项对照

| 目标项 | 当前 | 判定 |
| --- | --- | --- |
| 投资驾驶舱（一级 + 默认） | 路由默认已是 `investmentHome`；**菜单无入口**；页内文案偏「首页/我的资产」 | **FAIL**（壳不一致） |
| 我的组合 | 菜单是「持仓股」→ `holdings`，不是 `/portfolio` | **FAIL** |
| 机会 | 仅「股票筛选」；无独立「机会」命名 | **FAIL** |
| 交易计划 | 「明日交易计划」文案偏旧 | **部分**（路由对，文案/层级偏） |
| 事件监控 | 无独立一级；异动监控埋在研究中心 | **FAIL** |
| 自选股独立 | 仍是**旧首项**，不是「与驾驶舱并列的独立产品项」 | **FAIL**（位置语义旧） |
| 设置独立 | 有 | **PASS（仅有入口）** |

---

## 5. 修复建议（只列范围，不改代码）

> 以下为恢复/对齐建议，**本审计未执行任何修改。**

### 5.1 必须恢复 / 重接的文件

| 优先级 | 文件 | 建议 |
| --- | --- | --- |
| **P0** | `frontend/src/App.vue` | 重新接入 `createProductMenuOptions` / `rebuildProductMenu`（或等价）；去掉驱动侧栏的旧内联 `menuOptions` 树；`activeKey` 默认改为驾驶舱；保留 `GetGroupList` 注入自选分组的逻辑（挂到 `stock` 节点） |
| **P0** | `frontend/src/navigation/productMenu.js` | 1）纳入版本管理；2）按**用户目标扁平树**改标签与一级项（「首页」→「投资驾驶舱」；提升「机会」「交易计划」「事件监控」「自选股」为一级；收敛研究/助手/运维到设置或隐藏） |
| **P1** | `frontend/src/router/router.js` | **保留工作区现状**（已含默认驾驶舱与产品路由）；建议正式提交，避免再次被 HEAD 旧路由覆盖 |
| **P1** | 前端产物 | 接线后 `npm run build`，再 `wails build -skipbindings -s`（勿清 `stock.db`） |

### 5.2 建议不要指望「仅 git checkout」就能还原

- HEAD **没有**已接线的产品版 `App.vue`。  
- 仓库内 **未找到** 含 `createProductMenuOptions` 的其它 `App.vue` 副本。  
- 需依据：`PHASE11_J6_NAVIGATION_IMPLEMENTATION_REPORT.md` / `PHASE15_A_MENU_CLEANUP_IMPLEMENTATION_REPORT.md` + 用户目标树 **重接**，而不是简单 `git restore`。

### 5.3 「事件监控」映射建议（实施时再定，本审计不改）

| 候选路由 | 现名 | 备注 |
| --- | --- | --- |
| `tradingDayMonitor` | 今日流程 | 更接近「交易日事件/流程」 |
| `research?name=异动监控` | 异动监控 | 行情异动，非交易日流程 |

用户目标「事件监控」需在修复切片中显式选定其一，避免两套并存。

---

## 6. PASS / FAIL 汇总

| # | 检查项 | 结论 |
| --- | --- | --- |
| 1 | 路由具备驾驶舱/组合/机会/计划/设置等 | **工作区 PASS**；**HEAD FAIL** |
| 2 | 菜单组件显示目标结构 | **FAIL**（旧平铺） |
| 3 | Git：菜单文件是否被回滚 | **实质：产品菜单未入库 + App 未接线**（表现为旧菜单） |
| 4 | 与用户设计目标一致 | **FAIL** |
| — | **总评** | **FAIL** |

### 恢复范围（结论清单）

1. **`App.vue`** — 重新挂接产品菜单（当前 = 旧壳）  
2. **`productMenu.js`** — 跟踪入库，并改成目标一级树（当前孤儿且为 15-A 形态）  
3. **`router.js`** — 保留并提交工作区产品路由（防回退）  
4. **重建 `frontend/dist` + exe** — 使 GUI 与源码一致  

---

*只读审计结束；未修改任何代码、API 或数据库。*
