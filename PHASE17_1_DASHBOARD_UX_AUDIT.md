# PHASE17.1 Dashboard UX 实现前审计（只读）

**日期：** 2026-09-07  
**性质：** 只读实现前审计（**未改代码 / 未改 API / 未改策略逻辑**）  
**范围：** 首页 `InvestmentHome.vue` 三个体验问题  
**依据源码：** `InvestmentHome.vue` · `investmentHome.ts` · `opportunityCard.js` · `marketColor.js` · `portfolio/decision` · `portfolio/home`

---

## 0. 总览

| # | 问题 | 根因摘要 | 建议处理层 |
| --- | --- | --- | --- |
| 1 | 今日交易流程过大 | **纵向**大卡片步骤条（每步整行 + ↓），占首屏 | **仅前端布局/CSS**（可改横向步骤条） |
| 2 | 今日机会同一股票重复 | 一候选 × 多持仓对比 → 每条 highlight 一行，股票列只显示候选 | **优先展示层去重/改列**；后端不必为首页去重 |
| 3 | 账户今日盈亏颜色 | 本地 `pnlColor` 用 **绿涨红跌**；与 A 股 / `marketColor.js` **红涨绿跌** 相反 | **前端展示**（统一到 `marketColor`/`pnlColor`）；计算逻辑不动 |

---

## 一、交易流程组件

### 1.1 文件位置

| 项 | 值 |
| --- | --- |
| 页面 | `frontend/src/components/InvestmentHome.vue` |
| 步骤定义 | 同文件常量 `BETA_TRADING_FLOW`（约 L405–436） |
| 状态计算 | `currentFlowStepKey` / `betaTradingFlowSteps`（约 L438–462） |
| 模板 | `section.beta-flow-card` → `div.beta-flow-steps`（约 L589–635） |
| 样式 | 同文件 `<style>`：`.beta-flow-steps` / `.beta-flow-step` / `.beta-flow-arrow`（约 L1111+） |
| 独立组件 | **无**（未抽成 `TradingFlow*.vue`） |

### 1.2 数据来源

| 输入 | 用途 |
| --- | --- |
| `planUserStatus(guidancePlan)` | 由交易计划上下文映射 `none` / `pending_confirm` / `prepared` / `locked` |
| `guidancePlan` | `planContext.current_plan` 或 `next_plan`（与 `GET /api/investment/home` 并行的计划读路径） |
| `BETA_TRADING_FLOW` | **纯前端**五步文案与路由：发现机会 → 生成计划 → 准备价格 → 批准冻结 → 模拟成交 |

**不**直接绑定后端 `trading_status` 五段 PASS/FAIL 流水线（遗留 pipeline 区块在 `SHOW_LEGACY_HOME_BLOCKS` 下隐藏）。  
当前「已完成 / 下一步」是 **UI 引导态**，按计划卡片状态推算当前焦点步，**不触发交易**。

### 1.3 当前布局方式

```text
flex-direction: column   ← 纵向堆叠
每步：整宽 button（序号圆 + 标题 + hint + Tag）
步间：单独一行「↓」箭头
```

单步 `padding: 10px 12px`、圆角边框卡片，五步 + 四条箭头 ≈ **大半屏高度**，与 Runtime 截图「今日交易流程」占首屏一致。

同页已有横向参考：遗留 `.pipeline`（`display: flex; flex-wrap: wrap`）仍在样式中，但主路径已不用。

### 1.4 能否改为横向步骤条？

| 问题 | 答案 |
| --- | --- |
| 是否可行 | **是** |
| 是否要改 API | **否** |
| 是否要改步骤语义 | **否**（保留 `BETA_TRADING_FLOW` + `goFlowStep`） |
| 实现面 | 调整 `.beta-flow-steps` 为 `row` + 连接线/`→`；缩小 padding / 可折叠 hint；窄屏可 `flex-wrap` |
| Naive `n-steps` | 可选，非必须；现有 button+状态 class 已够 |

**实现建议（本审计不实施）：** 仅改模板结构与 CSS，保持点击路由与 `current`/`done`/`upcoming` 逻辑。

---

## 二、今日机会（重复同一股票）

### 2.1 数据链（CandidatePool → 首页）

```text
CandidatePool（当日策略池，decision.Service 取最新）
    ↓ 取 Top / 评分后的单一 CandidateRow
portfolio/decision.Build → OpportunityAttention
    · candidate_code / candidate_score   ← 通常 1 个候选
    · highlights[]                       ← 对每个「分差够大」的持仓一条
    ↓ JSON: decision_summary.opportunity_attention
GET /api/investment/home
    ↓
investmentHome.ts mapHome → toOpportunityCards(opp, hints, 10)
    ↓ 每个 highlight → 一张 card（candidate 相同，holding 不同）
InvestmentHome opportunityTableRows
    ↓ 股票列渲染 candidate_stock.display
n-data-table「今日机会」→ 同一 code 多行
```

关键文件：

| 层 | 文件 | 行为 |
| --- | --- | --- |
| 候选池 | `backend/portfolio/decision/service.go` | 装载 `CandidatePool` → `in.Candidate` |
| 对比构建 | `backend/portfolio/decision/build.go` `buildOpportunity` | 一候选 vs 多持仓，写入 `Highlights` |
| 类型 | `decision/types.go` | `OpportunityHighlight{HoldingCode, HoldingScore, Reason}` |
| 适配 | `frontend/src/utils/opportunityCard.js` `toOpportunityCards` | **按 highlight 展开多卡** |
| 首页表 | `InvestmentHome.vue` `opportunityTableRows` | 行 key=`code-holding-i`；**主列是候选** |

### 2.2 返回数据是否已经重复 `stock_code`？

| 视角 | 结论 |
| --- | --- |
| `opportunity_attention.candidate_code` | **不重复**（单字段一个候选） |
| `highlights[].holding_code` | 各不相同（不同持仓） |
| 首页表格「股票」列 | **视觉重复**：每行都画同一个 `candidate_code` |

因此：**不是 API 误返回多条相同候选记录**，而是 **对比展开的产品语义在表上被展示成「多条机会」**。

### 2.3 是否多个 signal 导致重复？

**否（就当前 home 链路而言）。**  
同一候选的 `user_label`（如「建议研究」）在每行相同；行差异来自 `holding_score` / `score_gap`（截图中风险列 +27 / +15 等），不是多条独立 signal 事件。

（`daily_attention` 的 OPPORTUNITY/TOMORROW 另有 `pickOpportunityItems`，**当前主表未用该路径**。）

### 2.4 是否需要后端去重？

| 选项 | 建议 |
| --- | --- |
| 为首页改 `buildOpportunity` 只留一条 highlight | **不推荐作为首步**——会削弱决策页「相对哪只仓」的审计信息 |
| 后端另出 `home_opportunity_rows` 去重 DTO | **非必须**；增加 API 面，本切片禁止改 API 也更贴合「展示层处理」 |
| **展示层** | **推荐** |

### 2.5 是否仅展示层处理？

**是，足够。** 可选方案（实现阶段择一，本审计不改）：

1. **按 `candidate_code` 折叠为 1 行**：保留最大 `score_gap`（或持仓数摘要）；最贴「今日机会列表」心智。  
2. **改列语义**：股票列仍为候选，但增加「对比持仓」列，并限制 Top1 候选只展示 TopK highlight（或默认 1）。  
3. **文案**：副标题标明「候选 vs 持仓对比」，降低「重复机会」误解。

**禁止（本问题范围）：** 改 CandidatePool 生成、改评分、改 `scoreGapReview` 策略阈值。

---

## 三、盈亏颜色（账户今日盈亏）

### 3.1 字段来源

| 项 | 值 |
| --- | --- |
| 首页字段 | `portfolio.dailyPnl` ← `portfolio_summary.daily_pnl` |
| 映射 | `frontend/src/api/investmentHome.ts` `nullableNum(ps.daily_pnl)` |
| 后端组装 | `backend/portfolio/home/assemble.go`：优先 Dashboard `Summary.DailyPnL`（`daily_report_delta`），**禁止用持仓浮盈代替** |
| 语义 | 相对上一结算日报的 **账户权益差**（Phase17-A.2 tooltip 已写明） |

计算链正确；问题在 **着色**。

### 3.2 正负颜色判断位置

| 位置 | 逻辑 | 约定 |
| --- | --- | --- |
| `InvestmentHome.vue` 本地 `pnlColor`（约 L375–378） | `n > 0 → #18a058`（绿）；`n < 0 → #d03050`（红） | **西式：绿涨红跌** |
| `PortfolioDashboard.vue` 本地 `pnlColor`（约 L192–195） | **同上** | 同左 |
| `frontend/src/utils/marketColor.js` | `up → marketUp #ec0000`；`down → marketDown #00b578` | **A 股：红涨绿跌**（Phase16.26-A） |
| `designTokens.js` | `sysSuccess #18a058` / `sysDanger #d03050` 标注 **非行情涨跌色** | 设计已区分 |

首页「账户今日盈亏」绑定的是 **本地错误约定**，与设计 token / `marketColor.pnlColor` **不一致**。  
Runtime 中盈利显示为绿色，符合本地函数，但 **不符合 A 股用户预期与项目已有规范**。

### 3.3 是否多个页面共用？

| 页面 | 是否共用同一实现 |
| --- | --- |
| 首页 `InvestmentHome` | 本地函数副本 |
| 我的组合 `PortfolioDashboard`（账户今日盈亏 + 累计浮盈 + 持仓今日浮盈等） | **另一份相同逻辑的本地副本** |
| `marketColor.js` / `pnlColor` | **已有规范实现，但上述两页未 import** |

→ **多页面共用错误约定（复制粘贴）**，未共用正确工具函数。

### 3.4 实现建议（不实施）

- 删除/替换两页本地 `pnlColor`，改为 `import { pnlColor } from '../utils/marketColor'`（或 `useCssVar: false` 取 hex）。  
- **不改** `daily_pnl` 计算、不改 API、不改 Snapshot。  
- 回归：首页 + 组合总览 + 持仓表相关着色一并目检。

---

## 四、实现边界（给下一切片）

| 允许 | 禁止 |
| --- | --- |
| `InvestmentHome.vue` 流程区 CSS/DOM 紧凑化或横向步骤 | 改交易状态机 / Freeze / Gateway |
| `opportunityTableRows` / 列定义展示去重或改列 | 改 CandidatePool / `buildOpportunity` 策略阈值（除非单独立项） |
| 首页 + 组合统一引用 `marketColor.pnlColor` | 改 `daily_pnl` 公式或日报逻辑 |
| 文案微调（对比说明） | 新 API、改后端 response shape（非必须时） |

---

## 五、结论

| # | 审计结论 |
| --- | --- |
| 1 流程过大 | **确认**：纵向大卡片；**可**横向步骤条，仅 UI |
| 2 机会重复 | **确认**：单候选 × 多 highlight 展开；**展示层处理即可**；**不必**后端去重作首步 |
| 3 盈亏颜色 | **确认**：本地绿涨红跌 vs 规范红涨绿跌；首页与组合需对齐 `marketColor` |

**实现前就绪：PASS（可开 Phase17.1 实现切片）。**  
本文件为审计交付；**未修改任何代码。**
