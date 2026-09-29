# 参数预设目录与观察扫描（均线趋势 / 突破）实现报告

设置页「新建预设」从静默克隆改成模板弹窗。内置观察策略在加载时合并进列表。本刀只服务信号观察快照，不进入交易。

## 结论

| 项 | 结果 |
|---|---|
| 新建预设走弹窗：名称、模板、用途说明、可选复制来源 | **PASS**（`applyCreatePreset` + 设置页弹窗） |
| 系统扫描模板不新建第二条同 id，确认后选中已有内置并提示「已是系统内置」 | **PASS** |
| 冰点模板新建 `strategy-…`，写入 `usageNote` 与 `templateId`，并复制来源旋钮 | **PASS** |
| 内置不能删、不能改 id；选中时设置页和筛选页显示用途说明 | **PASS**（界面约束 + merge 锁名称/id） |
| `default` 已有旋钮在合并时保留，不会被目录覆盖 | **PASS** |
| `ext_ma_trend_v1` 快照标签 `MA_TREND` | **PASS**（goja） |
| `ext_breakout_v1` 快照标签 `BREAKOUT_N` | **PASS**（goja） |
| `default` / 空 strategy 忽略 `scanMode`，不产出上述标签 | **PASS** |
| 非 default 的快照删除键仍只匹配自己的 `strategy_id` | **PASS**（未改删除条件） |
| Research 默认阈值 60 能看见 `MA_TREND`（74）和 `BREAKOUT_N`（73） | **PASS** |
| `ext_vol_mom_v1` / `ext_ma_pullback_v1` / `ext_meanrev_watch_v1` 可选中看说明；JS 返回空命中；筛选页点生成快照提示「算法尚未接入」 | **PASS**（引擎未接；前端拦截扫描） |
| 实盘 / 全市场行情跑一遍 | **NEED REVIEW**（本环境未接行情） |
| 盈利或可交易 | **未声称**。这些是观察标签，不是买卖指令 |

## 新建预设

确认后的分支：

1. 模板是系统扫描（截面动量、均线趋势、突破，以及三个 planned）：不新增行，把当前预设切到该内置 id。
2. 模板是「冰点参数变体」：要求名称；从「复制参数自」（默认是默认参数预设）复制冰点旋钮；`templateId=ice_point`，`scanKind=ice`，`builtin=false`。
3. 名称为空：不创建。

内置 id：`default`、`ext_xsmom_v1`、`ext_ma_trend_v1`、`ext_breakout_v1`、`ext_vol_mom_v1`、`ext_ma_pullback_v1`、`ext_meanrev_watch_v1`。旧配置里缺哪条，加载时补上。已有 `default` 的 RSI 等旋钮保留，显示名锁回「默认参数预设」。

设置页帮助文案列出不在本页做的方向：Dual Thrust 日内、Qlib Alpha158、价值+质量基本面、跨资产双动量。

## 扫描规则

与截面动量同一条 `StartSignalScanSnapshot(session, signalParamsJson, strategyId, strategyName)`。`strategy_id` 决定 JS 分支。`default` 和空 id 永远走冰点。

- **均线趋势 V1**：最后一根收盘 > MA20，且 MA20 > MA60。样本不足、价格非法则跳过该股。标签 `MA_TREND`。可用 `maTrendShort` / `maTrendLong` 覆盖，短均线不小于长均线时退回 20/60。
- **突破观察 V1**：收盘 > 不含当日的近 N 日最高价，默认 N=20（唐奇安）。标签 `BREAKOUT_N`。`breakoutLookback` 或 `breakoutN` 可覆盖。
- **planned**（量价动量、均线回踩、均值回归）：JS 返回空列表，不回落到冰点标签。筛选页在发起扫描前提示「算法尚未接入」。

均线趋势和突破按股票独立判断，JS 分块仍是 400。只有截面动量继续一次吃完整份已准备名单。

## 测试

```text
node /tmp/verify-signal-presets.mjs
# 由 esbuild 打包 frontend/scripts/verify-signal-presets.mjs

go test ./backend/data/ -count=1 -run 'TestRunSignalScanBatchJS|TestSignalScanBatchSpan|TestCalcResearchSignalScore|TestPredictDirectionFromTag'
```

结果：预设脚本打印 `verify-signal-presets: ok`；`ok go-stock/backend/data`。

Bundle 由源码重建，不是手改：

```text
npx --yes esbuild@0.25.5 scripts/scansignals/scan-batch.ts --bundle --platform=neutral --format=iife --global-name=SignalScanBatch --outfile=backend/data/signal_scan_bundle.js
```

## 「冰」标签

默认冰点扫描在 RSI 处于冰点区、且最后一根没有更高优先级标签时，本来就会算出「冰」。先前快照白名单不含「冰」，所以快照和筛选多选都看不到。

现在「冰」进入快照白名单和筛选选项。设置页单独有「冰」折叠（冰点阈值、出冰回溯），数值仍写在 `common`，不换存储键。筛选多选的选项来自 `buildScreenSignalFilterOptions()`，包含「冰」。

## 未改

`BuildCandidatePool`、Trade Candidate、TradePlan、Approve/Freeze、Broker、Research 列表「只看最新一条快照」的过滤方式。桌面壳里的弹窗点击未在本环境跑通（Wails 应用，没有浏览器里的设置页）。
