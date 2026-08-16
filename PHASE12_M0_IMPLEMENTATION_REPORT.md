# PHASE12-M0 — 数量契约层实施报告

**Date:** 2026-08-16  
**范围:** 数量语义统一 + Fill 读路径修复  
**未改:** `PositionState.Calculate`、T+1 Unlock 政策、TradePlan 执行、OrderQuantityPolicy、Gateway / fillBuy

---

## 1. 结论

| 目标 | 结果 |
|------|------|
| Canonical qty 语义落地（契约层） | ✅ 常量 + Fill.`Qty()` + LotRecord 注释 |
| Fill.`volume` → canonical qty | ✅ 装载 SQL 改为 `f.volume AS quantity` |
| 修复 `loadLotsByCode` | ✅ `service.go` |
| 回归测试（唯一来源 / 防错读 quantity） | ✅ 4 个新测全 PASS；原 Calculate 测仍绿 |

---

## 2. 变更文件（窄范围）

| 文件 | 变更 |
|------|------|
| `backend/portfolio/positionstate/service.go` | `loadLotsByCode` 使用 `fillLotQtySelectSQL`（`f.volume`） |
| `backend/portfolio/positionstate/quantity_contract.go` | **新建** Fill 物理列常量 + SELECT 契约 |
| `backend/portfolio/positionstate/types.go` | `LotRecord.Quantity` 注明 canonical / 来自 `volume` |
| `backend/portfolio/positionstate/load_lots_test.go` | **新建** 装载回归测 |
| `backend/papertrading/models.go` | `PaperSimFill.Qty()`；`Volume` 注释（物理列保留） |

**未改文件（刻意）：** `calculate.go`、`t1_unlock.go`、`broker.go`、`rescale_trade_plan_cash.go`、TradingRule / QuantityPolicy。

---

## 3. Canonical 语义（M0）

```text
成交数量唯一物理来源: paper_sim_fills.volume
内存/契约名:            qty  （Fill.Qty()；LotRecord.Quantity 装载后）
禁止:                   读取不存在的 fills.quantity 作为成交数量
```

| API | 含义 |
|-----|------|
| `positionstate.FillQtyPhysicalColumn` | `"volume"` |
| `PaperSimFill.Qty()` | 返回 `Volume`（不新增第二列） |

DB **无 rename**；Order / Position 物理列本阶段不动（属后续 M3）。

---

## 4. 修复要点

**之前（P0）：**

```sql
SELECT …, f.quantity AS quantity FROM paper_sim_fills …
-- SQLite: no such column: f.quantity → lots 空 → is_new/holding_days 失真
```

**之后：**

```sql
SELECT …, f.volume AS quantity FROM paper_sim_fills …
-- 映射到 LotRecord.Quantity（canonical qty）
```

`Calculate` 未改；仅输入 lots 装载变正确。

---

## 5. 测试

```text
go test ./backend/portfolio/positionstate/ -count=1 -v
→ ok

新增:
  TestFillQtyPhysicalColumn_IsVolume
  TestLoadLotsByAccount_UsesFillVolumeAsCanonicalQty
  TestLoadLots_WrongQuantityColumn_YieldsNoLots   # 证明 f.quantity 不可用
  TestPaperSimFill_QtyCanonicalAlias

原有 Case1–4 / BugFix / MorningUnlock: PASS
```

---

## 6. 风险与回滚

| 项 | 说明 |
|----|------|
| 观察层数值变化 | 生产路径 `is_new_position` / `holding_days` 可能「突然正确」——属装载修复，非策略变更 |
| 交易执行 | 无影响（未改 fillBuy / Unlock 写路径；Unlock 本已读 `f.volume`） |
| 回滚 | 还原 `service.go` SELECT 即可；**不建议**回滚到错列名 |

---

## 7. 边界确认

```text
[x] 未修改 Calculate
[x] 未修改 T+1 PositionUnlockJob 逻辑
[x] 未修改 TradePlan 执行 / Rescale 算法
[x] 未引入 OrderQuantityPolicy
[x] 未新增 Fill 第二物理数量列
```

---

## 8. Sign-off

Phase12-M0 完成：**Fill.`volume` 为唯一成交数量来源**，PositionState lots 装载已对齐；交易逻辑与状态机计算器保持冻结。
