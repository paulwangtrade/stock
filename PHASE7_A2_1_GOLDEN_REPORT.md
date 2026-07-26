# Phase7-A2-1 Golden Observation Report

> 性质：**只读验证** — 未改业务代码 / 行情逻辑 / 缓存 / 数据库；**未** commit  
> 基线 commit：`297caa0220f3e2e4bb9efa0f427058c6fbc2ced1`  
> 验证时间：2026-07-27（本地 live 拉取）

---

## 1. 验证对象

| 路径 | 调用链 |
|---|---|
| **Old** | `EastMoneyKLineApi.GetKLineDataBefore` → 解析 OHLC/volume/amount |
| **New** | `KlineService.GetBars` → `EastMoneyKlineAdapter` → **同一** `GetKLineDataBefore` → `[]Bar` |

两端共用同一 `EastMoneyKLineApi` 实例，保证行情源与缓存行为一致；比较的是 Adapter 映射是否引入数值/顺序偏差。

---

## 2. 样本列表

| Code | 市场 | 行业 |
|---|---|---|
| `000001.SZ` | 深圳 | 银行 |
| `000858.SZ` | 深圳 | 白酒 |
| `600000.SH` | 上海 | 银行 |
| `600519.SH` | 上海 | 白酒 |
| `002415.SZ` | 深圳 | 安防/科技 |

覆盖：深/沪、银行/白酒/科技。

---

## 3. 固定参数

| 参数 | 取值 |
|---|---|
| `period` | `101`（日 K） |
| `adjust` | `""`（不复权）或 `"qfq"`（前复权） |
| `limit` | `60` 或 `120` |
| `endTime` | zero → `20500101`（取最新） |

每个样本跑 3 组参数：`(101,"",60)` / `(101,"qfq",60)` / `(101,"",120)` → **共 15 组用例**。

比较字段：bar 数量、第一/最后一根时间、open/high/low/close、volume、amount。  
允许：JSON 顺序差异、字符串格式差异。  
禁止：OHLC 数值差异、bar 顺序变化。

---

## 4. Old / New 对比结果

模式：`live`（实盘；部分标的东财 HTTP EOF 后走既有腾讯回退，新旧路径同源回退）。

| Status | Code | Market | Industry | period | adjust | limit | oldN | newN | first | last |
|---|---|---|---|---|---|---:|---:|---:|---|---|
| PASS | 000001.SZ | SZ | 银行 | 101 | `""` | 60 | 60 | 60 | 2026-04-28 | 2026-07-24 |
| PASS | 000001.SZ | SZ | 银行 | 101 | `qfq` | 60 | 60 | 60 | 2026-04-28 | 2026-07-24 |
| PASS | 000001.SZ | SZ | 银行 | 101 | `""` | 120 | 120 | 120 | 2026-01-23 | 2026-07-24 |
| PASS | 000858.SZ | SZ | 白酒 | 101 | `""` | 60 | 60 | 60 | 2026-04-28 | 2026-07-24 |
| PASS | 000858.SZ | SZ | 白酒 | 101 | `qfq` | 60 | 60 | 60 | 2026-04-28 | 2026-07-24 |
| PASS | 000858.SZ | SZ | 白酒 | 101 | `""` | 120 | 120 | 120 | 2026-01-23 | 2026-07-24 |
| PASS | 600000.SH | SH | 银行 | 101 | `""` | 60 | 60 | 60 | 2026-04-28 | 2026-07-24 |
| PASS | 600000.SH | SH | 银行 | 101 | `qfq` | 60 | 60 | 60 | 2026-04-28 | 2026-07-24 |
| PASS | 600000.SH | SH | 银行 | 101 | `""` | 120 | 120 | 120 | 2026-01-23 | 2026-07-24 |
| PASS | 600519.SH | SH | 白酒 | 101 | `""` | 60 | 60 | 60 | 2026-04-28 | 2026-07-24 |
| PASS | 600519.SH | SH | 白酒 | 101 | `qfq` | 60 | 60 | 60 | 2026-04-28 | 2026-07-24 |
| PASS | 600519.SH | SH | 白酒 | 101 | `""` | 120 | 120 | 120 | 2026-01-23 | 2026-07-24 |
| PASS | 002415.SZ | SZ | 安防/科技 | 101 | `""` | 60 | 60 | 60 | 2026-04-28 | 2026-07-24 |
| PASS | 002415.SZ | SZ | 安防/科技 | 101 | `qfq` | 60 | 60 | 60 | 2026-04-28 | 2026-07-24 |
| PASS | 002415.SZ | SZ | 安防/科技 | 101 | `""` | 120 | 120 | 120 | 2026-01-23 | 2026-07-24 |

**汇总：PASS=15 / FAIL=0 / TOTAL=15**

---

## 5. 差异统计

| 指标 | 计数 |
|---|---:|
| bar_count_mismatch | 0 |
| first_time_mismatch | 0 |
| last_time_mismatch | 0 |
| order_mismatch | 0 |
| ohlc_mismatch | 0 |
| volume_mismatch | 0 |
| amount_mismatch | 0 |

结论：在 live 样本与参数矩阵下，**无 OHLC / 量额 / 顺序 / 根数差异**。Adapter 映射与直连旧 API 数值对齐。

---

## 6. 是否允许进入 Phase7-A2-2

**允许进入 Phase7-A2-2 规划/设计（非自动批准大规模迁移）。**

建议 A2-2 仍按 inventory 安全顺序：

1. 先做调用点清单确认（App 展示 / AI，非 Paper/Execution）  
2. 小范围试点 + 同类 Golden  
3. Paper / Morning / Execution 取价最后  

本 Golden **不**覆盖：`GetEastMoneyKLineWithMA`、分钟线、港股、Signal `prepareStockBars`。

---

## 7. 元数据

| 项 | 值 |
|---|---|
| 业务代码是否修改 | **否** |
| 是否 commit | **否** |
| 验证脚本 | 临时 `tmp_phase7a21_golden/`（非产品；可删，未纳入版本库提交） |
| 输出报告 | `PHASE7_A2_1_GOLDEN_REPORT.md` |
