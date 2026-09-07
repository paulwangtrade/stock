package main

import (
	"fmt"

	"go-stock/backend/database"
	"go-stock/backend/db"
	"go-stock/backend/marketstate"
)

const (
	TradingStatusReady                = "READY"
	TradingStatusBlockedSchemaInvalid = "BLOCKED_SCHEMA_INVALID"
	TradingStatusBlockedDBIntegrity   = "BLOCKED_DB_INTEGRITY"
)

// TradingPreflightResult 是自动交易 cron 注册前的数据库门禁结果。
// 它不检查策略、Rank/Score、资金或订单规则。
// Phase11-B: Market 为只读会话快照，不单独阻断 Ready（Ready 仍仅 DB/schema）。
type TradingPreflightResult struct {
	Status string                    `json:"status"`
	Ready  bool                      `json:"ready"`
	Schema db.SchemaValidationResult `json:"schema"`
	Reason string                    `json:"reason,omitempty"`
	Market marketstate.Snapshot      `json:"market"`
}

// TradingPreflightCheck 验证 DB 完整性 + 自动交易所需 schema；失败不阻止 App/UI/行情启动。
func TradingPreflightCheck() TradingPreflightResult {
	market := marketstate.SnapshotNow()
	if !database.IsTradingSafe() {
		safety := database.LastResult()
		reason := stringsOr(safety.Error, "database integrity check failed")
		if safety.RecoveryHint != "" {
			reason = reason + " | " + safety.RecoveryHint
		}
		return TradingPreflightResult{
			Status: TradingStatusBlockedDBIntegrity,
			Ready:  false,
			Reason: reason,
			Market: market,
		}
	}
	schema := validateApplicationSchema()
	result := TradingPreflightResult{
		Status: TradingStatusBlockedSchemaInvalid,
		Schema: schema,
		Market: market,
	}
	if schema.Ready() {
		result.Status = TradingStatusReady
		result.Ready = true
		return result
	}
	result.Reason = fmt.Sprintf(
		"schema invalid: version=%d/%d missingTables=%v missingColumns=%v missingIndexes=%v errors=%v",
		schema.CurrentVersion,
		schema.RequiredVersion,
		schema.MissingTables,
		schema.MissingColumns,
		schema.MissingIndexes,
		schema.Errors,
	)
	return result
}

func stringsOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
