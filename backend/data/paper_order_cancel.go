package data

import (
	"errors"
	"time"

	"go-stock/backend/db"

	"gorm.io/gorm"
)

// ErrOrderNotCancellable 订单不可撤：非 pending，或已被他途改写（CAS 影响行数=0）。
var ErrOrderNotCancellable = errors.New("order not cancellable")

// CancelPaperOrder 仅允许 pending→cancelled（条件更新）；成功后写 order_cancelled 审计。
// 本 PR 不向 EventHub 发布 cancelled（契约未扩展）。
func (p *PaperTradingApi) CancelPaperOrder(orderID uint) error {
	EnsurePaperTradingTables()
	if db.Dao == nil || orderID == 0 {
		return ErrOrderNotCancellable
	}
	now := time.Now()
	return db.Dao.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&PaperOrder{}).
			Where("id = ? AND status = ?", orderID, PaperOrderStatusPending).
			Updates(map[string]any{
				"status":     PaperOrderStatusCancelled,
				"updated_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrOrderNotCancellable
		}
		var order PaperOrder
		if err := tx.First(&order, orderID).Error; err != nil {
			return err
		}
		return writePaperOrderEvent(tx, &order, PaperOrderEventCancelled, "", "order cancelled", now)
	})
}
