# Phase6.7-F 股票筛选页面性能只读审计

> 日期：2026-07-26  
> 范围：只读分析「股票筛选」页 **生成快照** / **搜索** 为何可达数分钟～十几分钟  
> **禁止项已遵守**：未改策略 / 数据库 / 行情接口 / 计算逻辑 / 任何代码

---

## 0. 页面与入口定位

| 项 | 位置 |
|---|---|
| 菜单 | `frontend/src/App.vue` →「股票筛选」`stockScreen` |
| 壳组件 | `frontend/src/components/stockScreen.vue` |
| 实际逻辑 | `frontend/src/components/allStockList.vue`（`AllStockList`） |

---

## 1. 完整调用链

### 1.1 「生成快照」

```text
UI: allStockList.vue
  按钮「生成快照」@click → runBackendSnapshotScan()
    ↓ await IsSignalScanRunning()          // Wails: App.IsSignalScanRunning
    ↓ await RunSignalScanSnapshot(session, paramsJson, strategyId, strategyName)
         // Wails 同步绑定：前端 Promise 一直等到后端整次扫描结束
    ↓（期间 EventsOn signalScanProgress / signalScanDone 更新进度条）
    ↓ ParseSignalScanSnapshotPayload(snap)
    ↓ applySnapshotHistorySelection / 本地展示

API（Wails Bind，非 HTTP）:
  app_signal_scan.go
    App.RunSignalScanSnapshot
      → EnsureMigrationLevel(extended)
      → data.NewSignalScanApi().RunFullMarketSnapshot(..., onProgress)
      → EventsEmit("signalScanDone")

Service:
  backend/data/signal_scan_api.go
    RunFullMarketSnapshot
      ① fetchAllMarketStocks
           → StockDataApi.GetAllStocks(page,500,...) 循环翻页
           → fetchXuanguStocks → 东财选股 HTTP
      ② buildIndexCloseMap
           → EastMoneyKLineApi.GetKLineData("000001.SH", 日K, 120)
      ③ prepareStockBars × 全市场（并发上限 32）
           → 每只股票 GetKLineData(code, "101", 120 根)
           → 本地 kline_cache（TTL 见下）
      ④ RunSignalScanBatchJS（每批 400 只，顺序批次）
           → goja 嵌入 signal_scan_bundle.js（与前端 icePointSignals 同源）
      ⑤ 组装 SignalScanResultPayload → JSON
      ⑥ DB：删同日/时段/策略旧快照 → Create(signal_scan_snapshots)

返回:
  SignalScanSnapshot（含 ResultJSON、DurationMs、HitTotal…）
  前端 await 结束后再解析并切到 snapshot 数据源
```

常量（源码硬编码）：

| 常量 | 值 | 含义 |
|---|---|---|
| `signalScanFetchPageSize` | 500 | 名单分页 |
| `signalScanKlineBars` | 120 | 每只日 K 根数 |
| `signalScanKlineConcurrency` | 32 | K 线拉取并发 |
| `signalScanJSChunkSize` | 400 | JS 信号批大小 |

### 1.2 「搜索」

搜索按钮 → `handleSearch()` → `runScreenFiltersNow()` → 分支：

#### 路径 A：已在快照模式（`signalDataSource === 'snapshot'`）

```text
本地 applySnapshotPayload（内存过滤）
→ 几乎无网络；秒级或更快
```

#### 路径 B：勾选了信号标签（`hasSignalFilter` = 强/趋/转/突/弹/买…）

```text
loadStocks → loadWithSignalFilter()   // 前端同步长任务
  ① fetchAllStocksMatchingFilters
       → 循环 await GetAllStocks(page,500, keyword, industry, …)
       → 东财选股全量翻页（可至数千只）
  ② scanRowsLastBarSignals(all, concurrency=24)
       → 前端 watchlistSignalScan.js
       → 每只股票 await GetStockEastMoneyKLine(...)  // 再走 Wails→Go→东财 K 线
       → summarizeBuySignal（浏览器侧 JS 指标/策略）
  ③ 按信号标签本地过滤 → 表格展示

前端 tableLoading / signalScanLoading 全程为 true，UI 等待整次完成。
```

#### 路径 C：无信号标签（普通关键字/行业搜索）

```text
GetAllStocks(当前页, pageSize, keyword, industry, …)   // 单页选股
→ 表格先展示
→ scanPageSignals(当前页 rows)  // 并发 12，仅当前页 K 线+信号
→ 相对全市场轻量，但仍可能数秒～数十秒（视 pageSize / 网络）
```

关键字联想（输入框 `@update`）另走：

```text
GetAllStockInfoList({ searchKeyWord })  // 本地 all_stock_info 表，通常很快
```

**结论：** 用户体感「搜索很慢」多数不是 keyword 本身，而是 **勾选信号筛选后的实时全量扫描（路径 B）**，或 **生成快照（§1.1）**。

---

## 2. 耗时分析（估算 + 实测）

### 2.1 历史实测：「生成快照」`duration_ms`（运行库）

来源：`build/bin/data/stock.db` → `signal_scan_snapshots`

| id | trade_date | scanned | hit | duration_ms | 约等于 |
|---|---|---|---|---|---|
| 4 | 2026-07-24 | 5445 | 126 | **2,400,155** | **≈40.0 分钟** |
| 3 | 2026-07-22 | 5444 | 54 | **1,930,369** | **≈32.2 分钟** |
| 2 | 2026-07-20 | 5442 | 56 | **1,205,806** | **≈20.1 分钟** |
| 1 | 2026-07-17 | 5443 | 83 | **1,507,088** | **≈25.1 分钟** |

与用户「数分钟甚至十几分钟」完全吻合；冷启动/限流时可达 **20～40 分钟**。

### 2.2 分项估算（全市场 ≈5445 只）

| 阶段 | 机制 | 量级估算 | 占主导？ |
|---|---|---|---|
| **股票列表读取** | 东财选股 `xuangu/list`，500/页 ≈11 页 | 约 **5～30 s**（视网络） | 次要 |
| **行情获取** | 选股结果已带 NEW_PRICE 等字段；快照不另拉全市场 tick | **含在列表 HTTP** | 次要 |
| **K 线读取** | 每只 1 次日 K×120；并发 32；东财 `push2his` HTTP | **主导：十余分钟～数十分钟** | **是** |
| **指标/策略评分** | goja `RunSignalScanBatchJS`，约 5445/400 ≈14 批顺序 | 约 **数十秒～数分钟**（CPU） | 中等 |
| **数据库** | Delete + Create 1 行（ResultJSON ~25～60KB） | **&lt;1 s** | 可忽略 |
| **指数 K 线** | 上证 1 次 | **&lt;1 s** | 可忽略 |

粗算 K 线主导：

```text
5445 / 32 ≈ 170 轮
若单次 HTTP 有效耗时 1～3 s（含 TLS/限流/失败重试）
→ 170 × (1～3) ≈ 3～8.5 分钟（理想）
实测 20～40 分钟 → 有效 RTT 更高，或大量失败/慢响应/串行化放大
```

### 2.3 「搜索」路径 B（勾选信号）估算

| 阶段 | 估算 |
|---|---|
| 全量 GetAllStocks 翻页 | 与快照名单类似，**数秒～数十秒** |
| 前端每只 `GetStockEastMoneyKLine`（并发 24） | **与后端快照同量级瓶颈**；另加 **Wails 每调用一次跨界** |
| 浏览器侧 `summarizeBuySignal` | 相对网络次要 |
| **合计** | 行业缩小后可为数分钟；**全市场信号搜索可达十几分钟+** |

路径 C（无信号标签）：通常 **秒级～一分钟内**（单页 + 页内扫描）。

### 2.4 缓存实际作用

| 缓存 | TTL / 范围 | 对一次全市场扫描 |
|---|---|---|
| 后端 `kline_cache`（日 K latest） | **60 秒** | 首次全市场几乎全是 miss；扫描本身远超 60s，**跨扫描复用弱** |
| 前端 `dailyBarsCache` | 5 分钟 / 最多 8000 key | 同会话二次搜索有帮助；**首次全量仍冷** |
| 信号快照表 | 持久化 | **读历史快照很快**；「生成」仍全量重算 |

---

## 3. 问题确认清单

| 编号 | 现象 | 判定 | 说明 |
|---|---|---|---|
| **A** | 用户请求触发全市场实时计算 | **是（快照 / 信号搜索）** | `RunFullMarketSnapshot` / `loadWithSignalFilter` 对数千只现算 |
| **B** | 重复行情/K 线请求 | **部分是** | 名单 HTTP 与每只 K 线分离；同只股票在「快照」与「前端实时扫描」可各拉一遍；页内扫描与全量扫描也会重复 |
| **C** | 没有缓存 | **弱缓存 ≠ 无** | 有 kline_cache / 前端 Map，但 latest 日 K TTL=60s，**无法支撑「预计算结果」产品形态**；无「信号结果缓存」供搜索直查 |
| **D** | 同步阻塞计算 | **是** | Go 内 `wg.Wait()` + 顺序 JS 批；Wails `await RunSignalScanSnapshot`；前端 `await scanRowsLastBarSignals` |
| **E** | 前端等待后台任务完成 | **是** | 「生成快照」虽 emit 进度，但 **按钮逻辑仍 await 整次返回**；非「提交任务→稍后查结果」 |

---

## 4. 与专业量化系统的架构差异

### 当前（交互触发）

```text
点击「生成快照」/「信号搜索」
  → 当场拉全市场名单
  → 当场拉数千只日 K
  → 当场算指标/信号
  → 返回（或落一张快照 JSON）
  → UI 全程 loading
```

痛点：把 **批处理 ETL** 塞进 **交互请求**；网络扇出 × 全市场 = 分钟～十分钟级延迟。

### 推荐（流水线 + 查询）

```text
行情/K 线采集（定时或盘后）
  ↓
后台计算任务（信号/评分，可增量）
  ↓
结果缓存 / 快照表 / 列存索引
  ↓
页面只做：筛选条件查询 + 分页展示
  （「生成快照」= 触发/查看任务状态，而非同步算完）
```

| 维度 | 当前 | 推荐 |
|---|---|---|
| 触发 | UI 点击同步算 | 调度/手动触发异步 Job |
| 数据 | 点一下现拉东财 | 本地 K 线库预热 |
| 计算 | 请求路径内 goja/浏览器 JS | Worker 池离线算 |
| 查询 | 算完才有表 | 秒级读快照/索引 |
| UX | 进度条 + 长时间卡死感 | 任务队列 + 历史可选 |

现有 `signal_scan_snapshots` 已具备「结果缓存」雏形，但 **写入仍绑定在同步全量扫描完成之后**。

---

## 5. 优化路线（仅建议，本阶段不实施）

### P0 — 不改架构即可缓解

1. **默认引导用历史快照筛选**，避免无意识点「生成快照」/勾满信号后点搜索。  
2. **搜索前强制行业/关键字缩小宇宙**（路径 B 候选从 5k → 几百）。  
3. **UI 文案**：标明「全市场扫描预计 20～40 分钟」；禁用重复点击（已有 `IsSignalScanRunning`）。  
4. **盘后一次性生成快照**，盘中只读 snapshot（页面已有「点击生成盘后快照，系统不会自动扫描」提示，可强化）。  
5. **提高日 K latest 缓存 TTL（盘后场景）** 或扫描期间禁用 TTL 驱逐——仍属小改，但可缩短二次扫描（需后续改码阶段评估）。

### P1 — 缓存 / 异步任务

1. **异步 Job**：`RunSignalScanSnapshot` 立即返回 `job_id`；进度事件保留；完成后写快照；前端轮询/订阅。  
2. **本地日 K 预热表**（按交易日批量入库），扫描只读本地，避免数千次东财 HTTP。  
3. **信号结果表**（code × trade_date × strategy → tag/score），搜索变 SQL/内存索引。  
4. **前端实时扫描限流**：有信号标签时禁止无行业全市场；或改为「仅当前页/自选」。

### P2 — 重构计算流水线

1. 采集 → 标准化 K 线仓 → 因子/信号 Worker → 物化视图。  
2. 策略计算迁出「请求线程」与「Wails UI 线程」。  
3. goja 单 VM 顺序批 → 多进程/原生 Go 指标（若需进一步压 CPU）。  
4. 与专业系统对齐：**页面永远查询，不算全市场**。

---

## 6. 根因一句话

> **「生成快照」与「勾选信号后的搜索」都会在用户点击时对约 5400 只 A 股逐只拉取日 K 并计算信号；K 线 HTTP 扇出是主耗时（实测快照 20～40 分钟），且前端用 `await` 同步等待整次完成，因此体感为数分钟到十几分钟。**

---

## 7. 证据索引（只读）

| 证据 | 位置 |
|---|---|
| 生成快照 UI | `allStockList.vue` → `runBackendSnapshotScan` |
| 搜索 UI | `handleSearch` → `runScreenFiltersNow` → `loadStocks` / `loadWithSignalFilter` |
| Wails API | `RunSignalScanSnapshot` / `GetAllStocks` / `GetStockEastMoneyKLine` |
| 全市场扫描 | `backend/data/signal_scan_api.go` `RunFullMarketSnapshot` |
| JS 批算 | `backend/data/signal_scan_runner.go` |
| 前端逐股 K 线 | `frontend/src/utils/watchlistSignalScan.js` `fetchDailyBars` |
| K 线 TTL=60s | `backend/data/kline_cache.go` `klineCacheTTL` |
| 实测 DurationMs | `signal_scan_snapshots` id=1..4（见 §2.1） |

---

**本报告结束。未修改任何代码。**
