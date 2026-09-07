# Phase17-A「Alpha Research Engine 数据能力」只读审计

**审计时间**：2026-09-01  
**性质**：只读数据能力审计。**不修改代码、不新增表、不改变交易链。**  
**依据**：
- [`PHASE16_D0_DECISION_OUTCOME_AUDIT.md`](PHASE16_D0_DECISION_OUTCOME_AUDIT.md)（基线；本审计在其上更新 D2/D3 进展）
- [`PHASE16_D2_OUTCOME_PROJECTION_IMPLEMENTATION_REPORT.md`](PHASE16_D2_OUTCOME_PROJECTION_IMPLEMENTATION_REPORT.md)
- [`PHASE16_D3_OUTCOME_API_IMPLEMENTATION_REPORT.md`](PHASE16_D3_OUTCOME_API_IMPLEMENTATION_REPORT.md)
- [`PHASE12_D_BACKTEST_DESIGN.md`](PHASE12_D_BACKTEST_DESIGN.md)
- [`PHASE14_SIGNAL_BACKTEST_RUNTIME_AUDIT.md`](PHASE14_SIGNAL_BACKTEST_RUNTIME_AUDIT.md)

**目标**：评估当前数据库与只读服务是否支撑 **Alpha Research Engine** 五类分析能力，并明确距离「真正 Alpha 研究引擎」的数据缺口。

---

## 0. 执行摘要

| 分析能力 | 现网支持度 | 一句话 |
|----------|------------|--------|
| **1. Signal 历史统计**（如标签成功率） | ⚠️ **25%** | 原始 hits 在 JSON 快照中可扫；**无**归一化 signal 表、**无**标签 cohort API、前端回测不入库 |
| **2. Strategy 表现统计**（如策略收益） | ⚠️ **20%** | 策略名散落多表；**无**策略级 PnL 账本；仅账户级 `paper_sim_daily_reports` |
| **3. CandidatePool Rank 有效性** | ⚠️ **30%** | Rank/Score 落库完整；**无** Rank→成交→收益 的 ex-post 评估与 IC 统计 |
| **4. Decision 选择质量** | ⚠️ **15%** | `QuantDecision` **不落库**；obsfeedback 评的是 **持仓决策态**，非买入决策 |
| **5. Trade 结果归因** | ✅ **55%** | D2 `OutcomeProjection` + FIFO 可算 round-trip；**未物化**、无 cohort 层 |

**综合判断**：

```
Execution / Observation Engine  ≈  75–80%  （发现→计划→成交→持仓解释）
Alpha Research Engine           ≈  25–30%  （历史 cohort、策略 alpha、Rank IC、决策 ex-post）
```

**距离真正 Alpha Research Engine 的核心缺口**：缺少 **归一化 Signal Event 历史**、**物化 Outcome / RoundTrip 研究层**、**策略与标签维度的 cohort 聚合服务**，以及 **与生产账本隔离的后端回测引擎**。

---

## 1. 数据库与读模型清单

### 1.1 与 Alpha 研究相关的物理表

| 层 | 表 | 模型 | Alpha 相关字段 |
|----|-----|------|----------------|
| **Signal** | `signal_scan_snapshots` | `SignalScanSnapshot` | `trade_date`, `session`, `strategy_id`, `result_json`（hits 数组） |
| **Opportunity** | `candidate_pools` | `CandidatePool` | `trade_date`, `source`, `config_json` |
| | `candidate_pool_items` | `CandidatePoolItem` | **`rank`, `score`**, `signal_snapshot_id`, `signal_tag`, `strategy_name`, 可选 `decision_id` |
| **Decision（写链投影）** | `trade_plans` | `TradePlan` | `pool_id`, `decision_provider`, `provider_mode`, `risk_*` |
| | `trade_plan_items` | `TradePlanItem` | `status`, `score`, `strategy_name`, **`fill_id`**, `risk_code`, `skipped` |
| **Execution** | `paper_sim_fills` | `PaperSimFill` | `plan_id`, `side`, `price`, `volume`, `filled_at` |
| | `paper_sim_orders` | `PaperSimOrder` | 幂等链至 plan_item |
| | `paper_sim_runs` | `PaperSimRun` | 执行批次观测 |
| **Account** | `paper_sim_accounts` | `PaperSimAccount` | `realized_pnl`（账户级，**sell 路径未可靠维护 per-lot**） |
| | `paper_sim_daily_reports` | `PaperSimDailyReport` | 日级 equity / turnover（**无策略维度**） |
| **用户行为** | `user_opportunity_actions` | `UserOpportunityAction` | WATCH/IGNORE；**无 outcome FK** |
| **Exit 意图** | `exit_review_outcomes` | `ExitReviewOutcome` | 用户复评；**非**交易结果 |
| **Research 标注** | `data/research_candidates.json` | sidecar | status/tags；**非** DB 表 |
| **Obs 反馈** | `data/paper_observation_history.json` | `obsfeedback.Record` | T+N forward return；**无 signal FK** |

### 1.2 只读投影（无表，查询时组装）

| 投影 | 包 | Alpha 价值 |
|------|-----|------------|
| `OpportunityProjection` | `opportunity/projection` | 横截面解释；**无历史快照** |
| **`OutcomeProjection`** | `opportunity/outcome` | **Round-trip FIFO**；`realized_return_pct`, `holding_days` |
| `tradeplanorigin` | `tradeplanorigin` | Plan → Signal 溯源 |
| `portfolio/provenance` | `portfolio/provenance` | 持仓 → Plan → Signal |
| `PositionAttribution` | `papertrading/attribution` | Buy fill lots；**忽略 sell** |
| `HoldingEvaluation` | `papertrading` | 开放仓 unrealized |
| `obsfeedback.PerformanceView` | `obsfeedback` | 持仓决策态 win_rate（**域不同**） |

### 1.3 明确不落库

| 对象 | 说明 |
|------|------|
| `QuantDecision` | Phase0 契约；**无 TableName / AutoMigrate** |
| Signal 单行事件 | hits 嵌在 `result_json` |
| Outcome / RoundTrip | D2 内存投影；**无 `outcome_round_trips` 表** |
| 前端 `backtestEngine` 结果 | 浏览器计算；**不持久化** |
| `backend/backtest/` | Phase12-D **设计 only**；未实现 |

---

## 2. 五项能力逐项审计

### 2.1 Signal 历史统计（例：某标签过去成功率）

#### 现有什么

| 来源 | 内容 | 局限 |
|------|------|------|
| `signal_scan_snapshots.result_json` | 每条 hit：`tag`, `signal_price`, `signal_time`, `SECUCODE` | **非关系行**；跨日统计需扫 JSON |
| `SignalScanSnapshot.strategy_id` | 快照级策略 | 非 hit 级 |
| 前端 `SignalBacktestPanel` + `backtestEngine.js` | 单股日频回测、胜率/回撤 | **客户端**；不入库；Wails K 线调用曾半残 |
| `obsfeedback` | T+N `win_rate` / `avg_return` | 基准 = **观测日市价**；`DecisionState` = 持仓态；**无 tag** |

#### 能否回答「标签 X 历史成功率」？

| 路径 | 可行？ | 条件 |
|------|--------|------|
| **A. 扫全库 snapshot JSON，按 tag 聚合** | ⚠️ 离线可行 | 需定义「成功」（如 T+5 收益>0）；需 K 线；**无现成 API** |
| **B. 标签 → 成交 → Outcome CLOSED** | ⚠️ 部分 | 仅 **入池且成交** 的子集；via `signal_snapshot_id` + FIFO |
| **C. 含 WATCH 未成交信号** | ❌ | `user_opportunity_actions` 无回报 |
| **D. 含扫描到但未入池的 hit** | ⚠️ 仅 A 路径 | 量大、无 outcome 链 |

#### 支持度：**25%**

**已有**：原始信号历史（JSON 快照）、hit 级 tag/price/time。  
**缺失**：
- 归一化 **`signal_events`** 表（或物化视图）
- **`signal_tag_cohort_stats`** 聚合（count, win_rate, avg_forward_return, horizon）
- 标签成功率与 **实际交易 outcome** 的统一口径
- 后端 Signal Backtest 引擎与结果存储（Phase12-D 未实现）

---

### 2.2 Strategy 表现统计（例：某策略收益）

#### 现有什么

| 来源 | 策略标识 | 收益字段 |
|------|----------|----------|
| `signal_scan_snapshots` | `strategy_id`, `strategy_name` | 无 |
| `candidate_pool_items` | `strategy_name`, `strategy_version` | 无 |
| `trade_plan_items` | `strategy_name`, `strategy_version` | 无直接 PnL |
| `paper_sim_fills` → Outcome | 经 plan/item 间接 | **`realized_return_pct`**（D2，按 leg） |
| `paper_sim_daily_reports` | 无 | 账户 equity / turnover |
| `paper_sim_accounts.realized_pnl` | 无 | 账户汇总；**非策略归因** |
| `strategyintent` / Strategy Schema | 规则版本绑定 | **意图层**；无绩效表 |

#### 能否回答「策略 trend_breakout 累计收益」？

| 路径 | 可行？ | 说明 |
|------|--------|------|
| Join fills → plan_item.strategy_name → Outcome FIFO | ⚠️ **读模型可行** | 需全量扫描；strategy 字符串不一致风险 |
| 按 Strategy Schema ID 聚合 | ❌ | Schema 与成交链 **无稳定 FK** |
| 策略级 time-series equity | ❌ | 无 `strategy_performance_daily` |
| 与 benchmark alpha | ⚠️ | obsfeedback 有 alpha；**非策略域** |

#### 支持度：**20%**

**已有**：策略名字段贯穿 Signal→Pool→Plan→Fill；Outcome 可派生 leg 级 realized return。  
**缺失**：
- **策略维度 PnL 账本**（日/周/累计）
- **`strategy_id` 与成交的统一主键**（现多为 name 字符串）
- 策略回测 vs 实盘模拟 **对照层**
- 多策略组合 attribution（Brinson 类）—— **完全无**

---

### 2.3 CandidatePool Rank 有效性

#### 现有什么

| 字段 | 表 | 说明 |
|------|-----|------|
| `rank`, `score` | `candidate_pool_items` | **权威** Rank/Score |
| `pool_id`, `trade_date` | `candidate_pools` | 池版本 |
| 成交链 | `trade_plan.pool_id` → items → fills | 可知 **哪些 rank 被选中执行** |
| Outcome | D2 投影 | 可知 **成交后收益** |

#### 典型研究问题 vs 现网

| 问题 | 能否回答 | 方式 |
|------|----------|------|
| Rank1 是否比 Rank5 更易成交？ | ⚠️ | SQL/离线：pool items left join plan items |
| Rank 与 realized return 相关性（IC） | ⚠️ 离线 | 需 cohort：每个 (pool, stock) 的 rank + outcome |
| Top decile 超额收益 | ❌ API | 无 decile 聚合 |
| 未入选高 rank 的 counterfactual | ❌ | 无「假设买入」存储 |
| Rank 随时间漂移 | ⚠️ | 多 pool 版本可对比；无专用分析 |

#### 支持度：**30%**

**已有**：Rank/Score **落库完整**；Pool→Plan→Fill→Outcome **链可拼接**。  
**缺失**：
- **Rank IC / decile spread** 预聚合
- **Pool cohort 研究 API**（按 trade_date 窗口）
- 未成交 pool item 的 **forward outcome**（需 signal 级 forward，非 FIFO）
- A/B：`score` vs 实际 alpha 的校准曲线

---

### 2.4 Decision 选择质量

#### 现有什么

| 来源 | 「决策」语义 | 持久化 |
|------|--------------|--------|
| `QuantDecision` | ENTER/WATCH/BLOCKED 等 | **否** |
| `TradePlanItem.status` | pending/filled/skipped | ✅ |
| `TradePlanItem.risk_code` | 风控拒绝 | ✅ |
| `OpportunityProjection.decision` | BUY_CANDIDATE/WATCH/… | 读时组装 |
| `obsfeedback` + `/observation/performance` | HOLD_WATCH / EXIT_CANDIDATE 等 | JSON + 聚合 API |
| `PortfolioDecisionSummary` | 组合级 HOLD/REVIEW | 读模型 |
| `CandidatePoolItem.decision_id` | 可选 QuantDecision.id | 稀疏 |

#### 能否回答「BUY_CANDIDATE 决策是否正确」？

| 决策类型 | ex-post 评估 | 现网 |
|----------|--------------|------|
| **买入 pending → filled** | 应对照 Outcome | ⚠️ Outcome 有；**无 decision_id 锚点** |
| **skipped / risk rejected** | 应对照 counterfactual forward | ❌ |
| **WATCH 未入 plan** | 应对照 forward return | ❌（仅 obs 域近似） |
| **Portfolio Decision G.3** | 组合关注是否正确 | ⚠️ 启发式；非 lot outcome |

#### 支持度：**15%**

**已有**：TradePlanItem 执行结果；obsfeedback 对 **持仓决策态** 的 win_rate（API 已有）。  
**缺失**：
- **`DecisionOutcome`** 实体（decision_id → realized / forward）
- **QuantDecision 历史归档**（哪怕 JSONL）
- **skipped vs filled** 的 counterfactual 框架
- Opportunity 级 **决策正确率**（与 obsfeedback 域对齐）

---

### 2.5 Trade 结果归因

#### 现有什么（D0 → D3 更新）

| 能力 | D0 结论 | **现网（D2/D3 后）** |
|------|---------|----------------------|
| Buy fill → Plan → Signal | ✅ | ✅ 不变 |
| Sell ↔ Buy 配对 | ❌ | ✅ **`matchFIFOLegs` FIFO**（读模型） |
| Round-trip realized return | ❌ | ✅ **`OutcomeProjection.performance.realized_return_pct`** |
| holding_days | 仅 open lot | ✅ **OPEN/CLOSED 均可** |
| 持久化 outcome | ❌ | ❌ **仍无表** |
| MFE/MAE | obsfeedback 近似 | ❌ MVP **未输出** |
| Attribution API | buy-only lots | ✅ + Outcome API `GET /outcomes` |

#### 归因链路（当前最佳路径）

```
paper_sim_fills (buy+sell)
    → outcome.matchFIFOLegs
    → OutcomeProjection { signal, opportunity, decision, entry, exit, performance }
    → 可经 signal_snapshot_id / pool rank 向上归因
```

#### 支持度：**55%**（五维中最高）

**已有**：
- FIFO round-trip **读模型完整**（D2/D3）
- Plan/Signal 溯源（tradeplanorigin、provenance）
- HTTP 单股/列表查询

**仍缺失**：
- **物化 `outcome_round_trips`**（D1 可选表；未做）
- sell fill **无 `matched_buy_fill_id` 列**（每次读时 FIFO）
- **账户级 realized_pnl 与 leg 级不一致**（D0 已知）
- **cohort 归因 API**（按 tag/strategy/rank 汇总）
- exit_reason **与成交绑定** 的权威字段

---

## 3. 现有服务与 API 对照

| API / 服务 | 路径 | Alpha 相关能力 | 域 |
|------------|------|----------------|-----|
| Signal 列表 | `GET /api/opportunities/list` | 当日 scan hits | 浏览 |
| Opportunity 投影 | `GET /api/opportunities/projections` | 横截面四层 | 解释 |
| **Outcome 投影** | `GET /api/opportunities/outcomes` | **Round-trip 结果** | **结果** |
| Observation Performance | `GET /api/papertrading/observation/performance` | win_rate, by_state | **持仓决策** |
| Position Attribution | `GET /api/papertrading/observation/positions/attribution` | buy lots | 持仓 |
| Portfolio Provenance | `GET /api/portfolio/positions/{code}/provenance` | 来源 | 溯源 |
| Investment Home | `GET /api/investment/home` | 启发式 opportunity_cards | 首页 |
| Portfolio Decision | `GET /api/portfolio/decision-dashboard` | 目标 vs 实际 | 组合 |
| Research Candidates | `GET /api/research/candidates` | 研究标注 | **非** Trade Pool |
| Candidate Pool | `GET /api/candidate_pool/list` | 研究页快照 | 非权威 Plan Pool |
| Signal Backtest | 前端 only | 单股回测 | **不入库** |
| Backend Backtest | — | **未实现** | — |

**结论**：没有任何 API 直接提供 **「按 signal_tag / strategy_id 的历史 cohort 统计」**。

---

## 4. 距离真正 Alpha Research Engine 的数据缺口

### 4.1 缺口分层

```
┌─────────────────────────────────────────────────────────────┐
│  L4  研究产品层：Rank IC 报告、策略 leaderboard、标签热力图    │  ❌ 无
├─────────────────────────────────────────────────────────────┤
│  L3  Cohort 聚合层：tag/strategy/rank/decision 维度统计 API   │  ❌ 无
├─────────────────────────────────────────────────────────────┤
│  L2  Outcome 物化层：round_trips / signal_outcomes 表或 parquet │  ❌ 无（D2 仅内存）
├─────────────────────────────────────────────────────────────┤
│  L1  事件归一化层：signal_events、decision_snapshots 归档     │  ❌ 无
├─────────────────────────────────────────────────────────────┤
│  L0  原始事实层：snapshots, pools, plans, fills               │  ✅ 75%
└─────────────────────────────────────────────────────────────┘
```

### 4.2 关键缺口 Register（按优先级）

| ID | 缺口 | 影响 | 建议形态 |
|----|------|------|----------|
| **G1** | Signal hit **非关系化** | 无法高效按 tag 做 SQL cohort | 物化 `signal_events` 或 ETL 扫 JSON |
| **G2** | Outcome **未持久化** | 每次研究全量 FIFO；无历史切片 | 可选表 `outcome_round_trips` 或日批 parquet |
| **G3** | **无 cohort 统计 API** | 标签成功率/策略收益需 ad-hoc 脚本 | `GET /api/research/cohorts?dim=tag&window=90d` |
| **G4** | **QuantDecision 不落库** | Decision 质量无法 ex-post | 归档 JSONL 或 `decision_snapshots` |
| **G5** | **无后端回测引擎** | Signal 统计依赖前端或手工 | Phase12-D `backend/backtest/` |
| **G6** | **策略 ID 不统一** | 跨表 join 靠 name 字符串 | Strategy Schema ↔ fill 稳定 FK |
| **G7** | **NO_TRADE / skipped counterfactual** | 选择质量只评成交子集 | Signal forward outcome（未成交也评） |
| **G8** | **MFE/MAE entry-based** | 风险调整 alpha 不完整 | K 线窗口 + Outcome 扩展（D1 已设计） |
| **G9** | **obsfeedback 域错位** | 易与 Opportunity outcome 混淆 | 产品层严格分区；统一 dashboard 标注来源 |
| **G10** | **user_opportunity_actions 无 outcome** | 无法评「用户忽略是否错过 alpha」 | 行为 → forward return 研究 join |

### 4.3 「真正 Alpha Research Engine」最低数据闭环

要稳定回答研究问题，至少需要：

1. **Signal Event Store** — 每个 `(snapshot_id, stock_code, tag, signal_time, signal_price)` 一行  
2. **Outcome Store** — 每个 round-trip 或 NO_TRADE 一行，含 `realized_return_pct`, `holding_days`, anchors  
3. **Cohort Aggregator** — 按 `tag | strategy_id | rank_decile | decision_status` rollup  
4. **Forward Evaluator** — 对未成交 signal 计算 T+N forward（与 obsfeedback 分离或复用 K 线引擎）  
5. **Backtest Sandbox** — 与 `paper_sim_*` 隔离的历史模拟（Phase12-D）  

当前 **仅 L0 成熟**；L2–L4 **基本空白**。

---

## 5. 五维能力矩阵（汇总）

| 维度 | 原始数据 | 读模型 | 聚合 API | 持久化研究层 | 综合 |
|------|----------|--------|----------|--------------|------|
| Signal 历史统计 | ⚠️ JSON | ❌ | ❌ | ❌ | **25%** |
| Strategy 表现 | ⚠️ 名字段 | ⚠️ via Outcome | ❌ | ❌ | **20%** |
| Rank 有效性 | ✅ rank/score | ⚠️ 可 join | ❌ | ❌ | **30%** |
| Decision 质量 | ⚠️ plan item | ⚠️ obs 域 | ⚠️ obs only | ❌ | **15%** |
| Trade 归因 | ✅ fills | ✅ Outcome | ⚠️ list only | ❌ | **55%** |

---

## 6. 可复用的近期交付（Phase16）

| 交付 | 对 Alpha 研究的价值 |
|------|---------------------|
| **OutcomeProjection（D2）** | 首次提供 **entry→exit→return** 统一 DTO；cohort 上游 |
| **GET /outcomes（D3）** | 研究入口 API；待加 summary / group_by |
| **OpportunityProjection（C0/C1）** | 横截面特征；可作 cohort 维度源 |
| **Investment Dashboard 设计（E）** | 产品消费 outcome summary；**非**研究引擎 |

**D0 审计中「Outcome 不存在」已部分闭合**；但 **「Alpha 研究引擎」仍不成立** — 因有 outcome **无 cohort**。

---

## 7. Phase17 建议方向（设计，非本阶段编码）

| 阶段 | 目标 | 依赖 |
|------|------|------|
| **17-B** | Signal Event 物化 ETL（从 snapshot JSON） | L0 |
| **17-C** | Outcome 日批物化 + `summary?group_by=tag` API | D2/D3 |
| **17-D** | Rank IC / decile 研究 API | Pool + Outcome |
| **17-E** | Decision 归档 + ex-post 质量 | QuantDecision 落库策略 |
| **17-F** | Backend Backtest MVP（Phase12-D 实现） | 隔离 sim |

**原则**：
- 研究层 **只读**；不写 TradePlan / Broker  
- 新表仅 **物化/归档**；不改变写链  
- obsfeedback 与 Opportunity Outcome **双轨标注**，不混口径  

---

## 8. 审计结论

### 8.1 数据库是否「支持」五项分析？

| # | 问题 | 结论 |
|---|------|------|
| 1 | Signal 历史统计 | **原始数据有，研究能力无** — 需归一化 + cohort |
| 2 | Strategy 表现统计 | **字段有，账本无** — 需策略维 Outcome 聚合 |
| 3 | Rank 有效性 | **Rank 数据完整，评估无** — 需 IC/decile 服务 |
| 4 | Decision 选择质量 | **执行态有，决策态无** — QuantDecision 不落库 |
| 5 | Trade 结果归因 | **D2 读模型可用，未物化** — 五维中最接近就绪 |

### 8.2 与 D0 对比

| 项 | D0（2026-09-01 早） | 17-A（现） |
|----|---------------------|------------|
| Round-trip return | ❌ | ✅ OutcomeProjection |
| Outcome API | ❌ | ✅ GET /outcomes |
| 综合 outcome 能力 | ~20% | **~55%（归因维）** |
| Alpha Research Engine | ~25% | **~25–30%**（cohort 层仍空） |

Outcome 闭环 **显著改善交易归因**；**未自动升级为 Alpha 研究引擎** — 因缺少 **历史事件存储 + 多维 cohort 聚合 + 回测沙箱**。

### 8.3 一句话

> 现网数据库是合格的 **「交易事实仓 + 只读解释层」**，还不是 **「Alpha 研究仓」**。  
> 最小下一步：**物化 Signal Events + 物化 Outcomes + Cohort Summary API** — 三者齐备前，「某标签历史成功率」仍只能离线 ad-hoc，无法产品化。

---

## 9. 附录：关键表字段速查

### Signal（JSON hit）

`tag`, `signal_price`, `signal_time`, `schema_version`, `SECUCODE` → 映射 `stock_code`

### CandidatePoolItem

`rank`, `score`, `signal_snapshot_id`, `signal_tag`, `strategy_name`, `decision_id?`

### TradePlanItem

`status`, `fill_id`, `score`, `strategy_name`, `risk_code`, `side`

### PaperSimFill

`plan_id`, `plan_item_id`, `side`, `price`, `volume`, `filled_at` — **无** `matched_buy_fill_id`

### OutcomeProjection（DTO）

`outcome_status`, `performance.realized_return_pct`, `performance.holding_days`, `metadata.fifo_policy`

### obsfeedback Record

`symbol`, `observation_date`, `decision_state`, `market_price` — **无** `signal_snapshot_id`
