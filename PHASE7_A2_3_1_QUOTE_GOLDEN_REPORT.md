# Phase7-A2-3-1 Quote Consumer Migration Golden Report

> 性质：**只读验证** — 未改业务逻辑 / QuoteService interface / LegacyQuoteAdapter / 缓存策略 / Paper / Execution / TradePlan / Risk  
> **未** `git add` / **未** commit  
> 验证时间：2026-07-27（本地 live 单次拉取 + 同快照映射对照）

---

## 1. 测试环境

| 项 | 值 |
|---|---|
| **Commit A** | `e609206` — `Phase7-A2-3-1: migrate watchlist quote consumers to QuoteService` |
| 父提交 | `2297c0c`（realtime quote cache foundation） |
| 模式 | 同进程双路径：Old=`GetStockCodeRealTimeData` 一次 live；New=`QuoteService`→`LegacyQuoteAdapter`→`quoteToStockInfo` 对**同一** `[]StockInfo` 快照映射（避免二次网络导致价差） |
| 辅证 | Live E2E：同进程对同一批样本再调 `LegacyQuoteAdapter.GetQuotes`（`000001.SZ` 等 Agent 码）→ `quoteToStockInfo`，与 Old 快照比较（OHLC/timestamp 一致） |
| Consumer 码 | 关注列表常用腾讯码：`sz000001` / `sh600519` / `sz000858`（由 `EastMoneyToTencentCode` 自样本转换） |
| 临时验证器 | `tmp_phase7_a231_quote_golden/`（验证后删除，不入库） |
| DB | 临时 sqlite（仅满足 `NewStockDataApi`→`GetSettingConfig`；无业务写入） |

### 定向单测（禁止 `go test ./...`）

| 命令 | 结果 |
|---|---|
| `go test ./backend/marketdata/ -count=1 -run LegacyQuote` | **ok**（~32s） |
| `go test . -run QuoteToStockInfo\|…`（全 package） | **未跑通**：工作区污染（`app_domready.go` 与 `app.go` 重复 `domReady`；平台文件仍按旧 `GetStockInfos` 双返回值）— **与本 Golden 无关**，未改业务代码 |

---

## 2. 样本列表

| code | market | 覆盖 | old | new | result |
|---|---|---|---|---|---|
| `000001.SZ` | SZ | 深圳 / 大盘银行 | live `GetStockCodeRealTimeData(sz000001)` | 同快照 → Adapter → `quoteToStockInfo` | **PASS** |
| `600519.SH` | SH | 上海 / 消费白酒 | live `GetStockCodeRealTimeData(sh600519)` | 同上 | **PASS** |
| `000858.SZ` | SZ | 深圳 / 消费白酒 | live `GetStockCodeRealTimeData(sz000858)` | 同上 | **PASS** |

**汇总：PASS=3 / FAIL=0 / TOTAL=3**（快照路径；Live E2E 亦 3/3 PASS）

---

## 3. 字段 diff

允许：数值字符串表示差异（`1308.00` vs `1308`；空 volume/amount `""` vs `"0"`），按 float 语义等价判定。  
禁止：OHLC / 名称 / 时间戳语义偏差。  
`FetchedAt` 为 Adapter 侧时钟，**不参与** old/new 数值 diff。

### 3.1 `000001.SZ`（平安银行）

| field | old | new | diff |
|---|---|---|---|
| code | sz000001 | sz000001 | 0 |
| name | 平安银行 | 平安银行 | 0 |
| price | 11.08 | 11.08 | 0 |
| open | 11.11 | 11.11 | 0 |
| high | 11.16 | 11.16 | 0 |
| low | 11.04 | 11.04 | 0 |
| volume | *(empty)* | 0 | 0（语义等价：parseFloat 空→0） |
| amount | *(empty)* | 0 | 0（同上） |
| timestamp | 2026-07-27 11:38:06 | 2026-07-27 11:38:06 | 0 |

### 3.2 `600519.SH`（贵州茅台）

| field | old | new | diff |
|---|---|---|---|
| code | sh600519 | sh600519 | 0 |
| name | 贵州茅台 | 贵州茅台 | 0 |
| price | 1288.51 | 1288.51 | 0 |
| open | 1308.00 | 1308 | 0（float 等价） |
| high | 1308.00 | 1308 | 0（float 等价） |
| low | 1279.58 | 1279.58 | 0 |
| volume | *(empty)* | 0 | 0（语义等价） |
| amount | *(empty)* | 0 | 0（语义等价） |
| timestamp | 2026-07-27 11:37:56 | 2026-07-27 11:37:56 | 0 |

### 3.3 `000858.SZ`（五粮液）

| field | old | new | diff |
|---|---|---|---|
| code | sz000858 | sz000858 | 0 |
| name | 五粮液 | 五粮液 | 0 |
| price | 73.89 | 73.89 | 0 |
| open | 73.66 | 73.66 | 0 |
| high | 74.46 | 74.46 | 0 |
| low | 73.51 | 73.51 | 0 |
| volume | *(empty)* | 0 | 0（语义等价） |
| amount | *(empty)* | 0 | 0（语义等价） |
| timestamp | 2026-07-27 11:37:21 | 2026-07-27 11:37:21 | 0 |

### 3.4 观测说明

1. **volume / amount**：本次 live 上游 `StockInfo` 为空字符串；经 `parseFloat("")→0` → `FormatFloat→"0"` 回写。消费层若直接读 string，可能看到 `""` vs `"0"`；**数值语义均为 0**，与 A2-2-1 Golden 同类观测一致，不构成行情偏差。  
2. **open/high 文本**：`1308.00` ↔ `1308` 为 float 往返格式化差异，数值相等。  
3. **Consumer 路径**：`GetStockInfos` / `GetStockInfosRealtimeBatch` 取数入口已为 `fetchRealtimeStockInfos`→`QuoteService.GetQuotes`；本 Golden 验证该链相对旧直连 API 的市场字段一致性（不含 `addStockFollowData` 成本叠加）。

---

## 4. 是否 PASS

### **PASS**

已证明：在相同 `StockInfo` 快照下，Consumer 新路径  
`QuoteService` → `LegacyQuoteAdapter` → `quoteToStockInfo`  
对 **code / name / price / open / high / low / volume / amount / timestamp（Date+Time）** 相对直连 `GetStockCodeRealTimeData` **无语义/数值变化**（3/3）。

| 检查 | 结果 |
|---|---|
| 行情源是否改变 | **否**（仍委托同一 Legacy API） |
| Adapter / bridge 是否引入数值偏差 | **否**（3/3 PASS） |
| 是否触碰 Paper / Execution / Spec | **否** |
| 是否修改业务代码 / interface / 缓存 | **否** |

---

## 5. 是否建议进入下一阶段

| 建议 | 说明 |
|---|---|
| **建议进入下一阶段** | A2-3-1 Watchlist/Monitor Quote Consumer 迁移字段一致性已验证 |
| 可选后续 | A2-4 Paper Observation 显示层 Quote overlay（设计已有）；或清理工作区 DomReady/平台签名污染以便全 package 单测 |

---

## 6. 元数据

| 项 | 值 |
|---|---|
| 业务源码修改 | **否** |
| git add / commit | **否** |
| 残留交付物 | 仅本报告 `PHASE7_A2_3_1_QUOTE_GOLDEN_REPORT.md`（临时验证目录已删除） |
| 等待 | 人工审核 |
