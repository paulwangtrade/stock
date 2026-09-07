package positionsizing

import (
	"go-stock/backend/portfolio"
	"go-stock/backend/tradingconfig"
)

// Method identifies which sizing algorithm produced a proposal.
// Phase6.5-D MVP: only fixed_amount is implemented.
type Method string

const (
	MethodFixedAmount    Method = tradingconfig.SizingMethodFixedAmount
	MethodPortfolioAware Method = tradingconfig.SizingMethodPortfolioAware

	DefaultMaxSinglePositionWeight = 0.20
	DefaultReserveCashRatio        = 0.0
	DefaultMinOrderAmount          = 1_000.0
)

// Request is the sizing input. FixedAmountSizer ignores candidate-level fields
// and Portfolio. PortfolioAwareSizer reads Portfolio + CandidateCount + policy fields.
// Callers inject a read-only Portfolio Snapshot. Sizers must never load it from DB.
type Request struct {
	StockCode               string
	Score                   float64
	Rank                    int
	Portfolio               *portfolio.Snapshot
	CandidateCount          int     // equal-split denominator; not Rank
	MaxGrossExposurePct     float64 // 0 → allocation default 0.85
	MaxSinglePositionWeight float64 // 0 → 0.20
	ReserveCashRatio        float64 // 0 → no reserve (P1 compatible)
	MinOrderAmount          float64 // 0 → 1000 (aware only)
}

// AmountReason is a read-only explanation of PlannedAmount. Not consumed by
// Freeze / Gateway / fill. Must not be written into TradePlan.Message.
type AmountReason struct {
	Method          string   `json:"method"`
	Equity          float64  `json:"equity"`
	Cash            float64  `json:"cash"`
	CurrentExposure float64  `json:"current_exposure"`
	Budget          float64  `json:"budget"`
	CandidateCount  int      `json:"candidate_count"`
	RawAmount       float64  `json:"raw_amount"`
	FinalAmount     float64  `json:"final_amount"`
	CappedBy        []string `json:"capped_by,omitempty"`
}

// SizingProposal is the budget proposal written into Trade Plan paths.
// Draft-stage PlannedVolume is typically 0 until Morning materialization.
type SizingProposal struct {
	PlannedAmount float64              `json:"planned_amount"`
	PlannedVolume int64                `json:"planned_volume"`
	Method        Method               `json:"method"`
	Source        tradingconfig.Source `json:"source"`
	AmountReason  *AmountReason        `json:"amount_reason,omitempty"`
}

// PositionSizer computes a SizingProposal from injected Request + TradingConfig (never reads JSON or DB).
// PortfolioAwareSizer may read req.Portfolio; FixedAmountSizer must not.
type PositionSizer interface {
	Propose(req Request) SizingProposal
}
