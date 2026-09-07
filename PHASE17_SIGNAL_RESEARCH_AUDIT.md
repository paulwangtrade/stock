# PHASE17 Signal Research Infrastructure Audit（只读）

**日期：** 2026-09-07  
**性质：** **只读审计**（未改代码 / 未实现回测引擎 / 未改交易链）  
**目标：** 确认事件监控 · SignalEvent · SignalPrice · Snapshot · CandidatePool 是否已具备**信号历史回测**基础。  

**上游参照：**  
- [PHASE17_A_ALPHA_RESEARCH_CAPABILITY_AUDIT.md](./PHASE17_A_ALPHA_RESEARCH_CAPABILITY_AUDIT.md)  
- [PHASE14_SIGNAL_BACKTEST_RUNTIME_AUDIT.md](./PHASE14_SIGNAL_BACKTEST_RUNTIME_AUDIT.md)  
- [PHASE16_D1_OUTCOME_MODEL_DESIGN.md](./PHASE16_D1_OUTCOME_MODEL_DESIGN.md) / D2–D3 Outcome 实现  
- [PHASE17_3_KLINE_SIGNAL_CONFIG_DESIGN.md](./PHASE17_3_KLINE_SIGNAL_CONFIG_DESIGN.md)  

---

## 0. 三问总答

| # | 问题 | 结论 |
| --- | --- | --- |
| 1 | 当前能否做信号回测？ | **能做「弱形态 / 半成品」**：单票日 K + 前端引擎可跑；**不能**做基于历史 Snapshot hits 的系统级标签回测 / cohort |
| 2 | 距离策略回测差什么？ | 归一化 Signal 事件表、信号锚定 forward（T+1/3/5/MFE/MAE）物化或统一服务、多标的组合级回测账本、与生产执行隔离的后端引擎 |
| 3 | 是否需要独立 Strategy Backtest Engine？ | **策略级 / 组合级：需要（中长期）**；**信号标签研究 MVP：不必先建大引擎**，可先 Forward Evaluator + 复用日 K |

---

## 一、SignalEvent 保存审计

### 1.1 落存形态（关键事实）

| 形态 | 说明 |
| --- | --- |
| **无独立 `signal_events` 表** | SignalEvent 是 `SignalScanHit` 内嵌字段（Phase14-C），塞进 Snapshot `result_json` |
| Snapshot 表 | `signal_scan_snapshots`：`trade_date` / `session` / `strategy_id` / `strategy_name` / `signal_params_json` / `result_json` |
| 图表侧 | 冰点等在前端按 K 线**重算 markers**；**不**把每次标注写入 DB |

### 1.2 字段能力对照

| 需求字段 | 是否具备 | 落点 |
| --- | --- | --- |
| 时间 | **部分** | hit.`signal_time`；Snapshot.`trade_date`+`session`；缺统一 bar timestamp 索引 |
| 股票 | **是** | `SECUCODE` / `SECURITY_CODE` |
| 价格 | **是（冻结）** | `signal_price` + `signal_price_status`（与 `NEW_PRICE` 分离） |
| 类型 | **部分** | `tag`（强/冰/趋…）；非枚举化 `SignalEvent.type` |
| 来源 | **部分** | Snapshot.`strategy_id`/`strategy_name`/`session`；params 副本；无 Provider ID 契约（17.3/17.5 设计中） |

**结论：** 扫描快照**已能冻结一批「信号发生记录」**，但不是可查询的事件流水表；历史回测要扫 JSON 或重放算法。

### 1.3 事件监控 vs 研究存储

| 表面 | 角色 |
| --- | --- |
| 菜单「事件监控」 | 指向策略中心 / 异动·扫描类研究 UI（`productMenu` → research） |
| 全市场扫描 | `StartSignalScanSnapshot` → 写 Snapshot；页面可读历史列表 |
| 策略中心「信号回测」Tab | **前端** `SignalBacktestPanel` + `backtestEngine.js`；拉日 K 后本地算，**不读 Snapshot hits 库** |

---

## 二、信号发生后的收益指标

### 2.1 需求 vs 现网

| 指标 | 能否获取 | 如何 / 缺口 |
| --- | --- | --- |
| T+1 收益 | **条件可以** | `obsfeedback` 支持 horizon T+1/5/10，但评的是**持仓观测决策**，**无 signal_snapshot_id** |
| T+3 收益 | **弱** | obsfeedback **默认无 T+3 常量**（有 1/5/10）；可用日 K **现算**，无现成 API |
| T+5 收益 | **条件可以** | 同上 obsfeedback T+5；**未**挂到 SignalHit |
| 最大上涨 (MFE) | **部分** | obsfeedback 窗口 high；Opportunity Outcome **MVP Performance 无 MFE 字段**（设计有、实现偏 realized） |
| 最大回撤 (MAE) | **部分** | 同上 |

### 2.2 分层真相

```text
A. 信号锚定 forward（理想研究）
   SignalHit(signal_time, signal_price)
     → 日K[T+1..T+N] → return / max_up / max_dd
   现状：数据原料（Snapshot + K线）有；**统一 Forward 服务未产品化**

B. 成交锚定 realized（执行反馈）
   Signal → Pool → TradePlan → Fill → OutcomeProjection
   现状：D2/D3 **FIFO round-trip** 有 realized_return / holding_days；
         NO_TRADE 行可列未成交；**仍缺**标准 T+N / MFE 物化

C. 持仓观测 forward（obsfeedback）
   Holding decision 快照 → T+N
   现状：可跑；**域错位**，不能当「信号回测」
```

**结论：** 「信号后 T+1/3/5 / 最大涨跌」**在数据上可算，在产品上未接通 Signal 主键**。

---

## 三、链路追踪：信号 → 计划 → 成交 → 结果

```text
SignalScanSnapshot.result_json (hits + SignalPrice)
        ↓ signal_snapshot_id
CandidatePoolItem (tag / score / rank / strategy_name)
        ↓ pool_id
TradePlan / TradePlanItem
        ↓ plan_id / plan_item_id
paper_sim_fills (buy/sell)
        ↓ FIFO 读模型
Opportunity OutcomeProjection (OPEN/CLOSED/NO_TRADE)
        ↓
Provenance / TradePlanOrigin / Explanation（解释侧）
```

| 环节 | 可追踪？ | 说明 |
| --- | --- | --- |
| 信号产生 | **是（快照级）** | 列表/解析 Snapshot |
| → TradePlan | **是** | pool_id / origin 投影 |
| → 成交 | **是** | fill.`plan_id` |
| → 结果 | **部分** | OutcomeProjection realized；未成交 NO_TRADE；**缺**信号级 forward cohort |
| 图表 SignalEvent | **弱** | 多即时重算，未与 Snapshot hit ID 对齐 |

**已具备「执行闭环归因」的骨架；未具备「全信号历史研究闭环」。**

---

## 四、与「信号回测 / 策略回测」差距

### 4.1 当前已有积木

| 积木 | 用途 |
| --- | --- |
| Snapshot + SignalPrice | 历史命中冻结 |
| CandidatePool | 排序/策略上下文 |
| OutcomeProjection | 成交结果反馈 |
| 日 K + Freshness | forward / 重放原料 |
| `backtestEngine.js` | 单票日频交易模拟（佣金/T+1/回撤曲线） |
| `SignalBacktestPanel` | UI 入口（研究 Tab） |
| obsfeedback | 持仓决策 forward（旁路） |

### 4.2 信号回测仍缺

1. **归一化事件流**（可按 tag/date/strategy 查询，而非只扫 JSON）  
2. **Signal-anchored Forward Evaluator**（T+1/3/5 + MFE/MAE，挂 `snapshot_id`+code+tag）  
3. **标签 cohort API**（胜率、均收益、样本数）  
4. 回测与 **生产扫描参数/冰点 Provider** 的版本对齐（避免「回测一套、扫描一套」）  
5. （可选）批量多标的回测任务与结果落库  

### 4.3 策略回测额外还缺

| 缺口 | 说明 |
| --- | --- |
| 组合现金/权重约束引擎（后端） | 前端 MVP 仅单票 |
| 策略版本 / 参数版本账本 | params 在 Snapshot 有副本，缺系统化 |
| 与 paper 实盘可比的成本与成交假设 | 部分在 FE 引擎 |
| Walk-forward / 样本外框架 | 无 |
| 隔离生产 Broker | 必须；禁止回测写 paper_sim |

---

## 五、设计建议（不实现）

### 5.1 是否需要独立 Strategy Backtest Engine？

| 目标 | 建议 |
| --- | --- |
| **信号研究 MVP**（标签 T+N、胜率） | **不需要**先上独立大引擎：Snapshot hits + 日 K Forward Evaluator + cohort 聚合即可 |
| **策略/组合级回测**（多票、仓位纪律、与实盘对比） | **需要**独立只读 Backtest Engine（后端或 Worker），与 PaperBroker **隔离** |
| 现有 `backtestEngine.js` | 可作单票原型；**不能**替代研究级存储与 cohort |

### 5.2 推荐顺序（研究向）

1. SignalEvent Adapter（图表/扫描输出契约，17.5 方向）— 仍可不建表  
2. Forward Evaluator v0（按 hit 算 T+1/5/MFE；T+3 一并支持）  
3. Cohort 只读 API（by tag / strategy / session）  
4. 可选物化 `signal_forward_outcomes`（性能）  
5. 若要做组合策略对比 → 独立 Strategy Backtest Engine  

### 5.3 明确不做（本审计边界）

- 不改交易执行、不自动下单  
- 不把 obsfeedback 误标为信号回测  
- 不要求立刻新建全量 `signal_events` 表（可后置）  

---

## 六、合规确认

| 要求 | 状态 |
| --- | --- |
| 只读审计 | **是** |
| 未实现 | **是** |

---

## 附录 — 关键路径

- `backend/models/signal_scan_snapshot.go` — Snapshot / Hit / SignalPrice  
- `backend/data/signal_snapshot_repo.go` · `signal_price.go`  
- `backend/models/candidate_pool.go`  
- `backend/opportunity/outcome/` — OutcomeProjection  
- `backend/obsfeedback/` — T+N（持仓观测域）  
- `frontend/src/utils/backtestEngine.js` · `SignalBacktestPanel.vue`  
- `frontend/src/components/allStockList.vue` — 扫描 / 事件监控主链  

---

*Phase17 Signal Research Infrastructure Audit 结束。*
