# PHASE17.1「跟踪股票菜单入口」实现前审计（只读）

**日期：** 2026-09-07  
**性质：** 只读审计（**未改代码 / 未新增页面 / 未重构菜单**）  
**背景：** 机会页信号「跟踪」→ 跟踪列表页已存在；Phase17 产品菜单恢复后 **无可见入口**。  
**对照菜单：** 投资驾驶舱 · 我的组合 · 机会 · 交易计划 · 事件监控 · 自选股 · 设置

---

## 0. 总判

| 问题 | 结论 |
| --- | --- |
| 跟踪列表页面是否存在 | **是** — `WatchedOpportunities.vue` |
| 路由是否存在 | **是** — `#/opportunities/watched` · `watchedOpportunities` |
| 菜单是否有可见入口 | **否** — 仅在 `productMenu.js` 设置子项且 **`show: false`** |
| 跟踪写读链路是否仍在 | **是** — `POST /api/opportunities/action` → `GET /api/watchlist` |
| 与「自选股」是否同一数据 | **否** — 跟踪 ≠ `followed_stock` / `GetFollowList` |

**根因：** 页面与路由完整；菜单恢复时把「我的跟踪机会」藏进设置隐藏区，产品一级「机会」下未挂子菜单。

---

## 1. 页面组件审计

### 1.1 搜索命中（产品「跟踪股票」主路径）

| 关键词 | 主命中 | 说明 |
| --- | --- | --- |
| 跟踪 / 我的跟踪机会 | `WatchedOpportunities.vue` | **跟踪列表页** |
| Watchlist（机会观察） | `api/watchlist.ts` · `backend/api/watchlist.go` · `opportunity/watchlist.go` | 读模型 |
| Tracked / Watching | `latest_action=WATCH` / `watching_count` | 状态语义 |
| Follow | `stock.vue` / `GetFollowList` / `followed_stock` | **自选股**，与跟踪机会 **分离** |
| WatchlistStock* | `WatchlistStockCard.vue` 等 | **自选行情卡片**，非本页 |

### 1.2 页面

| 项 | 值 |
| --- | --- |
| **页面文件** | `frontend/src/components/WatchedOpportunities.vue` |
| **组件名** | `WatchedOpportunities`（路由懒加载） |
| **页内标题** | 「我的跟踪机会」 |
| **作用** | 展示当前 **WATCH** 中的机会观察列表；取消跟踪（写 IGNORE）；跳转/创建交易计划 Draft；开 K 线 |
| **功能状态** | **可用（GREEN）** — `onMounted` → `fetchWatchlist`；空态引导回「选股 / 机会」 |

相关前端（非列表页本体）：

| 文件 | 作用 |
| --- | --- |
| `frontend/src/components/stockScreen.vue` | 菜单「机会」落地壳，内嵌 `allStockList.vue` |
| `frontend/src/components/allStockList.vue` | 信号表「跟踪 / 取消跟踪」按钮 |
| `frontend/src/api/watchlist.ts` | `GET /api/watchlist` |
| `frontend/src/api/opportunities.ts` | `POST /api/opportunities/action`（WATCH/IGNORE） |
| `frontend/src/api/watchlistCreatePlan.ts` | `POST /api/tradeplans/watchlist-draft` |

---

## 2. Router 审计

**文件：** `frontend/src/router/router.js`

| 项 | 值 |
| --- | --- |
| **path** | `/opportunities/watched` |
| **name** | `watchedOpportunities` |
| **component** | `() => import('../components/WatchedOpportunities.vue')` |
| **Hash URL** | `#/opportunities/watched` |

**结论：** 路由 **已存在，无需补充**。可直链打开；缺的是菜单可见入口。

关联：

| name | path | 关系 |
| --- | --- | --- |
| `stockScreen` | `/stock-screen` | 「机会」一级；写跟踪的入口页 |
| `stock` | `/stock` | 「自选股」；**不是**跟踪列表 |

---

## 3. 菜单审计

**文件：** `frontend/src/navigation/productMenu.js`

### 3.1 当前可见一级（产品壳）

```text
投资驾驶舱
我的组合
机会              → stockScreen（无可见子项）
交易计划
事件监控
自选股            → stock（分组子项）
设置
```

### 3.2 「跟踪」相关菜单项现状

```text
设置
  └── …（隐藏）
      └── 「我的跟踪机会」 → watchedOpportunities   show: false
```

| 检查 | 结果 |
| --- | --- |
| 是否在版本库菜单定义中 | **是**（`routeLink('watchedOpportunities', …)`） |
| 是否对用户可见 | **否**（`show: false`） |
| 「机会」下是否有子菜单 | **否**（一级直达 `stockScreen`） |

### 3.3 应挂在何处（仅建议，本审计不改）

| 方案 | 结构 | 评价 |
| --- | --- | --- |
| **A（优先）** | 机会 → 机会列表 + 跟踪股票/我的跟踪机会 | 与「机会页点跟踪」心智一致；与自选隔离 |
| B | 自选股 → 跟踪股票 | **易混** followed vs WATCH；页内已写明「非自选」 |
| C | 设置下改为 `show: true` | 能进，但产品路径弱 |

推荐文案对齐：菜单可用「跟踪股票」或保留「我的跟踪机会」（页标题已是后者）；**实现切片再定，本审计不改。**

示意（方案 A，非实施）：

```text
机会
  ├── 机会列表     → stockScreen
  └── 跟踪股票     → watchedOpportunities   （或「我的跟踪机会」）
```

---

## 4. 数据链检查

### 4.1 机会页点击「跟踪」

```text
stockScreen.vue
  └── allStockList.vue
        watchOpportunityRow(row)
          → buildWatchActionPayload / isOpportunityWatched
          → postOpportunityAction({ action: WATCH | IGNORE, scanBatchKey, … })
                POST /api/opportunities/action
```

| 项 | 值 |
| --- | --- |
| **API（写）** | `POST /api/opportunities/action` |
| **动作** | `WATCH`（跟踪）/ `IGNORE`（取消） |
| **Store** | **无独立 Pinia/Vuex store**；列表页本地 `ref`；机会表行上 `latest_user_action` |
| **持久化** | 机会用户动作表（`UserOpportunityAction` / opportunity 包）；**latest=WATCH** 视为跟踪中 |
| **非目标表** | `followed_stock`（自选）**不写入** |

### 4.2 跟踪列表读取

```text
WatchedOpportunities.vue
  → fetchWatchlist()
      GET /api/watchlist
        → opportunity.WatchlistView（仅当前 WATCHING）
```

| 项 | 值 |
| --- | --- |
| **API（读）** | `GET /api/watchlist` |
| **挂载** | `RegisterWatchlistRoutes` / `WatchlistAssetMiddleware`（`main.go` 已挂） |
| **能否正常读** | **能**（页面与 API 仍在；缺的是菜单入口，不是读链断裂） |

### 4.3 与自选股边界

| | 跟踪机会 | 自选股 |
| --- | --- | --- |
| 入口页 | 机会 / 信号「跟踪」 | 自选股菜单 |
| 列表页 | `WatchedOpportunities` | `stock.vue` |
| 读 API | `/api/watchlist` | Wails `GetFollowList` 等 |
| 产品语义 | 机会观察 → 可建计划 | 关注池 / 行情 |

---

## 5. 最小修复方案（只列范围，不实施）

### 5.1 需要修改

| 文件 | 原因 |
| --- | --- |
| **`frontend/src/navigation/productMenu.js`** | 将 `watchedOpportunities` 从设置隐藏区挪到 **「机会」可见子树**（或为「机会」增加 children：机会列表 + 跟踪入口）；去掉对该项的 `show: false`（或仅对该项可见） |
| **`frontend/src/App.vue`（可选，小改）** | 若「机会」改为带子菜单，确认 `activeKey` / `directKeys` 在 `watchedOpportunities` 路由下高亮正确（例如高亮「机会」父级或子 key） |

### 5.2 不需要修改

| 项 | 原因 |
| --- | --- |
| **后端** | Watchlist / Action API 已存在 |
| **API** | 路由与契约完整 |
| **数据模型** | 无需新表 |
| **新页面** | `WatchedOpportunities.vue` 已够用 |
| **router.js** | path/name 已注册 |
| **机会页跟踪按钮逻辑** | 写链正常，非本缺口 |

### 5.3 明确不做（本问题范围）

- 不把跟踪并入自选股数据  
- 不重构整棵产品菜单  
- 不新增第二套跟踪列表页  

---

## 6. PASS / FAIL（入口维度）

| # | 检查项 | 结论 |
| --- | --- | --- |
| 1 | 页面组件存在且可用 | **PASS** |
| 2 | Router 映射存在 | **PASS** |
| 3 | 产品菜单可见入口 | **FAIL**（仅隐藏项） |
| 4 | 写读数据链完整 | **PASS** |
| — | **入口修复就绪** | **PASS**（最小改 `productMenu.js` ± App 高亮） |

---

*只读审计结束；未修改任何代码。*
