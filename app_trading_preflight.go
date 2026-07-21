package main

import (
	"fmt"

	"go-stock/backend/db"
)

const (
	TradingStatusReady                = "READY"
	TradingStatusBlockedSchemaInvalid = "BLOCKED_SCHEMA_INVALID"
)

// TradingPreflightResult 是自动交易 cron 注册前的数据库门禁结果。
// 它不检查策略、Rank/Score、资金或订单规则。
type TradingPreflightResult struct {
	Status string                    `json:"status"`
	Ready  bool                      `json:"ready"`
	Schema db.SchemaValidationResult `json:"schema"`
	Reason string                    `json:"reason,omitempty"`
}

// TradingPreflightCheck 仅验证自动交易所需 schema；失败不阻止 App/UI/行情启动。
func TradingPreflightCheck() TradingPreflightResult {
	schema := validateApplicationSchema()
	result := TradingPreflightResult{
		Status: TradingStatusBlockedSchemaInvalid,
		Schema: schema,
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
