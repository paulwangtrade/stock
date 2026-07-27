package papertrading

import (
	"fmt"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
)

// PlanPaperStatus is the read-only Plan → Order → Fill traceability view.
type PlanPaperStatus struct {
	PlanID       uint            `json:"planId"`
	Enabled      bool            `json:"enabled"`
	OrdersTotal  int             `json:"ordersTotal"`
	FilledCount  int             `json:"filledCount"`
	RejectCount  int             `json:"rejectCount"`
	FilledVolume int64           `json:"filledVolume"`
	FilledAmount float64         `json:"filledAmount"`
	Orders       []PaperSimOrder `json:"orders"`
	Fills        []PaperSimFill  `json:"fills"`
}

// TodayStatus is the read-only daily paper-trading dashboard snapshot.
type TodayStatus struct {
	Enabled       bool               `json:"enabled"`
	TradeDate     string             `json:"tradeDate"`
	PlanID        uint               `json:"planId"`
	Run           *PaperSimRun       `json:"run,omitempty"`
	Account       *PaperSimAccount   `json:"account,omitempty"`
	Positions     []PaperSimPosition `json:"positions"`
	Orders        []PaperSimOrder    `json:"orders"`
	Fills         []PaperSimFill     `json:"fills"`
	OrdersTotal   int                `json:"ordersTotal"`
	FilledCount   int                `json:"filledCount"`
	RejectCount   int                `json:"rejectCount"`
	FilledVolume  int64              `json:"filledVolume"`
	FilledAmount  float64            `json:"filledAmount"`
	Cash          float64            `json:"cash"`
	Equity        float64            `json:"equity"`
	UnrealizedPnl float64            `json:"unrealizedPnl"`
	RealizedPnl   float64            `json:"realizedPnl"`
}

// GetPlanPaperStatus returns all paper-sim orders/fills for a plan (read only).
// When the feature is disabled or schema absent, it returns an empty enabled=false view.
func GetPlanPaperStatus(planID uint) (*PlanPaperStatus, error) {
	out := &PlanPaperStatus{PlanID: planID, Enabled: IsEnabled()}
	if db.Dao == nil {
		return out, fmt.Errorf("papertrading: db not initialized")
	}
	if !db.Dao.Migrator().HasTable(&PaperSimOrder{}) {
		return out, nil
	}

	var orders []PaperSimOrder
	if err := db.Dao.Where("plan_id = ?", planID).Order("id asc").Find(&orders).Error; err != nil {
		return out, err
	}
	out.Orders = orders
	out.OrdersTotal = len(orders)
	for _, o := range orders {
		switch o.Status {
		case OrderStatusFilled:
			out.FilledCount++
			out.FilledVolume += o.FilledVolume
			out.FilledAmount += o.FilledPrice * float64(o.FilledVolume)
		case OrderStatusRejected:
			out.RejectCount++
		}
	}

	var fills []PaperSimFill
	if err := db.Dao.Where("plan_id = ?", planID).Order("id asc").Find(&fills).Error; err != nil {
		return out, err
	}
	out.Fills = fills
	return out, nil
}

// GetDefaultAccount returns the default paper-sim account (read only), or nil if none.
func GetDefaultAccount() (*PaperSimAccount, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("papertrading: db not initialized")
	}
	if !db.Dao.Migrator().HasTable(&PaperSimAccount{}) {
		return nil, nil
	}
	var acc PaperSimAccount
	if err := db.Dao.Where("name = ?", "paper_sim_default").First(&acc).Error; err != nil {
		return nil, nil
	}
	return &acc, nil
}

// GetPositions returns positions for an account (read only).
func GetPositions(accountID uint) ([]PaperSimPosition, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("papertrading: db not initialized")
	}
	if !db.Dao.Migrator().HasTable(&PaperSimPosition{}) {
		return nil, nil
	}
	var positions []PaperSimPosition
	if err := db.Dao.Where("account_id = ?", accountID).Order("stock_code asc").Find(&positions).Error; err != nil {
		return nil, err
	}
	return positions, nil
}

// GetTodayStatus returns a read-only snapshot of today's paper-sim activity.
func GetTodayStatus(tradeDate string) (*TodayStatus, error) {
	if tradeDate == "" {
		tradeDate = time.Now().Format("2006-01-02")
	}
	out := &TodayStatus{Enabled: IsEnabled(), TradeDate: tradeDate}
	if db.Dao == nil {
		return out, fmt.Errorf("papertrading: db not initialized")
	}
	if !db.Dao.Migrator().HasTable(&PaperSimOrder{}) {
		return out, nil
	}

	if plan, err := data.NewTradePlanRepo().GetFrozenByTradeDate(tradeDate); err == nil && plan != nil {
		out.PlanID = plan.ID
	}

	if db.Dao.Migrator().HasTable(&PaperSimRun{}) {
		var run PaperSimRun
		q := db.Dao.Where("trade_date = ?", tradeDate).Order("id desc")
		if out.PlanID > 0 {
			q = q.Where("plan_id = ?", out.PlanID)
		}
		if err := q.First(&run).Error; err == nil {
			out.Run = &run
			if out.PlanID == 0 {
				out.PlanID = run.PlanID
			}
		}
	}

	acc, err := GetDefaultAccount()
	if err != nil {
		return out, err
	}
	out.Account = acc
	if acc != nil {
		positions, err := GetPositions(acc.ID)
		if err != nil {
			return out, err
		}
		out.Positions = positions
		out.UnrealizedPnl = acc.UnrealizedPnl
		out.RealizedPnl = acc.RealizedPnl
		out.Equity = acc.Equity
		out.Cash = acc.Cash
	}

	var orders []PaperSimOrder
	oq := db.Dao.Where("trade_date = ?", tradeDate).Order("id asc")
	if out.PlanID > 0 {
		oq = oq.Where("plan_id = ?", out.PlanID)
	}
	if err := oq.Find(&orders).Error; err != nil {
		return out, err
	}
	out.Orders = orders
	out.OrdersTotal = len(orders)
	for _, o := range orders {
		switch o.Status {
		case OrderStatusFilled:
			out.FilledCount++
			out.FilledVolume += o.FilledVolume
			out.FilledAmount += o.FilledPrice * float64(o.FilledVolume)
		case OrderStatusRejected:
			out.RejectCount++
		}
	}

	var fills []PaperSimFill
	if out.PlanID > 0 {
		if err := db.Dao.Where("plan_id = ?", out.PlanID).Order("id asc").Find(&fills).Error; err != nil {
			return out, err
		}
	} else if len(orders) > 0 {
		ids := make([]uint, 0, len(orders))
		for _, o := range orders {
			ids = append(ids, o.ID)
		}
		if err := db.Dao.Where("order_id IN ?", ids).Order("id asc").Find(&fills).Error; err != nil {
			return out, err
		}
	}
	out.Fills = fills
	return out, nil
}
