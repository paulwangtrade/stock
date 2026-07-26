# Phase7-A1 Implementation Report
# Market Data Facade Interface + Legacy Adapter

> 基线：Git tag **`v0.6.7-snapshot-freeze`**（HEAD `f1951b7`）
> 依据：`PHASE7_A_DESIGN_REVIEW.md` → 结论 **A. Ready for Phase7-A1 implementation**（按 §5 收紧范围）
> 性质：**只新增只读接口 + Legacy Adapter + 单测 + 文档**；零调用方迁移、零交易链改动、零 schema 改动
> 是否 commit：**否**（工作区保留，待人工确认）

---

## 1. 新增文件

全部为**新增**，无任何既有文件被修改。

| 文件 | 行数 | 职责 |
|---|---:|---|
| `backend/marketdata/interfaces.go` | 90 | `KlineService` / `QuoteService` 接口、`Bar` / `Quote` 最小模型、统一错误 |
| `backend/marketdata/kline_service.go` | 151 | 周期/复权规范化、`NormalizeBarsRequest`、`FormatEndTime`、`ParseBarTime` |
| `backend/marketdata/quote_service.go` | 48 | 代码规范化去重（`NormalizeCode(s)`）、`FindQuote` |
| `backend/marketdata/adapter/eastmoney_kline_adapter.go` | 86 | `EastMoneyKlineAdapter` → 委托 `data.EastMoneyKLineApi.GetKLineDataBefore` |
| `backend/marketdata/adapter/legacy_quote_adapter.go` | 90 | `LegacyQuoteAdapter` → 委托 `data.StockDataApi.GetStockCodeRealTimeData` |
| `backend/marketdata/marketdata_test.go` | 263 | 规范化、委托、映射、错误处理、领域边界守护测试 |

目录结构与任务书一致（`backend/` 下按能力分包，沿用项目既有 `backend/data`、`backend/cache` 风格）：

```text
backend/marketdata/
    interfaces.go
    kline_service.go
    quote_service.go
    marketdata_test.go
    adapter/
        eastmoney_kline_adapter.go
        legacy_quote_adapter.go
```

### 接口签名（最终实现）

```go
type KlineService interface {
    GetBars(code string, period string, adjust string, limit int, endTime time.Time) ([]Bar, error)
}

type QuoteService interface {
    GetQuote(code string) (*Quote, error)
    GetQuotes(codes []string) ([]Quote, error)
}
```

- `endTime` 零值 = 取最新（映射为旧实现约定的 `20500101`）；分钟线格式化为 `YYYYMMDDHHmmss`，其余为 `YYYYMMDD`。
- 统一错误：`ErrEmptyCode` / `ErrEmptyCodes` / `ErrInvalidLimit` / `ErrUnsupportedPeriod` / `ErrUnsupportedAdjust` / `ErrNoData` / `ErrProviderUnavailable`，调用方用 `errors.Is` 判定，替代当前各处「返回空切片、无法区分空数据与失败」的写法。

### 数据模型（领域隔离）

`Bar`：`Code / Period / Adjust / Time / TimeText / Open / High / Low / Close / Volume / Amount / Amplitude / ChangePercent / ChangeValue / TurnoverRate`

`Quote`：`Code / Name / Price / Open / PreClose / High / Low / Volume / Amount / ChangePercent / ChangeValue / Bid / Ask / Market / Date / Time / FetchedAt`

**明确排除**（旧 `data.StockInfo` 中存在但不进入本层）：`CostPrice`、`CostVolume`、`Profit`、`ProfitAmount`、`ProfitAmountToday`、`FollowPrice`、`FollowProfit`、`AlarmPrice`、`AlarmChangePercent`、`Sort`、`Groups` 以及任何 TradePlan / Risk / Position / Order 字段。此约束由 `TestModelsStayMarketDataOnly` 用反射守护，后续 Phase 若渗透交易域字段会直接测试失败。

另一处收益：旧 `KLineData` 全字段为 `string`，`Bar` 统一为 `float64` + 解析后的 `time.Time`，消除下游各自 `strconv` 的重复解析。

---

## 2. 架构变化

### Before（v0.6.7 现状，仍然如此运行）

```text
Consumer（App / Agent tools / prepareStockBars / cron / Paper Quote / scripts）
   |
   +--> data.EastMoneyKLineApi.GetKLineData(Before)   （多入口直连）
   |
   +--> data.StockDataApi.GetStockCodeRealTimeData    （多入口直连）
```

### After（Phase7-A1 新增旁路，尚无生产调用方）

```text
Consumer（未迁移，仍走 Before 路径）
   |
   |   ......（A2 起逐步切换到下方）
   v
marketdata.KlineService / marketdata.QuoteService   ← 只读接口（本次新增）
   |
   v
marketdata/adapter.EastMoneyKlineAdapter            ← Legacy Adapter（本次新增）
marketdata/adapter.LegacyQuoteAdapter
   |
   v
data.EastMoneyKLineApi / data.StockDataApi          ← 旧实现，未改一行
   |
   v
EastMoney / Tencent / Sina Provider + kline_cache
```

要点：本次是**加法**，不是替换。删掉 `backend/marketdata/` 整个目录即可完全回滚，系统行为与 `v0.6.7-snapshot-freeze` 完全一致。

---

## 3. 调用关系

| 调用方 | 目标 | 状态 |
|---|---|---|
| `EastMoneyKlineAdapter.GetBars` | `EastMoneyKLineApi.GetKLineDataBefore(code, period, adjust, limit, end)` | 纯委托，参数透传，不新增缓存 |
| `LegacyQuoteAdapter.GetQuotes` | `StockDataApi.GetStockCodeRealTimeData(codes...)` | 纯委托，仅做去空去重 |
| `LegacyQuoteAdapter.GetQuote` | 复用 `GetQuotes` + `FindQuote` | 不额外发请求 |
| **生产代码 → marketdata** | — | **零**（本包目前只被自身测试引用） |

依赖方向：`marketdata`（零业务依赖，仅 `errors` / `strings` / `time`）← `marketdata/adapter` → `backend/data`。`backend/data` 不感知 `marketdata`，无循环依赖，无 import 方向倒置。

Adapter 依赖的是**最小方法集接口**（`eastMoneyKlineFetcher` / `legacyRealtimeFetcher`），因此单测可注入 fake、零网络；未来替换 provider 也不必改接口层。

---

## 4. 未迁移范围（Phase7-A1 明确不做）

以下调用方**全部保持原样**，A1 未触碰：

- `app.go` / `app_*.go` 中的 Wails K 线与行情导出（`GetStockEastMoneyKLine*` 等）
- `prepareStockBars`、信号扫描取数路径、`signal_scan_bundle.js` 及任何信号公式
- Agent tools：`tool_kline.go`、`tool_eastmoney_kline.go`、`tool_market_data.go` 等
- Paper Trading / Execution / Position 的行情与价格获取
- `LookupFollowBaselinePrice`、`GetCommonKLineData`、港股 K 线、`scripts/scansignals`
- `backend/cache/stock_price_cache.go`（`FollowRealtimePriceCache`）与前端 `dailyBarsCache` / `indexKlineCache`
- 指标（MA / RSI / MACD / ATR / 指数 MA20）Go 与 JS 双路径，`IndicatorService` / `SignalService` **未定义、未实现**

同样未做：超集切片、共享 Quote TTL 策略、删除任何旧 API（`GetKLineData` / `GetKLineData2` 均保留）、DB schema 变更。

### 明确未改文件（按 Design Review §5.1-4 要求列出）

`backend/strategy/**`、`backend/tradeplan/**`（及 trade plan 相关 service）、`backend/execution/**`、paper trading 相关文件、risk / position 相关文件、`backend/data/eastmoney_kline_api.go`、`backend/data/stock_data_api.go`、`backend/cache/**`、所有 `frontend/**`、所有 migration / schema 文件。

> **关于 `git diff` 输出的说明**：当前工作区在本任务开始前即为 dirty 状态（`git status --porcelain` 共 591 条，`git diff --stat` 涉及 75 个已跟踪文件、约 3902 insertions / 3572 deletions），全部来自 Phase6.7 之后尚未归档的历史工作，**与 Phase7-A1 无关**。Phase7-A1 对已跟踪文件的改动量为 **0**，在 `git status` 中仅体现为两条未跟踪条目：`?? backend/marketdata/` 与 `?? PHASE7_A1_IMPLEMENTATION_REPORT.md`。提交时须按路径显式 `git add backend/marketdata PHASE7_A1_IMPLEMENTATION_REPORT.md`，**禁止 `git add -A`**。

---

## 5. 交易链影响评估

| 检查项 | 结论 |
|---|---|
| Frozen Spec 是否仍是唯一交易价量来源 | **是**。marketdata 无写路径、无价格决策，未被任何交易代码引用 |
| TradePlan / Risk / Position / Paper / Execution 是否改动 | **否**，零文件变更 |
| 现有 K 线 / 行情行为是否变化 | **否**。旧 API 未改，缓存策略未改，无 feature flag 生效 |
| DB schema / 新表 | **否** |
| 用户可见行为 | **无变化**（新包无调用方） |
| 回滚成本 | 删除 `backend/marketdata/` 目录，无残留引用 |

风险等级：**Low**（与 Design Review 对 A1「仅接口 + Adapter」的评级一致）。

---

## 6. 验证结果

| 命令 | 结果 |
|---|---|
| `go vet ./backend/marketdata/...` | PASS（无输出） |
| `go test ./backend/marketdata/...` | **ok** `go-stock/backend/marketdata` 10.043s；`adapter` 无测试文件（测试集中在外部测试包 `marketdata_test`） |
| `go build ./backend/... .` | PASS |
| `go build ./...` | **FAIL — 与本次改动无关**，见下 |
| `go test ./backend/data -run 'TestSignalScanTaskRegistry\|TestKLine\|TestNormalize' -count=1` | **ok** 27.668s |
| `go test ./backend/data`（全包） | **FAIL — 与本次改动无关**，见下 |
| `go vet ./backend/data` | 1 条既有告警（`crawler_api.go:26` context leak），非本次引入 |

### `go build ./...` 失败原因（预先存在的 dirty workspace）

```text
# go-stock/tmp_diag_upcoming
tmp_diag_upcoming\main2.go:6:6: main redeclared in this block
# go-stock/tmp_manual_generate_runtime_verify
tmp_manual_generate_runtime_verify\main.go:24:6: main redeclared in this block
# go-stock/tmp_phase67e_activate
tmp_phase67e_activate\main2.go:21:6: main redeclared in this block
```

三个目录均为 `git status` 中的未跟踪临时诊断脚本（早于本任务存在，`?? tmp_diag_upcoming/` 等），每个目录含多个 `func main`。按任务要求「不要修改无关文件」，**未清理、未修改**。排除后 `go build ./backend/... .` 通过。

### `go test ./backend/data`（全包）失败原因（预先存在，非本次引入）

全包跑到 629s 触发 `-timeout` panic，栈顶阻塞在 chromedp 浏览器进程等待：

```text
github.com/chromedp/chromedp.(*ExecAllocator).Allocate.func2()
os.(*Process).Wait(...)
FAIL	go-stock/backend/data	629.248s
```

来源是该包内的浏览器 / 网络集成测试（`web_search_api_integration_test.go`、`eastmoney_kline_api_integration_test.go` 等，`init()` 里 `db.Init` 并真实拉取外部行情）。本次**未修改 `backend/data` 任何文件**，因此该失败与 Phase7-A1 无因果关系；定向单元测试（含 Phase6.7-G 的 `TestSignalScanTaskRegistry`）全部通过。

### 测试覆盖点

1. **Legacy Adapter 可以调用旧实现** — fake 记录 `GetKLineDataBefore` / `GetStockCodeRealTimeData` 的实际入参，断言调用次数与透传值（`sh600000` / `101` / `qfq` / `limit` / `20500101`）。
2. **Interface 返回结构正确** — `Bar` OHLC / 量额 / 换手率 / `Time` 与 `TimeText`；`Quote` 价格、涨跌、买卖一、`FetchedAt`；`GetQuote("600000")` 能命中 `sh600000`。
3. **错误处理存在** — `ErrEmptyCode` / `ErrEmptyCodes` / `ErrInvalidLimit` / `ErrUnsupportedPeriod` / `ErrUnsupportedAdjust` / `ErrNoData` / `ErrProviderUnavailable`；上游 error 透传；非法入参时**不调用**旧实现（断言 `calls == 0`）。
4. **不影响现有交易模块** — `TestModelsStayMarketDataOnly` 反射扫描 `Bar` / `Quote` 字段名，出现 cost/profit/position/order/plan/risk/alarm/follow/qty/stoploss/account 类字段即失败。
5. 全部测试使用 fake，**零网络、零 DB**，可在 CI 稳定运行。

---

## 7. 验收对照

| 门禁 | 结果 |
|---|---|
| 新增 Market Data 接口 | PASS（`KlineService` / `QuoteService` + `Bar` / `Quote`） |
| Legacy Adapter 可工作 | PASS（委托 + 映射 + 单测覆盖） |
| 无调用方迁移 | PASS（生产代码零引用） |
| 无交易链修改 | PASS（零既有文件改动） |
| 无数据库修改 | PASS |
| `go test` 通过 | PASS（`backend/marketdata/...` 全绿；`backend/data` 定向单测全绿，全包失败为既有集成测试环境依赖） |
| 小改动 / 可回滚 | PASS（纯新增 6 个文件，删除目录即回滚） |

---

## 8. 下一步建议（Phase7-A2 前置）

建议**批准进入 Phase7-A2，但拆成独立小 PR，且第一个 PR 不含缓存策略变更**：

1. **A2-0 Call-site Inventory（必做前置）**：以 `PHASE6_7_H_MARKET_DATA_LAYER_AUDIT.md` 为底稿重扫一遍 K 线 / 行情调用点（含 Agent tools、`LookupFollowBaselinePrice`、`scripts/scansignals`、前端两层缓存），产出可勾选清单。缺这一步直接迁移会漏掉隐藏调用链。
2. **A2-1 非交易路径试点**：先迁 1～2 个只读、非交易调用方（建议 Agent `tool_kline` 或指数 MA20 取数），加 golden 对比「迁移前后 Bar 序列一致」。Design Review 已否决在 A1 内做试点，试点应作为 A2 第一个 PR。
3. **A2-2 Quote 迁移顺序**：按 Design Review §3.2 的安全顺序，**自选/看板 → AI 推荐 → cron**，Paper Trading / Open Position / Execution 取价**放到最后**且需 Frozen Spec 回归测试全绿。
4. **A2-3 缓存与超集切片**：`KlineService` 内做超集缓存 + 切片（解决不同 `limit` 导致的 cache miss）、Quote 共享 TTL；此时才允许触碰 `kline_cache` 与 `FollowRealtimePriceCache`，需带命中率指标。
5. **Indicator / Signal 收敛**：继续推迟。前置条件是先建立指标 golden 回归集（Go vs JS 数值对齐），**A2 内禁止改公式**。
6. **本次产物提交建议**：`Phase7-A1: add marketdata read-only interfaces and legacy adapters`，仅含 `backend/marketdata/**` + 本报告；提交前用 `git diff --cached --stat` 确认无 `tmp_*` 与前端/交易文件混入。

**结论：建议进入 Phase7-A2，但以「A2-0 调用点清点 + A2-1 非交易试点」开始，不要一次性迁移。**

---

## 9. 元数据

| 项 | 值 |
|---|---|
| 基线 tag | `v0.6.7-snapshot-freeze`（HEAD `f1951b7`） |
| 新增文件 | 6（`backend/marketdata/**`）+ 本报告 |
| 修改既有文件 | **0** |
| 修改 schema | **否** |
| 是否 commit | **否** |
