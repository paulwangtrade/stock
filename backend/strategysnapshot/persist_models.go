package strategysnapshot

import "time"

// StrategySnapshotRow is the SQLite row for an immutable StrategySnapshot document.
// payload_json is the sole authoritative body; signal/risk/entry JSON are optional mirrors.
type StrategySnapshotRow struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	SnapshotID      string    `gorm:"column:snapshot_id;size:128;uniqueIndex;not null" json:"snapshot_id"`
	SnapshotVersion string    `gorm:"column:snapshot_version;size:32" json:"snapshot_version"`
	Scope           string    `gorm:"column:scope;size:16;index" json:"scope"`
	PlanID          uint      `gorm:"column:plan_id;index;not null" json:"plan_id"`
	PlanItemID      uint      `gorm:"column:plan_item_id;index" json:"plan_item_id"`
	TradeDate       string    `gorm:"column:trade_date;size:10;index" json:"trade_date"`
	PayloadJSON     string    `gorm:"column:payload_json;type:text;not null" json:"payload_json"`
	SignalJSON      string    `gorm:"column:signal_json;type:text" json:"signal_json,omitempty"`
	RiskJSON        string    `gorm:"column:risk_json;type:text" json:"risk_json,omitempty"`
	EntryJSON       string    `gorm:"column:entry_json;type:text" json:"entry_json,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (StrategySnapshotRow) TableName() string { return "strategy_snapshots" }

// PlanStrategyRefRow links a TradePlan / PlanItem to a StrategySnapshot.snapshot_id.
// plan_item_id = 0 denotes the plan-scope snapshot.
type PlanStrategyRefRow struct {
	ID                   uint      `gorm:"primaryKey" json:"id"`
	PlanID               uint      `gorm:"column:plan_id;not null;uniqueIndex:uidx_plan_strategy_ref" json:"plan_id"`
	PlanItemID           uint      `gorm:"column:plan_item_id;not null;uniqueIndex:uidx_plan_strategy_ref" json:"plan_item_id"`
	StrategySnapshotID string    `gorm:"column:strategy_snapshot_id;size:128;index;not null" json:"strategy_snapshot_id"`
	CreatedAt          time.Time `json:"created_at"`
}

func (PlanStrategyRefRow) TableName() string { return "plan_strategy_refs" }
