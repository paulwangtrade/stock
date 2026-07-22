package execution

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/db"

	"gorm.io/gorm/clause"
)

// realStubStore RealStub 持久化端口（仅 real_stub；与 Paper 隔离）。
type realStubStore interface {
	UpsertOrder(order *broker.TradeOrder) error
	ListOrders() ([]broker.TradeOrder, error)
	HasReport(reportID string) (bool, error)
	ListReportIDs() ([]string, error)
	RecordReport(reportID, execID, reportType, clientOrderID, localOrderID string, payload any) error
	// InsertFillIfAbsent 按 exec_id 幂等写入成交事实；inserted=false 表示重复。
	InsertFillIfAbsent(fill *data.RealStubFill) (inserted bool, err error)
	ListFillExecIDs() ([]string, error)
	ListFills() ([]data.RealStubFill, error)
}

type gormRealStubStore struct{}

func newGormRealStubStore() realStubStore {
	data.EnsureRealStubTables()
	return &gormRealStubStore{}
}

func (s *gormRealStubStore) UpsertOrder(order *broker.TradeOrder) error {
	if order == nil || db.Dao == nil {
		return fmt.Errorf("execution: real stub upsert nil")
	}
	row := tradeOrderToRealStubRow(order)
	return db.Dao.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "local_order_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"client_order_id", "account_id", "stock_code", "stock_name", "side", "status",
			"price", "volume", "filled_price", "filled_volume", "leaves_quantity", "fee", "reason", "strategy_tag",
			"reject_code", "reject_reason", "exec_backend", "broker_order_id", "external_order_id",
			"broker_status", "filled_at", "updated_at",
		}),
	}).Create(&row).Error
}

func (s *gormRealStubStore) ListOrders() ([]broker.TradeOrder, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("execution: db nil")
	}
	var rows []data.RealStubOrder
	if err := db.Dao.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]broker.TradeOrder, 0, len(rows))
	for i := range rows {
		out = append(out, realStubRowToTradeOrder(rows[i]))
	}
	return out, nil
}

func (s *gormRealStubStore) HasReport(reportID string) (bool, error) {
	if reportID == "" || db.Dao == nil {
		return false, nil
	}
	var n int64
	err := db.Dao.Model(&data.RealStubReportLedger{}).Where("report_id = ?", reportID).Count(&n).Error
	return n > 0, err
}

func (s *gormRealStubStore) ListReportIDs() ([]string, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("execution: db nil")
	}
	var ids []string
	err := db.Dao.Model(&data.RealStubReportLedger{}).Pluck("report_id", &ids).Error
	return ids, err
}

func (s *gormRealStubStore) RecordReport(reportID, execID, reportType, clientOrderID, localOrderID string, payload any) error {
	if reportID == "" || db.Dao == nil {
		return fmt.Errorf("execution: empty report id")
	}
	raw, _ := json.Marshal(payload)
	entry := data.RealStubReportLedger{
		ReportID:      reportID,
		ExecID:        execID,
		ReportType:    reportType,
		ClientOrderID: clientOrderID,
		LocalOrderID:  localOrderID,
		PayloadJSON:   string(raw),
		CreatedAt:     time.Now(),
	}
	err := db.Dao.Create(&entry).Error
	if err != nil && isUniqueConflict(err) {
		return nil
	}
	return err
}

func (s *gormRealStubStore) InsertFillIfAbsent(fill *data.RealStubFill) (bool, error) {
	if fill == nil || strings.TrimSpace(fill.ExecID) == "" || db.Dao == nil {
		return false, fmt.Errorf("execution: invalid real stub fill")
	}
	if fill.CreatedAt.IsZero() {
		fill.CreatedAt = time.Now()
	}
	err := db.Dao.Create(fill).Error
	if err != nil {
		if isUniqueConflict(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *gormRealStubStore) ListFillExecIDs() ([]string, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("execution: db nil")
	}
	var ids []string
	err := db.Dao.Model(&data.RealStubFill{}).Pluck("exec_id", &ids).Error
	return ids, err
}

func (s *gormRealStubStore) ListFills() ([]data.RealStubFill, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("execution: db nil")
	}
	var rows []data.RealStubFill
	err := db.Dao.Order("id ASC").Find(&rows).Error
	return rows, err
}

func isUniqueConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || strings.Contains(msg, "duplicate entry")
}

func tradeOrderToRealStubRow(o *broker.TradeOrder) data.RealStubOrder {
	row := data.RealStubOrder{
		LocalOrderID:    o.ID,
		ClientOrderID:   o.ClientOrderID,
		AccountID:       o.AccountID,
		StockCode:       o.StockCode,
		StockName:       o.StockName,
		Side:            o.Side,
		Status:          o.Status,
		Price:           o.Price,
		Volume:          o.Volume,
		FilledPrice:     o.FilledPrice,
		FilledVolume:    o.FilledVolume,
		LeavesQuantity:  o.LeavesQuantity,
		Fee:             o.Fee,
		Reason:          o.Reason,
		StrategyTag:     o.StrategyTag,
		RejectCode:      o.RejectCode,
		RejectReason:    o.RejectReason,
		ExecBackend:     o.ExecBackend,
		BrokerOrderID:   o.BrokerOrderID,
		ExternalOrderID: o.ExternalOrderID,
		BrokerStatus:    o.BrokerStatus,
		CreatedAt:       o.CreatedAt,
		UpdatedAt:       o.UpdatedAt,
	}
	if !o.FilledAt.IsZero() {
		t := o.FilledAt
		row.FilledAt = &t
	}
	return row
}

func realStubRowToTradeOrder(row data.RealStubOrder) broker.TradeOrder {
	o := broker.TradeOrder{
		ID:              row.LocalOrderID,
		AccountID:       row.AccountID,
		StockCode:       row.StockCode,
		Symbol:          row.StockCode,
		StockName:       row.StockName,
		Side:            row.Side,
		Status:          row.Status,
		Price:           row.Price,
		Volume:          row.Volume,
		FilledPrice:     row.FilledPrice,
		FilledVolume:    row.FilledVolume,
		LeavesQuantity:  row.LeavesQuantity,
		Fee:             row.Fee,
		Reason:          row.Reason,
		StrategyTag:     row.StrategyTag,
		RejectCode:      row.RejectCode,
		RejectReason:    row.RejectReason,
		ClientOrderID:   row.ClientOrderID,
		ExecBackend:     row.ExecBackend,
		BrokerOrderID:   row.BrokerOrderID,
		ExternalOrderID: row.ExternalOrderID,
		BrokerStatus:    row.BrokerStatus,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
	if row.FilledAt != nil {
		o.FilledAt = *row.FilledAt
	}
	return o
}
