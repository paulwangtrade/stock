package papertrading

import (
	"fmt"
	"time"

	"go-stock/backend/db"
)

// DashboardToday is the V1 observation view for "今日模拟交易".
type DashboardToday struct {
	Enabled        bool         `json:"enabled"`
	TradeDate      string       `json:"tradeDate"`
	PlanCount      int          `json:"planCount"`
	PlanID         uint         `json:"planId,omitempty"`
	RunStatus      string       `json:"runStatus"`
	Trigger        string       `json:"trigger,omitempty"`
	Actor          string       `json:"actor,omitempty"`
	ExecutionID    string       `json:"executionId,omitempty"`
	OrdersTotal    int          `json:"ordersTotal"`
	FilledCount    int          `json:"filledCount"`
	RejectCount    int          `json:"rejectCount"`
	FilledAmount   float64      `json:"filledAmount"`
	Cash           float64      `json:"cash"`
	Equity         float64      `json:"equity"`
	UnrealizedPnl  float64      `json:"unrealizedPnl"`
	InitialCash    float64      `json:"initialCash"`
	Message        string       `json:"message,omitempty"`
	RunStartedAt   *time.Time   `json:"runStartedAt,omitempty"`
	RunFinishedAt  *time.Time   `json:"runFinishedAt,omitempty"`
	DataSourceNote string       `json:"dataSourceNote"`
}

// DashboardPositionRow is a position row with derived fields for display.
// Phase7-A3-1: MarkPrice is DisplayPrice for UI compat; PersistedMarkPrice keeps DB mark.
type DashboardPositionRow struct {
	StockCode          string     `json:"stockCode"`
	StockName          string     `json:"stockName"`
	TotalVolume        int64      `json:"totalVolume"`
	AvailableVolume    int64      `json:"availableVolume"`
	LockedVolume       int64      `json:"lockedVolume"`
	AvgCost            float64    `json:"avgCost"`
	MarkPrice          float64    `json:"markPrice"` // compat = DisplayPrice
	PersistedMarkPrice float64    `json:"persistedMarkPrice"`
	DisplayPrice       float64    `json:"displayPrice"`
	MarketValue        float64    `json:"marketValue"`
	UnrealizedPnl      float64    `json:"unrealizedPnl"`
	ReturnRate         float64    `json:"returnRate"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	QuoteSource        string     `json:"quoteSource,omitempty"`
	QuoteUpdatedAt     *time.Time `json:"quoteUpdatedAt,omitempty"`
	T1Locked           bool       `json:"t1Locked"`
}

// DashboardPositions is the V1 positions observation view.
type DashboardPositions struct {
	Enabled                   bool                   `json:"enabled"`
	AccountID                 uint                   `json:"accountId,omitempty"`
	Cash                      float64                `json:"cash"`
	MarketValue               float64                `json:"marketValue"` // persisted account snapshot
	Equity                    float64                `json:"equity"`
	UnrealizedPnl             float64                `json:"unrealizedPnl"`
	InitialCash               float64                `json:"initialCash"`
	ObservationMarketValue    float64                `json:"observationMarketValue"`
	ObservationUnrealizedPnl  float64                `json:"observationUnrealizedPnl"`
	ObservationEquity         float64                `json:"observationEquity"`
	QuoteOverlay              bool                   `json:"quoteOverlay"`
	Positions                 []DashboardPositionRow `json:"positions"`
	DataSourceNote            string                 `json:"dataSourceNote"`
}

// DashboardRuns is the V1 run ledger observation view.
type DashboardRuns struct {
	Enabled        bool          `json:"enabled"`
	TradeDate      string        `json:"tradeDate,omitempty"`
	Runs           []PaperSimRun `json:"runs"`
	Total          int           `json:"total"`
	DataSourceNote string        `json:"dataSourceNote"`
}

const dashboardDataSourceNote = "Phase6.6 Paper Trading MVP · paper_sim_* · 仅用于策略观察和模拟分析，不代表真实交易账户"

// GetDashboardToday builds the today observation DTO (read only).
func GetDashboardToday(tradeDate string) (*DashboardToday, error) {
	st, err := GetTodayStatus(tradeDate)
	if err != nil {
		return nil, err
	}
	out := &DashboardToday{
		Enabled:        st.Enabled,
		TradeDate:      st.TradeDate,
		PlanID:         st.PlanID,
		OrdersTotal:    st.OrdersTotal,
		FilledCount:    st.FilledCount,
		RejectCount:    st.RejectCount,
		FilledAmount:   st.FilledAmount,
		Cash:           st.Cash,
		Equity:         st.Equity,
		UnrealizedPnl:  st.UnrealizedPnl,
		DataSourceNote: dashboardDataSourceNote,
	}
	if st.PlanID > 0 {
		out.PlanCount = 1
	}
	if st.Account != nil {
		out.InitialCash = st.Account.InitialCash
		if out.Cash == 0 && st.Account.Cash != 0 {
			out.Cash = st.Account.Cash
		}
		if out.Equity == 0 && st.Account.Equity != 0 {
			out.Equity = st.Account.Equity
		}
		if out.UnrealizedPnl == 0 && st.Account.UnrealizedPnl != 0 {
			out.UnrealizedPnl = st.Account.UnrealizedPnl
		}
	}
	if st.Run != nil {
		out.RunStatus = st.Run.Status
		out.Trigger = st.Run.Trigger
		out.Actor = st.Run.Actor
		out.ExecutionID = st.Run.ExecutionID
		out.Message = st.Run.Message
		t := st.Run.StartedAt
		out.RunStartedAt = &t
		out.RunFinishedAt = st.Run.FinishedAt
	} else if !st.Enabled {
		out.RunStatus = RunStatusSkippedDisabled
		out.Message = "enablePaperTrading=false"
	} else {
		out.RunStatus = "no_run"
		out.Message = "no paper_sim_runs for trade date"
	}
	return out, nil
}

// GetDashboardPositions builds the positions observation DTO (read only).
func GetDashboardPositions() (*DashboardPositions, error) {
	out := &DashboardPositions{
		Enabled:        IsEnabled(),
		Positions:      []DashboardPositionRow{},
		DataSourceNote: dashboardDataSourceNote,
	}
	if db.Dao == nil {
		return out, fmt.Errorf("papertrading: db not initialized")
	}
	acc, err := GetDefaultAccount()
	if err != nil {
		return out, err
	}
	if acc == nil {
		return out, nil
	}
	out.AccountID = acc.ID
	out.Cash = acc.Cash
	out.MarketValue = acc.MarketValue
	out.Equity = acc.Equity
	out.UnrealizedPnl = acc.UnrealizedPnl
	out.InitialCash = acc.InitialCash

	positions, err := GetPositions(acc.ID)
	if err != nil {
		return out, err
	}
	// Phase7-A3-1: read-only Quote overlay; does not write paper_sim_positions.
	rows := BuildObservationPositionRows(positions, observationQuoteFetcher())
	out.Positions = rows
	applyObservationAccountTotals(out, rows)
	return out, nil
}

// ListRuns returns recent paper_sim_runs (newest first). tradeDate empty → all dates.
func ListRuns(tradeDate string, limit, offset int) (*DashboardRuns, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	out := &DashboardRuns{
		Enabled:        IsEnabled(),
		TradeDate:      tradeDate,
		Runs:           []PaperSimRun{},
		DataSourceNote: dashboardDataSourceNote,
	}
	if db.Dao == nil {
		return out, fmt.Errorf("papertrading: db not initialized")
	}
	if !db.Dao.Migrator().HasTable(&PaperSimRun{}) {
		return out, nil
	}
	q := db.Dao.Model(&PaperSimRun{})
	if tradeDate != "" {
		q = q.Where("trade_date = ?", tradeDate)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return out, err
	}
	out.Total = int(total)

	var runs []PaperSimRun
	if err := q.Order("id desc").Limit(limit).Offset(offset).Find(&runs).Error; err != nil {
		return out, err
	}
	out.Runs = runs
	return out, nil
}
