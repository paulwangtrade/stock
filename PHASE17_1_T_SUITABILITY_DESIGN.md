# PHASE17.1 Holding T-Suitability — 设计文档

**日期：** 2026-09-07  
**阶段：** Phase17.1 第二阶段（**仅设计，不实现**）  
**上游审计：** [PHASE17_1_T_SUITABILITY_IMPLEMENTATION_AUDIT.md](./PHASE17_1_T_SUITABILITY_IMPLEMENTATION_AUDIT.md)  
**约束：** 不改交易执行 / 不自动买卖 / 不改 ExitEval state / 不新建表 / 不覆盖 mark_price。  

**本阶段动作：** 输出本设计后 **暂停**，等待实现确认。

---

## 0. 设计目标与定位

### 0.1 目标

建立 **`HoldingTSuitability`**：持仓「做 T」适宜性评价层（Holding Intelligence）。

| 是 | 不是 |
| --- | --- |
| 持仓智能 / 只读适宜性投影 | Trading Signal |
| 解释「现在做 T 是否操作上可行 + 环境是否合适」 | Buy Signal / Sell Signal |
| 消费 HoldingEval → Explanation → HealthScore | ExitEval 状态机输入 |

### 0.2 架构位置

```text
Entry
  Signal → TradePlan → Fill
Holding Evaluation
  Position → HoldingEval → Explanation → HealthScore → HoldingTSuitability
Exit Evaluation
  ExitEval（只读取持仓评价结果作展示旁路；不因 T-Suitability 改 state）
```

**硬边界：**

- `HoldingTSuitability` **只读投影**；不写 DB、Broker、Settlement、TradePlan。  
- ExitEval **只读取**既有 Holding 链结果；**不**因 `Level` 改写 NORMAL/WATCH/REVIEW。  
- 组合页既有「卖出」Draft 入口与「做 T」列 **解耦**（适宜性 Drawer 内禁止交易按钮）。

---

## 1. DTO 设计

### 1.1 `HoldingTSuitability`

```go
// HoldingTSuitability — Phase17.1 read-only T-suitability projection.
// Not a trade signal. Does not mutate ExitEval / mark / Broker.
type HoldingTSuitability struct {
	StockCode         string    `json:"stock_code"`
	Level             string    `json:"level"` // suitable | caution | unsuitable
	Reasons           []string  `json:"reasons"`
	CanSell           bool      `json:"can_sell"`
	Freshness         string    `json:"freshness"`          // FRESH | STALE | UNKNOWN
	VolatilityStatus  string    `json:"volatility_status"`  // ACTIVE | NORMAL | LOW | UNKNOWN
	HealthGrade       string    `json:"health_grade"`       // A|B|C|D|""
	EvaluatedAt       time.Time `json:"evaluated_at"`
}
```

### 1.2 常量（实现阶段落地）

| 字段 | 允许值 |
| --- | --- |
| `Level` | `suitable` · `caution` · `unsuitable` |
| `Freshness` | 复用 Explanation：`FRESH` · `STALE` · `UNKNOWN` |
| `VolatilityStatus` | `ACTIVE` · `NORMAL` · `LOW` · `UNKNOWN` |
| `HealthGrade` | `A` · `B` · `C` · `D` · `""`（缺省） |

### 1.3 字段来源

| 字段 | 真源 | 说明 |
| --- | --- | --- |
| `CanSell` | `PositionState.can_sell`（paper_sim） | `available_qty > 0`；**不以** Follow 仓为准 |
| `Freshness` | Explanation.`freshness.status`（或 price_status） | 已有 Kline / quote freshness；可映射 `PRICE_STALE` |
| `HealthGrade` | `HoldingHealthScore.grade` | 只读拷贝；**不单独硬决定** `unsuitable`（见 §2.3） |
| `VolatilityStatus` | 调用方注入的 5m K 振幅启发式 | `(high−low)/close` 近 N 根；**禁止** MACD/RSI |
| `Reasons` | 规则引擎输出的解释码 | 无 BUY/SELL 文案 |
| `EvaluatedAt` | `as_of` / `time.Now()` | 投影时刻 |

### 1.4 输入契约（纯函数可测）

```go
type HoldingTSuitabilityInput struct {
	StockCode        string
	CanSell          bool
	AvailableQty     int64   // optional gate: < 1 lot → treat as not sellable
	LockedQty        int64   // >0 && !CanSell → LOCKED_POSITION
	Freshness        string  // FRESH|STALE|UNKNOWN
	HasPriceStaleTag bool    // Explanation risk_hints contains PRICE_STALE
	VolatilityStatus string  // ACTIVE|NORMAL|LOW|UNKNOWN；缺省 UNKNOWN
	HealthGrade      string  // A|B|C|D|""
	AsOf             time.Time
}
```

**原则：** `BuildHoldingTSuitability(input)` **不强制** HTTP 拉东财；5m 波动由上游（FE 已有 K / 可选 BE provider）注入。缺省 `VolatilityStatus=UNKNOWN` → 第三层前按 caution 倾向处理（见 §2.2）。

---

## 2. 评价规则设计（v0）

规则按层叠加；**第一层硬否决优先**。最终 `Level` 取各层约束下的最严结果（`unsuitable` > `caution` > `suitable`）。

### 2.1 第一层 — 交易可操作性门槛（硬）

若满足任一：

| 条件 | Reason 码 | Level |
| --- | --- | --- |
| `CanSell == false` | `LOCKED_POSITION` 或 `NO_SELLABLE` | **unsuitable** |
| T+1 锁仓（`!CanSell` 且 `LockedQty > 0` / 全日锁定） | `LOCKED_POSITION` | **unsuitable** |
| 行情过期：`Freshness == STALE` 或 `HasPriceStaleTag` | `PRICE_STALE` | **unsuitable** |

**说明：**

- `available_qty == 0` 与 `can_sell=false` 对齐 PositionState。  
- 部分锁定但仍 `can_sell=true`（S3）：**不**因存在锁定量硬否；可在 Reasons 附加 `PARTIAL_LOCKED`（caution 提示，可选 v0）。  
- 本层触发后：**仍可继续填** `VolatilityStatus` / `HealthGrade` 供 Drawer 展示，但 `Level` 保持 `unsuitable`。

### 2.2 第二层 — 行情活跃度（5m 振幅）

**计算（注入或本地）：**

```text
对近 N 根 5m bar（建议 N=12，约 1 小时）：
  amp_i = (high_i - low_i) / close_i   (close_i > 0)
  amp   = mean(amp_i) 或 median(amp_i)
```

**映射（v0 阈值，实现可配置常量）：**

| VolatilityStatus | 建议阈值（占 close） | 对 Level 影响 |
| --- | --- | --- |
| `ACTIVE` | amp ≥ 1.0% | 不降级（保持上层结果） |
| `NORMAL` | 0.4% ≤ amp < 1.0% | 不降级 |
| `LOW` | amp < 0.4% | → 至少 **caution**；Reason `LOW_VOLATILITY` |
| `UNKNOWN` | 无 5m / 不足 N 根 | → 至少 **caution**；Reason `VOLATILITY_UNKNOWN` |

**禁止：** MACD、RSI、复杂因子库。

### 2.3 第三层 — 持仓健康（软）

读取 `HealthGrade`；**不直接决定 `unsuitable`**（相对审计草案的明确产品裁定）。

| HealthGrade | 行为 |
| --- | --- |
| A / B | 保持第一、二层结果；可附加正向/中性 Reason（可选，v0 可不加） |
| C | 至少 **caution**；Reason `HEALTH_WATCH`（增加观察） |
| D | 至少 **caution**；Reason `HEALTH_RISK`（风险提示）；**不**因此升为 unsuitable |
| 空 / 缺失 | 不降级硬否；Reason `HEALTH_UNKNOWN`（可选 caution） |

**与第一层关系：** 若第一层已 `unsuitable`，Health 仅充实 Reasons / 展示字段。

### 2.4 合成示例

| 场景 | 期望 Level | 关键 Reasons |
| --- | --- | --- |
| 可卖 + FRESH + NORMAL/ACTIVE + Health A/B | `suitable` | （可空或 `OPERABLE`） |
| 可卖 + FRESH + LOW + Health A | `caution` | `LOW_VOLATILITY` |
| 可卖 + FRESH + ACTIVE + Health D | `caution` | `HEALTH_RISK` |
| T+1 锁仓 | `unsuitable` | `LOCKED_POSITION` |
| 行情 STALE | `unsuitable` | `PRICE_STALE` |
| 可卖 + UNKNOWN 波动 + Health C | `caution` | `VOLATILITY_UNKNOWN`, `HEALTH_WATCH` |

### 2.5 Reasons 词表（v0）

| Code | 含义 |
| --- | --- |
| `LOCKED_POSITION` | T+1 / 不可卖锁定 |
| `NO_SELLABLE` | 无可卖数量 |
| `PRICE_STALE` | 行情/评价价过期 |
| `LOW_VOLATILITY` | 近窗 5m 振幅偏低 |
| `VOLATILITY_UNKNOWN` | 缺 5m 波动输入 |
| `HEALTH_WATCH` | Health C — 增加观察 |
| `HEALTH_RISK` | Health D — 风险提示 |
| `HEALTH_UNKNOWN` | 无 HealthScore |
| `PARTIAL_LOCKED` | 可选：部分锁定但仍可卖 |

**禁止**出现在 Reasons：`BUY` / `SELL` / 「建议买入」/「建议卖出」等交易指令语义。

---

## 3. 数据链与挂载（最小方案）

### 3.1 方案对比

| 方案 | 描述 | 成本 | 判定 |
| --- | --- | --- | --- |
| A. 挂 portfolio snapshot projection | Snapshot 需再编 PositionState+Health+波动 | 中高 | 否（Snapshot 职责是账本/展示价） |
| **B. 挂既有 exit-evaluation response** | 行上已有 Explanation + HealthScore；旁路加 `t_suitability` | **低** | **采纳（MVP）** |
| C. 新增只读 GET API | 独立 endpoint | 中 | 后置；非阻塞 |

### 3.2 裁定：**B（最小）**

```text
BuildExitEvaluation
  → HoldingEval → Explanation → HealthScore
  → [NEW] EnrichHoldingWithTSuitability / BuildHoldingTSuitability
  → ProjectExitEvaluation（拷贝 t_suitability 到行；Exit state 公式不变）
```

- JSON：`ExitEvaluationStockRow.t_suitability`（可选对象）。  
- **不**改变 Exit label / policy 阈值。  
- 波动：v0 允许 `VolatilityStatus=UNKNOWN`（FE 后续用已拉 5m 覆盖展示，或第二刀注入）；**禁止**在 ExitEval 热路径强制扫全市场 5m HTTP。

### 3.3 组合页数据流

```text
PortfolioDashboard
  Snapshot（持仓 + can_sell / PositionState）
  + exit-evaluation（health + t_suitability）
  → 「做 T」列 + HoldingTSuitabilityDrawer
```

若某票缺 `t_suitability`：列显示「—」；Drawer 空态说明评价未就绪（不 panic）。

### 3.4 禁止

- 新表 / 新 migration  
- Suitability 写 mark / 触发 Broker  
- ExitEval 因 Level 改 state  

---

## 4. 前端设计

### 4.1 「我的组合」列

| 项 | 设计 |
| --- | --- |
| 列名 | **做 T** |
| 展示 | `suitable` → 🟢 可考虑 · `caution` → 🟡 观察 · `unsuitable` → 🔴 不适合 |
| 点击 | 打开 `HoldingTSuitabilityDrawer` |
| 与「卖出」列 | **并列独立**；适宜性列不触发卖出 |

列 tooltip：「持仓做 T 适宜性评价 · 非交易指令」。

### 4.2 `HoldingTSuitabilityDrawer`

只读展示：

- 评价等级（Level + 中文标签）  
- Reasons（中文映射，无下单措辞）  
- 行情 Freshness  
- 波动状态 VolatilityStatus  
- 健康等级 HealthGrade  
- EvaluatedAt  

**禁止：**

- 买入按钮  
- 卖出按钮  
- 「建议买入 / 建议卖出 / 立即做 T」等交易建议文字  

页脚 disclaimer：「适宜性评价仅供观察，不生成买卖单。」

### 4.3 文案映射（展示层）

| Level | Chip | Drawer 标题语气 |
| --- | --- | --- |
| suitable | 🟢 可考虑 | 操作环境相对适宜（非指令） |
| caution | 🟡 观察 | 需观察 · 非指令 |
| unsuitable | 🔴 不适合 | 当前不适合做 T 操作评估（非指令） |

---

## 5. 测试设计

包建议：`backend/papertrading/holding_t_suitability_test.go`（与 C2/C3 同包风格）。

| # | 用例 | 输入要点 | 期望 |
| --- | --- | --- | --- |
| 1 | 正常可卖持仓 | CanSell=true, FRESH, NORMAL/ACTIVE, Health A/B | `suitable` |
| 2 | T+1 锁仓 | CanSell=false, LockedQty>0 | `unsuitable` + `LOCKED_POSITION` |
| 3 | 行情过期 | Freshness=STALE 或 PRICE_STALE | `unsuitable` + `PRICE_STALE` |
| 4 | 低波动 | CanSell=true, FRESH, LOW, Health A | `caution` + `LOW_VOLATILITY` |
| 5 | Health D | CanSell=true, FRESH, ACTIVE, Health D | `caution` + `HEALTH_RISK`（**非** unsuitable） |
| 6 | nil / 空输入 | `Build...(nil)` 或零值 Input | 不 panic；安全默认（如 unsuitable/caution + UNKNOWN 字段） |

附加（可选）：

- Health C → caution + `HEALTH_WATCH`  
- Exit projection：挂载后 Exit state 与挂载前 **字节级一致**（回归守门）

---

## 6. 实现边界清单（供确认）

| 项 | 做 | 不做 |
| --- | --- | --- |
| DTO + `BuildHoldingTSuitability` | ✅ | — |
| 挂 `exit-evaluation` 行字段 | ✅ MVP | 独立 GET（可后置） |
| 新表 | — | ❌ |
| 改 ExitEval 规则 | — | ❌ |
| 组合「做 T」列 + Drawer | ✅ | Drawer 内买卖按钮 ❌ |
| 5m 波动 | 注入 / UNKNOWN | 投影内强制全量拉 K ❌ |
| MACD/RSI | — | ❌ |

---

## 7. 与审计差异说明（产品裁定）

| 点 | 审计草案 | 本设计裁定 |
| --- | --- | --- |
| Health D | 倾向 unsuitable | **仅 caution + `HEALTH_RISK`**（不硬否） |
| 波动枚举名 | high/normal/low | **ACTIVE/NORMAL/LOW**（任务对齐） |
| 挂载 | A/B/C 待选 | **B：exit-evaluation 旁路** |

---

## 8. 阶段结论

1. **设计完成** — DTO / 三层规则 / 挂载 B / 前端列+Drawer / 测试表已冻结。  
2. **等待实现确认** — 确认后再开第三阶段编码。  
3. **本文件外不改代码。**

---

*PHASE17.1 T-Suitability 设计结束 · 暂停等待实现授权。*
