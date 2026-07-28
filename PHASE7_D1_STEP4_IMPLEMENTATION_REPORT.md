# Phase7-D1 Step4 Implementation Report

> 任务：C-OBS-001 价格来源展示优化（仅展示层）  
> 约束：未改后端业务逻辑 / 未改数据库 / 未 git add / 未 git commit

---

## 1. 修改文件

| 文件 | 变更 |
|---|---|
| `frontend/src/components/PaperTradingObservation.vue` | 增加价格来源标签、tooltip 解释、账户级 overlay 状态标签 |

> `frontend/src/api/paperObservation.ts` 未修改（现有字段映射已完整覆盖所需字段）。

---

## 2. 展示规则

### 2.1 行级（当前价列）

使用既有字段：
- `quoteSource`
- `markPrice`
- `displayPrice`
- `persistedMarkPrice`
- `quoteUpdatedAt`

映射规则：
- `live` → **实时行情**
- `open_fallback` → **开盘价/备用价格**
- `persisted`（及空值）→ **历史成交价**

展示方式：
- 当前价数值后展示来源标签（`NTag`）
- hover 标签显示 tooltip，内容包括：
  - 展示价格（display/mark）
  - 历史成交价（persistedMarkPrice）
  - 来源中文说明
  - 更新时间

### 2.2 账户级（持仓区标题旁）

使用既有字段 `quoteOverlay`：
- `true`：显示 **实时行情覆盖中**
- `false`：显示 **使用历史成交价**

---

## 3. 验证方式

### 3.1 代码路径核对

- 当前价列已改为自定义 `render`，并接入 `quoteSourceLabel()` 映射
- 持仓区域新增 `quoteOverlay` 状态标签
- 所有展示均来自前端已接收字段，无新增 API 契约

### 3.2 页面手工验收建议

在“模拟盘观察”页面验证：
1. 行情为 live 时：标签显示“实时行情”
2. 行情为 open_fallback 时：标签显示“开盘价/备用价格”
3. 行情为 persisted 或空时：标签显示“历史成交价”
4. tooltip 可看到展示价、历史成交价、来源、更新时间
5. 顶部可看到 `quoteOverlay` 状态标签切换

---

## 4. 交易链路影响评估

| 维度 | 影响 |
|---|---|
| QuoteService | 无影响 |
| Observation 计算（mark/display/overlay） | 无影响 |
| PnL / ReturnRate 计算 | 无影响 |
| `paper_sim_positions` | 无影响 |
| Fill / Broker / Settlement / Execution | 无影响 |
| TradePlan / Risk | 无影响 |
| DB 字段 / migration | 无影响 |

结论：本次为纯前端解释性增强，不改变任何交易或计算逻辑。

---

## 5. 状态

```text
[x] C-OBS-001 展示优化完成
[x] 使用既有字段 quoteSource/quoteOverlay/persistedMarkPrice/markPrice/displayPrice
[x] 仅前端展示改动
[x] 未修改后端业务逻辑
[x] 未修改数据库
[x] 未 git add
[x] 未 git commit
```
