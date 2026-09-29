# 截面动量 V1（ext_xsmom_v1）实现报告

观察用选股侧刀：只走现有 SignalScan → snapshot。不进入 Trade Candidate / Broker / 下单。

## 结论

| 项 | 结果 |
|---|---|
| `strategy_id=ext_xsmom_v1` 的 JS 扫描产出 `XS_MOM_TOP` | **PASS**（goja 单测，重建后的 bundle） |
| `default` / 空 strategy 仍走冰点，且 params 里的 `scanMode=xsmom` 不能改写它 | **PASS** |
| 快照删除键：非 default 只删自己的 `strategy_id`（既有逻辑，未改） | **PASS**（代码审查；动量 id 常量 ≠ `default`） |
| bundle 由 `scripts/scansignals/scan-batch.ts` 重建，不是手改 | **PASS** |
| Research `min_score=60` 能看见该标签 | **PASS**（`CalcResearchSignalScore` 基分 76） |
| 实盘/全市场东财 K 线跑一遍 | **NEED REVIEW**（本环境未接行情） |
| Research 列表按 `strategy_id` 过滤 | **NEED REVIEW**（已知限制，本刀未改语义） |

## 行为

对扫描时刻有足够日线的股票：

1. `ret = close[t] / close[t-20] - 1`。K 线不足、价格非有限正数、收益非正：跳过该股，不让整批失败。
2. 在**本次 JS 调用收到的名单**里按 `ret` 降序（收益相同再按代码）取前 N。
3. 只给这些名字打标签 `XS_MOM_TOP`。默认不按 RSI 否决（强动量经常高 RSI）；`xsmomRsiMax` 显式设置后才否决。RSI 仍用现有 `calcRSI` 写入 hit，供研究分使用。
4. `strategy_id` 为 `ext_xsmom_v1` 时**替换**冰点 summarize，不叠加冰点标签。
5. `strategy_id` 为空或 `default` 时永远走冰点，即使 `signalParamsJson` 写了 `scanMode`。
6. 其它非 default 的 id，仅当 `scanMode` 为 `xsmom` 或 `ext_xsmom_v1` 时走动量。

全市场入口里，冰点仍按 400 只分块。动量改为**一次**把已准备名单交给 JS，避免并发准备顺序把截面切成随机 400 只。Universe 定向扫描未改，仍是冰点。

默认参数（可被 `signalParamsJson` 顶层键覆盖，不经过冰点 merge，避免被丢掉）：

- `xsmomLookback`：20
- `xsmomTopN`：20
- `xsmomRsiMax`：不设则关闭

## 评分路径

`SignalScanHit` 没有 `score`。`ParseSnapshotHits` 能解析 `items[]` 时，Research **不会**走到 `parseFlexibleSnapshotRows`。因此选了更小的改动：在 `CalcResearchSignalScore` 给 `XS_MOM_TOP` 基分 **76**（`recentSignalDaysAgo=0` 时不扣时效；RSI≥35 不加分）。76 > 默认阈值 60。`PredictDirectionFromTag` 记为「看多」。这是观察分，不是交易指令。

## 如何调用

现有 Wails / App，不要新接口：

```text
StartSignalScanSnapshot(session, signalParamsJson, "ext_xsmom_v1", "截面动量V1")
```

同步版同样参数：`RunSignalScanSnapshot`。二者都进 `RunFullMarketSnapshot`。

`session`：`close` 或 `midday`。`signalParamsJson` 可空（用默认 20 / 20），或例如：

```json
{"xsmomLookback":20,"xsmomTopN":20}
```

股票筛选页的「生成快照」会把**当前参数预设 id** 当作 `strategy_id`。默认预设是 `default`，仍是冰点。要在该页触发动量，需要一条 id 为 `ext_xsmom_v1` 的预设（名称建议「截面动量V1」），再点生成。本刀没有改默认预设列表。

## 如何核对

数据库 `signal_scan_snapshots`：

```sql
SELECT id, trade_date, session, scope, strategy_id, strategy_name, hit_total, status
FROM signal_scan_snapshots
WHERE strategy_id = 'ext_xsmom_v1'
ORDER BY created_at DESC;
```

`result_json` 里 `strategyId` 应为 `ext_xsmom_v1`，`items[].tag` 应为 `XS_MOM_TOP`。同日同时段的 `strategy_id=default`（以及历史空 strategy）行不应被这次扫描删除：删除条件在 `RunFullMarketSnapshot` 里，id 不是 `default` 时只匹配 `strategy_id = ?`。

已有读取接口：`GetLatestSignalScanSnapshotByStrategy(tradeDate, session, "ext_xsmom_v1")`。筛选页若选中该预设 id，会走这条接口。

## Research「最新快照」限制

`ListResearchCandidatesFromLatestSnapshot` 只取研究范围内 `status=done`、`created_at` 最新的一条，**不按 strategy_id 过滤**。因此：

- 若最新一条就是 `ext_xsmom_v1`，Research 会看到 `XS_MOM_TOP`（分数约 76）。
- 若之后又跑了 default 冰点扫描，Research 会改看冰点快照，动量行仍在库里但不会出现在该列表。

这是已知限制，本刀没有改过滤语义。单测 `TestListResearchCandidates_LatestSnapshotIgnoresStrategy` 把这个行为锁住。

## 测试

```text
go test ./backend/data/ -count=1 -run 'TestRunSignalScanBatchJS|TestSignalScanBatchSpan|TestCalcResearchSignalScore|TestPredictDirectionFromTag|TestListResearchCandidates_'
```

结果：`ok go-stock/backend/data`。

覆盖：前 N 与排序、负收益/短样本/零价/空 K 线跳过、收益相同时按代码、default 忽略 scanMode、非 default 的显式 scanMode、RSI 门槛默认关闭、回看天数覆盖、研究分过 60、最新快照不按策略过滤。

Bundle 重建命令（与 `scripts/build-windows.ps1` 相同参数，esbuild 锁在前端 lock 的 0.25.5）：

```text
npx --yes esbuild@0.25.5 scripts/scansignals/scan-batch.ts --bundle --platform=neutral --format=iife --global-name=SignalScanBatch --outfile=backend/data/signal_scan_bundle.js
```

## 改动文件

- `scripts/scansignals/scan-batch.ts` — 模式开关与截面动量
- `frontend/src/utils/signalTagConstants.js` — 快照标签白名单加入 `XS_MOM_TOP`
- `frontend/src/utils/signalBuyGuide.js` — 筛选下拉注释（下拉数据来自同一白名单）
- `backend/data/signal_scan_bundle.js` — 由上述源重建
- `backend/data/signal_scan_runner.go` — JS 入参增加 `strategyId`
- `backend/data/signal_scan_api.go` — 全市场扫描把 strategy id 传入 JS；动量一次截面
- `backend/data/signal_scan_xsmom_test.go` — 新
- `backend/data/research_candidate_pool.go` — 标签分与方向
- `backend/data/research_candidate_pool_test.go` — 分数与「最新快照」限制
- `PHASE_EXT_XSMOM_V1_IMPLEMENTATION_REPORT.md` — 本报告

未改：`BuildCandidatePool`、Trade Candidate、TradePlan、Approve/Freeze、Broker、Track-A、Strategy Schema 执行器、Research 列表过滤条件。
