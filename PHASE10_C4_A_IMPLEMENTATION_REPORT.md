# PHASE10-C.4-A Implementation Report

> **类型：** 实现报告（未 commit，待验收）  
> **日期：** 2026-08-07  
> **依据：** [PHASE10_C4_SESSION_B_OBSERVATION_TRIGGER_DESIGN.md](./PHASE10_C4_SESSION_B_OBSERVATION_TRIGGER_DESIGN.md)、[PHASE10_C4_IMPLEMENTATION_PREFLIGHT.md](./PHASE10_C4_IMPLEMENTATION_PREFLIGHT.md)  
> **切片：** exclusive `fillMode` + Session B @15:10 → `RunExecution`（观察采数）

---

## 1. 修改文件

| 文件 | 变更 |
|------|------|
| `backend/papertrading/config.go` | `Config.FillMode`；读/写/测试注入时 `NormalizeFillMode`；`EffectiveFillMode()` |
| `backend/papertrading/fill_cron.go` | **新增** `FillModeA/B`、`FillCronExclusive`、`ActorCronSessionB` |
| `backend/papertrading/fill_cron_test.go` | **新增** exclusive / 默认 A 测试 |
| `app_paper_trading.go` | 按 FillMode 注册 open **或** session_b；`runPaperTradingSessionBJob` → `RunExecution` |
| `backend/papertrading/gateway_test.go` | 源码标记：session_b key / `0 10 15 * * 1-5` / Gateway |

**未改：** Gateway / Broker / FillProvider / CloseFill / job 幂等 / DB schema / Metrics / UI / `app_windows|linux|darwin.go`（仍只调既有 `InitPaperTradingJobs`）。

---

## 2. 配置变化

`data/paper_trading_mvp.json`（相对进程 cwd，通常 `build/bin/data/`）：

```json
{
  "enablePaperTrading": true,
  "fillMode": "A"
}
```

| `fillMode` | 含义 |
|------------|------|
| 缺省 / `A` / 非法值 | **默认 A**：注册 `paper_trading_open` @ `0 31 9 * * 1-5` |
| `B` | 注册 `paper_trading_session_b` @ `0 10 15 * * 1-5`；**不**注册 open |

Settle 仍始终注册：`0 5 15 * * 1-5`（非 Fill）。

**采数操作：** 将运行目录配置改为 `"fillMode":"B"` 后**重启** exe（cron 在 `InitPaperTradingJobs` 启动时注册）。采完改回 `"A"` 并重启。

---

## 3. Scheduler 行为

```text
InitPaperTradingJobs
  fillMode = EffectiveFillMode()   // default A
  (open, sessionB) = FillCronExclusive(fillMode)

  if open:      AddFunc(09:31) → runPaperTradingOpenJob
                  → RunExecution(actor=cron)
  if sessionB:  AddFunc(15:10) → runPaperTradingSessionBJob
                  → RunExecution(actor=cron:session_b)
  always:       AddFunc(15:05) → SettlementJob
```

**Fill 链路（B，时钟∈Session B 时）：**

```text
Scheduler → RunExecution → Session B → SelectFillProvider → CloseFillProvider → paper_sim_*
```

Price 仍注入 `DefaultOpenPriceProvider`；B 窗由 Gateway **覆盖**为 Close（既有行为）。

---

## 4. 风险说明

| 风险 | 说明 |
|------|------|
| 忘记切回 `fillMode=A` | 长期无 09:31 开盘模拟；需运维 checklist |
| Close 未就绪 @15:10 | fail-closed reject；不会回退 Open |
| 同日 A+B | **未实现**；exclusive 避免双满仓 / order UNIQUE 冲突 |
| 启动后改 json | 不热更新 cron；需重启 |
| Preflight 未 Ready | 整段 PaperTrading cron 不注册（既有） |

---

## 5. 测试结果

```
go test ./backend/papertrading/ -count=1
ok

覆盖：
- NormalizeFillMode / FillCronExclusive（A only / B only / default A）
- EffectiveFillMode via SetConfigForTest
- Phase10C2A wiring markers + C.4-A session_b spec 源码断言
```

未要求真实盘中成交样本（符合任务范围）。

---

## 6. 验收对照（实现口径）

| 项 | 状态 |
|----|------|
| 默认 A：仍注册 09:31 open | ✅ 代码路径 `FillCronExclusive("")/(A)` → open |
| `fillMode=B`：注册 15:10 session_b | ✅ spec `0 10 15 * * 1-5` |
| 经 `RunExecution` | ✅ |
| 未改 Gateway/Broker/Provider/DB/UI | ✅ |
| 未 commit | ✅ 待验收 |

---

**等待验收；未执行 git commit / push。**
