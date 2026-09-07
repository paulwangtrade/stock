# PHASE17 Final Acceptance Report

**日期：** 2026-09-08  
**性质：** **只读终验**（不新增功能 · 不修复 · 不提交）  
**目标：** 冻结 Phase17 已交付能力边界，核对「代码 / 报告 / UI 入口」一致性与交易链隔离。  

**冻结对象（本报告）：** 用户点名的实现矩阵  
C1 · C2 · C3 · C4 · C5/T-Suitability · 17.2 Data Truth · **17.4 Kline Signal Timeframe** · 17.5 Position Origin  

**明确不纳入「已实现冻结」的设计-only 旁路（仅登记）：**  
- Phase17.3 Holding Evaluation Snapshot（设计）  
- Phase17.4 Signal Outcome Projection（设计；与下方「17.4 Kline」**编号撞车**）  
- Phase17.4.1 Signal Identity（设计）

---

## 0. 总判（冻结结论）

| 维度 | 结论 |
| --- | --- |
| **产品能力（工作区）** | Holding Intelligence 主能力 **基本齐备且可达**（组合页） |
| **架构隔离** | **PASS** — 评价层旁路挂载；未见改 CandidatePool / TradePlan 生成 / Broker / Settlement |
| **Git 冻结完整性** | **FAIL / 不完整** — tag `phase17-checkpoint` **未包含** T-Suitability 源码与接线；工作区仍有未提交改动 |
| **文档编号** | **FINDING** — 「17.4」同时用于 Kline Timeframe（已实现）与 Signal Outcome（仅设计） |
| **本阶段动作** | **仅记录 · 停止**（按指令不修复） |

**建议冻结表述（供后续人工决策，本报告不执行）：**  
- **能力冻结**：以工作区功能矩阵为准（见 §1）。  
- **源码冻结**：当前 **不能** 声称 tag = 完整 Phase17；需另一次 checkpoint 才可对齐 C5/菜单修复。

---

## 1. 功能矩阵：代码 · 报告 · UI

图例：✅ 一致 · ⚠ 部分 · ❌ 缺口 · — 设计意图无代码

| ID | 能力 | 报告 | 代码（工作区） | UI 入口 | 三方一致？ | 验收 |
| --- | --- | --- | --- | --- | --- | --- |
| **C1** | Position Model | `PHASE17_C1_POSITION_MODEL_AUDIT.md` | —（审计-only，无专用新模块） | — | ✅ 报告声明不改代码 | **PASS**（基线审计） |
| **C2** | Explanation | `PHASE17_C2_…_IMPLEMENTATION.md` | `position_evaluation_explanation.go` (+test) | 经 ExitEval → 健康 Drawer 信号/原因；组合页解释 enrichment | ✅ | **PASS** |
| **C3** | HealthScore | `PHASE17_C3_…_IMPLEMENTATION.md` | `holding_health_score.go` (+test) | 我的组合 → **健康 / 健康摘要** → `HoldingHealthDrawer` | ✅ | **PASS** |
| **C4** | Portfolio Display | `PHASE17_C4_…_IMPLEMENTATION.md` | `PortfolioDashboard` + `holdingHealthDisplay.js` + Drawer | 同上列/抽屉 | ✅ | **PASS** |
| **C5** | T-Suitability | `PHASE17_C5_T_SUITABILITY_AUDIT.md` + `PHASE17_1_T_SUITABILITY_*` | `holding_t_suitability.go` (+test)；Exit 挂载 `t_suitability`；FE Drawer/mapper | 我的组合 → **做 T** → `HoldingTSuitabilityDrawer`；菜单「持仓做T」观察台 | ⚠ **工作区齐 / tag 缺** | **CONDITIONAL PASS**（见 FINDING-G1） |
| **17.2** | Data Truth | `PHASE17_2_PORTFOLIO_DATA_TRUTH_*` | `portfolio/readmodel` `quote_price` / `quote_timestamp` / `price_freshness`；`portfolioQuoteDisplay.js` | 估值价 · 行情价+时间 · 持仓今日浮盈 | ✅ | **PASS** |
| **17.4** | Kline Signal Timeframe | `PHASE17_4_KLINE_SIGNAL_TIMEFRAME_IMPLEMENTATION_COMPLETE.md` | `signalTimeframe.js`；`StockLightweightKlineChart.vue` 日K门闩 | K 线工具栏「冰点/买卖点」；非日K提示 | ✅ | **PASS** |
| **17.5** | Position Origin | `PHASE17_5_POSITION_ORIGIN_EXPLANATION_IMPLEMENTATION.md` | `PositionOriginDrawer.vue` · `positionOriginDisplay.js` | 来源 chip / **为什么买入**；完整溯源入口 | ✅ | **PASS** |

### 1.1 入口路径（用户视角 · 工作区）

```text
我的组合 → 组合总览
  ├─ 健康 / 健康摘要 → HoldingHealthDrawer          (C3/C4)
  ├─ 做 T → HoldingTSuitabilityDrawer               (C5 / 17.1)
  ├─ 来源 / 为什么买入 → PositionOriginDrawer         (17.5)
  └─ 估值价 · 行情价 · 持仓今日浮盈                   (17.2)

我的组合 → 持仓做T → HoldingTPanel                   (观察台；菜单可见性修复在工作区)

K 线页 → 周期切换 → 冰点仅日K                        (17.4 Kline)
```

### 1.2 编号与范围说明（记录）

| 名称 | 状态 | 是否本终验「实现冻结」 |
| --- | --- | --- |
| 17.4 **Kline Signal Timeframe** | 已实现 + 报告 | **是** |
| 17.4 **Signal Outcome Projection** | 仅设计 | **否** |
| 17.4.1 Signal Identity | 仅设计 | **否** |
| 17.3 Holding Evaluation Snapshot | 仅设计 | **否** |
| C5 审计 vs Phase17.1 实现 | 审计→设计→实现文档链存在 | 实现以 17.1 报告为准 |

---

## 2. 架构检查：评价层 vs 交易执行

### 2.1 交易链是否被评价层修改

| 环节 | 评价层关系 | 结论 |
| --- | --- | --- |
| **CandidatePool** | 只读引用（Explanation hints / Origin）；不改 Score/Rank/生成 | **未改生产逻辑（审计口径）** |
| **TradePlan** | 只读来源叙事；不改 Freeze/Execute | **隔离** |
| **Execution** | ExitEval **拷贝** Explanation/Health/TSuit；注释明确 **不驱动 Exit state** | **隔离** |
| **Broker** | `holding_t_suitability.go` 等声明不调用 Broker；无买卖指令 DTO | **隔离** |
| **Settlement** | Data Truth：**禁止** quote→mark；Settlement 估值列与行情列分离 | **隔离** |

### 2.2 管道（持仓智能）

```text
HoldingEval
  → Explanation (C2)
  → HealthScore (C3)
  → TSuitability (C5/17.1)
  → ExitEval（旁路字段拷贝；state 不因 Health/T 改变）
```

**Holding Intelligence ⊥ Trading Execution：**  
评价输出为只读投影 / Drawer；组合页无自动卖出按钮；T-Suitability 文案为「非交易指令」。

**架构验收：PASS**（在「不扩大范围重审全仓 diff」前提下，以声明、挂载点与既有实现报告交叉确认）。

---

## 3. Git 检查

| 项 | 值 |
| --- | --- |
| **当前 branch** | `release/v0.1.0-beta`（跟踪 `origin/release/v0.1.0-beta`） |
| **HEAD commit** | `6fee00c16f0b295dee268ce9e70c5e4472b6d936` |
| **短 hash** | `6fee00c` |
| **Message** | `checkpoint: Phase17 holding intelligence and portfolio truth` |
| **Tag** | `phase17-checkpoint` → 同上 commit |
| **远程** | `origin/release/v0.1.0-beta` = `6fee00c`；`refs/tags/phase17-checkpoint` = `6fee00c` |

### 3.1 工作区相对 tag（关键）

**已修改（未提交）示例：**

- `backend/papertrading/exit_evaluation.go`（含 TSuitability 挂载）  
- `backend/papertrading/holding_evaluation_observation.go`  
- `frontend/src/api/paperObservation.ts`（TSuitability mapper）  
- `frontend/src/components/PortfolioDashboard.vue`（做 T 列等）  
- `frontend/src/App.vue` / `productMenu.js`（菜单可见性）

**未跟踪（未进 tag）示例：**

- `backend/papertrading/holding_t_suitability.go`  
- `backend/papertrading/holding_t_suitability_test.go`  
- `frontend/src/components/HoldingTSuitabilityDrawer.vue`  
- `frontend/src/utils/holdingTSuitabilityDisplay.js`  
- 若干 Phase17 文档：`PHASE17_1_T_SUITABILITY_*`、`PHASE17_4_1_*`、`PHASE17_MENU_VISIBILITY_AUDIT.md`、`PHASE17_RELEASE_BUILD_REPORT.md` 等  

**Tag 内已有（抽查）：** C2/C3 核心 go、Health/Origin Drawer、`signalTimeframe.js`、17.2 readmodel 字段、C4/17.2/17.4Kline/17.5 实现报告。

**Tag 内缺失（抽查）：** T-Suitability 全套源码；Exit 上 `t_suitability` 接线（checkpoint 版 Exit 仅 HealthScore）。

---

## 4. Build 状态（记录 · 本终验未重跑）

来源：`PHASE17_RELEASE_BUILD_REPORT.md`（2026-09-08 菜单审计后构建）+ 当前磁盘产物。

| 项 | 结果 | 备注 |
| --- | --- | --- |
| **npm run build** | PASS（~2m 28s） | Vite production |
| **go test**（Phase17 相关包） | PASS | `papertrading` / `portfolio/...` / `tradeplanorigin` |
| **go test ./backend/...** | FAIL（既有） | `TestBuildDraftTradePlan_UsesPositionSizerForAmount`（strategy；**非本阶段修复**） |
| **wails build -skipbindings -s** | PASS（~38.15s） | 使用当时前端 dist |
| **exe** | `D:\stock\build\bin\go-stock.exe` | ~92.4 MB；LastWriteTime **2026-09-08 00:04:41** |

**FINDING-B1：** 该 exe 由**含工作区前端改动的 dist** 打出，**不等于**仅 checkout `phase17-checkpoint` 的可复现构建。

**本终验：** 按指令 **未再次**执行 build/test（只记录既有状态）。

---

## 5. Findings 清单（只记录 · 不修复）

| ID | 严重度 | 说明 |
| --- | --- | --- |
| **FINDING-G1** | **高（冻结）** | Tag `phase17-checkpoint` **不含** T-Suitability 实现；工作区有完整 C5 能力但 **未 commit**。Git 冻结 ≠ 产品冻结。 |
| **FINDING-G2** | 中 | 菜单可达性修复（持仓做T 子菜单等）与部分 Phase17 报告仍为 **untracked/modified**，未进 tag。 |
| **FINDING-N1** | 中（文档） | **「Phase17.4」双重含义**：Kline Timeframe（实现）vs Signal Outcome（设计）。终验矩阵取用户指定的 **Kline**。 |
| **FINDING-N2** | 低 | C5 审计文档 vs Phase17.1 实现文档双名并存；对外需约定「C5 = 17.1 T-Suitability」。 |
| **FINDING-T1** | 低（债） | 全量 `backend/...` 仍有 PositionSizer 接线测试失败（既有债）。 |
| **FINDING-T2** | 低 | 测试覆盖评审已指出 C4/FE mapper 契约测偏弱（不阻塞能力存在性）。 |
| **FINDING-B1** | 中 | 现网 exe 基于脏工作区构建；从 tag 干净检出 **无法**复现含做 T 适宜性的同一二进制。 |
| **FINDING-D1** | 信息 | 17.3 / Signal Outcome / Signal Identity 为设计暂停项，**不应**标为 Phase17 已实现验收 PASS。 |

---

## 6. 冻结声明（Acceptance）

### 6.1 可宣布「能力层 Phase17 Holding Intelligence 主路径完成」（工作区）

- C2 Explanation · C3 HealthScore · C4 Display · 17.2 Data Truth · 17.4 Kline Timeframe · 17.5 Origin：**代码 + 报告 + UI 一致**。  
- C5 T-Suitability：**工作区一致**；**Git tag 不一致**。  
- C1：审计基线完成（无实现义务）。

### 6.2 不可宣布「源码 tag 已完整冻结 Phase17」

直至 T-Suitability 与菜单修复等进入新的 commit/tag（**本报告不执行该操作**）。

### 6.3 架构冻结

Holding Intelligence **与** Trading Execution **隔离** — **ACCEPT**。

---

## 7. 停止

Phase17 Final Acceptance Audit **完成**。  
按用户指令：**发现问题只记录 · 不修复 · 到此停止。**

---

*报告路径：`PHASE17_FINAL_ACCEPTANCE_REPORT.md`*
