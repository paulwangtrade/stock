# PHASE17 A.2 + B.1 集成构建与 Runtime 验收

**日期：** 2026-09-06  
**性质：** 集成发布验收（**未新增功能 / 未重置源码 / 未删业务库**）  
**分支：** `release/v0.1.0-beta`  
**覆盖：**  
- [PHASE17_A2_PORTFOLIO_IMPLEMENTATION_COMPLETE.md](./PHASE17_A2_PORTFOLIO_IMPLEMENTATION_COMPLETE.md)  
- [PHASE17_B1_KLINE_OPTIMIZATION_IMPLEMENTATION_COMPLETE.md](./PHASE17_B1_KLINE_OPTIMIZATION_IMPLEMENTATION_COMPLETE.md)  
- [PHASE17_B1_1_WORKSPACE_INTEGRITY_CHECK.md](./PHASE17_B1_1_WORKSPACE_INTEGRITY_CHECK.md)  

**市场状态：** 周末 → **IDLE**

---

## 0. 总判

# **PASS**

| 维度 | 结论 |
| --- | --- |
| 最新 exe 构建 | **PASS**（23:54:37，嵌入当日 23:54:18 dist） |
| A.2 Portfolio 语义文案 | **PASS**（dist 含账户/持仓盈亏命名 + as_of） |
| B.1 K 线 IDLE 缓存 / 假 miss / 停 poll | **PASS** |
| 回归（首页双槽 / TradePlan / Opportunity） | **PASS**（产物 + 单测） |
| 业务数据 | **保留**（`stock.db` 仍在） |

---

## 1. 新 exe 信息

| 字段 | 值 |
| --- | --- |
| **路径** | `D:\stock\build\bin\go-stock.exe` |
| **Build / 文件时间** | **2026-09-06 23:54:37** |
| **大小** | **92.31 MB**（96,792,576 bytes） |
| **SHA256** | `D54E6DE245545226A95AE98AE702740D428A3F23EA03663F2B1F7B096671B7C1` |
| 嵌入 dist 时间 | **2026-09-06 23:54:18**（exe **晚于** dist） |
| 启动 | **23:54:55** / PID **17028** |
| 业务库 | `build/bin/data/stock.db` = **True** |

### Build 过程

```text
Stop-Process go-stock
Remove-Item build\bin\go-stock.exe   # 仅 exe
npm run build                        # frontend/dist
wails build -skipbindings -s         # Clean Bin Dir=false
→ Built 'D:\stock\build\bin\go-stock.exe' in 20.15s  (exit 0)
```

未 `git reset` / 未删 `stock.db*`。

---

## 2. A.2 Portfolio 验收

嵌入前端 UTF-8 探针（`frontend/dist/assets`）：

| 位置 | 文案 / 能力 | 嵌入结果 |
| --- | --- | --- |
| 首页 `InvestmentHome-*.js` | **账户今日盈亏** | **true** |
| 首页 | **今日执行** / **下一交易日准备**（双槽） | **true** |
| 首页 | **今日机会** | **true** |
| 组合 `PortfolioDashboard-*.js` | **账户今日盈亏** | **true** |
| 组合 | **持仓今日浮盈** | **true** |
| 组合 | **累计浮盈** | **true** |
| 组合 | **数据截至**（as_of） | **true** |
| 组合 | **日报差**（basis 标签） | **true** |

**语义（未改计算，仅展示）：**

| UI | 口径 |
| --- | --- |
| 账户今日盈亏 | 相对上一结算日报权益差（非盘中浮盈合计） |
| 持仓今日浮盈 | (行情 − 昨收) × 数量 |
| 累计浮盈 | (mark − 成本) × 数量 |
| 数据截至 | snapshot/dashboard `as_of` |

**判定：A.2 = PASS**（产物级；GUI 点选建议人工再确认一眼）。

---

## 3. B.1 K 线验收

| 场景 | 方法 | 结果 |
| --- | --- | --- |
| 非交易时间优先缓存 | 热缓存探针 `GetKLineDataBefore(sz300408,101,limit=800)` | 缓存 120 → 返回 120，**~26 ms** → **PASS** |
| 假 miss（bars &lt; limit） | 同上 + Go 单测 | **PASS** |
| IDLE 停 60s poll | `isKlineMarketLive(now)=false` + `setupPoll` 门控 | **PASS** |
| 避免 latest 重复请求 | IDLE 有本地 bars 快路径 | **PASS** |
| LIVE 实时性保留 | `TestKlineCacheTTLAtLiveVsIdle`（盘中 60s / 周末 24h） | **PASS（逻辑）** |
| 前端 TTL 档 | `klineCache-*.js` 含 `5*60*1e3` / `30*60*1e3` | **true** |

```text
go test … TestKLineCacheGetLatestAllowsFewerBarsThanLimit|TestKlineCacheTTLAtLiveVsIdle → ok
idle_kline_probe → RESULT=PASS
live_now=false
```

**判定：B.1 = PASS**

---

## 4. 回归

| 项 | 证据 | 结论 |
| --- | --- | --- |
| 首页计划双槽 | dist「今日执行」「下一交易日准备」；`homePlanDisplay.test.mjs` **6/6 pass** | **PASS** |
| TradePlan | `TradePlanUpcoming-*.js` 存在（~105KB），含「查看解释」「交易计划」 | **PASS** |
| Opportunity | 首页「今日机会」嵌入；`OpportunityProjectionDrawer` chunk 仍被 InvestmentHome 引用 | **PASS** |
| 启动健康 | loading 100% 启动完成；非交易日跳过自选价监控 | **PASS** |

说明：启动日志偶现冷路径东财 EOF，与热缓存 B.1 路径无关。

---

## 5. 截图

| 文件 | 说明 |
| --- | --- |
| `_p17_release_acceptance/integrated_runtime_2354.png` | 集成 exe 启动后主屏截图 |
| `_p17_release_acceptance/idle_kline_probe.go` | B.1 探针副本（可选） |
| `_p17_b1_acceptance/*` | 既有 B.1 验收留存 |

人工建议：打开「我的资产 / 我的组合 / 点开 K 线」各确认一眼。

---

## 6. PASS / FAIL 汇总

| # | 项 | 结论 |
| --- | --- | --- |
| 1 | exe 信息（路径/时间/大小/SHA256） | **PASS** |
| 2 | A.2 Portfolio 字段语义嵌入 | **PASS** |
| 3 | B.1 K 线 IDLE / 假 miss / 停 poll | **PASS** |
| 4 | 回归双槽 / TradePlan / Opportunity | **PASS** |
| 5 | 保留 stock.db / 不重置源码 | **PASS** |
| — | **总评** | **PASS** |

---

## 7. 使用

```powershell
Set-Location D:\stock\build\bin
# 若已有实例请先结束，再：
.\go-stock.exe
```

当前已启动：PID **17028**（路径同上）。

---

**Phase17 A.2 + B.1 集成发布 Runtime 验收：PASS。**
