package opportunity

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/papertrading"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	ActionView   = "VIEW"
	ActionWatch  = "WATCH"
	ActionIgnore = "IGNORE"

	userOpportunityActionSchemaVersion = "opportunity_user_action.v1"
	defaultCreatedBy                   = "ui:opportunity-list"
)

var validOpportunityActions = map[string]struct{}{
	ActionView:   {},
	ActionWatch:  {},
	ActionIgnore: {},
}

// UserOpportunityAction is an append-only user behavior log for a single opportunity projection.
type UserOpportunityAction struct {
	ID             string    `gorm:"primaryKey;size:64" json:"id"`
	AccountID      uint      `gorm:"index:idx_uoa_acct_batch_time,priority:1;not null" json:"account_id"`
	ScanBatchKey   string    `gorm:"size:120;index:idx_uoa_acct_batch_time,priority:2;not null" json:"scan_batch_key"`
	OpportunityID  string    `gorm:"size:64;index;not null" json:"opportunity_id"`
	StockCode      string    `gorm:"size:32;index;not null" json:"stock_code"`
	Action         string    `gorm:"size:16;not null" json:"action"`
	CreatedBy      string    `gorm:"size:120;not null" json:"created_by"`
	CreatedAt      time.Time `gorm:"index:idx_uoa_acct_batch_time,priority:3;not null" json:"created_at"`
	SchemaVersion  string    `gorm:"size:32;default:opportunity_user_action.v1" json:"schema_version,omitempty"`
}

func (UserOpportunityAction) TableName() string { return "user_opportunity_actions" }

// UserOpportunityActionSummary is the read model merged into opportunity list entries.
type UserOpportunityActionSummary struct {
	ID            string    `json:"id"`
	Action        string    `json:"action"`
	CreatedAt     time.Time `json:"created_at"`
	CreatedBy     string    `json:"created_by,omitempty"`
	OpportunityID string    `json:"opportunity_id"`
}

// SaveUserOpportunityActionInput is the write contract for POST action.
type SaveUserOpportunityActionInput struct {
	AccountID     uint
	ScanBatchKey  string
	OpportunityID string
	StockCode     string
	Action        string
	CreatedBy     string
	Secucode      string
	SignalTime    string
	SignalTag     string
}

func EnsureSchema(gdb *gorm.DB) error {
	if gdb == nil {
		gdb = db.Dao
	}
	if gdb == nil {
		return errors.New("opportunity: db not initialized")
	}
	return gdb.AutoMigrate(&UserOpportunityAction{})
}

func validateAction(action string) error {
	if _, ok := validOpportunityActions[strings.TrimSpace(action)]; !ok {
		return fmt.Errorf("invalid action %q", action)
	}
	return nil
}

// SaveUserOpportunityAction inserts a new action row (append-only).
func SaveUserOpportunityAction(in SaveUserOpportunityActionInput) (*UserOpportunityAction, error) {
	if err := EnsureSchema(db.Dao); err != nil {
		return nil, err
	}
	code := NormalizeStockCode(in.StockCode)
	if code == "" && strings.TrimSpace(in.Secucode) != "" {
		code = SecucodeToStockCode(in.Secucode)
	}
	if code == "" {
		return nil, errors.New("stock_code is required")
	}
	if err := validateAction(in.Action); err != nil {
		return nil, err
	}

	batchKey := strings.TrimSpace(in.ScanBatchKey)
	opportunityID := strings.TrimSpace(in.OpportunityID)
	if opportunityID == "" {
		if batchKey == "" {
			return nil, errors.New("scan_batch_key or opportunity_id is required")
		}
		opportunityID = BuildOpportunityID(batchKey, in.Secucode, in.SignalTime, in.SignalTag)
	}
	if batchKey == "" {
		return nil, errors.New("scan_batch_key is required")
	}

	accountID := in.AccountID
	if accountID == 0 {
		acc, err := papertrading.GetDefaultAccount()
		if err != nil {
			return nil, err
		}
		accountID = acc.ID
	}

	createdBy := strings.TrimSpace(in.CreatedBy)
	if createdBy == "" {
		createdBy = defaultCreatedBy
	}

	now := time.Now()
	row := &UserOpportunityAction{
		ID:            "uoa_" + uuid.NewString(),
		AccountID:     accountID,
		ScanBatchKey:  batchKey,
		OpportunityID: opportunityID,
		StockCode:     code,
		Action:        strings.TrimSpace(in.Action),
		CreatedBy:     createdBy,
		CreatedAt:     now,
		SchemaVersion: userOpportunityActionSchemaVersion,
	}
	if err := db.Dao.Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// LoadLatestActionsByBatch returns newest action per opportunity_id for a scan batch.
func LoadLatestActionsByBatch(accountID uint, scanBatchKey string) (map[string]*UserOpportunityActionSummary, error) {
	out := map[string]*UserOpportunityActionSummary{}
	if db.Dao == nil || accountID == 0 || strings.TrimSpace(scanBatchKey) == "" {
		return out, nil
	}
	if err := EnsureSchema(db.Dao); err != nil {
		return nil, err
	}

	var rows []UserOpportunityAction
	err := db.Dao.Where("account_id = ? AND scan_batch_key = ?", accountID, strings.TrimSpace(scanBatchKey)).
		Order("created_at desc").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for i := range rows {
		oid := rows[i].OpportunityID
		if _, exists := out[oid]; exists {
			continue
		}
		out[oid] = actionToSummary(&rows[i])
	}
	return out, nil
}

func actionToSummary(row *UserOpportunityAction) *UserOpportunityActionSummary {
	if row == nil {
		return nil
	}
	return &UserOpportunityActionSummary{
		ID:            row.ID,
		Action:        row.Action,
		CreatedAt:     row.CreatedAt,
		CreatedBy:     row.CreatedBy,
		OpportunityID: row.OpportunityID,
	}
}
