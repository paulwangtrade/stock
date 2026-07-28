# Phase7-D1 Step2 Implementation Report

> 任务：C-OBS-002 前端展示收口（名称空白兜底）  
> 约束：未改后端业务逻辑 / 未改 DB / 未 git add / 未 git commit

---

## 1. 修改文件

| 文件 | 变更 |
|---|---|
| `frontend/src/components/PaperTradingObservation.vue` | 调整持仓表「名称」列渲染规则 |

> `frontend/src/api/paperObservation.ts` 未修改（字段映射已正确）。

---

## 2. 修改原因

问题背景：
- API 层已做 `stockName` enrich，但在 lookup 未命中等场景下，`stockName` 仍可能为空。
- 页面此前直接绑定 `stockName`，空值会导致名称列显示空白。

本次改动目标：
- 仅在前端展示层做兜底，提升可读性，不改变任何业务逻辑。

---

## 3. 实现内容

在 `PaperTradingObservation.vue` 的 `positionColumns` 中，将「名称」列改为自定义 `render`：

- 优先显示：`stockName`
- 若为空：显示 `stockCode`
- 若两者都为空：显示 `—`

即满足规则：**禁止名称列空白**。

---

## 4. 验证方式

### 4.1 逻辑验证（代码级）

已确认名称列渲染逻辑为：
1. `stockName` 非空 → 显示名称  
2. `stockName` 为空且 `stockCode` 非空 → 显示代码  
3. 两者都空 → 显示 `—`

### 4.2 页面验收建议（手工）

在「模拟盘观察」页面检查三类数据行：
- 有名称行：显示原名称
- 空名称行：显示代码
- 异常空行：显示 `—`

---

## 5. 影响评估

| 维度 | 影响 |
|---|---|
| 交易链路 | **无影响** |
| 后端 API | 无影响 |
| DB / 表结构 | 无影响 |
| Fill / Broker / Settlement / Execution | 无影响 |
| T+1 计算规则 | 无影响 |
| TradePlan / Risk | 无影响 |

结论：本次为纯前端展示收口，属于低风险 UX 修正。

---

## 6. 状态

```text
[x] C-OBS-002 前端展示收口完成
[x] 保持 stockCode / stockName 字段不变
[x] 未修改后端业务逻辑
[x] 未修改数据库
[x] 未 git add
[x] 未 git commit
```
