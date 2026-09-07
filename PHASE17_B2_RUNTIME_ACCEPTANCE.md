# PHASE17-B.2 Runtime 验收

**日期：** 2026-09-07  
**性质：** **只读 Runtime 验收**（未改代码、未修复、未重编）  
**依据：** [PHASE17_B2_IMPLEMENTATION_COMPLETE.md](./PHASE17_B2_IMPLEMENTATION_COMPLETE.md)  
**验收环境：** Windows · 运行中 `go-stock.exe`

---

## 0. 总判

# **FAIL**

| 维度 | 结论 |
| --- | --- |
| 源码 / `frontend/dist` 是否含 B.2 | **PASS**（`PortfolioDashboard-DgynV85E.js` 含「持仓摘要」「卖出计划」） |
| **桌面 exe 是否嵌入 B.2** | **FAIL**（exe 早于 dist，运行实例无 B.2 UI） |
| 组合→K线摘要 / 成本线 / footer 卖出（GUI） | **未通过**（被过期 exe 阻断） |
| B.1 缓存快路径（旁证） | **PASS**（热缓存探针仍 ~26ms） |

**阻断根因：** 当前运行的桌面包 **未嵌入** 2026-09-07 的 B.2 前端产物。

---

## 1. 环境快照

| 项 | 值 |
| --- | --- |
| 运行进程 | PID **19608**，启动 **2026-09-07 11:54:58** |
| exe 路径 | `D:\stock\build\bin\go-stock.exe` |
| **exe mtime** | **2026-09-06 23:54:37** |
| **dist mtime** | **2026-09-07 12:06:35** |
| B.2 页面 chunk | `PortfolioDashboard-DgynV85E.js`（含持仓摘要） |
| 市场时钟 | 当日 12:18 上海 · `isKlineMarketLive=false`（午休 IDLE） |
| 截图 | `_p17_b2_acceptance/runtime_stale_exe_1217.png` |

```text
exe  23:54:37 (9/6)  ← 运行中 / 嵌入旧前端
dist 12:06:35 (9/7)  ← 含 B.2，未打进 exe
```

---

## 2. 分项验收

### 2.1 我的组合页面

| 检查 | 结果 | 说明 |
| --- | --- | --- |
| PortfolioDashboard 可打开 | **未 GUI 点验** | 进程在跑；本验收未自动化导航 |
| 持仓列表正常 | **未 GUI 点验** | — |
| 点击股票名称/code | **未 GUI 点验** | 入口源码存在 `StockLink` → `openStockKline(m, row)` |

**源码侧（非 Runtime GUI）：** 接线已在 `PortfolioDashboard.vue`。  
**Runtime GUI：** 因 exe 无 B.2，即使打开组合页，也 **不会** 出现新摘要/footer 行为。

### 2.2 K 线 Modal 摘要字段

| 字段 | 预期 | Runtime | 备注 |
| --- | --- | --- | --- |
| 股票名称 | 显示 | **FAIL / 阻断** | dist 有「持仓摘要」模板；exe 无 |
| 股票代码 | 显示 | **FAIL / 阻断** | 同上 |
| 持仓数量 | 显示 | **FAIL / 阻断** | 同上 |
| 成本价 | 显示 | **FAIL / 阻断** | 同上 |
| 当前价 | 显示（有 overlay） | **FAIL / 阻断** | 同上 |
| 浮盈 | 显示 `pnl` | **FAIL / 阻断** | 同上 |

**dist 静态确认（实现在产物中）：**

```text
PortfolioDashboard-DgynV85E.js  持仓摘要=true  卖出计划=true
```

### 2.3 成本线

| 检查 | 结果 |
| --- | --- |
| Chart 支持 `costPrice` / `costVolume` | **源码 PASS**（既有 props；有价则 `showLongPosition=true`） |
| 组合开 K 注入 cost props | **源码 PASS**；**Runtime FAIL**（exe 未嵌入接线） |
| 不影响默认 K 线 / 周期切换 | **未在新包上验证**（逻辑上未改图表核心） |

### 2.4 Footer「卖出计划」

| 检查 | 结果 |
| --- | --- |
| 按钮可点击 | **Runtime FAIL / 阻断**（旧 exe 无 footer 接线） |
| 打开 SellDraftDialog | **未验证** |
| 参数为 position row | **源码设计 PASS**（`openSellDialog(klineModal.positionRow)`） |

### 2.5 边界

| 场景 | 结果 |
| --- | --- |
| 有持仓开 K | **未 GUI 验证**（阻断） |
| 无持仓开 K | **未 GUI 验证**；源码：`positionRow` 空则无 prepend/footer、无 cost 注入 |
| 快速切换多股票 | **未 GUI 验证** |

### 2.6 回归

| 项 | 结果 | 证据 |
| --- | --- | --- |
| K 线缓存 | **PASS（旁证）** | 热缓存探针 limit=800→120 bars，~26ms |
| LIVE 轮询逻辑 | **逻辑未改**；当时午休 `live=false` | 未做盘中实机 poll 观测 |
| 其他入口开 K | **未对比 GUI** | 未改 `stock.vue` 等入口 |

---

## 3. FAIL 问题清单（只记录，不修复）

### F1 — 桌面 exe 未嵌入 B.2（阻断）

| 项 | 内容 |
| --- | --- |
| **现象** | 运行中的 `go-stock.exe`（23:54:37）早于含 B.2 的 `frontend/dist`（12:06:35）；组合开 K **看不到**持仓摘要 / 卖出计划 footer / 组合注入的成本线 |
| **路径** | `D:\stock\build\bin\go-stock.exe` vs `D:\stock\frontend\dist` / `PortfolioDashboard-DgynV85E.js` |
| **建议修复点** | 关闭进程后执行 `npm run build` + `wails build -skipbindings -s`（保留 `stock.db`），再用**新 exe**重跑本验收清单；**不要**用旧 exe 验 B.2 |

### F2 — GUI 条目未完成点验

| 项 | 内容 |
| --- | --- |
| **现象** | 在 F1 解除前，摘要字段、成本线、footer、边界切换均无法给出真实 GUI PASS |
| **路径** | PortfolioDashboard → StockKlineModal（运行时） |
| **建议修复点** | 新 exe 启动后按实现完成报告 §4 手工清单逐条点验并补截图 |

---

## 4. 非 FAIL 旁证（实现已落地于 dist）

| 检查 | 结果 |
| --- | --- |
| 源码 `openStockKline(model, row)` | 存在 |
| `:cost-price` / `:cost-volume` / `#prepend` / `#footer` | 存在于 `PortfolioDashboard.vue` |
| Vite 产物含「持仓摘要」「卖出计划」 | **true** |
| B.1 假 miss / 快路径 | 探针 **PASS** |

→ **实现完成 ≠ Runtime 桌面验收完成。**

---

## 5. PASS / FAIL 汇总表

| # | 验收项 | 结论 |
| --- | --- | --- |
| 1 | 我的组合点击开 K | **FAIL**（exe 过期阻断） |
| 2 | Modal 摘要六字段 | **FAIL**（exe 过期阻断） |
| 3 | 成本线 | **FAIL**（exe 过期阻断） |
| 4 | Footer 卖出计划 | **FAIL**（exe 过期阻断） |
| 5 | 边界场景 | **FAIL**（未在新包验证） |
| 6 | 回归缓存/LIVE/其它入口 | 缓存旁证 **PASS**；其余 **未完整 GUI 验证** |
| — | **总评** | **FAIL** |

---

## 6. 重验前置条件（建议，本步骤未执行）

1. 结束当前 `go-stock`  
2. 用当前 `frontend/dist`（已含 B.2）执行 `wails build -skipbindings -s`  
3. 确认新 exe mtime **≥** dist mtime  
4. 启动新 exe，逐条重跑 §2  

---

*只读验收；未修改任何代码。*
