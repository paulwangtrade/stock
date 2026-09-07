# PHASE17 Portfolio Data Consistency Audit（只读）

**日期：** 2026-09-07  
**性质：** **只读审计**（未改代码 / API / DB / 交易链）  
**目标：** 厘清「我的组合」盈亏与价格口径，定位一致性风险与最小修复方向。  
**上游：**  
- [PHASE16_PORTFOLIO_DATA_FRESHNESS_AUDIT.md](./PHASE16_PORTFOLIO_DATA_FRESHNESS_AUDIT.md)  
- [PHASE17_A2_PORTFOLIO_DASHBOARD_AUDIT.md](./PHASE17_A2_PORTFOLIO_DASHBOARD_AUDIT.md)  
- [PHASE17_A2_PORTFOLIO_IMPLEMENTATION_COMPLETE.md](./PHASE17_A2_PORTFOLIO_IMPLEMENTATION_COMPLETE.md)（语义文案已部分落地）  

---

## 0. 执行摘要

| 问题 | 结论 |
| --- | --- |
| 账户「今日盈亏」是盘中浮盈吗？ | **否** — 是相对**上一结算日报权益**的差额（`daily_report_delta`） |
| 「当前价 / 行情价」实时吗？ | **仅请求时点尽力实时** — `include_display=1` 拉 Quote；**无轮询**；失败则退 mark / 显示 — |
| 何时刷新？ | **用户点刷新 / 进页 onMounted**；无盘中自动推送 |
| Settlement 后是否覆盖？ | **覆盖账本 mark** → 权益/市值/累计浮盈随之变；**不写** display overlay；次日账户今日盈亏基准变为新日报 |
| 最大一致性风险 | **双价格体系**（mark 驱动权益 vs display 驱动「行情/持仓今日浮盈」）+ 命名曾混淆（A2 已部分缓解） |

**总判：** 架构是**故意拆分口径**，不是单点 bug；剩余债主要是**新鲜度、时间戳、防误解**，不是重建 Portfolio。

---

## 一、Portfolio Dashboard 字段来源

### 1.1 双 API 拼装（现行）

```text
GET /api/portfolio/snapshot?include_display=1
  → 现金 / 权益 / 市值 / 持仓行（mark 事实 + 可选 Quote overlay）

GET /api/portfolio/dashboard
  → 账户今日盈亏 / basis / 风险 / 今日成交 fills
```

前端：`PortfolioDashboard.vue` 并行请求；总览权益取 Snapshot，账户今日盈亏取 Dashboard。

### 1.2 字段对照表

| UI（当前文案，A2 后） | 绑定 | 计算口径 | 真源 |
| --- | --- | --- | --- |
| 总权益 | `snapshot.equity` | cash + Σ(mark×qty) | paper_sim 账本 |
| 现金 | `snapshot.cash` | 账本现金 | `paper_sim_accounts` |
| 市值 | `snapshot.market_value` | Σ(mark×qty) | mark |
| **账户今日盈亏** | `dashboard.summary.daily_pnl` | equity_now − 上一日报 equity | 日报表 + 当前 mark 权益 |
| `daily_pnl_basis` | 同 Dashboard | `daily_report_delta` / `unavailable` | 有无前日报 |
| 成本 | `avg_cost` | 持仓成本 | `paper_sim_positions` / 成交摊薄 |
| **估值价 (mark)** | `mark_price` | 账本估值 | 成交写入 + EOD Settlement |
| **行情价 (display)** | `display_price` | Quote.Price（或 Open fallback） | GET 时 QuoteService；**不落库** |
| 持仓市值列 | `market_value` | mark×qty | mark（非 display） |
| **累计浮盈** | `pnl` / unrealized | (mark − cost)×qty | mark |
| **持仓今日浮盈** | `today_pnl` | (display − pre_close)×qty | Quote；缺价则为 null |
| 收益率 | `pnl_percent` | 相对成本 · mark | mark |

### 1.3 四个关键问题（直接回答）

#### 1）今日盈亏是否是真正盘中盈亏？

**账户级「账户今日盈亏」：不是。**

```text
daily_pnl = 当前账户权益(mark) − 上一纸面结算日报权益
```

- 含：当日成交、Settlement 估值变化相对昨日报的累计效果  
- **不含：** 盘中 Quote overlay  
- **不等于：** Σ 持仓「持仓今日浮盈」  
- 无前日报 → `daily_pnl = null`，basis=`unavailable`

**持仓级「持仓今日浮盈」：才是相对昨收的盘中口径**（有 Quote 时），且与账户今日盈亏**故意不等**。

#### 2）当前价格是否实时？

| 列 | 实时性 |
| --- | --- |
| 行情价 `display_price` | **请求瞬间**上游 Quote；成功则接近实时；失败/非 live → 不展示假现价（A2：无 live 时显示 —，不用 mark 冒充） |
| 估值价 `mark_price` | **非实时** — 成交价或上次 Settlement 收盘/开盘价 |
| 总权益所用价 | **始终 mark**，不跟盘中跳动 |

#### 3）什么时候刷新？

| 触发 | 行为 |
| --- | --- |
| 进入「我的组合」`onMounted` | 拉 snapshot + dashboard |
| 用户点「刷新」 | 同上 |
| 盘中自动轮询 / WS | **无** |
| 成交 / Settlement 后台 | 改 DB mark；**UI 需再刷新**才看见 |

`as_of`：Snapshot/Dashboard 组装时钟（展示「数据截至」）；**不是**逐票 Quote 时间戳（多数情况下仍缺 per-quote time）。

#### 4）Settlement 后是否覆盖？

| 对象 | Settlement 行为 |
| --- | --- |
| `paper_sim_positions.mark_price` | **是，批量覆盖**（MarkPricer / Open 回退） |
| 账户权益 / 市值 / 累计浮盈 | **随后按新 mark 计算** |
| `display_price` / Quote overlay | **不写库**；下次 GET 再拉 |
| `paper_sim_daily_reports` | 写入当日结算快照；**成为次日 `daily_pnl` 的对比基准** |
| T+1 锁仓 | Settlement **不**解锁当日买（unlock 在下一交易日晨间） |

---

## 二、Position 数据链

```text
上游行情 QuoteService（腾讯等）
        │  仅 GET snapshot?include_display=1
        ▼
display_price / quote_pre_close / today_pnl     ← 展示 overlay，不落库
        │
成交 fill / EOD Settlement
        ▼
paper_sim_positions.mark_price                  ← 账本真源
        ▼
portfolio.Snapshot（无 overlay）
        ▼
readmodel.Build(+ Quotes) → PositionView
        ▼
frontend PortfolioDashboard
  · 权益/市值/累计浮盈 ← mark
  · 行情列/持仓今日浮盈 ← display（若有）
```

### 2.1 是否存在价格延迟？

**是（结构性）。**

| 延迟源 | 说明 |
| --- | --- |
| mark 滞后 | 盘中仅成交更新该票 mark；未成交票停在昨收 Settlement |
| display 采样点 | 仅刷新瞬间；随后 UI 静止直至再刷 |
| Quote 失败 | overlay 缺失 → 持仓今日浮盈为空；权益仍用旧 mark |
| 双轨观感 | 用户看「行情」已变，顶部权益不动 → 感觉「数据不一致」 |

### 2.2 是否存在字段误用？

| 风险 | 现状 |
| --- | --- |
| 用 display 改 equity | **代码禁止**（readmodel 注释与测试） |
| 用累计浮盈冒充账户今日 | **禁止**（home Assemble 只用 dashboard daily_pnl） |
| 用 mark 冒充行情列 | A2 后 UI **避免**；无 live 显示 — |
| `DisplayPnL`（相对成本·display） | API 有字段；主表累计列用的是 **mark pnl**，一般不混用 |

### 2.3 是否存在命名误导？

| 历史问题 | A2 后状态 |
| --- | --- |
| 账户/持仓都叫「今日盈亏」 | **已区分**：「账户今日盈亏」vs「持仓今日浮盈」+ tooltip + basis 标签 |
| 「当前价」含糊 | **已分列**「估值价 / 行情价」 |
| 仍可能误解 | 用户仍可能把顶部盈亏当成「今天涨了多少钱（盘中）」——需持续文案/教育 |

---

## 三、当前问题 · 风险 · 最小修复

### 3.1 当前问题清单

1. **口径分裂（by design）**：权益跟 mark，行情跟 Quote；盘中视觉不一致。  
2. **账户今日 ≠ 盘中浮盈**：语义已标注，仍易被扫读误解。  
3. **无自动刷新 / 缺 Quote 时间戳**：新鲜度不可见。  
4. **Settlement 前 mark 陈旧**：长持仓盘中权益「冻」在昨收附近（除非有成交）。  
5. （次要）Holding Health / Exit 评价另有价源声明，与组合页未统一「评价用价」说明。

### 3.2 风险

| 等级 | 风险 |
| --- | --- |
| 高 | 用户按「盘中盈亏」决策，实际看的是日报差 |
| 中 | 刷新间隔长 → 以为系统卡死或数据错 |
| 中 | 对比券商 App「现价驱动总资产」产生信任危机 |
| 低 | API 字段名 `daily_pnl` / `today_pnl` 英文相近，集成方误用 |

### 3.3 最小修复方案（设计，本审计不实现）

**原则：不重建 Portfolio；不改交易执行；优先展示层。**

| 优先级 | 方案 | API 变化？ |
| --- | --- | --- |
| P0 | 保持 A2 文案/tooltip；总览旁固定一句：「权益按账本估值；账户今日盈亏=日报差，≠Σ持仓今日浮盈」 | **否** |
| P0 | 行情列旁展示 `display_quote_source` + 可选「刷新于 as_of」 | **否**（字段已有 source / as_of） |
| P1 | 可选：Quote 响应带 `quote_time`，UI 显示「行情截至 HH:mm」 | **小**（若上游已有则透出；无则可选） |
| P1 | 可选：手动刷新外增加「盘中每 N 分钟软刷新 overlay」（仍不写 mark） | **否** |
| P2 | 文档/测试契约页：三口径对照表（账户今日 / 持仓今日 / 累计） | **否** |
| 不做 | 用 Quote 重写 mark/equity 充当「真·盘中总资产」 | 会破坏 Settlement/日报一致性 |
| 不做 | 大规模 DB 迁移 | — |

### 3.4 是否需要 API 变化？

| 需求 | 建议 |
| --- | --- |
| 解决误解 / 一致性叙事 | **不需要**改 API — 前端语义 + 已有 `daily_pnl_basis` / `display_quote_source` 足够 |
| 展示行情时刻 | **可选**小扩展：`quote_as_of` per position 或全局 |
| 强制盘中权益跟 Quote | **不建议**；若产品坚持，应新字段 `display_equity` 与账本 equity **并列**，禁止覆盖 mark |

---

## 四、建议结论

1. **数据链正确、口径分裂是设计选择**，不是「算错了」。  
2. **账户今日盈亏不是盘中盈亏**；盘中看「持仓今日浮盈」+ 行情列。  
3. **Settlement 覆盖 mark 并冻结日报**；overlay 从不被 Settlement 持久化。  
4. **最小修复 = 展示与新鲜度**，不是改账本；API 默认不变。  

---

## 五、合规确认

| 要求 | 状态 |
| --- | --- |
| 只读审计 | **是** |
| 未修改代码 | **是** |

---

## 附录 — 关键路径

- `backend/portfolio/snapshot.go` · `project.go` · `dashboard.go`  
- `backend/portfolio/readmodel/build.go` · `view.go`  
- `backend/papertrading/settlement.go` · `broker.go`（成交写 mark）  
- `frontend/src/components/PortfolioDashboard.vue`  
- `frontend/src/api/portfolioSnapshot.ts` · `portfolioDashboard.ts`  

---

*Phase17 Portfolio Data Consistency Audit 结束。*
