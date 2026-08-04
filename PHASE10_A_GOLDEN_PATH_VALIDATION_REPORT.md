# PHASE10_A Golden Path Validation Report

> **性质：** 运行时验证（未改业务代码；未执行 Approve / Freeze / PaperTrading）  
> **日期：** 2026-08-05 00:19 +08  
> **对象：** `trade_plan_id=12`  
> **DB：** `D:\stock\build\bin\data\stock.db`  
> **调用：** `strategy.RunMorningIntentMaterialize`（与 `POST /api/tradeplans/materialize-morning` 同一编排；OpenPriceFn=`RealtimeOpenPriceProvider`）  

---

## 0. Verdict

| 项 | 结果 |
|----|------|
| Phase10-A 物化主链（selected → limit → volume → readiness recheck） | **通过** |
| `materialized_items` | **3** |
| 幂等（limit / volume 不被破坏性覆盖） | **通过** |
| `readiness_ready` | **false**（预期：2 票无 Intent 锚点，属 DATA-003） |
| Approve / Freeze / Paper | **未执行**（按范围） |

---

## 1. 调用前确认

### 1.1 Plan meta

| 字段 | 值 |
|------|-----|
| id | 12 |
| trade_date | 2026-08-03 |
| status | **draft** |
| pricing_stage | **after_close_intent** |
| freeze_at / approved_at | null |
| plan_version | 1 |

### 1.2 Items（调用前）

| stock_code | intent_status | ref_price | ref_source | limit | volume |
|------------|---------------|-----------|------------|-------|--------|
| sh600487 | **selected** | 52.43 | strategy_snapshot | 0 | 0 |
| sz300394 | （空） | 0 | | 0 | 0 |
| sh688525 | （空） | 0 | | 0 | 0 |
| sz001309 | **selected** | 399.89 | strategy_snapshot | 0 | 0 |
| sz301308 | **selected** | 532.51 | strategy_snapshot | 0 | 0 |

→ 3 票具备 Phase10-A 物化前置；2 票无 selected（不在/未锚点），将 legacy_skip。

---

## 2. Materialize 调用 #1

### 2.1 Open quotes（Realtime）

| code | open |
|------|------|
| sh600487 | 48.50 |
| sz001309 | 352.03 |
| sz301308 | 323.11 |

`pending_open=0`，`gap_skip=0`，`priced=3`，`legacy_skip=2`。

### 2.2 API 等价结果信封

| 字段 | 值 |
|------|-----|
| success | **true** |
| plan_id | 12 |
| materialized_items | **3** |
| readiness_ready | **false** |
| pricing_stage | morning_materialized |
| message | morning intent materialize completed |

### 2.3 blockers（调用后）

均指向未物化空 Intent 票 `sz300394`, `sh688525`：

- `INTENT_NOT_MATERIALIZED`
- `LIMIT_PRICE_MISSING`
- `TARGET_VOLUME_BELOW_LOT`
- `QG-E1` / `ENTRY_PRICE_MISSING`

**解读：** 已物化的 3 票不再挡主路径；Ready 被无锚点行拖住 → 与 DATA-003 / Anchor Layer 设计一致，**不视为 Phase10-A 接线失败**。

---

## 3. 物化结果（items）

| stock_code | intent | limit_price | open_ref | target_volume |
|------------|--------|-------------|----------|---------------|
| sh600487 | **priced** | **54.0029** (=52.43×1.03) | 48.50 | **1800** |
| sz001309 | **priced** | **411.8867** | 352.03 | **200** |
| sz301308 | **priced** | **548.4853** | 323.11 | **100** |
| sz300394 | （空） | 0 | 0 | 0 |
| sh688525 | （空） | 0 | 0 | 0 |

Plan：`pricing_stage` → **morning_materialized**；仍为 **draft**；未 approve / freeze。

---

## 4. 幂等（调用 #2）

| 观测 | 结果 |
|------|------|
| LimitPrices | pricedCount=0，legacySkip=5（已 priced 不再当 selected 重写） |
| TargetVolumes | sized=0，**idempotentSkip=3** |
| materialized_items | 仍为 **3** |
| limit_price | **未变**（54.0029 / 411.8867 / 548.4853） |
| target_volume | **未变**（1800 / 200 / 100） |
| intent | 仍为 priced |

→ **幂等通过**：不覆盖有效 limit，不改已 sizing 的 volume。

---

## 5. 范围遵守

| 动作 | 状态 |
|------|------|
| 修改业务代码 | 否 |
| Approve | 否 |
| Freeze | 否 |
| PaperTradingJob | 否 |

临时验证入口：`tmp_phase10a_golden`（验证后可删；非产品代码）。

---

## 6. Phase10-A 闭环结论

```text
Draft after_close_intent (#12)
  → selected + ref  (已有)
  → Morning Materialize (Realtime open)
  → limit_price + target_volume + priced ×3
  → Readiness recheck (Ready=false，仅因 2 票无锚点)
  → Approve/Freeze 未跑（范围外）
```

| 能力 | 判定 |
|------|------|
| A.1 编排 API 语义 | **PASS** |
| 禁假价 + 真实 open | **PASS** |
| 幂等 | **PASS** |
| 全计划 Ready | **FAIL（预期）** — 需 DATA-003 / Phase10-B 或剔除无 Intent 行 |
| 最新日 #19 Golden | **不适用** — 无 selected（见 Precheck） |

---

## 7. 建议后续

1. UI 对 #12：`trade_date=2026-08-03` 刷新，确认 stage / Readiness 与本报告一致。  
2. 完整 Ready：处理无锚点票（跳过策略或 Phase10-B Anchor）。  
3. 勿将本计划 Freeze 作为 A 验收必需项。  
4. 登记：Golden Path **物化段 PASS**；Ready 段受 DATA-003 阻挡。
