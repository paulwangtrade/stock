# Phase7-A2-1 Commit A Preflight

> 性质：**只读审计** — 未改代码、未 git add / commit / reset / clean  
> 基线 HEAD：`940f273644371783a8580c1fbce51482e1f694b9`（Phase7-A1）  
> 审计对象：未提交的 Agent 东财 plain K 迁移（Commit A）

---

## 1. 文件范围

| 文件 | 状态 | 属于 A2-1？ |
|---|---|---|
| `backend/data/tool_eastmoney_kline.go` | modified | **是** |
| `backend/data/kline_service_access.go` | untracked (new) | **是** |
| `backend/data/tool_eastmoney_kline_test.go` | untracked (new) | **是** |
| `backend/agent/tools/marketdata_wire.go` | untracked (new) | **是** |
| `backend/agent/tools/data_tools_wrapper.go` | modified | **是**（仅 `GetEastMoneyKLine` 闭包） |
| `PHASE7_A2_1_COMMIT_A_REPORT.md` | untracked (new) | **是**（实现报告） |

跟踪文件 diff 统计：`tool_eastmoney_kline.go` + `data_tools_wrapper.go` → **2 files, +54 / −25**。

工作区另有预先存在的 dirty（如 `signal_scan_api.go` / `paper_*` / `execution/*` / `strategy/*`），**不属于**本 Commit A 清单；选择性提交时不得纳入。

---

## 2. 依赖边界检查

### 2.1 实际 DAG（`go list`）

```text
marketdata          → 仅 std（errors/strings/time）     ← 纯接口层
marketdata/adapter  → data + marketdata                 ← Legacy 桥（A1 既定）
data                → marketdata（接口），不 import adapter / agent/tools
agent/tools         → data + marketdata + adapter       ← 注入工厂 / wrapper
```

| 检查项（用户表述） | 实测 | 判定 |
|---|---|---|
| data 不能 import agent/tools | **无**该 import | **Pass** |
| marketdata 包保持纯接口层 | root 包仅 std lib | **Pass** |
| 不存在循环依赖 | data↛adapter；adapter→data 单向 | **Pass**（无 cycle） |
| 「adapter 不能 import data」 | adapter **确实** import data | **澄清**：A1 设计即为 Legacy Adapter 委托 `EastMoneyKLineApi`；**允许且必要**。禁止的是反向 `data → adapter`（会形成环）。本实现通过 `SetKlineServiceFactory` 由 `agent/tools` 注入，避免 data 直引 adapter。 |

### 2.2 结论

依赖边界 **合格**；无循环依赖。用户清单中「adapter 不能 import data」应按 **「data 不能 import adapter」** 理解。

---

## 3. 行情语义检查（GetEastMoneyKLine）

### 3.1 参数映射

| 语义 | 迁移前 | 迁移后 | 一致？ |
|---|---|---|---|
| code | `stockCode` 原样 | `GetBars(code, …)` → Adapter 透传 `GetKLineDataBefore` | **是** |
| period | `normalizeKLineType` → `GetKLineData` / `GetAdjustedKLine` | 同函数 → `GetBars(..., kType, …)` | **是** |
| adjust | 日K+非空：非 qfq/hfq 则强制 qfq；否则 trim | `resolveEastMoneyKLineAdjust` 同规则 | **是** |
| start | 工具未支持历史 start | 仍不支持 | **是**（N/A） |
| end | 隐式最新（`GetKLineData`→`20500101`） | `endTime=zero` → Adapter `LatestEndFlag` | **是** |
| limit | 工具默认 60 | 不变 | **是** |
| 上游 | `GetKLineDataBefore` | Adapter 仍调 `GetKLineDataBefore` | **是**（源/缓存未改） |

### 3.2 OHLC / volume / amount

Adapter `klineToBar` 从同一 `KLineData` 解析 float；数值来源与旧链相同。

| 字段 | 一致性 | 备注 |
|---|---|---|
| time | **是** | markdown 用 `Bar.TimeText` ← `KLineData.Day` |
| open/high/low/close | **是**（同源） | 展示改为 `convertor.ToString(float)`，字符串格式可能与旧原始字符串略有差异（如尾随 0），**数值**一致 |
| volume / amount | **是**（同源） | 万手换算仍 `Volume/10000/100` |

**残留（非阻塞 Commit A）：** markdown 字符串化格式差异 → 留给 Commit B Golden 用数值比较锁定。

---

## 4. 禁止迁移检查

| 目标 | A2-1 diff 是否改动 | 结论 |
|---|---|---|
| `GetEastMoneyKLineWithMA` | **否**（仍 `api.GetKLineWithMA`） | Pass |
| `prepareStockBars` / Signal Snapshot | **否**（不在 A2-1 文件集） | Pass |
| Indicator 公式 / computeSMA | **否** | Pass |
| Paper Trading | **否**（工作区其它 dirty 与 A2-1 无关） | Pass |
| MorningOpenPrice | **否** | Pass |
| Execution | **否** | Pass |

---

## 5. 测试（本轮复跑）

| 命令 | 结果 |
|---|---|
| `go test ./backend/marketdata/...` | **PASS** |
| `go test ./backend/agent/tools/...` | **PASS** |
| `go test ./backend/data -run TestEastMoneyKLine` | **PASS** |

---

## 6. Diff 边界 / 是否可以 commit

### Diff 边界

- **允许纳入：** §1 六项（5 代码/测试 + Commit A 报告）；可选纳入本 Preflight 文档。  
- **禁止纳入：** 任何其它 dirty（trade/paper/execution/strategy/signal/frontend/tmp_* 等）。  
- **方式：** 路径级 `git add`；**禁止** `git add -A`。

### 是否可以 commit

**可以。** 满足：范围正确、依赖无环、plain K 语义同源、禁止域未迁、指定测试 PASS。

### 推荐 commit message

```text
Phase7-A2-1: migrate agent EastMoney K-line tool to KlineService
```

### 建议 add 清单

```powershell
git add -- `
  backend/data/tool_eastmoney_kline.go `
  backend/data/kline_service_access.go `
  backend/data/tool_eastmoney_kline_test.go `
  backend/agent/tools/marketdata_wire.go `
  backend/agent/tools/data_tools_wrapper.go `
  PHASE7_A2_1_COMMIT_A_REPORT.md `
  PHASE7_A2_1_COMMIT_A_PREFLIGHT.md
git diff --cached --name-only   # 人工确认仅上述路径
```

---

## 7. 元数据

| 项 | 值 |
|---|---|
| 是否修改代码 | **否**（本 preflight） |
| 是否执行提交 | **否** |
| 下一步 | 用户确认后做选择性 Commit A；再开 Commit B Golden |
