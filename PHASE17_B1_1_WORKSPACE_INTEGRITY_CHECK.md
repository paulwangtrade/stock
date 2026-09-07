# PHASE17-B.1.1 工作区完整性检查

**日期：** 2026-09-06（复检）  
**性质：** **只读检查**（未修改业务代码）  
**背景：** Phase17-B.1 期间 D: 磁盘满、`build/` 曾约 43GB、`App.vue` 截断后 git 恢复  
**分支：** `release/v0.1.0-beta`

---

## 0. 总判

| 检查项 | 结论 |
| --- | --- |
| 1. git status | **脏工作区**（~2320 行）；无冲突；B.1/A.2 改动多未提交 |
| 2. 最近修改文件完整性 | **关键源码完整**（无空 Vue / 无截断模板） |
| 3. App.vue 入口 | **完整且 = HEAD**（截断已消除） |
| 4. frontend build | **`dist` 新鲜可用**（23:30，含 B.1/A.2 产物） |
| 5. build 目录占用 | **~1.14GB**（以 DB 为主）；**无** 43GB 堆积 |
| 6. 临时文件 | **大量** `_p16_*` / `_v1_*` 等噪音；根目录误落 `` `--` `` |
| 7. exe 有效性 | **PE 有效但过期**（20:38 ≪ dist 23:30） |

**一句话：** 源码与 Vite 产物健康；桌面 `go-stock.exe` 未重编，不能代表最新前端；磁盘与 43GB 危机已缓解。

---

## 1. git status

| 指标 | 数值 |
| --- | --- |
| porcelain 总行 | **~2320** |
| 修改 | **~190** |
| 未跟踪 | **~2074** |
| 删除 | **56** |
| 冲突 | **0** |

### 与 B.1 / A.2 相关（摘录）

| 路径 | 状态 |
| --- | --- |
| `backend/data/kline_cache.go` / `_test.go` | `M` |
| `backend/data/eastmoney_kline_api.go` | `M`（相关列表中有） |
| `frontend/src/components/StockLightweightKlineChart.vue` | `M` |
| `frontend/src/utils/tradingSession.js` | `M` |
| `frontend/src/utils/aShareSessionClock.js` / `klineCache.js` | `??` |
| `frontend/src/components/PortfolioDashboard.vue` | `??`（A.2；仓库侧此前可能未跟踪） |
| `frontend/src/api/portfolioSnapshot.ts` / `portfolioDashboard.ts` | `??` |
| `frontend/src/components/InvestmentHome.vue` | `M` |
| `frontend/src/App.vue` | **干净**（工作区哈希 = HEAD） |
| `PHASE17_B1_*` / `PHASE17_A2_*` 报告 | `??` |

**风险：** 工作区极脏，B.1/A.2 难单独审 diff；误提交风险高。非本检查修复范围。

---

## 2. 最近修改文件完整性

抽查关键文件：

| 文件 | 字节 | 行约 | `<template>` 闭合 | 判定 |
| --- | --- | --- | --- | --- |
| `StockLightweightKlineChart.vue` | 101,520 | ~3293 | OK | 完整 |
| `PortfolioDashboard.vue` | 28,756 | ~954 | OK | 完整 |
| `InvestmentHome.vue` | 38,772 | ~1171 | OK | 完整 |
| `klineCache.js` / `aShareSessionClock.js` / `tradingSession.js` | 正常 | — | — | 完整 |
| `kline_cache.go` / `eastmoney_kline_api.go` | 正常 | — | — | 完整 |

- `frontend/src` 下 **无** ≤20B 空/残文件。  
- kline selftest：`chartMarkers.klineCache.selftest: OK`。

---

## 3. App.vue 入口完整性

| 项 | 结果 |
| --- | --- |
| 路径 | `frontend/src/App.vue` |
| 大小 / 行 | **35,250 B / ~1145 行** |
| 模板 | `<template>…</template>` **闭合** |
| `git hash-object` vs `HEAD:frontend/src/App.vue` | **相同** `bc90b40e…` |
| porcelain | **无改动** |

**结论：** 磁盘满导致的截断 **已恢复且未再漂移**。入口 `main.js` / `index.html` 存在且可读。

（旁注：`ai-assistant-web/frontend/src/App.vue` 为子项目修改，与主壳截断事件无关。）

---

## 4. frontend build 状态

| 项 | 结果 |
| --- | --- |
| Node / npm | v24.18.0 / 11.16.0 |
| `node_modules/vite` | 存在 |
| `frontend/dist` | **存在**；228 文件 / ~8.4MB |
| dist mtime | **2026-09-06 23:30:16**（晚于 A.2 源码改动） |

产物抽样：

| chunk | 说明 |
| --- | --- |
| `klineCache-*.js` | B.1 内存 TTL chunk 存在 |
| `StockKlineModal-*.js` | 存在 |
| `PortfolioDashboard-*.js` | 存在；含「累计浮盈」等 A.2 文案 |
| `InvestmentHome-*.js` | 存在 |

**结论：** Vite client build **与当前前端工作树一致可用**。本检查未再次执行 `npm run build`（只读；沿用现有 dist）。

---

## 5. build 目录占用（`D:\stock\build`）

| 项 | 值 |
| --- | --- |
| D: 剩余 | **~42.3 GB** |
| C: 剩余 | **~14.3 GB** |
| `build` 合计 | **~1.14 GB / 14 文件** |
| 43GB 历史中间物 | **不存在**（此前清理后未回潮） |

### Top 占用

| 文件 | 约大小 | 性质 |
| --- | --- | --- |
| `bin/data/stock.db` | 516 MB | **运行时用户库** |
| `bin/data/stock.db.backup.20260906230815` | 516 MB | 备份 |
| `bin/go-stock.exe` | 92 MB | 可执行文件 |
| logs / wal | ~数 MB | 附属 |

**结论：** 当前占用健康；体积主因是 **DB 而非编译垃圾**。

---

## 6. 临时文件

| 类型 | 现状 |
| --- | --- |
| 根目录 `_p16_14_*` / `_v1_*` / `_s_*` / `_u_*` / `_rc_*` | 大量审计/构建日志与 UIA 抓取 |
| `_rc_backend_test.txt` | **~5MB**（偏大日志） |
| 目录 `_s_smoke_home`、`_t_stage_bak`、`_audit_tool` 等 | 临时/审计目录 |
| 字面文件 `` `--` `` | **14,476 B**，内容为误落 `package main`（2026-07-21） |
| 空文件 | 如 `_p16_14_uia_provenance_scan2.txt`、`_s_smoke_stdout.txt` = 0B |

**建议（未执行）：** 归档或删除上述临时证据与 `` `--` ``；**勿**删 `stock.db`。

---

## 7. exe 产物有效性

| 检查 | 结果 |
| --- | --- |
| 路径 | `D:\stock\build\bin\go-stock.exe` |
| 大小 | 92.31 MB |
| PE 头 | **MZ**（有效 Windows PE） |
| mtime | **2026-09-06 20:38:27** |
| vs `frontend/dist` | dist **23:30** → exe **早约 2.5h+**，且早于 B.1/A.2 前端落地 |

**结论：**

- 文件 **可启动层面有效**；  
- **内容过期**：未嵌入最新 Vite 产物，**不能**用该 exe 验收 Phase17-B.1 / A.2 UI。  
- 需要时：关闭进程后 `wails build`（或文档中的 skipbindings 流程）重编。

---

## 8. 风险项汇总

| ID | 风险 | 等级 |
| --- | --- | --- |
| R1 | exe 与最新前端不同步 | **高**（验收误判） |
| R2 | git 工作区极度脏 | 中 |
| R3 | 误删 `stock.db` / 把 DB 当编译垃圾 | 中 |
| R4 | 临时文件与 `` `--` `` 污染根目录 | 低 |
| R5 | C: 余量有限（Go cache 仍可能很大） | 低–中 |
| R6 | App.vue 截断 | **已关闭**（与 HEAD 一致） |

---

## 9. 建议清理项（仅建议，本检查未执行）

1. 需桌面验收时：**重编** `go-stock.exe`。  
2. 确认后删除 `stock.db.backup.*`（约 0.5GB）。  
3. 清理根目录 `_p16_*` / `_v1_*` / `_rc_*` 等临时日志与 `` `--` ``。  
4. 可选：`go clean -cache`（腾 C:）。  
5. **禁止**无备份删除 `build/bin/data/stock.db`。

---

## 10. 检查结论

| 历史问题 | 复检现状 |
| --- | --- |
| D 盘满 | **已缓解**（~42GB free） |
| build ~43GB | **已消除**（现 ~1.1GB，DB 为主） |
| App.vue 截断 | **已恢复 = HEAD** |
| 工作区健康（开发） | **源码 + dist OK** |
| 工作区健康（发布 exe） | **需重编** |

**Phase17-B.1.1 复检完成：可继续后续切片；桌面 exe 验收前必须重编。**

---

*只读审计输出；未修改任何业务代码。*
