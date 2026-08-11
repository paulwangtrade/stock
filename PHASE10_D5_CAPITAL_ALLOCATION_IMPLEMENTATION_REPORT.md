# PHASE10-D.5 Capital Allocation Foundation Implementation Report

> **日期：** 2026-08-11  
> **前置：** [D.2 Snapshot](./PHASE10_D2_PORTFOLIO_SNAPSHOT_DESIGN.md) · [D.4 架构](./PHASE10_D4_CAPITAL_ALLOCATION_ARCHITECTURE_DESIGN.md)  
> **约束：** 纯计算；不改 FixedAmount / TradePlan / Gateway / PaperTrading / Observation；不实现新 Sizer；不卖出预支。

---

## Verdict

已新增独立包 `backend/allocation`：由 `PortfolioSnapshot` + `AllocationPolicy` 计算本轮 **Available Capital**。  
**未**接入 Draft / Sizer / Execution。模拟盘买入仍为 **FixedAmount ≈ 100000/票**。

```text
PortfolioSnapshot ──注入──► allocation.Budget (纯函数，无 DB)
                              │
                              ▼
                    CapitalAllocationResult
                              │
                    （本阶段无消费方）
```

---

## 1. 修改文件

| 文件 | 说明 |
|------|------|
| `backend/allocation/types.go` | Request / Policy / Result / `Allocator` 接口 |
| `backend/allocation/budget.go` | 纯函数 `Budget` |
| `backend/allocation/budget_test.go` | 现金充足/不足、敞口、reserved、不改输入、无户、负值 |
| `PHASE10_D5_CAPITAL_ALLOCATION_IMPLEMENTATION_REPORT.md` | 本报告 |

**未改：** `positionsizing`（含 FixedAmountSizer）、`strategy`、`papertrading`、`execution`、前端、Observation API、数据库 schema。

---

## 2. 设计说明

### 2.1 输入

```text
CapitalAllocationRequest {
  Snapshot   *portfolio.Snapshot   // 调用方注入；本包不读库
  Policy     AllocationPolicy      // MaxGrossExposurePct；缺省 0.85
  PendingBuy float64               // 在途买单名义；默认 0。不计 planned sell
}
```

`MaxSingleNamePct` / `MaxNames` 留在 Policy 上供未来 Equal/Score/Risk，**v1 总额公式不使用**。

### 2.2 输出

```text
CapitalAllocationResult {
  available_capital
  cash_available          // cash − reserved_cash − pending_buy（可暂为负，不回写 Snapshot）
  exposure_limit          // gross_cap = pct × equity
  gross_headroom
  reason                  // OK | NO_ACCOUNT | BLOCKED | CASH | GROSS | NEGATIVE_INPUT
  binding
}
```

### 2.3 公式

```text
available_capital = max(0, min(
  cash - reserved_cash - pending_buy,
  gross_cap - current_exposure
))
```

`gross_cap = MaxGrossExposurePct × total_equity`（pct≤0 用默认 0.85，属政策缺省，不是修复账户数据）。

### 2.4 异常（不自动修复源数据）

| 条件 | reason | 源 Snapshot |
|------|--------|-------------|
| nil / `found=false` | NO_ACCOUNT | 不 INSERT、不改 cash |
| `BlockNewEntries` | BLOCKED | 不改 |
| cash/reserved/pending/exposure/equity < 0 | NEGATIVE_INPUT | 负值原样保留 |
| cash 头寸 ≤ 0 | CASH | 不改 |
| 敞口头寸 ≤ 0（且现金仍正） | GROSS | 不改 |

输出预算地板为 0，避免负的可买额；**不**把负现金改成 0 写回 Snapshot。

### 2.5 未来扩展

`Allocator` 接口 = `Budget(Request) Result`。未来 Equal/Score/Risk **消费** Result，在 `positionsizing` 实现，不在本包实现权重。

---

## 3. 测试结果

```text
go test ./backend/allocation/
ok   go-stock/backend/allocation
```

| 用例 | 结果 |
|------|------|
| 现金充足（800k 现金 / 200k 敞口 / 85% 帽 → 650k） | PASS |
| 现金不足（cash=0 → available=0, CASH） | PASS |
| 敞口限制（exposure 900k > 85%×equity → GROSS） | PASS |
| reserved_cash + pending_buy 扣减 | PASS |
| Budget 后 Snapshot/Request 字段不变 | PASS |
| 无账户 / Found=false | PASS |
| 负现金 NEGATIVE_INPUT 且不改写 | PASS |

---

## 4. 是否影响交易链路

| 路径 | 影响 |
|------|------|
| `resolvePlanAmountViaSizer` / FixedAmountSizer | **无**（未改） |
| TradePlan `TargetAmount` | **无** |
| Execution Gateway / Fill / Settlement | **无** |
| Observation | **无** |
| 模拟盘 100000/票 | **保持** |

本包当前 **零生产调用点**。

---

## 5. 约束复核

- 无 DB、无卖出预支、无新 Sizer  
- 未 `git add .`；提交仅本切片文件（见 commit）
