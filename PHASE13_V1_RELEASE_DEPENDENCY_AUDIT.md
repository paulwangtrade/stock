# PHASE13-V.1 — Release Dependency Audit

> **切片：** Phase13-V.1 Step 1（只读审计）  
> **日期：** 2026-08-10  
> **对照：** `release/v0.1.0-beta` @ **`19cd8e6`** · tag `v0.1.0-beta` · Phase13-U / U.1  
> **目标：** 确认 B1/D1/D2 实际依赖树与 Beta **最小可编译闭包**  
> **本步：** **未**改代码 / **未** commit / **未** push / **未** tag

---

## 0. Verdict

| 项 | 结论 |
|----|------|
| B1（商业包） | tip **已齐**；**不**依赖四缺包 |
| D1（explain/risk/assistant） | tip **有源码**；**直接**缺 `strategysnapshot`、`marketstate` |
| D2（product API/UI） | tip **有源码**；经 D1 间接缺上述两包 |
| 预存缺口（base 起） | tip `tradeplans.go` **直接** import `approvegate`，并调用 **未入库** 的 `handleApprove/Freeze/Readiness` |
| 传递依赖 | `readiness` → `qualitygate`；`approvegate` → `readiness`+`qualitygate`；`marketstate` → `tradingcalendar`（**已在 tip**） |
| 建议提交 | **仅**五包 Go 源 + 三份 tip 已引用的 tradeplans handler；**排除** Phase10 脏树 / 日志 / 测试产物 |

**一句话：** Clean build 失败不只是「少四个商业相邻包」，而是 **D1 缺两包 + tip API/Strategy 预存的 Approve/Readiness 链未闭包**（含 `qualitygate` 与三份 handler 文件）。

---

## 1. Git 锚点

| 项 | 值 |
|----|-----|
| Branch | `release/v0.1.0-beta` |
| Tip | `19cd8e6` |
| Base | `6bfa2b5` |
| Commits | `bcb0656` B1 → `f6b5391` D1 → `441bd97` D2 → `19cd8e6` UTF-8 |
| Tip 树中四包 | **全部缺失**（`git ls-tree HEAD:backend/{approvegate,readiness,marketstate,strategysnapshot}` 空） |
| WT 状态 | 五包均为 `??`（含 `qualitygate`）；另有 `?? backend/api/tradeplans_{approve,freeze,readiness}.go` |

---

## 2. B1 / D1 / D2 依赖树

### 2.1 B1 — Commercial（`bcb0656`）

```text
featuregate
entitlement ──► featuregate
subscription ──► entitlement, user, featuregate
license（自洽）
usagemetrics ──► featuregate（+ tests entitlement）
user ──► featuregate
app_feature_gate*（壳）
```

| 对四缺包 | **无直接依赖** |
|----------|----------------|
| Broker/Gateway/Execution | **无** |

### 2.2 D1 — Product capabilities（`f6b5391`）

```text
strategyexplain ──► featuregate, entitlement, usagemetrics
                 └──► strategysnapshot     ❌ tip 缺失

riskreport ──► featuregate, entitlement, usagemetrics
           └──► marketstate              ❌ tip 缺失
           └──► papertrading（只读 Observation）

assistant ──► featuregate, entitlement, usagemetrics
          └──► riskreport, papertrading（只读）
          └──► strategysnapshot          ❌ tip 缺失
assistant/provider ──► assistant
```

### 2.3 D2 — Product API / Demo UI（`441bd97` + `19cd8e6`）

```text
api/product_capabilities.go ──► assistant, riskreport, strategyexplain,
                                 featuregate, entitlement, usagemetrics
                              （间接需要 strategysnapshot + marketstate）

api/product_commercial_demo.go ──► entitlement, featuregate
frontend CommercialDemo / CapabilityPanel / App·main 接线
```

### 2.4 Tip 上 **非 B1/D1/D2 引入**、但阻塞整仓 `wails build` 的链

（自 `6bfa2b5` 起已在 tip）

```text
backend/api/tradeplans.go
  import approvegate                         ❌ tip 缺失包
  ServeHTTP → handleApprove / handleFreeze / handleReadiness
              ❌ tip 缺失文件（仅 WT 有）

backend/strategy/morning_intent_materialize.go
  import readiness                           ❌ tip 缺失包

backend/api/tradeplans_materialize_morning.go
  （经 strategy 间接需要 readiness）
```

---

## 3. Direct vs Transitive（相对 tip 编译）

### 3.1 Direct（tip 源码 `import` 或未定义符号）

| 依赖 | 引用方（tip） | 性质 |
|------|---------------|------|
| **`strategysnapshot`** | `strategyexplain/*`, `assistant/{builder,service}.go` | D1 **直接** |
| **`marketstate`** | `riskreport/sources.go` | D1 **直接** |
| **`approvegate`** | `api/tradeplans.go` | tip API **直接** |
| **`readiness`** | `strategy/morning_intent_materialize.go` | tip Strategy **直接** |
| **`handleApprove/Freeze/Readiness`** | `tradeplans.go` 路由 | tip API **直接缺文件** |

### 3.2 Transitive（闭包必须一并入库）

| 依赖 | 被谁需要 | tip 已有？ |
|------|----------|:----------:|
| **`qualitygate`** | `readiness`；`approvegate`；WT `tradeplans_{approve,freeze,readiness}.go` | **否** |
| **`tradingcalendar`** | `marketstate` | **是** |
| `models` / `papertrading` / `data` / `logger` / `db` / `strategy` | 各包常规 | **是** |

```text
                    ┌─ strategysnapshot ◄── D1 explain/assistant
D1/D2 编译 ─────────┤
                    └─ marketstate ──► tradingcalendar ✓

整仓 wails 编译 ────┬─ approvegate ──► readiness ──► qualitygate
                    │              └──────────────► qualitygate
                    ├─ readiness ◄── strategy morning
                    └─ tradeplans_{approve,freeze,readiness}.go
                         ──► approvegate, readiness, qualitygate, …
```

### 3.3 工作区包自依赖（实现侧，供审查）

| 包 | import（go-stock） |
|----|-------------------|
| `strategysnapshot` | `models` |
| `marketstate` | `tradingcalendar` |
| `readiness` | `models`, `qualitygate` |
| `qualitygate` | `models` |
| `approvegate` | `data`, `db`, `logger`, `models`, `qualitygate`, `readiness`, `strategy` |

**边界：** 上述包文档声明只读 gate / 会话分类 / 快照；**不**作为本切片修改 Broker/Gateway/Execution/Fill/Settlement 的理由。`approvegate`→`strategy` 为既有 Approve/Freeze 协作，属 tip 已引用链。

---

## 4. 建议提交范围（V.1 Step 2）

### 4.1 必须纳入（Beta 运行 / clean build）

| 路径 | 理由 |
|------|------|
| `backend/strategysnapshot/*.go` | D1 直接 |
| `backend/marketstate/*.go` | D1 直接 |
| `backend/readiness/*.go` | tip strategy + 传递 |
| `backend/qualitygate/*.go` | readiness/approvegate 传递 |
| `backend/approvegate/*.go`（**仅 .go**） | tip tradeplans 直接 |
| `backend/api/tradeplans_approve.go` | tip 已路由、符号缺失 |
| `backend/api/tradeplans_freeze.go` | 同上 |
| `backend/api/tradeplans_readiness.go` | 同上 |

建议 **单 commit**（或「deps 五包」+「api handlers」两 commit），message 聚焦：*close beta compile dependency gap*。

### 4.2 禁止纳入

| 排除 | 原因 |
|------|------|
| `backend/approvegate/data/`、`logs/` | 运行产物 / 本地配置，非源码 |
| Phase10 大面积 dirty（observation UI、execution 改动等） | 非本闭包；未审查 |
| WT 对 `app_trading_preflight` / `app_market_state` / `main` Safety 等 diff | tip 当前 preflight **不** import marketstate；属增强非本最小闭包 |
| 三面板 ProductCapability 嵌入 | T/U.1 有意排除 |
| Broker / Gateway / Execution / Fill / Settlement 任何修改 | 硬禁止 |
| `git add .` | 脏树过大 |

### 4.3 非本切片（已在 tip）

`tradingcalendar`、B1/D1/D2 商业与产品包、papertrading 只读消费 —— **无需重提**。

---

## 5. 验证计划（Step 2 后）

| 步骤 | 要求 |
|------|------|
| Clean checkout | 新 worktree / 干净目录 @ 闭包 commit（**非**脏主工作区） |
| `frontend` `npm run build` | PASS |
| `wails build`（或等价 `go build` + 前端已构建） | PASS |
| Startup smoke | 空数据目录可起；不要求全交易 cron 绿 |
| Push / Tag | **禁止** |

---

## 6. 风险与说明

| 风险 | 说明 |
|------|------|
| Approve/Freeze handler 首次入库 | tip **早已**声明路由；补文件是修复「半截 API」，不是新开 Phase10 功能面 |
| `approvegate` 体积 / 测 | 含 acceptance 测试；可一并入库以保包完整，不扩 scope 到改 Execution |
| Tag `v0.1.0-beta` 仍指 `19cd8e6` | 闭包 commit 之后 tag **过时**；本切片 **不**移动 tag（另令） |

---

## 7. 验收（本审计步）

- [x] 确认 B1/D1/D2 实际依赖树  
- [x] 列出 direct / transitive  
- [x] 给出建议提交范围与排除项  
- [x] 产出 `PHASE13_V1_RELEASE_DEPENDENCY_AUDIT.md`  
- [x] Step 2：最小入库 + clean 验证 → 见 `PHASE13_V1_RELEASE_DEPENDENCY_CLOSURE_REPORT.md`（tip `f014ac7`）

---

## 8. Step 2 编译跟进（补丁范围）

首批五包 + handlers 入库后，clean `go build` 仍失败，额外直接依赖：

| 缺口 | 处理（最小） | 不纳入 |
|------|--------------|--------|
| `models.TradePlan/Item` 缺 Intent/Pricing 等字段 | **仅**补齐 struct 字段（与 WT 对齐 22 行级） | 不改 Execution 语义 |
| `riskreport/sources.go` → 未入库的 `papertrading.Build*Observation` | **降级** `FetchLiveSources`：仅 `marketstate`；observation 标记 `degraded_beta_tip` | **不**拉入 Phase10 Observation 大包 |

此为 Beta **可编译闭包**必要补丁，不是 Observation 功能完整化。

### 8.1 第二轮编译缺口（strategy / assistant）

| 缺口 | 处理 | 说明 |
|------|------|------|
| tip `morning_intent_materialize.go` 引用未入库的 `MorningOpenPriceFunc` 等 | 入库 `morning_price_materialize.go` + `morning_position_materialize.go`（及测试） | tip **已依赖**；补齐符号，非改写既有 strategy 算法文件 |
| `morning_position` → `tradingconfig` | 入库 `backend/tradingconfig`（Provider/LegacyAdapter） | 只读配置门面；**不**改 Broker/Gateway/Execution |
| `assistant` → `papertrading.ExecutionSummaryView` | 仅入库 **类型** `execution_summary_view.go` | **不**入库 Phase10 ExecutionReadService 大包 |
