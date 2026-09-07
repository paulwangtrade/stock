package data

import (
	"errors"
	"fmt"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// ErrInvalidManualTradeIntentTransition 非法手工 Intent 状态转换。
var ErrInvalidManualTradeIntentTransition = errors.New("invalid manual trade intent status transition")

// ManualTradeIntentStatusPatch 状态更新时可选回写执行关联字段（不触发下单）。
type ManualTradeIntentStatusPatch struct {
	ClientOrderID string
	OrderID       uint
	ErrorCode     string
	ErrorMessage  string
}

// ManualTradeIntentRepo 模拟盘人工交易意图持久化（不接 Execution / Paper）。
type ManualTradeIntentRepo struct{}

func NewManualTradeIntentRepo() *ManualTradeIntentRepo {
	return &ManualTradeIntentRepo{}
}

// EnsureManualTradeIntentTables 供测试或延迟初始化（主路径走 extended AutoMigrate）。
func EnsureManualTradeIntentTables() error {
	if db.Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return db.Dao.AutoMigrate(&models.ManualTradeIntent{})
}

// CreateManualTradeIntent 创建 draft 意图；强制 status=draft。
func (r *ManualTradeIntentRepo) CreateManualTradeIntent(intent *models.ManualTradeIntent) error {
	if db.Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if intent == nil {
		return fmt.Errorf("intent is nil")
	}
	if intent.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}
	if intent.Side == "" {
		intent.Side = "buy"
	}
	if intent.Side != "buy" && intent.Side != "sell" {
		return fmt.Errorf("side must be buy or sell")
	}
	if intent.ManualSource == "" {
		intent.ManualSource = models.ManualSourcePaperTradingPanel
	}
	if intent.OrderKind == "" {
		intent.OrderKind = models.ManualTradeOrderKindNormal
	}
	now := time.Now()
	intent.Status = models.ManualTradeIntentStatusDraft
	intent.CreatedAt = now
	intent.UpdatedAt = now
	intent.ClientOrderID = ""
	intent.OrderID = 0
	intent.ErrorCode = ""
	intent.ErrorMessage = ""
	return db.Dao.Create(intent).Error
}

// GetManualTradeIntent 按 ID 读取。
func (r *ManualTradeIntentRepo) GetManualTradeIntent(id uint) (*models.ManualTradeIntent, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	if id == 0 {
		return nil, fmt.Errorf("id is required")
	}
	var intent models.ManualTradeIntent
	if err := db.Dao.First(&intent, id).Error; err != nil {
		return nil, err
	}
	return &intent, nil
}

// UpdateManualTradeIntentStatus 按允许的状态机转换更新 status（及可选执行关联补丁）。
func (r *ManualTradeIntentRepo) UpdateManualTradeIntentStatus(id uint, toStatus string, patch *ManualTradeIntentStatusPatch) error {
	if db.Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if id == 0 {
		return fmt.Errorf("id is required")
	}
	toStatus = normalizeManualTradeIntentStatus(toStatus)
	if toStatus == "" {
		return fmt.Errorf("toStatus is required")
	}

	var intent models.ManualTradeIntent
	if err := db.Dao.First(&intent, id).Error; err != nil {
		return err
	}
	if !isAllowedManualTradeIntentTransition(intent.Status, toStatus) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidManualTradeIntentTransition, intent.Status, toStatus)
	}

	now := time.Now()
	updates := map[string]any{
		"status":     toStatus,
		"updated_at": now,
	}
	if patch != nil {
		if patch.ClientOrderID != "" {
			updates["client_order_id"] = patch.ClientOrderID
		}
		if patch.OrderID > 0 {
			updates["order_id"] = patch.OrderID
		}
		if patch.ErrorCode != "" {
			updates["error_code"] = patch.ErrorCode
		}
		if patch.ErrorMessage != "" {
			updates["error_message"] = patch.ErrorMessage
		}
	}
	return db.Dao.Model(&models.ManualTradeIntent{}).Where("id = ?", id).Updates(updates).Error
}

func normalizeManualTradeIntentStatus(s string) string {
	switch s {
	case models.ManualTradeIntentStatusDraft,
		models.ManualTradeIntentStatusConfirmed,
		models.ManualTradeIntentStatusSubmitted,
		models.ManualTradeIntentStatusRejected:
		return s
	default:
		return ""
	}
}

func isAllowedManualTradeIntentTransition(from, to string) bool {
	switch from {
	case models.ManualTradeIntentStatusDraft:
		return to == models.ManualTradeIntentStatusConfirmed
	case models.ManualTradeIntentStatusConfirmed:
		return to == models.ManualTradeIntentStatusSubmitted || to == models.ManualTradeIntentStatusRejected
	default:
		// submitted / rejected 为终态
		return false
	}
}
