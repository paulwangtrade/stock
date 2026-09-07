package data

import (
	"errors"
	"fmt"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// ErrInvalidResearchTradeIntentTransition 非法 Intent 状态转换。
var ErrInvalidResearchTradeIntentTransition = errors.New("invalid research trade intent status transition")

// ResearchTradeIntentStatusPatch 状态更新时可选回写执行关联字段（不触发下单）。
type ResearchTradeIntentStatusPatch struct {
	ClientOrderID string
	OrderID       uint
	ErrorCode     string
	ErrorMessage  string
}

// ResearchTradeIntentRepo 研究交易意图持久化（不接 Execution / Paper）。
type ResearchTradeIntentRepo struct{}

func NewResearchTradeIntentRepo() *ResearchTradeIntentRepo {
	return &ResearchTradeIntentRepo{}
}

// EnsureResearchTradeIntentTables 供测试或延迟初始化（主路径走 extended AutoMigrate）。
func EnsureResearchTradeIntentTables() error {
	if db.Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return db.Dao.AutoMigrate(&models.ResearchTradeIntent{})
}

// CreateResearchTradeIntent 创建 draft 意图；强制 status=draft。
func (r *ResearchTradeIntentRepo) CreateResearchTradeIntent(intent *models.ResearchTradeIntent) error {
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
	if intent.ResearchSource == "" {
		intent.ResearchSource = models.ResearchSourceSignalScanSnapshot
	}
	now := time.Now()
	intent.Status = models.ResearchTradeIntentStatusDraft
	intent.CreatedAt = now
	intent.UpdatedAt = now
	// 创建时清空执行关联，避免误带入
	intent.ClientOrderID = ""
	intent.OrderID = 0
	intent.ErrorCode = ""
	intent.ErrorMessage = ""
	return db.Dao.Create(intent).Error
}

// GetResearchTradeIntent 按 ID 读取。
func (r *ResearchTradeIntentRepo) GetResearchTradeIntent(id uint) (*models.ResearchTradeIntent, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	if id == 0 {
		return nil, fmt.Errorf("id is required")
	}
	var intent models.ResearchTradeIntent
	if err := db.Dao.First(&intent, id).Error; err != nil {
		return nil, err
	}
	return &intent, nil
}

// UpdateResearchTradeIntentStatus 按允许的状态机转换更新 status（及可选执行关联补丁）。
func (r *ResearchTradeIntentRepo) UpdateResearchTradeIntentStatus(id uint, toStatus string, patch *ResearchTradeIntentStatusPatch) error {
	if db.Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if id == 0 {
		return fmt.Errorf("id is required")
	}
	toStatus = normalizeIntentStatus(toStatus)
	if toStatus == "" {
		return fmt.Errorf("toStatus is required")
	}

	var intent models.ResearchTradeIntent
	if err := db.Dao.First(&intent, id).Error; err != nil {
		return err
	}
	if !isAllowedResearchTradeIntentTransition(intent.Status, toStatus) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidResearchTradeIntentTransition, intent.Status, toStatus)
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
	return db.Dao.Model(&models.ResearchTradeIntent{}).Where("id = ?", id).Updates(updates).Error
}

func normalizeIntentStatus(s string) string {
	switch s {
	case models.ResearchTradeIntentStatusDraft,
		models.ResearchTradeIntentStatusConfirmed,
		models.ResearchTradeIntentStatusSubmitted,
		models.ResearchTradeIntentStatusRejected:
		return s
	default:
		return ""
	}
}

func isAllowedResearchTradeIntentTransition(from, to string) bool {
	switch from {
	case models.ResearchTradeIntentStatusDraft:
		return to == models.ResearchTradeIntentStatusConfirmed
	case models.ResearchTradeIntentStatusConfirmed:
		return to == models.ResearchTradeIntentStatusSubmitted || to == models.ResearchTradeIntentStatusRejected
	default:
		// submitted / rejected 为终态
		return false
	}
}
