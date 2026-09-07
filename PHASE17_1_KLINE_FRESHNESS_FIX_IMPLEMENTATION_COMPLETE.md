# PHASE17.1 K 线盘中新鲜度修复 — 实现完成

**日期：** 2026-09-07  
**依据：** [PHASE17_1_KLINE_ARCHITECTURE_AUDIT.md](./PHASE17_1_KLINE_ARCHITECTURE_AUDIT.md) · [PHASE17_1_KLINE_FRESHNESS_FIX_DESIGN.md](./PHASE17_1_KLINE_FRESHNESS_FIX_DESIGN.md)  
**性质：** 最小修复（**未改 API / 未改表结构 / 未改 TTL 常数 / 未改 Modal·Chart 架构 / 未改数据源**）

---

## 0. 结果

| 项 | 结果 |
| --- | --- |
| 后端日历 freshness 门闩 | **DONE** |
| `klineCacheGet` TTL+fresh | **DONE** |
| IDLE stale 短路禁止落后 last_bar | **DONE** |
| 前端最小日历失效 | **DONE**（防内存缓存盖住后端新数据） |
| `go test ./backend/data/ -run Kline\|KLineCache\|NormalizeKLine` | **PASS** |
| FE freshness smoke（node） | **PASS** |
| `npm run build` | **PASS** |

---

## 1. 修改文件

| 文件 | 变更 |
| --- | --- |
| `backend/data/kline_cache.go` | expected / fresh / grace；`klineCacheGetAt` |
| `backend/data/eastmoney_kline_api.go` | IDLE stale 仅 fresh 才短路 |
| `backend/data/kline_cache_test.go` | 场景 1–4 单测 |
| `frontend/src/utils/klineCache.js` | `feKlinePayloadCalendarFresh`；`getOrFetch` 命中后校验 |

未改：`StockKlineModal` / `StockLightweightKlineChart` / Wails 签名 / `kline_cache` 表 / TTL 秒数 / 东财主源。

---

## 2. 修改函数

### 后端 `kline_cache.go`

| 符号 | 作用 |
| --- | --- |
| `klineExpectedLatestBarDay(now)` | 非交易日 / &lt;09:30 → 上一交易日；交易日 ≥09:30 → 当日 |
| `klineLatestCalendarFresh(last, now)` | `normalize(last) >= normalize(expected)` |
| `klineLatestAllowStaleWithinGrace` | 拉取后 **60s** 宽限（防无当日 bar 时狂打；**非**改 TTL 常数表） |
| `klineCacheGet` → `klineCacheGetAt(..., now)` | latest：TTL 有效 **且**（fresh **或** grace）才返回；历史 end **跳过** freshness |

### 后端 `eastmoney_kline_api.go`

| 函数 | 修改点 |
| --- | --- |
| `GetKLineDataBefore` | IDLE + `GetStale`：仅 `klineLatestCalendarFresh(lastKLineDay(stale))` 为真才 `finalize`；否则落入增量/全量 |

### 前端 `klineCache.js`

| 符号 | 作用 |
| --- | --- |
| `feKlinePayloadCalendarFresh` | 周末/开盘前放行；工作日 ≥09:30 要求末 bar 日 ≥ 上海当日 |
| `getOrFetch` | 内存 hit 但不 fresh → delete key 并重新 fetch |

---

## 3. 实现逻辑（摘要）

```text
latest 读路径：
  TTL miss → 原逻辑
  TTL hit + historical end → 原样返回
  TTL hit + latest：
      last_bar_day（或 payload 末日）>= expected → 返回
      否则若 FetchedAt 在 60s 内 → 返回（宽限）
      否则 miss → 增量/全量 HTTP → save（写 last_bar_day）

IDLE 短路：
  仅当日历 fresh 才直接 stale
  last=09-04 且 now=09-07≥09:30 → 禁止短路
```

`expected` 复用 `tradingcalendar` + 上海时区；复用已有列 `last_bar_day`。

---

## 4. 测试结果

### Go（`go test ./backend/data/ -count=1 -run "Kline|KLineCache|NormalizeKLine"`）

**PASS**（含既有 cache/TTL/merge + 新增）：

| 场景 | 用例 | 结果 |
| --- | --- | --- |
| 1 | `last=09-04`，`now=2026-09-07 10:00` → `fresh=false` | **PASS** |
| 2 | 周末 expected=周五；周五 bar fresh | **PASS** |
| 3 | 历史 `end` 跳过 freshness，仍命中 | **PASS** |
| 4 | 交易日盘中 calendar-stale latest → `klineCacheGetAt` miss；GetStale 可读但不视为 fresh | **PASS** |
| 附加 | 午休 expected=当日；60s grace 可短暂返回落后包 | **PASS** |

### 前端

| 检查 | 结果 |
| --- | --- |
| node smoke：`feKlinePayloadCalendarFresh` Mon/Sat | **PASS** |
| `npm run build` | **PASS** |

说明：仓库无统一 `npm test` 脚本；以 Go 单测 + FE smoke + build 覆盖本切片。

---

## 5. 对公共组件入口的影响

| 入口 | 是否改组件 | 是否受益 |
| --- | --- | --- |
| StockKlineModal | **否** | **是**（经同一 API/缓存） |
| Portfolio K 线 | **否** | **是** |
| Opportunity / 选股 K 线 | **否** | **是** |
| Watchlist / 跟踪机会 K 线 | **否** | **是** |
| market 直嵌 Chart / HoldingT | **否** | **是**（同 Wails + 同 FE cache 工具） |

架构仍为：`StockKlineModal` → `StockLightweightKlineChart` → Wails → `kline_cache`；仅读写门闩变化。

---

## 6. 禁止项遵守

| 禁止 | 遵守 |
| --- | --- |
| 修改 API | 是 |
| 修改 / 新增缓存表 | 是 |
| 修改 TTL 常数档位 | 是（仅 freshness 用 60s grace 常量，未改 `klineCacheTTLAt` 表） |
| 修改 K 线组件架构 | 是 |
| 修改数据源 | 是 |
| 扩展需求 | 是 |

---

*Phase17.1 K 线盘中新鲜度修复实现完成。*
