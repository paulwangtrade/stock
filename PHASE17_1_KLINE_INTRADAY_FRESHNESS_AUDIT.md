# PHASE17.1 K 线盘中数据新鲜度审计（只读）

**日期：** 2026-09-07  
**性质：** 只读审计（**未改代码 / 未清缓存 / 未改 TTL / 未改 API / 未修复**）  
**现象：** 盘中自选「多周期K线」最新停在 **2026-09-04**；收盘后「生成快照」再开 K 线出现 **2026-09-07**。  
**日历注记：** 2026-09-04 周五；05–06 周末；**07 周一交易日**。停在 04 即缺本周首个交易日（及当日未完成日 K）。

---

## 0. 总判

| 问题 | 结论 |
| --- | --- |
| **主因分类** | **A. 缓存问题**（主）+ **D. 日期/交易日新鲜度判断缺失**（主） |
| 次因 | **B. 实时刷新受同一前端缓存短路**（poll 也走 `getOrFetch`） |
| 非主因 | C. 数据源本身（东财 latest 在正常 LIVE 全量/增量路径可用；快照扫描能拉到 07 说明源可更新） |
| 入口是否分裂 | **否** — 自选 / 组合 / 机会共用 `StockKlineModal` → `StockLightweightKlineChart` → 同一 Wails K 线 API |

**因果一句话：** Phase17-B.1 为加速加载引入 **IDLE 可返回过期 `kline_cache` stale**，且 **不校验 `last_bar_day` 是否落后于当前交易日**；前端内存缓存按「写入时 TTL」过期、**LIVE 轮询不绕过缓存**。盘中/午休若命中「仍止于 09-04」的缓存，就会一直看到 04；「生成快照」全市场拉 K 写入 SQLite 最新 bars 后，再 miss/重拉即可看到 07。

---

## 1. 自选股打开 K 线入口

| 项 | 值 |
| --- | --- |
| 页面 | `frontend/src/components/stock.vue`（自选） |
| 打开函数 | `openLightweightKlineModal(code, name, …)` |
| Modal | `StockKlineModal`（`modalShow6`） |
| 图表 | 内嵌 `StockLightweightKlineChart` |
| 代码传递 | `lwKlineCode = toEastMoneyCode(code)` → `:code="lwKlineCode"` |
| 名称 | `:stock-name="lwKlineName"` |
| 成本/仓位 | 自选 Follow 成本价/量（有则注入）；**无**组合 `positionRow` |
| market context | **无**独立 market/session 查询参数；周期在图表内选（默认日 K `101`） |

与组合差异（展示层 only）：

| | 自选 | 我的组合 |
| --- | --- | --- |
| Modal | 同 | 同 |
| Chart | 同 | 同 |
| 额外 props | 策略信号可选 | B.2：`costPrice` / `costVolume` / footer 卖出 |
| 数据链 | **同一** `GetStockEastMoneyKLine` | **同一** |

→ 新鲜度问题 **不是** 自选入口特有接线错误。

---

## 2. K 线数据请求链路

```text
StockKlineModal.vue
  └── StockLightweightKlineChart.vue
        loadData / refreshLatestPoll
          └── fetchEastMoneyKlineCached(code, name, klt, limit)
                └── frontend klineCache.getOrFetch(key)
                      └── Wails GetStockEastMoneyKLine / GetStockEastMoneyKLinePage
                            └── app.go → EastMoneyKLineApi.GetKLineDataBefore
                                  ├── SQLite kline_cache（TTL / stale）
                                  └── HTTP 东财 push2his …/kline/get（失败可腾讯 fallback）
```

| 层 | 文件 | 函数 / 要点 |
| --- | --- | --- |
| Modal 契约 | `StockKlineModal.vue` | 身份 prop：`code` |
| 图表加载 | `StockLightweightKlineChart.vue` | `loadData`、`fetchEastMoneyKlineCached`、`refreshLatestPoll` |
| FE 缓存 | `utils/klineCache.js` | `klineCacheKey` / `getOrFetch` / LIVE·IDLE TTL |
| LIVE 时钟 | `utils/aShareSessionClock.js` | `isKlineMarketLive` = 连续竞价时段 |
| Wails | `app.go` | `GetStockEastMoneyKLine` → `GetKLineDataBefore(..., end="")` |
| 后端拉取+缓存 | `backend/data/eastmoney_kline_api.go` | `GetKLineDataBefore` |
| 后端 TTL/表 | `backend/data/kline_cache.go` | `klineCacheGet` / `GetStale` / `klineMarketSessionLive` |

**日 K：** `klt=101`，`end=latest`（空/`20500101`）。  
**分钟 K：** `klt=1/5/15/…`，同链路；LIVE TTL 更短（后端 30s 档）。  
**最新补充：** 图表 `setupPoll`：仅 `isKlineMarketLive()` 时每 `realtimeIntervalMs`（默认 60s）调 `refreshLatestPoll` → **仍走 `fetchEastMoneyKlineCached`（会命中 FE 缓存）**。

---

## 3. 缓存审计

### 3.1 前端内存缓存

| 项 | 值 |
| --- | --- |
| Key | `{symbol}|{klt}|{上海当日 YYYY-MM-DD}` |
| LIVE TTL | 5 min |
| IDLE TTL | 30 min |
| 过期策略 | **写入时**写入 `expiresAt`；读取只比 `Date.now() > expiresAt` |
| 日期校验 | **无**（不看最后一根 bar 的交易日） |

**盘中风险：** 若在 **IDLE**（午休/早盘前）首次打开，后端返回止于 **09-04** 的 payload，FE 以 **IDLE 30min** 写入；之后进入 LIVE，只要未过期，`getOrFetch` / **poll 均命中同一 key**，继续显示 04。

### 3.2 后端 SQLite `kline_cache`

| 项 | 值 |
| --- | --- |
| Key | `stock_sec_id + klt + adjust_flag + end_key`（`latest`） |
| 日 K LIVE TTL | **60s** |
| 日 K IDLE（交易日非竞价） | **6h** |
| 周末/非交易日 IDLE | **24h** |
| 新鲜命中 | `klineCacheGet`：未过 TTL 则直接返回 |
| Stale | `klineCacheGetStale`：**忽略 TTL**，有 bars 就读 |

### 3.3 IDLE 短路（Phase17-B.1）— 与现象强相关

`eastmoney_kline_api.go` `GetKLineDataBefore`：

```text
1) TTL 内 cache → 返回
2) 若 end=latest 且 !LIVE 且存在 stale → 直接 finalize(stale)，不 HTTP
3) LIVE 且 stale 够长 → 增量 HTTP(约 8 根) merge；失败则仍返回 stale
4) 否则全量 HTTP
```

**重点确认（审计问题）：**

| 问题 | 答案 |
| --- | --- |
| 已有止于 2026-09-04 的日 K 缓存，盘中是否会直接返回？ | **会**，若处于 **IDLE**（午休/盘前/盘后）走步骤 2；或 FE 内存未过期；或 LIVE 增量 HTTP 失败走步骤 3 fallback stale |
| 是否应追加 2026-09-07？ | **产品期望：应**；**现码：IDLE 故意不拉**；也 **无**「`last_bar_day` < 当前交易日则强制刷新」规则 |

→ **缓存策略故意偏「省流量」**，缺少「交易日追赶」条件，导致可长期展示落后最后一根。

---

## 4. LIVE 门控审计

| | 前端 | 后端 |
| --- | --- | --- |
| 函数 | `isKlineMarketLive` ← `isAShareMarketOpenNow` | `klineMarketSessionLive` |
| 时段 | 09:30–11:30、13:00–15:00 上海 | 同 |
| 周末 | 否 | 否 |
| 节假日 | **前端未用交易日历**（仅星期） | **`tradingcalendar.IsTradingDay`** |

| 场景（2026-09-07） | LIVE? | 行为倾向 |
| --- | --- | --- |
| 连续竞价中 | **是**（前后端一致，若日历认定交易日） | 短 TTL + 可增量；但 FE 可能仍命中早前 IDLE 写入的内存缓存 |
| 午休 11:30–13:00 | **否 → IDLE** | **直接 stale，可停在 09-04** |
| 盘后 | IDLE | 同上（直至扫描/全量刷新写入新 last_bar） |

**结论：** 「盘中」若含午休，**不会**进 LIVE；即使用户口语称盘中，也符合 B.1 IDLE 短路。真正竞价中则依赖 FE 是否仍握着过期 payload、以及 LIVE 增量是否成功。

---

## 5. 「生成快照」影响

| 项 | 值 |
| --- | --- |
| UI | 机会/选股页 `allStockList.vue` 按钮「生成快照」→ `runBackendSnapshotScan` |
| API | `StartSignalScanSnapshot(...)`（后台全市场/策略信号扫描） |
| 与 K 线关系 | 扫描过程会大量调用 K 线读取（`GetStockEastMoneyKLine` / 扫描批处理），**写入/更新 `kline_cache` latest payload**（含当日 bar，若源已有） |
| 是否专用「刷新 K 线缓存」API | **无**；属扫描副作用 |
| 是否清 FE `klineCache` | **代码未见**主动 `clearKlineCache`；更多是 **后端已更新** + FE TTL 过后 miss，或重开进程 |

**为何快照后出现 09-07：**  
扫描强制走后端拉最新 K → `saveKLineCacheResult` 更新 `last_bar_day`/payload → 随后开 K 时 TTL miss 或增量/全量得到含 **2026-09-07** 的序列。  
**不是** Modal 换了另一套组件；是 **共享 SQLite K 线缓存被扫描写新**。

---

## 6. 入口对比

| 入口 | 组件链 | 数据链 |
| --- | --- | --- |
| A 自选 → K | `stock.vue` → Modal → Chart | 同 |
| B 组合 → K | `PortfolioDashboard` → Modal → Chart | 同（+ 持仓 props） |
| C 机会 → K | `allStockList` / `WatchedOpportunities` / Home → Modal → Chart | 同 |

→ **同一套新鲜度/缓存行为**；自选复现即可代表主链。

---

## 7. 分类结论（必须项）

### 7.1 问题属于

| 代号 | 是否 |
| --- | --- |
| **A. 缓存问题** | **是（主）** — FE 内存 TTL + BE IDLE stale 短路 + LIVE 失败回落 stale |
| **B. 实时行情刷新问题** | **部分** — poll 存在但 **复用 getOrFetch**，被 FE 缓存短路；IDLE 停 poll |
| **C. 数据源问题** | **否（非主）** — 快照扫描能更新到 07，说明源与写入路径可用 |
| **D. 日期/交易日判断问题** | **是（主）** — **无**「最后一根 < 当前交易日则强制刷新」；IDLE 不区分「缺交易日」与「已最新」 |
| **E. 其他** | 前后端 LIVE 定义在节假日可能不一致（前端只看星期）；本次周一非主因 |

### 7.2 最小修复范围（只列文件，不改代码）

| 优先级 | 文件 | 建议方向（实现切片再做） |
| --- | --- | --- |
| P0 | `backend/data/eastmoney_kline_api.go` | IDLE/stale 返回前：若 `last_bar_day` < 当前交易日（或上海日），**禁止纯 stale 短路**，走增量/全量 |
| P0 | `backend/data/kline_cache.go` | 可选：抽出「latest 是否日历新鲜」辅助，供 Get/Stale 决策 |
| P0 | `frontend/src/utils/klineCache.js` 和/或 `StockLightweightKlineChart.vue` | LIVE poll / 打开时：**绕过内存缓存**或按最后 bar 日失效；避免 IDLE 长 TTL 跨进 LIVE 仍命中 |
| P1 | `frontend/src/utils/aShareSessionClock.js`（可选） | 与后端交易日历对齐节假日（非本 bug 必需） |

**明确不需要：** 改自选/组合/机会入口接线；新建 Modal；改 Snapshot 业务语义；为修新鲜度去改策略信号算法。

---

## 8. 禁止项遵守

| 禁止 | 遵守 |
| --- | --- |
| 修改代码 / 清缓存 / 改 TTL / 改 API / 修复 | 是 |

---

*Phase17.1 K 线盘中新鲜度只读审计完成。*
