# PHASE10-C.4-A Runtime Validation Report

> **类型：** Runtime Validation（只读验收，未改业务代码，未 commit）  
> **日期：** 2026-08-07  
> **切片：** exclusive `fillMode`（A 默认 / B 观察采数）  
> **依据：** [PHASE10_C4_A_IMPLEMENTATION_REPORT.md](./PHASE10_C4_A_IMPLEMENTATION_REPORT.md)

---

## Verdict

**PASS** — Config 默认 A、临时 B、Scheduler 互斥注册、回归测试均符合 C.4-A 预期。未等待真实 15:10 成交。

---

## Preconditions

| 项 | 结果 |
|----|------|
| C.4-A 源码已实现、未 commit | 是 |
| 验证前 `build/bin/go-stock.exe` 不含 C.4-A 字符串 | 是（旧包） |
| 为验证重建 exe（`wails build -skipbindings -s`，**未改代码**） | 是 → `D:\stock\build\bin\go-stock.exe` @ 2026-08-07 12:18:59 |
| 新包含 `paper_trading_session_b` / `cron:session_b` | 是 |
| 配置路径（进程 cwd） | `build/bin/data/paper_trading_mvp.json` |

---

## 1. Config result

### 1.1 缺省 / A

| 检查 | 结果 |
|------|------|
| 配置内容 | `{"enablePaperTrading": true}`（无 `fillMode`） |
| 归一化 | `NormalizeFillMode("")` → **A**（单元测试 + 启动日志 `fillMode=A`） |
| 显式 `"A"` | 与缺省等价（`TestEffectiveFillMode_FromConfigCache` / `FillCronExclusive`） |

### 1.2 临时 B

| 检查 | 结果 |
|------|------|
| 临时写入 | `{"enablePaperTrading": true, "fillMode": "B"}` |
| 重启后日志 | `fillMode=B` |
| 验证后恢复 | `data/` 与 `build/bin/data/` 均已恢复为 `{"enablePaperTrading": true}` |
| 恢复后重启 | 再次注册 open@09:31 `fillMode=A`（见 Scheduler） |

**结论：** Config 层 PASS。

---

## 2. Scheduler result

日志来源：`D:\stock\build\bin\logs\info.log`

### 2.1 缺省 A（启动 @ 12:19:30）

```text
paper trading open cron registered key=paper_trading_open spec=0 31 9 * * 1-5 fillMode=A enablePaperTrading=true
paper trading settle cron registered key=paper_trading_settle spec=0 5 15 * * 1-5 enablePaperTrading=true
```

| 期望 | 实测 |
|------|------|
| 注册 09:31 Session A（`paper_trading_open`） | **是** |
| 不注册 15:10 Session B | **是**（本启动窗口无 `session_b`） |
| Settlement 15:05 保留 | **是** |

### 2.2 fillMode=B（启动 @ 12:20:24）

```text
paper trading session_b cron registered key=paper_trading_session_b spec=0 10 15 * * 1-5 fillMode=B enablePaperTrading=true
paper trading settle cron registered key=paper_trading_settle spec=0 5 15 * * 1-5 enablePaperTrading=true
```

| 期望 | 实测 |
|------|------|
| 不注册 09:31 Session A | **是**（本启动窗口无 `paper_trading_open`） |
| 注册 15:10 Session B | **是** `spec=0 10 15 * * 1-5` |
| Settlement 15:05 保留 | **是** |
| `actor=cron:session_b` | **契约确认**（见下；未触发真实 15:10 job） |

**Actor 契约（不依赖 15:10 实盘）：**

- `fill_cron.go`：`ActorCronSessionB = "cron:session_b"`
- `app_paper_trading.go`：`runPaperTradingSessionBJob` → `RunExecution(... Actor: papertrading.ActorCronSessionB)`
- 二进制含字符串 `cron:session_b`
- `gateway_test.go` 源码标记：`paper_trading_session_b` + `0 10 15 * * 1-5`

### 2.3 恢复缺省后（启动 @ 12:21:13）

再次仅见 open + settle（`fillMode=A`），与 2.1 一致。

**结论：** Scheduler 层 PASS（exclusive + settle 常驻）。

---

## 3. Regression result

```text
go test ./backend/papertrading/ -count=1
ok  	go-stock/backend/papertrading	(~10–11s)
```

聚焦用例（均 PASS）：

- `TestFillCronExclusive_A_RegistersOpenOnly`
- `TestFillCronExclusive_B_RegistersSessionBOnly`
- `TestEffectiveFillMode_FromConfigCache`

**结论：** Regression PASS。

---

## 4. Known limitations

1. **未等待真实 15:10 成交** — 本报告只验证 cron **注册**与 actor **契约**；CloseFill / Session B 成交路径依赖既有 C.2-C，需交易日 15:10 或手工触发另验。
2. **fillMode 不热更新** — 改 json 后必须重启 exe；验证中已按此操作。
3. **验证前旧 exe 不含 C.4-A** — 必须用含本切片的二进制；本次已 `wails build -skipbindings -s` 重建（未改代码）。
4. **Preflight 未 Ready 时整段不注册** — 本次启动正常注册，未覆盖 blocked 分支。
5. **同日 A→B 双 Fill 仍不可用** — exclusive 设计故意只挂一种 Fill cron。
6. **配置曾临时写 B** — 已恢复缺省；当前运行中的 exe 为恢复后 A 模式。

---

## 5. Scope / 禁止项核对

| 项 | 状态 |
|----|------|
| 修改业务代码 | **否** |
| commit | **否** |
| push | **否** |
| 临时改 `paper_trading_mvp.json`（验证用） | 是 → 已恢复 |
| 重建 exe（验证用） | 是 |

---

## 6. Sign-off

| 项 | 结果 |
|----|------|
| Config result | **PASS** |
| Scheduler result | **PASS** |
| Regression result | **PASS** |
| Overall | **PASS**（C.4-A Runtime Validation 可验收；真实 15:10 采数另开） |
