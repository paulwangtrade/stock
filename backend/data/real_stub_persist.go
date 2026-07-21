package data

import (
	"fmt"
	"time"

	"go-stock/backend/db"

	"gorm.io/gorm"
)

// RealStub 持久化（Phase2-C PR3）：仅服务 exec_backend=real_stub，与 paper_orders/paper_fills 隔离。
// 订单快照 + 回报幂等账本 + 成交事实（real_stub_fills）。不写 EventHub、不参与 FillPaperOrder。
//
// Real 订单累计字段语义（与 Paper 同名字段区分用途）：
//   FilledVolume    = 累计成交数量 (cumulative filled quantity)
//   FilledPrice     = 累计成交均价 avg = Σ(fill_qty*fill_price)/filled_volume
//   LeavesQuantity  = 剩余未成交 = volume - filled_volume
// ExecutionReport：LastQty/LastPrice=本笔；CumQty/AvgPrice 可为券商快照，累计以本地重算为准
// PartialFill：0<filled<volume → OMS=pending + broker_status=partially_filled
//              filled==volume → OMS=filled + broker_status=filled

const (
	RealStubReportTypeAck    = "ack"
	RealStubReportTypeReject = "reject"
	RealStubReportTypeFilled = "filled" // 兼容旧全成路径
	RealStubReportTypeTrade  = "trade"
	RealStubReportTypeCancel = "cancel"
)

// RealStubOrder Real 骨架订单快照（可按 client_order_id / local_order_id 恢复）。
type RealStubOrder struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	LocalOrderID    string     `gorm:"size:64;uniqueIndex:uidx_real_stub_local_order" json:"localOrderId"`
	ClientOrderID   string     `gorm:"size:64;uniqueIndex:uidx_real_stub_client_order" json:"clientOrderId"`
	AccountID       string     `gorm:"size:32" json:"accountId"`
	StockCode       string     `gorm:"size:16;index" json:"stockCode"`
	StockName       string     `gorm:"size:64" json:"stockName"`
	Side            string     `gorm:"size:8" json:"side"`
	Status          string     `gorm:"size:16;index" json:"status"` // OMS
	Price           float64    `json:"price"`
	Volume          int64      `json:"volume"`
	FilledPrice     float64    `json:"filledPrice"`    // Real：累计成交均价 Σ(qty*px)/filled_volume
	FilledVolume    int64      `json:"filledVolume"`   // Real：累计成交数量
	LeavesQuantity  int64      `json:"leavesQuantity"` // Real：剩余未成交 = volume - filled_volume
	Fee             float64    `json:"fee"`
	Reason          string     `gorm:"type:text" json:"reason"`
	StrategyTag     string     `gorm:"size:32" json:"strategyTag"`
	RejectCode      string     `gorm:"size:64" json:"rejectCode"`
	RejectReason    string     `gorm:"size:500" json:"rejectReason"`
	ExecBackend     string     `gorm:"size:32;index" json:"execBackend"`
	BrokerOrderID   string     `gorm:"size:64;index" json:"brokerOrderId"`
	ExternalOrderID string     `gorm:"size:64" json:"externalOrderId"`
	BrokerStatus    string     `gorm:"size:32" json:"brokerStatus"`
	FilledAt        *time.Time `json:"filledAt"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func (RealStubOrder) TableName() string { return "real_stub_orders" }

// RealStubReportLedger 回报幂等账本：同一 report_id 只应用一次。
type RealStubReportLedger struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	ReportID      string    `gorm:"size:128;uniqueIndex:uidx_real_stub_report_id;not null" json:"reportId"`
	ExecID        string    `gorm:"size:128;index" json:"execId"`
	ReportType    string    `gorm:"size:16;index;not null" json:"reportType"` // ack|reject|filled
	ClientOrderID string    `gorm:"size:64;index" json:"clientOrderId"`
	LocalOrderID  string    `gorm:"size:64;index" json:"localOrderId"`
	PayloadJSON   string    `gorm:"type:text" json:"payloadJson"`
	CreatedAt     time.Time `json:"createdAt"`
}

func (RealStubReportLedger) TableName() string { return "real_stub_report_ledger" }

// MigrateRealStubTables 迁移 RealStub 表（可与 MigratePaperTrading 一并调用）。
func MigrateRealStubTables(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return database.AutoMigrate(&RealStubOrder{}, &RealStubReportLedger{}, &RealStubFill{})
}

// EnsureRealStubTables 确保 RealStub 表存在。
func EnsureRealStubTables() {
	if db.Dao == nil {
		return
	}
	_ = MigrateRealStubTables(db.Dao)
}
