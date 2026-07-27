# Phase7-A3.1 Runtime Wiring Implementation Report

> 最小接入；未 `git add` / 未 commit；等待 Preflight  
> HEAD 基线含 Handler：`97ef5c7`

---

## 1. 修改文件

| 文件 | 说明 |
|---|---|
| `main.go` | 仅在 `ChainAssetMiddleware` 增加 `PaperObservationAssetMiddleware` |

### 修改行范围

```text
main.go ≈ L192–L198（AssetServer.Middleware 列表）
  + L196: api.PaperObservationAssetMiddleware,
```

相对 HEAD 的 diff：**仅 1 行新增**（无 Recovery / BrokerReconcile / PaperTrading）。

---

## 2. Runtime 调用链

```text
Wails AssetServer
  → ChainAssetMiddleware(
        CandidatePool,
        RealOrders,
        TradePlans,
        PaperObservationAssetMiddleware,   // ★
        OpsTradingDay,
      )
       ├─ GET /api/papertrading/status|dashboard/* 
       │     → PaperObservationHandler（97ef5c7）
       │     → GetDashboard* / Overlay
       └─ 其它路径 → next（不捕获 /run）
```

---

## 3. 排除项

| 排除 | 状态 |
|---|---|
| `PaperTradingAssetMiddleware` | 未引入 |
| `/run` / Job | 未接入 |
| Broker / Settlement / Paper Engine | 未改 |
| `RecoveryReadinessAssetMiddleware` | 已从工作区 middleware 列表拆除 |
| `BrokerReconcileAssetMiddleware` | 已从工作区 middleware 列表拆除 |
| 交易逻辑 | 未改 |

---

## 4. 测试结果

```text
go test ./backend/api/... -count=1
→ ok   go-stock/backend/api   14.005s

go build ./backend/...
→ ok
```

---

## 5. 待 Preflight

建议白名单：`main.go`（确认 `git diff -- main.go` 仅 Observation 一行）+ 本报告。  
**勿** `git add -A`；**勿**带 Recovery/BrokerReconcile/`papertrading.go`。
