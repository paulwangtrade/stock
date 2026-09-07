// Package shadow 提供 QuantDecision Go Shadow Producer（Phase2-A）。
//
// 边界（冻结）：
//   - 产出 models.QuantDecision（Schema v1, producer=go_engine）
//   - 仅 shadow candidate：不进入 UI / TradePlan / Execution / PaperBroker
//   - 不替换 js_legacy；交易链路仍走旧路径
//   - 可与 frontend quantDecisionCompare harness 做字段级 dual-run diff
package shadow
