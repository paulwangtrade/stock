# PHASE10_B.1 — Runtime Smoke Report

> **日期：** 2026-08-05  
> **性质：只读冒烟 — 未修改业务代码**  
> **git HEAD：** `b4e1ca590c9ab845b767e31b41186f9e4f11acec`（`b4e1ca5` = B.0.5 populate 接线）  
> **运行代码面：** **工作区** 已实现但未 commit 的 B.1（`kline_close.go` + `DefaultProvider` Chain）  
> **DB：** `D:\stock\build\bin\data\stock.db`  

---

## 0. 结论

| 项 | 结果 |
|----|------|
| KlineClose 已进入 draft→populate→DefaultProvider 链 | **PASS**（工作区） |
| ∉自选 + SourceDate 日 K Close → `selected` / `kline_close` | **PASS**（plan #23） |
| 未提前 materialize（limit/volume=0） | **PASS** |
| git HEAD 是否已含 B.1 | **否**（仍仅 Followed；B.1 待 commit） |

**总评：** 工作区 B.1 运行时行为满足 DATA-003 缓解冒烟标准；正式入库前需 scoped commit B.1 文件。

---

## 1. 调用链检查

### 1.1 生产路径（与 HEAD 接线 + 工作区 Provider）

```text
RunAfterClosePlanWorkflow
  → BuildCandidatePool(..., source_date)
  → BuildDraftTradePlanFromCandidatePool          // B.0.5: 已调 populate
       → populateAfterCloseExecutionIntent
            → afterCloseAnchorProvider
                 = DefaultProvider()              // 工作区 B.1
                      1) KlineCloseAnchorProvider // SourceDate 日线 Close
                      2) FollowedStockAnchorProvider
       → CreatePlanWithItems
```

### 1.2 DefaultProvider Chain 顺序（运行时断言）

```text
CHAIN_OK=true LEN=2
CHAIN_ORDER_KLINE_THEN_FOLLOWED=true
```

| Index | 类型 |
|-------|------|
| 0 | `KlineCloseAnchorProvider` |
| 1 | `FollowedStockAnchorProvider` |

### 1.3 相对 git HEAD 的缺口

| 路径 | HEAD `b4e1ca5` | 工作区 |
|------|----------------|--------|
| `kline_close.go` | 无 | `??` 新增 |
| `provider.go` DefaultProvider | Followed only | Kline → Followed（`M`） |
| `build_draft` populate 接线 | 有 | 有 |

冒烟使用 **`go run` 编译当前模块工作区**，故验证的是 B.1 实现面，而非干净 checkout 的 `b4e1ca5` 二进制。

---

## 2. 测试计划选择

| 条件 | 取值 |
|------|------|
| 不在 `followed_stock` | **`sz300836`**（计数=0） |
| `source_date` | **`2026-07-27`** |
| 日线 Close | **59.42**（东财 EOF 后腾讯 K 线回退成功；经 `KlineService`/`kline_cache` 路径） |
| pool | `#22`，`strategy_run`，ConfigJSON 含 `source_date` |
| trade_date | `2099-02-03`（合成，避免撞实盘日） |

直接 Resolve 探针：

```text
DIRECT_RESOLVE ok=true ref=59.4200 src="kline_close" asof="2026-07-27"
```

---

## 3. 数据库验证结果

**生成：** `BuildDraftTradePlanFromCandidatePool` → plan **#23**

| 字段 | 期望 | 实测 |
|------|------|------|
| `pricing_stage` | `after_close_intent` | `after_close_intent` |
| `pricing_policy_version` | ≥1 | `1` |
| `intent_status` | `selected` | **`selected`** |
| `ref_price` | >0 | **59.42** |
| `ref_source` | `kline_close` | **`kline_close`** |
| `ref_as_of` | source_date | `2026-07-27` |
| `limit_price` | 0 | **0** |
| `target_volume` | 0 | **0** |

```text
PLAN_ID=23 stage="after_close_intent" policy=1
ITEM code=sz300836 intent="selected" ref=59.4200 src="kline_close" asof="2026-07-27" limit=0.0000 vol=0
SMOKE_PASS=true
```

**未提前 materialize：** limit/volume 均为 0；`pricing_stage` 仍为 `after_close_intent`（非 `morning_materialized`）。

---

## 4. 观测备注

- 外网东财 HTTP 曾 EOF；腾讯 K 线回退成功后仍得到正确 `SourceDate` Close，说明 Provider 消费的是 **KlineService 聚合路径**（含 cache/fallback），而非自选价。  
- 本冒烟写入 pool#22 / plan#23（合成 trade_date）；属验证写，非业务代码变更。

---

## 5. 进入后续步骤的含义

| 问题 | 判定 |
|------|------|
| B.1 行为是否成立？ | **是**（工作区） |
| 是否可缓解 DATA-003（∉自选）？ | **是**（在日 K 可得时） |
| 干净 HEAD 是否已交付？ | **否** — 需 commit：`kline_close.go`、`kline_close_test.go`、`provider.go` |

---

## 6. Registry

```text
PHASE10-B.1   IMPLEMENTATION (worktree)  — smoke PASS
PHASE10-B.1   COMMIT                     — pending (not in b4e1ca5)
DATA-003      PARTIALLY MITIGATED        — kline_close path verified
```

---

## 7. 边界声明

- 未修改仓库业务源代码（仅临时 `go run` 探针 + 本报告）  
- 报告输出：`PHASE10_B1_RUNTIME_SMOKE_REPORT.md`  
