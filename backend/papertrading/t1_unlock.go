// T+1 Position Unlock (Phase10-C.6-C).
// Production job: unlocks locked volume whose buy fills have trade_date < current trading day.
// Does not call SettleNewTradingDay. Does not write orders, fills, Gateway, or Settlement.

package papertrading

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/tradingcalendar"

	"gorm.io/gorm"
)

const (
	unlockSkipDisabled      = "enablePaperTrading=false"
	unlockSkipNonTradingDay = "non trading day"
	unlockSkipNoAccount     = "no paper_sim account yet"
)

// UnlockResult is the observable outcome of PositionUnlockJob.
type UnlockResult struct {
	Enabled             bool   `json:"enabled"`
	Skipped             bool   `json:"skipped"`
	TradeDate           string `json:"tradeDate"`
	AccountID           uint   `json:"accountId"`
	PositionsScanned    int    `json:"positionsScanned"`
	PositionsUnlocked   int    `json:"positionsUnlocked"`
	UnlockVolumeTotal   int64  `json:"unlockVolumeTotal"`
	UnattributedUnlocks int    `json:"unattributedUnlocks"` // locked rows with no fills (historical backfill)
	Message             string `json:"message"`
}

// PositionUnlockJob is the production T+1 unlock entry (cron 09:20 via PositionState morning settlement).
func PositionUnlockJob(now time.Time) (*UnlockResult, error) {
	if now.IsZero() {
		now = time.Now()
	}
	today := tradingcalendar.FormatDate(now)
	out := &UnlockResult{Enabled: IsEnabled(), TradeDate: today}

	if !IsEnabled() {
		out.Skipped = true
		out.Message = unlockSkipDisabled
		logger.SugaredLogger.Infof("PositionUnlockJob skipped reason=%s trade_date=%s", out.Message, today)
		return out, nil
	}
	if !tradingcalendar.IsTradingDay(now) {
		out.Skipped = true
		out.Message = unlockSkipNonTradingDay
		logger.SugaredLogger.Infof("PositionUnlockJob skipped reason=%s trade_date=%s", out.Message, today)
		return out, nil
	}
	if db.Dao == nil {
		return nil, fmt.Errorf("papertrading: db not initialized")
	}
	if err := EnsureSchema(db.Dao); err != nil {
		return nil, err
	}

	acc, err := GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	if acc == nil {
		out.Skipped = true
		out.Message = unlockSkipNoAccount
		return out, nil
	}
	out.AccountID = acc.ID

	if err := UnlockT1EligiblePositions(acc.ID, today, out); err != nil {
		return out, err
	}
	out.Message = fmt.Sprintf(
		"t1 unlock done trade_date=%s scanned=%d unlocked=%d volume=%d unattributed=%d",
		today, out.PositionsScanned, out.PositionsUnlocked, out.UnlockVolumeTotal, out.UnattributedUnlocks,
	)
	logger.SugaredLogger.Infof("PositionUnlockJob %s account_id=%d", out.Message, acc.ID)
	return out, nil
}

// UnlockT1EligiblePositions applies T+1 unlock for one account on trading day `today` (YYYY-MM-DD).
//
//	unlock_volume = locked_volume - today's buy fill volume (trade_date == today)
//	locked_volume -= unlock_volume
//	available_volume += unlock_volume
//
// Same-day fills stay locked. Does not write orders/fills/cash/mark_price.
func UnlockT1EligiblePositions(accountID uint, today string, out *UnlockResult) error {
	today = strings.TrimSpace(today)
	if today == "" {
		return fmt.Errorf("papertrading: unlock today date is required")
	}
	if db.Dao == nil {
		return fmt.Errorf("papertrading: db not initialized")
	}
	if out == nil {
		out = &UnlockResult{}
	}

	var positions []PaperSimPosition
	if err := db.Dao.Where("account_id = ? AND locked_volume > 0", accountID).
		Order("id asc").Find(&positions).Error; err != nil {
		return err
	}
	out.PositionsScanned = len(positions)
	if len(positions) == 0 {
		return nil
	}

	now := time.Now()
	return db.Dao.Transaction(func(tx *gorm.DB) error {
		for i := range positions {
			p := &positions[i]
			todayQty, attributed, err := todayBuyFillVolume(tx, p.AccountID, p.StockCode, today)
			if err != nil {
				return err
			}
			unlock := p.LockedVolume - todayQty
			if unlock < 0 {
				unlock = 0
			}
			if unlock == 0 {
				continue
			}
			newLocked := p.LockedVolume - unlock
			newAvail := p.AvailableVolume + unlock
			if newLocked < 0 {
				newLocked = 0
			}
			if newAvail < 0 {
				newAvail = 0
			}
			// Keep available + locked == total when possible.
			if newAvail+newLocked != p.TotalVolume && p.TotalVolume >= 0 {
				newAvail = p.TotalVolume - newLocked
				if newAvail < 0 {
					newAvail = 0
				}
			}
			if err := tx.Model(&PaperSimPosition{}).Where("id = ?", p.ID).
				Updates(map[string]any{
					"locked_volume":    newLocked,
					"available_volume": newAvail,
					"updated_at":       now,
				}).Error; err != nil {
				return err
			}
			out.PositionsUnlocked++
			out.UnlockVolumeTotal += unlock
			if !attributed {
				out.UnattributedUnlocks++
				logger.SugaredLogger.Infof(
					"PositionUnlockJob unattributed_unlock account_id=%d code=%s volume=%d",
					p.AccountID, p.StockCode, unlock,
				)
			}
		}
		return nil
	})
}

func todayBuyFillVolume(tx *gorm.DB, accountID uint, stockCode, today string) (qty int64, attributed bool, err error) {
	if !tx.Migrator().HasTable(&PaperSimFill{}) || !tx.Migrator().HasTable(&PaperSimOrder{}) {
		return 0, false, nil
	}
	var n int64
	if err := tx.Table("paper_sim_fills AS f").
		Where("f.account_id = ? AND f.stock_code = ? AND f.side = ?", accountID, stockCode, "buy").
		Count(&n).Error; err != nil {
		return 0, false, err
	}
	if n == 0 {
		return 0, false, nil
	}
	var todayQty int64
	if err := tx.Table("paper_sim_fills AS f").
		Joins("JOIN paper_sim_orders AS o ON o.id = f.order_id").
		Where("f.account_id = ? AND f.stock_code = ? AND f.side = ? AND o.trade_date = ?",
			accountID, stockCode, "buy", today).
		Select("COALESCE(SUM(f.volume), 0)").
		Scan(&todayQty).Error; err != nil {
		return 0, false, err
	}
	return todayQty, true, nil
}
