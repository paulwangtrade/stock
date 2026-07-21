package data

import "time"

// RealStubFill Real 成交事实（与 paper_fills 隔离）。
// 每笔 TRADE 一行；exec_id 唯一幂等。订单累计由 Handler 重算写入 RealStubOrder。
type RealStubFill struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	ClientOrderID string    `gorm:"size:64;index;not null" json:"clientOrderId"`
	LocalOrderID  string    `gorm:"size:64;index" json:"localOrderId"`
	ExecID        string    `gorm:"size:128;uniqueIndex:uidx_real_stub_fill_exec;not null" json:"execId"`
	FillQty       int64     `json:"fillQty"`   // 本笔 (= LastQty)
	FillPrice     float64   `json:"fillPrice"` // 本笔 (= LastPrice)
	CumQty        int64     `json:"cumQty"`    // 本笔应用后本地累计成交量
	AvgPrice      float64   `json:"avgPrice"`  // 本笔应用后本地累计均价
	BrokerOrderID string    `gorm:"size:64" json:"brokerOrderId"`
	ReportID      string    `gorm:"size:128;index" json:"reportId"`
	CreatedAt     time.Time `json:"createdAt"`
}

func (RealStubFill) TableName() string { return "real_stub_fills" }
