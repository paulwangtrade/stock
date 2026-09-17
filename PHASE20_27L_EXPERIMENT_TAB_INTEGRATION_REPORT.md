# PHASE20-27L Experiment Tab Minimal Integration Report

## Goal

将已有 `ResearchExperimentFoundationPanel` 以最小范围挂载到 `researchIndex.vue`。

## Scope (allowed only)

1. 新增 `ResearchExperimentFoundationPanel` import  
2. 新增「研究实验」Tab 入口  
3. Tab 内容渲染已有 Panel  

## Pre-commit checklist

### Modified files (intended)

| File | Change |
|------|--------|
| `frontend/src/components/researchIndex.vue` | +1 import, +1 tab pane, +1 panel render |
| `PHASE20_27L_EXPERIMENT_TAB_INTEGRATION_REPORT.md` | this report |

### Diff summary (`researchIndex.vue`)

```diff
+const ResearchExperimentFoundationPanel = defineAsyncComponent(() => import('./ResearchExperimentFoundationPanel.vue'))

+      <n-tab-pane name="研究实验" display-directive="if">
+        <ResearchExperimentFoundationPanel />
+      </n-tab-pane>
```

- Insert location: import after `PaperTradingPanel`; tab after「模拟盘」、before「定时任务」.
- No layout refactor, no other tab edits, no deletions, no whole-file reformat.
- Line delta: **+4 / -0**.

### Confirm NOT included

| Area | Touched? |
|------|----------|
| Proposal | No |
| Strategy Version | No |
| Execution | No |
| Backtest | No |
| Experiment Foundation (`ResearchExperimentFoundationPanel.vue` 本体) | No（未改；见下方 blocker） |

## Blocker: Foundation Panel missing on remote

Expected existing file:

`frontend/src/components/ResearchExperimentFoundationPanel.vue`

Search results on this checkout (`origin/dev`, also checked `origin/main` / `origin/release/v0.1.0-beta`):

- **No** `ResearchExperimentFoundationPanel` symbol
- **No** `*Experiment*` Vue component under `frontend/`
- **No** Phase20 docs / prior 20-27* artifacts in git history on remote branches

Constraint **「不修改 Experiment Foundation」** observed: this phase did **not** create or edit the Foundation panel.

## Verification

### `npm run build` (frontend)

```text
cd frontend && npm run build
→ FAIL
Could not resolve "./ResearchExperimentFoundationPanel.vue"
from "src/components/researchIndex.vue"
```

Root cause: missing Foundation panel module (blocker above), not the tab-wiring diff itself.

### Frontend tests

`npm run test:quant` not re-run as a gate for this change: the only code edit is async tab mount; build already fails on unresolved import before tests would add signal.

## Constraints compliance

| Constraint | Status |
|------------|--------|
| 不修改 Experiment Foundation | Yes |
| 不修改 Proposal / Strategy Version / Execution / Backtest | Yes |
| 不整理 researchIndex.vue 其他 WIP | Yes |
| 不重构页面布局 / 不改其他 Tab / 不删代码 / 不格式化整文件 | Yes |
| 仅允许三处挂载改动 | Yes（+ report） |

## Unblock next step

Bring prior Phase20 Experiment Foundation artifact into the same tree (merge/push local `ResearchExperimentFoundationPanel.vue`), then re-run:

```bash
cd frontend
npm run build
npm run test:quant
```

Tab wiring in `researchIndex.vue` can remain as-is once the panel file is present.

## Verdict

**Tab integration wiring: DONE (minimal).**  
**End-to-end green build: BLOCKED** until `ResearchExperimentFoundationPanel.vue` exists in-repo from prior Phase20 work.
