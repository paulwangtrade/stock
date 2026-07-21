# Phase5-D.1 Runtime Smoke Report

Date: 2026-07-21

Build:
- Path: `D:\stock\build\bin\go-stock.exe`
- File time: 2026-07-21 12:25:05
- Size: 98471936 bytes
- Embedded version/commit: empty (startup log `version:` / `commit:` blank)
- Confirmed Phase5-D binary behavior: Migration Registry + Schema Validation log lines present

Database:
- Path: `D:\stock\build\bin\data\stock.db`
- Pre-check backup: `build/bin/data/stock.db.phase5d1.pre.bak`
- Snapshots: `build/phase5d1_before.json`, `build/phase5d1_after.json`, `build/phase5d1_current.json`

Startup sessions verified:
1. First smoke start: 2026-07-21 12:52:27
2. Current process: PID 40368, StartTime 2026-07-21 12:56:35, Responding=True

---

## Migration Registry

status: PASS

current version: 2

applied migrations:

| version | name | status | checksum | appliedAt |
|---------|------|--------|----------|-----------|
| 1 | *(legacy empty name)* | applied | *(empty)* | 2026-07-20 23:08:35 +08:00 |
| 2 | `add_candidate_decision_id` | applied | `7f2656c5217b44b6ceb80a331dc201286d62b433f04f860d964c3a081d235137` | 2026-07-21 12:52:28 +08:00 |

Existing stock.db (first start after Phase5-D build):

```text
2026-07-21 12:52:28.480 schema migration registry: from=1 to=2 applied=[2] skipped=[1]
```

- v1 skipped (already applied)
- v2 applied successfully (missing `decision_id` repaired)
- No silent skip of required migration
- No continue-to-trade without registry log

Second start (idempotent):

```text
2026-07-21 12:56:35.645 schema migration registry: from=2 to=2 applied=[] skipped=[1 2]
```

Fresh database:
- Not exercised in this runtime pass (production uses existing stock.db).
- Fresh path covered by Phase5-D unit/golden tests only.

---

## Schema Validation

status: READY

```text
2026-07-21 12:52:28.490 startup schema validation READY version=2
2026-07-21 12:56:35.683 startup schema validation READY version=2
```

BLOCKED_SCHEMA_INVALID:
- Not observed in this runtime session (would require damaging schema; forbidden by this task).

---

## Trading Gate

cron registered: yes

READY path evidence:

```text
2026-07-21 12:52:40.622 paper open buy cron registered
  (9:20 plan / 9:25 prepare / 9:30 execute / reconcile 9:35+intraday)
2026-07-21 12:56:36.668 paper open buy cron registered
  (9:20 plan / 9:25 prepare / 9:30 execute / reconcile 9:35+intraday)
```

Notes:
- Production logs one aggregate line covering candidate/plan/execution/reconcile registration.
- No `TRADING_START_BLOCKED` in current session.
- BLOCKED → cron NOT registered path not runtime-reproduced (forbidden to invalidate schema).

Application continues: yes (`loading-progress: 100% 启动完成`, process Responding=True, `http://127.0.0.1:18888/` → 200)

---

## UI Smoke

Watchlist:
PASS
- `loading-progress: 70% 正在加载自选股...` then `100% 启动完成`
- `followed_stock` count stable at 74
- Watchlist price polling initialized

KLine:
PASS
- Multiple chart opens without panic (examples: `sz300408`, `sh603629`, `sh688813`, `sz000021`)
- Eastmoney EOF fallback → `腾讯 K 线回退成功 ... count=160` (and similar)

Database Health:
PASS (observability via logs/DB read-only; no dedicated UI page found)
- Migration status observable in startup logs + `schema_migrations`
- Schema READY version=2 observable
- Read-only queries succeed while app running

Manual copy checks (Action 文案 / Decision·Candidate 展示 / signal marker visuals):
- Not screenshot-verified by this agent session; functional open/load/KLine evidence only.

---

## Data Integrity

CandidatePool:
before: pools=1, items=30 (trade_date=2026-07-17, status=ready, source=strategy_run)
after: pools=1, items=30 (same sample id/date/status/source; rank/score samples unchanged)

Decision:
before: N/A (no persistent `quant_decisions` / Decision snapshot table in stock.db)
after: N/A
note: `candidate_pool_items.decision_id` column added by expected v2 migration; existing values remain NULL; no Decision snapshot rewrite observed

TradePlan:
before: plans=1, items=5 (id=1, trade_date=2026-07-17, status=failed)
after: plans=1, items=5 (unchanged)

Order:
before: paper_orders=0
after: paper_orders=0

Particular confirmations:
- No Candidate rebuild
- No Rank recalculation
- No Score mutation (sample ranks/scores identical)
- No Decision Snapshot rewrite

Structural-only change (expected Phase5-D repair, not business rewrite):
- `hasDecisionID` false → true
- migration version 1 → 2

---

## Result

PASS

Phase5-D.1 PASS criteria met:
- Migration Registry normal
- Schema READY
- UI functional (Watchlist load + KLine)
- Business row data unchanged
- Trading cron correctly registered under READY

---

## Issues

(no code changes)

1. Binary embedded `version` / `commit` strings are empty in startup logs.
2. No dedicated Database Health UI page; migration/schema status is log/DB-only.
3. Runtime BLOCKED_SCHEMA_INVALID → trading-cron-skip path was not re-executed in this smoke (schema sabotage forbidden); covered by Phase5-D unit tests only.
4. Cron registration is logged as one aggregate line, not three separate `candidate/plan/execution cron registered` messages.
5. QuantDecision is not persisted as a SQL table; Decision count cannot be compared as a DB metric.
6. Watchlist Action 文案 / Decision·Candidate 展示 / KLine signal marker visuals were not screenshot-confirmed; only functional logs verified.
