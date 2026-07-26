# Phase7-A2-1 Commit A Report

> 范围：**仅** Agent 东财 plain K 线工具迁移  
> 计划：`PHASE7_A2_1_AGENT_KLINE_MIGRATION_PLAN.md`  
> 本任务：**实现 + 验证报告**；**未**执行 git commit；**未**进入 Commit B（Golden）

---

## 1. 修改文件列表

| 文件 | 变更 |
|---|---|
| `backend/data/tool_eastmoney_kline.go` | `GetEastMoneyKLine` / `EastMoneyKLineSection` 经 `KlineService.GetBars` 取数；保留 `ValidateStockCode`；**未改** `WithMA` |
| `backend/data/kline_service_access.go` | **新增**：`SetKlineService` / `SetKlineServiceFactory` / `GetKlineService`（打破 data↔adapter 循环依赖） |
| `backend/data/tool_eastmoney_kline_test.go` | **新增**：Section 走 Service 的 stub 单测 |
| `backend/agent/tools/marketdata_wire.go` | **新增**：懒加载注册 `EastMoneyKlineAdapter` |
| `backend/agent/tools/data_tools_wrapper.go` | `GetEastMoneyKLine` wrapper 显式 `NewEastMoneyKlineAdapter` + `EastMoneyKLineSectionWithService` |

**未修改：** `eastmoney_kline_api.go`、`kline_cache.go`、Signal/Strategy/TradePlan/Risk/Paper/Execution、schema、`GetEastMoneyKLineWithMA`。

---

## 2. 新旧调用链对比

### Before

```text
GetEastMoneyKLine (handler / eino wrapper)
  → NewEastMoneyKLineApi
  → EastMoneyKLineSection
      → api.GetAdjustedKLine / api.GetKLineData
          → GetKLineDataBefore → kline_cache / upstream
```

### After

```text
GetEastMoneyKLine (handler / eino wrapper)
  → ValidateStockCode（仍用 EastMoneyKLineApi，仅校验）
  → marketdata.KlineService.GetBars(..., endTime=zero)
      → EastMoneyKlineAdapter
          → EastMoneyKLineApi.GetKLineDataBefore  （行情源/缓存未改）
  → []Bar → 原 markdown 列映射
```

注入方式：`agent/tools` init 注册**懒加载**工厂（避免 tools 包 init 时 DB 未就绪 panic）；handler 用 `GetKlineService()`，wrapper 直接 `adapter.NewEastMoneyKlineAdapter()`。

---

## 3. 是否仍影响交易链

| 域 | 影响 |
|---|---|
| Signal Snapshot | **否** |
| Strategy / Candidate / TradePlan / Risk | **否** |
| Paper Trading / Execution | **否** |
| 行情源 / kline_cache / DB schema | **否**（仍委托同一 `GetKLineDataBefore`） |
| `GetEastMoneyKLineWithMA` | **否**（仍直连旧 API） |

结论：**不影响交易链。**

---

## 4. 测试结果

| 命令 | 结果 |
|---|---|
| `go test ./backend/marketdata/...` | **PASS** |
| `go test ./backend/data -run 'TestEastMoneyKLineSection\|…\|TestEastMoneyKlineAdapter…'` | **PASS** |
| `go test ./backend/agent/tools` | **PASS**（修复懒加载前曾因 init 调 `GetSettingConfig` panic，已改为 Factory） |

既有无关项：`go vet` 对 `crawler_api.go` context leak 告警（预先存在，未修）。

---

## 5. 后续 Golden Test 计划（Commit B，未做）

按计划 §4：

1. Fake / fixture：同入参下旧 `GetKLineDataBefore` 映射 vs `Adapter.GetBars`  
2. 必比字段：`time` / `open` / `high` / `low` / `close` / `volume` / `amount`  
3. 矩阵：日K、qfq、分钟、港股、非法入参  
4. 独立 commit：`Phase7-A2-1: add agent K-line migration golden tests`

---

## 6. Git / 提交说明

- **未** `git add -A`，**未** commit  
- 建议日后选择性提交仅上表文件 + 本报告（若纳入）  
- 工作区其它 dirty **禁止**一并提交  

---

## 7. 元数据

| 项 | 值 |
|---|---|
| 是否进入 Commit B | **否** |
| 是否新增 KlineService 接口方法 | **否** |
| 报告文件 | `PHASE7_A2_1_COMMIT_A_REPORT.md` |
