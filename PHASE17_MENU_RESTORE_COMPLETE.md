# PHASE17 菜单结构恢复 — 完成报告

**日期：** 2026-09-07  
**依据：** [PHASE17_MENU_STRUCTURE_AUDIT.md](./PHASE17_MENU_STRUCTURE_AUDIT.md)  
**性质：** 前端壳层菜单恢复（**未改业务逻辑 / 后端 / API / 交易链 / 未重构路由表**）

---

## 0. 结果

| 项 | 结果 |
| --- | --- |
| App.vue 接入 `productMenu.js` | **PASS** |
| 产品一级菜单对齐目标树 | **PASS** |
| `productMenu.js` 纳入 git index | **PASS**（`git add`，已 staged） |
| `router.js` 驾驶舱默认路由保留 | **PASS**（`/` → `investmentHome`） |
| `npm run build` | **PASS**（exit 0） |

---

## 1. 修改文件

| 文件 | 变更 |
| --- | --- |
| `frontend/src/App.vue` | 删除旧内联 `menuOptions`；引入 `createProductMenuOptions` / `findMenuItemByKey` / `forEachMenuItem`；`rebuildProductMenu()`；默认 `activeKey=investmentHome`；保留分组注入、`enableFund`/`enableAgent`、选中态 watch |
| `frontend/src/navigation/productMenu.js` | 按目标扁平树重组可见一级项；旧入口 `show: false` 隐藏；更新 `ROUTE_PAGE_TITLES` |
| `frontend/src/router/router.js` | **未改本切片逻辑**（工作区已有驾驶舱路由，保持） |

---

## 2. 恢复后的菜单结构

```text
投资驾驶舱     → investmentHome      /#/investment-home
我的组合       → portfolioDashboard  /#/portfolio
机会           → stockScreen         /#/stock-screen
交易计划       → tradePlanUpcoming   /#/trade-plan-upcoming
事件监控       → research?name=异动监控   （key: eventMonitor）
自选股         → stock               /#/stock（子项：全部 + 分组）
设置           → settings            /#/settings
```

语义对齐：

| 菜单 | 语义 |
| --- | --- |
| 自选股 | 独立研究入口 |
| 我的组合 | 持仓管理入口 |
| 机会 | 策略机会池入口 |
| 事件监控 | 市场事件/信号观察入口 |

**未恢复为可见项：** 持仓股、量化交易、研究中心平铺等（仅 `show: false` 保留 hash 调试入口）。

---

## 3. Git 完整性

| 路径 | 状态 |
| --- | --- |
| `frontend/src/navigation/productMenu.js` | **已 `git add`（staged / tracked）** |
| `frontend/src/App.vue` | 工作区已修改（未要求 commit） |
| `frontend/src/router/router.js` | 保留 `/` → `investmentHome` 及产品路由 |

---

## 4. 验收

### 4.1 构建

```text
Set-Location D:\stock\frontend
npm run build
→ vite build exit 0  PASS
```

产物旁证（`frontend/dist`）：含「投资驾驶舱」「我的组合」「机会」「交易计划」「事件监控」「自选股」「设置」。

### 4.2 菜单显示（源码 + 产物）

可见一级七项与目标一致（见 §2）。

### 4.3 路由点击

| 菜单 | route name | 预期 |
| --- | --- | --- |
| 投资驾驶舱 | `investmentHome` | 正常 |
| 我的组合 | `portfolioDashboard` | 正常 |
| 机会 | `stockScreen` | 正常 |
| 交易计划 | `tradePlanUpcoming` | 正常 |
| 事件监控 | `research` + query `异动监控` | 正常 |
| 自选股 | `stock` | 正常 |
| 设置 | `settings` | 正常 |

路由表未删改；仅菜单数据源切换。桌面 exe 需另行 `wails build -skipbindings -s` 后方可在 GUI 点验（本切片已完成 Vite 构建）。

---

## 5. 禁止项遵守

| 禁止 | 遵守 |
| --- | --- |
| 修改业务逻辑 | 是 |
| 修改后端 / API / 交易链 | 是 |
| 重构路由 | 是（未改 path/name 集合意图） |
| 重新设计菜单 / 新功能 / 改页面内容 | 是 |

---

*Phase17 菜单结构恢复实现完成。*
