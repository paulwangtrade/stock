# PHASE17.1「持仓做T」菜单入口审计（只读）

**日期：** 2026-09-07  
**性质：** 只读审计（**未改代码 / 未改菜单 / 未新增页面 / 未改交易逻辑**）  
**背景：** Phase17 菜单恢复后「持仓做T」不可见；对照跟踪股票曾因 `productMenu` `show:false` 缺失入口。  
**对照产品壳：** 投资驾驶舱 · 我的组合 · 机会 · 交易计划 · 事件监控 · 自选股 · 设置

---

## 0. 总判（先读）

| 问题 | 结论 |
| --- | --- |
| 「持仓做T」**页面**是否存在 | **是** — `HoldingTPanel.vue` |
| **路由**是否存在 | **是** — `#/holding-t` · `holdingT` |
| **菜单是否可见** | **否** — `productMenu.js` 设置子项且 **`show: false`**（与跟踪股票曾同类） |
| 产品「T 卖 / 做T 下单」主链是否在该页 | **否** — 主链在 **我的组合 / K 线 footer → SellDraftDialog → t-sell Draft → 交易计划** |
| `HoldingTPanel` 实际是什么 | **自选成本仓观察台**（Follow + 5 分钟 K）；文案写明不自动下单、等信号模型 |

**缺失点：** 仅 **可见菜单入口**（若目标是恢复该观察页）。  
**不要混淆：** 恢复菜单 ≠ 恢复 paper_sim T 卖能力（后者入口已在组合/K 线，未丢）。

---

## 1. 页面存在性

### 1.1 搜索命中

| 关键词 | 主命中 | 说明 |
| --- | --- | --- |
| 持仓做T / 做T | `HoldingTPanel.vue`、菜单文案 | 独立页 |
| T+0 | 页内无强绑定；观察占位 | — |
| t-sell | `SellDraftDialog` / `tradePlansTSell.ts` / `TradePlanUpcoming` | **卖出草稿主链**，不经 HoldingT |
| SellDraft | `PortfolioDashboard`、K 线 footer、`ExitReviewDrawer` | 持仓操作入口 |
| position trade | 组合持仓表「卖出」列 | paper_sim |

### 1.2 页面组件

| 项 | 值 |
| --- | --- |
| **文件路径** | `frontend/src/components/HoldingTPanel.vue` |
| **组件名称** | `HoldingTPanel` |
| **页内标题** | 「持仓辅助决策工具」 |
| **当前作用** | 列出 `GetFollowRealtimeList` 中 **costVolume > 0** 的自选成本仓；选中后看成本/现价/浮盈 + **5 分钟 K**；**T 策略观察区为占位**（「等待信号模型接入」「未生成交易建议」） |
| **功能状态** | **YELLOW** — 观察 UI 可用；**无** Draft/Approve/Freeze/fill；markers 常空 |

相关（非该页，但是「做 T 卖」产品链）：

| 文件 | 作用 |
| --- | --- |
| `SellDraftDialog.vue` | `POST /api/tradeplans/t-sell/draft` |
| `api/tradePlansTSell.ts` | T-sell API 客户端 |
| `PortfolioDashboard.vue` | 持仓行卖出 + B.2 footer「卖出计划」 |
| `viewmodels/tradePlan/tSellFlow.js` | 交易计划页 T-sell 执行态展示 |

---

## 2. Router 审计

**文件：** `frontend/src/router/router.js`

| 项 | 值 |
| --- | --- |
| **path** | `/holding-t` |
| **name** | `holdingT` |
| **component** | `() => import('../components/HoldingTPanel.vue')` |
| **Hash** | `#/holding-t` |

**结论：** 路由完整，**无需补路由**。可直链打开；缺的是菜单可见性。

---

## 3. 当前入口审计

| 代号 | 入口 | 是否存在 | 指向 |
| --- | --- | --- | --- |
| **A** | 菜单「持仓做T」 | **否（隐藏）** | 本应 → `holdingT` / `HoldingTPanel` |
| **B** | 我的组合持仓行 | **是** | 「卖出」→ `SellDraftDialog`（**paper_sim T-sell**，不是 HoldingT 页） |
| **C** | K 线工作台 footer | **是**（B.2） | 「卖出计划」→ 同 `SellDraftDialog` |
| **D** | 其他 | **部分** | 退出复评抽屉等也可挂 `SellDraftDialog`；旧 App 内联菜单曾有一级「持仓做T」（已废弃） |

```text
菜单「持仓做T」(隐藏) ──► HoldingTPanel (Follow 观察)
我的组合 / K线 footer   ──► SellDraftDialog ──► t-sell Draft ──► 交易计划 / Execution
```

两条链 **数据源与目标不同**，勿当成同一功能丢入口。

---

## 4. 菜单定位建议（只建议，不改）

当前一级：

```text
投资驾驶舱 · 我的组合 · 机会 · 交易计划 · 事件监控 · 自选股 · 设置
```

| 方案 | 结构 | 理由 |
| --- | --- | --- |
| **A** | 我的组合 → 持仓列表 + **持仓做T** | 与 Phase15「交易」下挂做 T、以及「仓位旁路工具」心智接近；**适合恢复 HoldingTPanel 可见性**（最小改 `productMenu`，类同跟踪股票） |
| **B** | 交易计划 → 做T计划 | **不推荐挂 HoldingTPanel**：该页不是计划列表；真正 T-sell Draft 创建后已进交易计划页 |
| **C** | **不进独立菜单**，仅持仓/K 线操作 | **推荐作为产品主路径**：与 paper_sim 卖出一致；HoldingT 仍可 hash 调试或长期降级 |

**综合建议：**

1. 若用户要的「做T」= **纸面/模拟卖出计划**：选 **C**（已具备 B/C 入口），**不必**仅为菜单把 HoldingT 抬回一级/二级。  
2. 若用户要的「做T」= **历史上的 HoldingT 观察页**：选 **A**，最小修复与跟踪股票同模式（见 §6）。  
3. 实现前应在产品上确认目标是 **A 观察页** 还是 **C 卖出链**，避免恢复错误入口造成「点了做T却不能卖」的预期落差。

---

## 5. 数据链检查

### 5.1 HoldingTPanel（菜单名所指向页面）

| 依赖 | 是否复用 |
| --- | --- |
| Position / paper_sim Snapshot | **否** — `GetFollowRealtimeList`（自选成本） |
| SellDraftDialog | **否** |
| TradePlan / t-sell | **否** |
| Execution / Gateway fill | **否** |
| K 线 | **是** — `GetStockEastMoneyKLine` + FE `klineCache`（自绘短窗，非 Lightweight Modal） |

→ **无完整交易闭环**；仅为观察壳。

### 5.2 产品 T-sell（完整链路，已有）

```text
Portfolio / K线 footer
  → SellDraftDialog
      → POST /api/tradeplans/t-sell/draft
          → TradePlan（source_session=t_sell 等）
              → 交易计划页批准/冻结
                  → Execution（既有纸面执行链）
```

| 依赖 | 状态 |
| --- | --- |
| Position（paper_sim 可卖量等） | **有**（`portfolioSellEntry`） |
| SellDraftDialog | **有** |
| TradePlan | **有** |
| Execution | **有**（计划页 T-sell 反馈/执行态） |

→ **卖出做 T 主链完整**；不依赖菜单「持仓做T」。

---

## 6. 输出汇总

### 6.1 当前状态

| 维度 | 状态 |
| --- | --- |
| `HoldingTPanel` | 存在、可路由打开、观察级 |
| Router `holdingT` | 存在 |
| 菜单 | 定义存在、`show: false`（设置隐藏区） |
| 真实 T-sell | 组合/K 线入口正常 |

### 6.2 缺失点

1. **产品壳无「持仓做T」可见项**（与跟踪股票同类：`productMenu` 隐藏）。  
2. **命名混淆风险**：菜单名像交易能力，页面是 Follow 观察台。  
3. `App.vue` `directKeys` **无** `holdingT`（若恢复菜单高亮，需一并考虑；本审计不改）。

### 6.3 最小修复方案（只列范围，不实施）

**若确认恢复 HoldingT 观察页入口（方案 A）：**

| 需要修改 | 原因 |
| --- | --- |
| `frontend/src/navigation/productMenu.js` | 将 `holdingT` 从设置 `show:false` 挪到 **我的组合** 可见子项（如：持仓列表 + 持仓做T） |
| `frontend/src/App.vue`（可选） | `directKeys` 增加 `holdingT` 以利高亮 |

| 不需要修改 | 原因 |
| --- | --- |
| 后端 / API / 交易逻辑 | 观察页与 T-sell 链均已存在 |
| `router.js` | 已注册 |
| 新建页面 | `HoldingTPanel` 已够 |
| SellDraft / t-sell | 与本次菜单恢复无关 |

**若确认产品主路径为组合卖出（方案 C）：**

| 动作 | 说明 |
| --- | --- |
| **可不改菜单** | 保持 HoldingT 隐藏；文档/引导指向「我的组合 → 卖出 / K 线卖出计划」 |
| 可选后续 | 文案区分「持仓观察」vs「卖出计划」，避免用户找错页 |

---

## 7. 与「跟踪股票」对照

| | 跟踪股票 | 持仓做T |
| --- | --- | --- |
| 页面 | `WatchedOpportunities`（完整列表+取消跟踪+建计划） | `HoldingTPanel`（观察占位） |
| 路由 | 有 | 有 |
| 菜单隐藏 | 曾 `show:false`（已恢复机会子项） | 现 `show:false` |
| 最小修复 | 挂「机会」子菜单 | 若要恢复页：挂「我的组合」；若要做 T 卖：**不必恢复该页** |

---

*只读审计结束；未修改任何代码。*
