# Phase7-D1 Step3 Implementation Report

> 任务：C-OBS-003 T+1 状态展示优化（仅展示层）  
> 约束：未改交易逻辑 / 未改 DB / 未 git add / 未 git commit

---

## 1. 修改文件

| 文件 | 变更 |
|---|---|
| `frontend/src/components/PaperTradingObservation.vue` | 增加 T+1 状态文案、标签与 tooltip 展示 |

> 未修改 `frontend/src/api/paperObservation.ts`（字段映射已满足需求）。

---

## 2. 修改原因

背景问题（C-OBS-003）：
- 当 `availableVolume=0` 且存在 `lockedVolume` 时，用户只能看到数量，不够直观理解“为何不可卖”。

目标：
- 在不改变任何计算与交易规则的前提下，明确传达：  
  **“T+1锁定，下一交易日可卖”**

---

## 3. 实现内容（仅 UI 展示）

### 3.1 行级展示（持仓表 `T+1冻结` 列）

- 保留原有字段使用：`availableVolume` / `lockedVolume` / `t1Locked`
- 新增状态判断（仅用于展示）：
  - `availableVolume <= 0 && lockedVolume > 0 && t1Locked`  
    → 标签显示 `T+1锁定`
  - 其他 `t1Locked` 场景  
    → 标签显示 `部分T+1锁定`
- 新增 tooltip：
  - 全锁定：`T+1锁定，下一交易日可卖`
  - 部分锁定：`存在T+1冻结数量，下一交易日逐步可卖`

### 3.2 页面级提示

- 新增计算属性 `hasFullyLockedT1`（仅前端判断）
- 当页面存在“全锁定”持仓时，在持仓区显示 warning tag：
  - `T+1锁定，下一交易日可卖`

---

## 4. 验证方式

### 4.1 代码级复核

已确认本次改动仅涉及：
- Naive UI 展示组件（`NTooltip` / `NTag`）
- 前端条件渲染与文案

未触及：
- API 入参/出参契约
- 任意后端计算逻辑

### 4.2 页面验收建议（手工）

在“模拟盘观察”持仓表验证三类样本：
1. `availableVolume=0` 且 `lockedVolume>0`  
   - 显示 `T+1锁定` 标签  
   - tooltip 显示“下一交易日可卖”  
   - 页面上方出现 warning 提示
2. `t1Locked=true` 但非全锁定  
   - 显示 `部分T+1锁定` 标签
3. `t1Locked=false`  
   - 不显示锁定标签，仅显示数量

---

## 5. 影响评估

| 维度 | 影响 |
|---|---|
| 交易链路 | 无影响 |
| `available_volume` / `locked_volume` 计算 | 无影响 |
| Settlement / `SettleNewTradingDay` | 无影响 |
| Broker / Fill / Execution | 无影响 |
| TradePlan / Risk | 无影响 |
| 数据库字段 / migration | 无影响 |

结论：本次为纯前端可读性优化，符合 Phase7-D1 Step3 约束。

---

## 6. 状态

```text
[x] C-OBS-003 展示优化完成
[x] 使用既有字段 availableVolume / lockedVolume / t1Locked
[x] 明确显示 “T+1锁定，下一交易日可卖”
[x] 未改后端逻辑/数据库/交易规则
[x] 未 git add
[x] 未 git commit
```
