# PHASE17.1 跟踪股票菜单入口恢复 — 实现完成

**日期：** 2026-09-07  
**依据：** [PHASE17_1_TRACKING_STOCK_MENU_AUDIT.md](./PHASE17_1_TRACKING_STOCK_MENU_AUDIT.md)  
**性质：** 最小菜单接线（**未新建页面 / 未改后端 / API / WATCH 链 / 路由表**）

---

## 0. 结果

| 项 | 结果 |
| --- | --- |
| 「跟踪股票」菜单可见 | **PASS**（机会子项） |
| 路由仍为 `/opportunities/watched` | **PASS**（未改 `router.js`） |
| `npm run build` | **PASS**（exit 0） |

---

## 1. 修改文件

| 文件 | 变更 |
| --- | --- |
| `frontend/src/navigation/productMenu.js` | 「机会」改为带子菜单：机会列表 + 跟踪股票；从设置隐藏区移除原 `show:false` 项；更新 `ROUTE_PAGE_TITLES` |
| `frontend/src/App.vue` | `directKeys` 增加 `watchedOpportunities`，保证高亮（productMenu 已接入，仅补选中态） |

未改：`WatchedOpportunities.vue`、`router.js`、`allStockList` WATCH 写链、后端 / API。

---

## 2. 目标菜单结构

```text
机会
  ├── 机会列表     → stockScreen           /#/stock-screen
  └── 跟踪股票     → watchedOpportunities  /#/opportunities/watched
```

父级「机会」点击仍进入机会列表并触发 `allStockListRefresh`（与恢复前一级「机会」行为一致）。

---

## 3. 验证

| 检查 | 结论 |
| --- | --- |
| 菜单显示「跟踪股票」 | **PASS**（`productMenu` 可见子项，无 `show:false`） |
| 点击 → `/opportunities/watched` | **PASS**（`name: watchedOpportunities`，路由原样） |
| 页面加载 | **PASS**（既有 `WatchedOpportunities.vue` + `GET /api/watchlist`） |
| 机会页 WATCH 不受影响 | **PASS**（未改 `allStockList` / action API） |

桌面目视：`wails build -skipbindings -s` 后点「机会 → 跟踪股票」。

---

## 4. 禁止项遵守

| 禁止 | 遵守 |
| --- | --- |
| 新建页面 | 是 |
| 修改后端 / API / WATCH 数据链 | 是 |
| 修改路由逻辑（router.js） | 是 |
| 修改机会算法 / Watchlist 结构 | 是 |
| 增加新入口（除恢复本菜单） | 是 |

---

*Phase17.1 跟踪股票菜单入口恢复完成。*
