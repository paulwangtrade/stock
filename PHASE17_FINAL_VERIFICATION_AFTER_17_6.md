# PHASE17 Final Verification（After 17.6）

**日期：** 2026-09-08  
**性质：** **只读终验**（不改代码 · 不修复 · 不提交）  
**范围：** C2 · C3 · C4 · 17.1 T-Suitability · 17.2 Data Truth · 17.5 Origin · **17.6 T Signal**  

**对照：** [PHASE17_FINAL_ACCEPTANCE_REPORT.md](./PHASE17_FINAL_ACCEPTANCE_REPORT.md) · [PHASE17_6_T_SIGNAL_LAYER_IMPLEMENTATION.md](./PHASE17_6_T_SIGNAL_LAYER_IMPLEMENTATION.md)

---

## 0. 总判

| 维度 | 结论 |
| --- | --- |
| **源码能力矩阵** | **PASS** — 七项均有后端/前端实现与菜单或页内入口 |
| **Go 编译 / 相关单测** | **PASS**（`papertrading` · `portfolio/readmodel` · `provenance`） |
| **FE 工具测** | **部分 PASS** — 17.6 / Health / Origin 通过；`portfolioQuoteDisplay` 断言失败（见 FINDING-F1） |
| **菜单 / 路由可达（代码层）** | **PASS** — 组合总览 + 持仓做T 已注册 |
| **已打包 exe 是否含 17.6 UI** | **FAIL / 过期** — `go-stock.exe` mtime **00:04:41**，早于 `HoldingTPanel.vue` **06:32:34** |
| **Git 冻结** | **仍不完整** — HEAD `6fee00c`；17.1/17.6 等工作区未全部入库 |

**一句话：** 源码与测试层面 Phase17（含 17.6）可验收；**当前磁盘 exe / 外部体验包不能代表含 17.6 的编译产物**，需重新 frontend + wails build 后才算「可运行二进制终验」。

---

## 1. 功能矩阵（源码 · 入口 · 测试）

| ID | 能力 | 关键代码 | 菜单 / 页面入口 | 本轮测试 | 状态 |
| --- | --- | --- | --- | --- | --- |
| **C2** | Explanation | `position_evaluation_explanation.go`；Exit 管道 Enrich | 组合页 → 健康 Drawer（信号/原因经 exit-evaluation） | `go test ./backend/papertrading/` ✅ | **PASS** |
| **C3** | HealthScore | `holding_health_score.go`；Exit 挂载 | 我的组合 → **健康 / 健康摘要** → `HoldingHealthDrawer` | 同上 ✅；`holdingHealthDisplay.test.mjs` ✅ | **PASS** |
| **C4** | Portfolio Display | `PortfolioDashboard.vue` + display util | 同上列/抽屉 | 代码存在；依赖 C3 映射 | **PASS**（入口） |
| **17.1** | T-Suitability | `holding_t_suitability.go`；Exit `t_suitability` | 组合 → **做 T** → `HoldingTSuitabilityDrawer` | papertrading ✅ | **PASS**（工作区） |
| **17.2** | Data Truth | `portfolio/readmodel` `quote_price` / `quote_timestamp` / `price_freshness` | 组合 → 估值价 / 行情价 / 持仓今日浮盈 | `go test ./backend/portfolio/readmodel/` ✅ | **PASS** |
| **17.5** | Position Origin | `PositionOriginDrawer.vue` | 组合 → 来源 / **为什么买入** | `positionOriginDisplay.test.mjs` ✅ | **PASS** |
| **17.6** | T Signal Layer | `holding_t_signal.go`；`holdingTSignal.js`；`HoldingTPanel.vue` | **我的组合 → 持仓做T** → T买/T卖观察 + ChartMarker | Go `BuildHoldingTSignal*` ✅；`holdingTSignal.test.mjs` ✅ | **PASS**（源码）；**exe 未含** |

### 1.1 用户路径（代码约定）

```text
菜单「我的组合」
  ├─ 组合总览  (#/portfolio)
  │    健康 / 做 T / 为什么买入 / 行情·今日浮盈
  └─ 持仓做T   (#/holding-t)
       T买观察 / T卖观察 / 5m markers（17.6）
```

路由：`router.js` → `portfolioDashboard` · `holdingT`  
菜单：`productMenu.js` 子项 + `App.vue` `activeKey` 含 `holdingT`

---

## 2. 编译确认

| 检查 | 结果 | 说明 |
| --- | --- | --- |
| `go build ./backend/papertrading/` | ✅ | 含 C2/C3/17.1/17.6 |
| 全量 `wails build`（本轮） | ⏭ 未重跑 | 只读终验；沿用既有产物 |
| `build/bin/go-stock.exe` | 存在 | LastWriteTime **2026-09-08 00:04:41** |
| `HoldingTPanel.vue` | 已改 | LastWriteTime **2026-09-08 06:32:34** |
| `release/go-stock-v0.1.0/go-stock.exe` | 存在 | 与 00:04 构建同源 → **不含 17.6 Panel 接线** |

**FINDING-B1：** 17.6 实现后 **未**再执行 `npm run build` + `wails build`；二进制终验不通过。

---

## 3. 测试确认

| 命令 / 范围 | 结果 |
| --- | --- |
| `go test ./backend/papertrading/ -count=1` | ✅ **ok** |
| `go test ./backend/portfolio/readmodel/ -count=1` | ✅ **ok** |
| `go test ./backend/portfolio/provenance/ -count=1` | ✅ **ok** |
| 定向 `-run Explanation\|HealthScore\|TSuitability\|BuildHoldingTSignal` | ✅ **ok** |
| `node holdingTSignal.test.mjs` | ✅ ok |
| `node holdingHealthDisplay.test.mjs` | ✅ ok |
| `node positionOriginDisplay.test.mjs` | ✅ ok |
| `node portfolioQuoteDisplay.test.mjs` | ❌ 断言 `'行情' !== 'Quote'`（FINDING-F1） |

未重跑全量 `./backend/...`（已知 strategy PositionSizer 债，与本矩阵无关）。

---

## 4. 菜单可达 · 页面可访问（静态核对）

| 入口 | 菜单注册 | 路由 | 组件 | 可达结论 |
| --- | --- | --- | --- | --- |
| 组合总览 | ✅ `productMenu` 子项 | `/portfolio` | `PortfolioDashboard.vue` | **可达** |
| 持仓做T | ✅ 子项「持仓做T」 | `/holding-t` | `HoldingTPanel.vue` | **可达（源码）** |
| 健康 Drawer | 页内列 | — | `HoldingHealthDrawer` | **可达** |
| 做 T 适宜性 Drawer | 页内列 | — | `HoldingTSuitabilityDrawer` | **可达** |
| 为什么买入 | 页内 | — | `PositionOriginDrawer` | **可达** |
| 17.6 T买/T卖观察 | HoldingT 页内卡片 | — | `buildHoldingTSignal` | **可达（源码）** |

本轮 **未**做 GUI 人工点击（只读静态 + 测试）；以代码接线为准。

---

## 5. Findings（只记录）

| ID | 严重度 | 说明 |
| --- | --- | --- |
| **FINDING-B1** | **高** | 当前 exe / 外部体验包 **早于 17.6**；要验证 17.6 UI 必须重新构建。 |
| **FINDING-G1** | 高（冻结） | Git HEAD 仍为 `6fee00c`；17.1/17.6 等仍在工作区，tag 不完整（与终验 Acceptance 一致）。 |
| **FINDING-F1** | 低 | `portfolioQuoteDisplay.test.mjs` 期望英文 `Quote`，实现为中文「行情」——展示层测债，**不影响** readmodel Go 测与组合列存在性。 |
| **FINDING-R1** | 信息 | 17.6 Panel 侧 freshness/Health/Suitability 仍为简化默认（实现报告已记）；与 Go 契约完整输入有差距。 |

---

## 6. 分项验收表

| 能力 | 编译（包） | 测试 | 菜单/入口 | 综合 |
| --- | --- | --- | --- | --- |
| C2 Explanation | ✅ | ✅ | ✅ | **PASS** |
| C3 HealthScore | ✅ | ✅ | ✅ | **PASS** |
| C4 Portfolio Display | ✅ | ⚠ FE quote 测债 | ✅ | **PASS** |
| 17.1 T-Suitability | ✅ | ✅ | ✅ | **PASS** |
| 17.2 Data Truth | ✅ | ✅ Go | ✅ | **PASS** |
| 17.5 Position Origin | ✅ | ✅ | ✅ | **PASS** |
| 17.6 T Signal | ✅ 源码 | ✅ | ✅ 源码菜单 | **PASS（源码） / FAIL（当前 exe）** |

---

## 7. 结论与停止

- **源码终验（含 17.6）：** 能力齐全，相关 Go/核心 FE 测通过，菜单与页面接线完整。  
- **可运行二进制终验：** **未通过**（exe 过期）。  
- **动作：** 按指令 **只出报告 · 不改代码 · 停止**。  

后续若需「可运行含 17.6」：另开任务执行 `frontend build` + `wails build`（及可选更新 `release/go-stock-v0.1.0`）——**本报告不执行**。

---

*PHASE17_FINAL_VERIFICATION_AFTER_17_6.md*
