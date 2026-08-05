# PHASE10-B.1 — KlineCloseAnchorProvider 验收文档

> **日期：** 2026-08-05  
> **性质：** 验收记录 — **未修改业务代码**  
> **相关 commit：** `0575b53` — `feat(anchor): add Phase10-B.1 KlineCloseAnchorProvider to DefaultProvider chain`  
> **运行库：** `D:\stock\build\bin\data\stock.db`  
> **二进制参考：** `D:\stock\build\bin\go-stock.exe`（2026-08-05 09:10 构建，含 B.1）

---

## 1. 背景

### 旧问题

AfterClose Execution Intent 的 `ref_price` 锚定长期依赖 **Followed snapshot**（`FollowedStockAnchorProvider` / 自选 `FollowPrice`·`Price`）。

当候选标的：

- 不在自选，或
- 自选价尚未写入 / 为 0

时，`populateAfterCloseExecutionIntent` 按设计 **soft-fail**：

- `intent_status = ""`
- `ref_price = 0`
- 行仍保留在 Draft 中

结果：早盘物化（`MaterializeMorningLimitPrices`）只处理 `intent_status=selected` 且 `ref_price>0` 的项 → 大量行无法物化 → Readiness 持续出现：

- `INTENT_NOT_MATERIALIZED`
- `LIMIT_PRICE_MISSING`
- `TARGET_VOLUME_BELOW_LOT`
- `NO_TRADEABLE_ITEMS`
- `ENTRY_PRICE_MISSING`

典型历史样本：plan **#19**（生成于 B.1 上线前，且多数标的当时不在自选）。

### 目标（B.1）

在不改变 Intent schema / 不提前写 Order Spec（limit/volume 仍为 0）的前提下，用 **SourceDate 日 K Close** 作为主锚点，缓解 Followed-only 覆盖不足（DATA-003）。

---

## 2. 实现

### 2.1 组件

| 文件 | 作用 |
|------|------|
| `backend/marketdata/anchor/kline_close.go` | `KlineCloseAnchorProvider`：按 `Context.SourceDate` 精确匹配日线 Close |
| `backend/marketdata/anchor/kline_close_test.go` | 单测（命中/空 SourceDate/链回退等） |
| `backend/marketdata/anchor/provider.go` | `DefaultProvider` 接入 Chain |

### 2.2 DefaultProvider Chain

```text
DefaultProvider()
  → Chain
       1) KlineCloseAnchorProvider   // SourceDate 日 K Close → ref_source=kline_close
       2) FollowedStockAnchorProvider // 回退：自选 FollowPrice/Price
```

调用链（AfterClose）：

```text
RunAfterClosePlanWorkflow / POST /api/tradeplans/generate-next
  → BuildCandidatePool(..., source_date in config_json)
  → BuildDraftTradePlanFromCandidatePool
       → populateAfterCloseExecutionIntent
            → afterCloseAnchorProvider = DefaultProvider()
       → CreatePlanWithItems   // limit_price=0, target_volume=0
```

### 2.3 KlineClose 成功条件（摘要）

- `StockCode` 非空且 `SourceDate` 非空、格式合法
- `KlineService.GetBars` 返回日线
- 存在 **日历日 = SourceDate** 的 bar，且 **Close > 0**
- **不使用** TradeDate、实时价、Open 作为 `ref_price`

失败 → `ok=false` → populate soft-fail（不伪造价格）。

---

## 3. 端到端验证（主验收）

### 3.1 样本

| 项 | 值 |
|----|-----|
| **plan_id** | **#26** |
| source_date | `2026-08-04` |
| trade_date | `2026-08-05` |
| 入口 | `POST /api/tradeplans/generate-next` → `POST /api/tradeplans/materialize-morning` |
| DB | `build/bin/data/stock.db` |

> TradePlan HTTP 挂在 Wails AssetServer；验收通过同一 `TradePlansHandler` + 运行库完成（与 exe 同源）。

### 3.2 AfterClose（generate-next）

| 检查项 | 结果 |
|--------|------|
| Draft / `pricing_stage=after_close_intent` | ✅ |
| **5/5 `intent_status=selected`** | ✅ |
| **`ref_source=kline_close`（5/5）** | ✅ |
| `ref_as_of=2026-08-04` | ✅ |
| `limit_price=0` / `target_volume=0`（未提前物化） | ✅ |

| 代码 | intent | ref_price | ref_source |
|------|--------|-----------|------------|
| sh603986 | selected | 356.02 | kline_close |
| sh600641 | selected | 24.41 | kline_close |
| sz002965 | selected | 28.94 | kline_close |
| sz002208 | selected | 10.69 | kline_close |
| sh601133 | selected | 23.19 | kline_close |

### 3.3 早盘物化（materialize-morning）

| 检查项 | 结果 |
|--------|------|
| API `success=true` | ✅ |
| `materialized_items=5` | ✅ |
| **`pricing_stage=morning_materialized`** | ✅ |
| 5 项 `intent_status=priced` + `limit_price>0` + `target_volume≥100` | ✅ |
| **`readiness_ready=true` / blockers=[]** | ✅ |

物化后摘要：

| 代码 | intent | limit_price | target_volume | open_ref |
|------|--------|-------------|---------------|----------|
| sh603986 | priced | 366.70 | 200 | 358.00 |
| sh600641 | priced | 25.14 | 3900 | 24.50 |
| sz002965 | priced | 29.81 | 3300 | 28.60 |
| sz002208 | priced | 11.01 | 9000 | 10.63 |
| sh601133 | priced | 23.89 | 4100 | 23.30 |

### 3.4 主验收结论

**PASS。** 在 SourceDate 已有日 K Close 的前提下，B.1 打通：

`KlineClose selected` → 早盘 Spec 物化 → **Readiness Ready**。

---

## 4. 边界验证

### plan #25（未来 / 无日线的 SourceDate）

| 项 | 值 |
|----|-----|
| plan_id | **#25** |
| source_date | `2026-08-06`（验收当日相对「未来」、尚无完整日 K） |
| trade_date | `2026-08-07` |

现象：

- KlineClose 无法命中 SourceDate bar → soft-fail
- 仅自选中的 `sh603986` 回退为 `strategy_snapshot` / `selected`
- 其余 4 只仍 `intent=""` / `ref_price=0`
- 早盘物化仅 1 项完整 Spec → `readiness_ready=false`

### 判定

**属于正确行为，不是 B.1 回归。**

Anchor 禁止伪造价格；无 SourceDate 日线 Close 时不得写入 `kline_close` selected Intent。

---

## 5. 当前限制

1. **依赖 SourceDate 对应交易日已有日 K Close**  
   - 选尚未收盘 / 尚未落库的日期 → KlineClose 失败（见 plan #25）。
2. **日线数据源可用性**  
   - 经 EastMoney + `kline_cache`；网络/cookie 异常时可短暂失败，再回退 Followed。
3. **Followed 仍为回退，不是主路径**  
   - 未自选且 K 线缺失时，行为与旧 soft-fail 相同。
4. **AfterClose 仍不写 Order Spec**  
   - `limit_price` / `target_volume` 必须由早盘物化写入。
5. **Ready ≠ Approve/Freeze**  
   - 本验收止于 Intent Readiness；不自动批准或冻结。

---

## 6. 验收清单（汇总）

| # | 项 | 结果 |
|---|-----|------|
| 1 | KlineClose 接入 `DefaultProvider`（先于 Followed） | ✅ |
| 2 | 有日 K 的 SourceDate → 多标的 `selected` + `kline_close`（plan #26） | ✅ |
| 3 | 早盘物化 → Spec + `morning_materialized` + Ready（plan #26） | ✅ |
| 4 | 无日 K 的 SourceDate → soft-fail / 不伪造（plan #25） | ✅ |
| 5 | 文档化当前限制（SourceDate 日 K 必备） | ✅ |

**总评：Phase10-B.1 KlineCloseAnchorProvider 验收通过。**

---

*文档结束（只生成文档，未改代码）。*
