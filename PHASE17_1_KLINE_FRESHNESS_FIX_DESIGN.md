# PHASE17.1 K 线盘中新鲜度 — 最小修复设计（只读）

**日期：** 2026-09-07  
**依据：** [PHASE17_1_KLINE_INTRADAY_FRESHNESS_AUDIT.md](./PHASE17_1_KLINE_INTRADAY_FRESHNESS_AUDIT.md)  
**性质：** **仅设计**（**未改代码 / 未改 API / 未改缓存实现 / 无实现片段**）  
**目标：** 用最小改动消除「交易日已切换但仍展示落后 last_bar（如停在上周五）」；保留 B.1 IDLE 省流量与历史回放行为。

---

## 0. 设计总判

| 项 | 结论 |
| --- | --- |
| 根因 | `end=latest` 路径：**TTL 命中**与 **IDLE stale 短路**均**不校验**「相对当前交易日历，最后一根是否已追上」 |
| 修复轴心 | 增加 **交易日 freshness** 判定；不新鲜则视为 cache miss，走既有增量/全量 HTTP |
| **推荐方案** | **A + 轻量 B**：仅后端决策；**复用已有** `kline_cache.last_bar_day` / payload 末 bar（**不改表、不改 API、不改 TTL 数值**） |
| 前端 C | **可选加固**，非首切片必需 |
| 明确不做 | 新 Wails API；清全库缓存；改 Snapshot；改 Modal 入口；改 LIVE 时段定义 |

---

## 1. 当前问题链（确认）

```text
GetStockEastMoneyKLine / Page
  → EastMoneyKLineApi.GetKLineDataBefore(end=latest|"")
       ① klineCacheGet          ← TTL 内直接返回（无日历新鲜度）
       ② IDLE + klineCacheGetStale ← 忽略 TTL，有 bars 即返回（B.1）
       ③ LIVE 增量 merge / 全量 HTTP
       → saveKLineCacheResult（写 payload + last_bar_day）
  → 前端 klineCache.getOrFetch
       ← 按写入时 LIVE5m / IDLE30m expiresAt；无 last_bar 校验
       ← refreshLatestPoll 仍走 getOrFetch（可被 FE 短路）
  → StockLightweightKlineChart 渲染末根 Day
```

| 层 | 文件 | 问题点 |
| --- | --- | --- |
| API 契约 | `app.go` → `GetStockEastMoneyKLine*` | **不变**；问题不在契约 |
| 后端决策 | `eastmoney_kline_api.go` `GetKLineDataBefore` | IDLE stale **无**「落后交易日」门闩 |
| 后端缓存读 | `kline_cache.go` `klineCacheGet` | TTL 命中即返回；**不看** `LastBarDay` |
| 后端元数据 | `KLineCacheRecord.LastBarDay` | **已有写入**；决策路径**未使用** |
| 前端缓存 | `klineCache.js` `getOrFetch` | 日历日仅在 key 的「上海日」；**不看** bars 末日 |
| 图表 | `StockLightweightKlineChart.vue` | poll/打开均依赖上述缓存 |

**因果对齐审计：** 午休/盘前 IDLE + 止于上交易日的 stale → 图表停在旧日；快照扫描写新 `last_bar_day` 后现象消失。

---

## 2. 交易日 freshness 判断设计

### 2.1 判断位置（唯一主闸）

| 优先级 | 位置 | 作用 |
| --- | --- | --- |
| **P0 主闸** | `GetKLineDataBefore` 在 **返回任何本地 latest 结果之前** | 统一覆盖 ① TTL 命中与 ② IDLE stale |
| **P0 辅助** | `kline_cache.go` 新增纯函数（见 §4） | 计算「期望末 bar 日」与「是否日历新鲜」；**不改 Put/表结构** |
| 可选 | `klineCacheGet` 对 `endKey==latest` 不新鲜返回 `nil` | 与主闸等价；二选一即可，避免重复逻辑分叉 |

**适用范围硬门：**

| 条件 | 是否做 freshness |
| --- | --- |
| `isLatestKLineEnd(end)`（`""` / `20500101` → `latest`） | **是** |
| 历史 `end`（固定日期回放/翻页 older） | **否** — 直接保持现逻辑 |
| HTTP 失败后的 fallback stale | **可保留返回 stale**（有总比无好）；但**禁止**在已知「日历落后」时走「故意不请求」的 IDLE 短路 |

### 2.2 期望末 bar 日 `expectedLastBarDay(now, klt)`

时区：**Asia/Shanghai**（复用 `chinaLocPrefer`）。  
日历：**`tradingcalendar.IsTradingDay` / `PrevTradingDay`**（与现 LIVE 门控同源）。

#### 日 K（`klt=101`）— 本 bug 主路径

记 `today = TruncateDay(now)`。

| 场景 | `expectedLastBarDay` | 理由 |
| --- | --- | --- |
| **非交易日**（周末/休市） | `PrevTradingDay(today)` | 期望停在上一交易日收盘；**允许 IDLE 长缓存** |
| **交易日 ∧ 墙钟 &lt; 09:30** | `PrevTradingDay(today)` | 开盘前通常尚无当日日 K；**避免无意义打点** |
| **交易日 ∧ 墙钟 ≥ 09:30** | `today`（`YYYY-MM-DD`） | 连续竞价已开始（含午休、盘后同日）：末根应至少追到**当日**（未完成日 K 也常出现当日 bar） |

> 午休（11:30–13:00）**不是**非交易日：仍属「交易日 ∧ ≥09:30」→ `expected=today`。这正是审计中 IDLE 短路停在上周五的修复点。

#### 分钟 K（`1/5/15/…`）

与日 K **同一 `expectedLastBarDay` 日历规则**（比的是末 bar 的**日期部分**）。  
LIVE 内实时性仍靠现有短 TTL + 增量；本闸只防「跨交易日仍停在旧日」。

#### 周/月（`102`/`103`）

首切片可 **暂用同一日历日比较**（`last_bar_day >= expected` 的字符串/日期比较）：周 K 末标签日通常 ≤ 本周最后交易日，一般不会误杀。若联调发现过严，实现切片再收窄为「仅 `101` + 分钟」——**设计默认先统一规则，降低分支**。

### 2.3 新鲜判定条件

```text
输入：lastBarDay（优先 row.LastBarDay；空则 lastKLineDay(bars)）, now, klt
expected = expectedLastBarDay(now, klt)

若 lastBarDay 为空 → 不新鲜（强制走网络路径）
若 parse(lastBarDay) >= parse(expected) → 新鲜
否则 → 不新鲜
```

**与 TTL 关系：**

- **新鲜 ∧ TTL 未过** → 可继续现有快速返回（IDLE 省流量保留）。
- **不新鲜** → **禁止** ①/② 短路；落入现有 ③ 增量（若支持）或全量 `fetchKLineDataBeforeNoCache`，成功后 `saveKLineCacheResult`。

**HTTP 后仍无当日 bar：**  
源尚未出当日日 K 时，拉回仍可能 `last_bar < expected`。允许：

- 写入新 `FetchedAt`（短/既有 TTL 重新起算）；
- **下一次**在 TTL 内若仍不新鲜 —— 设计选择：

| 选项 | 行为 | 推荐 |
| --- | --- | --- |
| X | TTL 内仍强制每次打开都 HTTP | 请求多 |
| **Y** | 「日历不新鲜」触发的刷新，写入后 **在短宽限内**（建议复用 LIVE 日 K **60s** 或固定 ≤2min）允许返回该结果，即使 `last < expected` | **推荐** — 避免开盘初期狂打；宽限过后再判 |

宽限实现落点：仅在「本次因 freshness miss 刚拉取并 save」后用 `FetchedAt`+短 TTL 自然覆盖即可（`klineCacheGet` 在 LIVE/短窗命中）；若仍走 IDLE 6h TTL 且 `last < expected`，主闸必须继续拒绝纯 stale——**宽限只通过 TTL 命中路径体现，且 TTL 在「交易日≥09:30」读路径应视为：不新鲜则不能用 6h IDLE 当新鲜**。更干净的说法：

> **日历不新鲜 ⇒ 永不走 IDLE `GetStale` 短路。**  
> TTL 命中路径：仅当 `新鲜` 才返回；若不新鲜直接 miss。  
> 开盘初期反复 miss：靠增量 limit 小请求 + 源侧无新 bar 时合并结果仍旧 —— 用 **短 TTL（60s）** 限制频率（`klineCachePut` 后 `FetchedAt=now`；下一次 `klineCacheGet`：若仍不新鲜则 **不要** 因 6h IDLE TTL 返回，即 Get 对 latest 增加 freshness，不新鲜当 miss；为限流可在「不新鲜但 FetchedAt 距今 &lt; 60s」时暂时返回 —— 这是唯一允许的「已知落后仍返回」窗口）。

### 2.4 不影响历史回放

| 机制 | 设计 |
| --- | --- |
| `endKey != latest` | **整段 freshness 跳过** |
| `GetStockEastMoneyKLinePage` / older 翻页 | 固定历史 end，行为不变 |
| 图表向前翻历史 | 不经 IDLE latest 短路逻辑变更 |

### 2.5 不影响非交易日

| 场景 | expected | 行为 |
| --- | --- | --- |
| 周六/日、休市 | 上一交易日 | `last_bar` 已是周五收盘 → **新鲜** → IDLE stale / 长 TTL **照常秒开** |
| B.1 周末验收 | 保持 | 不引入「必须等于日历今天」的误判 |

### 2.6 不增加不必要请求

| 约束 | 做法 |
| --- | --- |
| 仅 `latest` | 历史 end 零增量请求 |
| 非交易日已对齐 | 不强制 HTTP |
| 交易日 09:30 前 | expected=昨收，已对齐则不打 |
| 已追到当日 | IDLE/TTL 快速路径保留 |
| 开盘初期源无当日 bar | ≤60s 宽限，避免每个 tick 全量 |
| 增量优先 | 不新鲜时优先走现有 `supportsIncrementalLatestKLine` 短窗 merge，而非一律全量 |

---

## 3. 方案比较与推荐

### 方案 A — 仅后端 stale 判断

| | |
| --- | --- |
| 做法 | 只在 `GetKLineDataBefore` 的 IDLE stale 分支加日历判断 |
| 优点 | 改动面极小 |
| 缺点 | **漏掉** ① `klineCacheGet` TTL 命中（交易日午休 6h TTL 仍可直接返回落后 payload） |
| 评价 | **不完整**，不足以单独闭环审计场景 |

### 方案 B — 后端 + cache metadata

| | |
| --- | --- |
| 做法 | 用已有 `LastBarDay`（及 `lastKLineDay` 回退）做 freshness；TTL 命中与 IDLE stale **共用**判定函数 |
| 优点 | 元数据已在表中；**不改 schema、不改 API、不必改 TTL 常数**；一处函数两处门闩 |
| 缺点 | 需理清「宽限」与 expected 规则 |
| 评价 | **主推荐内核** |

### 方案 C — 后端 + 前端保护

| | |
| --- | --- |
| 做法 | 后端 B 之外：FE `getOrFetch` / poll 按末 bar 日失效，或 LIVE poll `force` 绕过内存缓存 |
| 优点 | 防「后端已新、FE 仍握 IDLE 30min 旧包」 |
| 缺点 | 双端日历易漂移（FE 无完整交易日历）；扩大改动面 |
| 评价 | **可选 P1 加固**，非最小闭环必需（后端修后，FE TTL 过期或换日 key 最终会跟；最糟是同日 IDLE→LIVE 同 key 最长 30min，可用极小 FE 补丁缩短） |

### 推荐：**最小方案 = B（含补全 A 的 TTL 门闩）**

```text
推荐切片（最小）：
  1) kline_cache.go：expectedLastBarDay + isKlineLatestCalendarFresh（+ 可选 shortGrace）
  2) GetKLineDataBefore /（或）klineCacheGet(latest)：不新鲜不得返回本地

可选下一刀（若验收仍见 FE 粘连）：
  3) klineCache.js 或 Chart：LIVE poll bypass / 末 bar 日失效
```

**不推荐**单独 A；**不推荐**首刀就上完整 C；**禁止**为修此问题改对外 API 或重建缓存表。

---

## 4. 修改文件清单（精确到函数 / 修改点）

> 下列为**拟改范围**，本文件**不含实现代码**。

### 4.1 必改（最小闭环）

#### 文件：`backend/data/kline_cache.go`

| 函数 | 修改点 |
| --- | --- |
| **新增** `klineExpectedLatestBarDay(now time.Time, klt string) string` | 按 §2.2 返回 `YYYY-MM-DD`；依赖 `tradingcalendar` + `chinaLocPrefer` |
| **新增** `klineLatestCalendarFresh(lastBarDay string, now time.Time, klt string) bool` | 按 §2.3；空 last → false |
| **可选新增** `klineLatestAllowStaleWithinGrace(fetchedAt time.Time, now time.Time) bool` | FetchedAt 起 60s 宽限（仅与「已知落后仍返回」联用） |
| `klineCacheGet` | 当 `endKey=="latest"`：解出 `LastBarDay`（或 bars 末日）后，若 **不新鲜** 且 **不在宽限**：`return nil`（当 miss）。**历史 endKey 不动** |
| `klineCacheGetStale` | **建议不改语义**（仍可忽略 TTL 读出）；由调用方决定是否采用。避免扫描等旁路误伤 |

#### 文件：`backend/data/eastmoney_kline_api.go`

| 函数 | 修改点 |
| --- | --- |
| `GetKLineDataBefore` | **①** `klineCacheGet` 返回后：若已在 Get 内做 freshness，可不再重复；若 Get 未改，则此处对 latest 再判，不新鲜则**忽略**该 cached，继续往下。**② IDLE 块**（`!klineMarketSessionLive` + `GetStale`）：仅当 `klineLatestCalendarFresh(...)` 为真才 `finalize(stale)`；否则**跳过短路**，进入增量/全量。**③** 增量失败 fallback stale：允许返回（降级），可选打 warn 日志。 |
| `finalizeKLines` / `saveKLineCacheResult` / `isLatestKLineEnd` | **不改契约**；save 已写 `LastBarDay`，保持 |

#### 测试（设计要求，实现切片时补）

| 文件 | 要点 |
| --- | --- |
| `backend/data/kline_cache_test.go`（或新建） | 表驱动：非交易日 / 开盘前 / 午休 / 盘后 / last=昨收 vs last=今日 |
| 既有 IDLE probe | 周末「已对齐」仍应快速本地命中 |

### 4.2 可选加固（非最小）

#### 文件：`frontend/src/utils/klineCache.js`

| 函数 | 修改点 |
| --- | --- |
| `getOrFetch` / `getCachedKline` | 可选：命中后若能解析末 bar 日且「上海交易日逻辑简化版」落后则当 miss；**或** |
| `resolveKlineCacheTtlMs` | 不改为「改 TTL 常数」；若只做 C，优先 **poll 绕过** 而非改 IDLE 30min 全局 |

#### 文件：`frontend/src/components/StockLightweightKlineChart.vue`

| 函数 | 修改点 |
| --- | --- |
| `refreshLatestPoll` | 可选：调用 fetcher 时跳过 FE cache（force miss） |
| `fetchEastMoneyKlineCached` | 可选增加 `force` 参数；默认 false 保持 B.1 |

### 4.3 明确不改

| 文件 / 项 | 原因 |
| --- | --- |
| Wails `GetStockEastMoneyKLine*` 签名 / 路由 | 禁止改 API |
| `kline_cache` 表结构 / 迁移 | metadata 已够用 |
| `klineCacheTTLAt` 各档秒数 | 非必须；靠 freshness 门闩 |
| `StockKlineModal` / 各入口页 | 共用链，入口无分裂 |
| Snapshot / 策略扫描 | 仅副作用写缓存，不作为修复依赖 |
| `aShareSessionClock.js` 节假日对齐 | P1 体验，非本 bug 最小集 |

---

## 5. 验收标准（实现后，本设计不执行）

| # | 场景 | 期望 |
| --- | --- | --- |
| 1 | 周一 ≥09:30（含午休），cache `last_bar=上周五` | **不得**纯 stale 秒回；应请求并尽量出现当日 bar（源有则有） |
| 2 | 周六，`last_bar=周五` | IDLE 本地命中，**无** latest HTTP（或可观测不强制） |
| 3 | 交易日 09:00，`last_bar=昨收` | 视为新鲜，允许 IDLE |
| 4 | 历史 end 翻页 | 行为与现网一致 |
| 5 | 组合/自选/机会开同一 Modal | 均受益（同 API） |
| 6 | B.1 周末秒开回归 | 不回退 |

---

## 6. 禁止项遵守（本文档）

| 禁止 | 遵守 |
| --- | --- |
| 修改代码 | 是 |
| 改 API | 是（设计亦不提议改契约） |
| 改缓存（实现/清库/改 TTL 落地） | 是 — 仅设计门闩与元数据**使用方式** |
| 实现代码 | 是 — 本文无 patch |

---

*Phase17.1 K 线盘中新鲜度最小修复设计结束。*
