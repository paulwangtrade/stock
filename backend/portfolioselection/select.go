// Package portfolioselection is the Portfolio Selector construction layer (Phase13).
//
// Pure functions only: no DB, TradePlan persist, PlanFilter, Execution, or Broker.
// Default consumers are observation / Shadow; write-chain wiring is not in this package.
package portfolioselection

import (
	"math"
	"sort"
	"strings"
)

const SchemaVersion = "portfolio_selector.v1"

// Policy drives construction. Zero values fall back to DefaultPolicy.
type Policy struct {
	PolicyID              string  `json:"policy_id"`
	MaxNames              int     `json:"max_names"`
	MaxSingleWeight       float64 `json:"max_single_weight"`
	MaxSectorWeight       float64 `json:"max_sector_weight"` // <=0 disables
	MaxNamesPerSector     int     `json:"max_names_per_sector"`
	ReserveCashRatio      float64 `json:"reserve_cash_ratio"`
	MinOrderAmount        float64 `json:"min_order_amount"`
	HoldingMode           string  `json:"holding_mode"` // allow_add | skip_holding | prefer_new
	IncludeHoldingsSector bool    `json:"include_holdings_in_sector_cap"`
	IncludeHoldingsSingle bool    `json:"include_holdings_in_single_cap"`
	AmountMethod          string  `json:"amount_method"` // equal_weight
}

// DefaultPolicy matches design v1 defaults (max_names=5, allow_add, equal_weight).
func DefaultPolicy() Policy {
	return Policy{
		PolicyID:              "portfolio_selector.v1.default",
		MaxNames:              5,
		MaxSingleWeight:       0.20,
		MaxSectorWeight:       0.30,
		MaxNamesPerSector:     0,
		ReserveCashRatio:      0.05,
		MinOrderAmount:        1000,
		HoldingMode:           "allow_add",
		IncludeHoldingsSector: true,
		IncludeHoldingsSingle: true,
		AmountMethod:          "equal_weight",
	}
}

// Constraints are injected runtime facts (caller supplies; package does not query DB).
type Constraints struct {
	AvailableCash   float64 `json:"available_cash"`
	EquityBase      float64 `json:"equity_base"`
	BuyBudget       float64 `json:"buy_budget"` // 0 => use available_cash
	BlockNewEntries bool    `json:"block_new_entries"`
}

// PoolItem is a read-only candidate row.
type PoolItem struct {
	StockCode string  `json:"stock_code"`
	StockName string  `json:"stock_name"`
	Industry  string  `json:"industry"`
	Rank      int     `json:"rank"`
	Score     float64 `json:"score"`
	Reason    string  `json:"reason"`
}

// Position is an injected holding.
type Position struct {
	StockCode   string  `json:"stock_code"`
	MarketValue float64 `json:"market_value"`
	Industry    string  `json:"industry"`
}

// Snapshot is a read-only book projection.
type Snapshot struct {
	Found       bool       `json:"found"`
	Cash        float64    `json:"cash"`
	Equity      float64    `json:"equity"`
	MarketValue float64    `json:"market_value"`
	Positions   []Position `json:"positions"`
}

// Input for Select.
type Input struct {
	TradeDate   string      `json:"trade_date"`
	PoolID      uint        `json:"pool_id"`
	Items       []PoolItem  `json:"items"`
	Snapshot    *Snapshot   `json:"snapshot,omitempty"`
	Policy      Policy      `json:"policy"`
	Constraints Constraints `json:"constraints"`
}

// SelectedName is one in-basket construction line.
type SelectedName struct {
	StockCode        string  `json:"stock_code"`
	StockName        string  `json:"stock_name"`
	Industry         string  `json:"industry"`
	PoolRank         int     `json:"pool_rank"`
	SelectionRank    int     `json:"selection_rank"`
	Score            float64 `json:"score"`
	TargetWeight     float64 `json:"target_weight"`
	TargetAmount     float64 `json:"target_amount"`
	AllocationReason string  `json:"allocation_reason"`
	AlreadyHolding   bool    `json:"already_holding"`
	WeightCapped     bool    `json:"weight_capped"`
	CashLimited      bool    `json:"cash_limited"`
}

// RejectedName is a construction reject (not PlanFilter skipped).
type RejectedName struct {
	StockCode     string  `json:"stock_code"`
	PoolRank      int     `json:"pool_rank"`
	Score         float64 `json:"score"`
	RejectReason  string  `json:"reject_reason"`
	RejectMessage string  `json:"reject_message"`
}

// Summary aggregates construction outcomes.
type Summary struct {
	MaxNames          int                `json:"max_names"`
	SelectedCount     int                `json:"selected_count"`
	RejectedCount     int                `json:"rejected_count"`
	TotalTargetAmount float64            `json:"total_target_amount"`
	TotalTargetWeight float64            `json:"total_target_weight"`
	CashUsed          float64            `json:"cash_used"`
	CashRemaining     float64            `json:"cash_remaining"`
	SectorWeights     map[string]float64 `json:"sector_weights"`
	Binding           string             `json:"binding"`
}

// SelectedPortfolio is the closed construction set.
type SelectedPortfolio struct {
	SchemaVersion    string         `json:"schema_version"`
	TradeDate        string         `json:"trade_date"`
	PoolID           uint           `json:"pool_id"`
	PolicyID         string         `json:"policy_id"`
	RankedInputCount int            `json:"ranked_input_count"`
	Selected         []SelectedName `json:"selected"`
	Rejected         []RejectedName `json:"rejected"`
	Summary          Summary        `json:"summary"`
}

// Select builds a closed SelectedPortfolio (hard max_names; not E.6 waitlist scan).
func Select(in Input) *SelectedPortfolio {
	pol := normalizePolicy(in.Policy)
	out := &SelectedPortfolio{
		SchemaVersion: SchemaVersion,
		TradeDate:     strings.TrimSpace(in.TradeDate),
		PoolID:        in.PoolID,
		PolicyID:      pol.PolicyID,
		Selected:      []SelectedName{},
		Rejected:      []RejectedName{},
		Summary: Summary{
			MaxNames:      pol.MaxNames,
			SectorWeights: map[string]float64{},
			Binding:       "none",
		},
	}

	work := rankAndDedup(in.Items)
	out.RankedInputCount = len(work)

	if in.Constraints.BlockNewEntries {
		for _, c := range work {
			out.Rejected = append(out.Rejected, RejectedName{
				StockCode: c.StockCode, PoolRank: c.Rank, Score: c.Score,
				RejectReason: "block_new_entries", RejectMessage: "construction blocked new entries",
			})
		}
		out.Summary.RejectedCount = len(out.Rejected)
		out.Summary.Binding = "blocked"
		return out
	}

	holdings := map[string]Position{}
	sectorMV := map[string]float64{}
	nameMV := map[string]float64{}
	if in.Snapshot != nil {
		for _, p := range in.Snapshot.Positions {
			code := normCode(p.StockCode)
			if code == "" {
				continue
			}
			holdings[code] = p
			nameMV[code] = p.MarketValue
			if pol.IncludeHoldingsSector {
				ind := industryOrUnknown(p.Industry)
				sectorMV[ind] += p.MarketValue
			}
		}
	}

	if pol.HoldingMode == "prefer_new" {
		sort.SliceStable(work, func(i, j int) bool {
			hi := holdings[normCode(work[i].StockCode)]
			hj := holdings[normCode(work[j].StockCode)]
			iHold := hi.StockCode != "" || hi.MarketValue > 0
			jHold := hj.StockCode != "" || hj.MarketValue > 0
			if iHold != jHold {
				return !iHold && jHold
			}
			return lessPool(work[i], work[j])
		})
	}

	equity := in.Constraints.EquityBase
	if equity <= 0 && in.Snapshot != nil {
		equity = in.Snapshot.Equity
	}
	cash := in.Constraints.AvailableCash
	if cash <= 0 && in.Snapshot != nil && in.Snapshot.Found {
		cash = in.Snapshot.Cash * (1 - pol.ReserveCashRatio)
		if cash < 0 {
			cash = 0
		}
	}
	budget := cash
	if in.Constraints.BuyBudget > 0 && in.Constraints.BuyBudget < budget {
		budget = in.Constraints.BuyBudget
	}

	K := pol.MaxNames
	if K <= 0 {
		K = 5
	}
	unit := 0.0
	if K > 0 && budget > 0 {
		unit = budget / float64(K)
	}
	method := strings.TrimSpace(pol.AmountMethod)
	if method == "" {
		method = "equal_weight"
	}

	cashLeft := budget
	binding := "none"

	for _, c := range work {
		code := normCode(c.StockCode)
		if code == "" {
			out.Rejected = append(out.Rejected, RejectedName{
				StockCode: c.StockCode, PoolRank: c.Rank, Score: c.Score,
				RejectReason: "invalid_code", RejectMessage: "empty stock code",
			})
			continue
		}
		if len(out.Selected) >= K {
			out.Rejected = append(out.Rejected, RejectedName{
				StockCode: code, PoolRank: c.Rank, Score: c.Score,
				RejectReason: "over_max_names", RejectMessage: "exceeded max_names",
			})
			binding = "max_names"
			continue
		}

		_, holding := holdings[code]
		if pol.HoldingMode == "skip_holding" && holding {
			out.Rejected = append(out.Rejected, RejectedName{
				StockCode: code, PoolRank: c.Rank, Score: c.Score,
				RejectReason: "already_holding", RejectMessage: "holding_mode=skip_holding",
			})
			continue
		}

		amt := unit
		reason := "rank_top_equal_weight"
		weightCapped := false
		cashLimited := false
		if holding && pol.HoldingMode == "allow_add" {
			reason = "add_to_holding"
		}
		if pol.HoldingMode == "prefer_new" && !holding {
			reason = "prefer_new_rank"
		}

		if equity > 0 && pol.MaxSingleWeight > 0 {
			capAmt := pol.MaxSingleWeight * equity
			if pol.IncludeHoldingsSingle {
				capAmt = math.Max(0, pol.MaxSingleWeight*equity-nameMV[code])
			}
			if amt > capAmt {
				amt = capAmt
				weightCapped = true
				reason = "weight_capped_equal"
			}
		}
		if amt > cashLeft {
			amt = cashLeft
			cashLimited = true
			reason = "cash_trimmed"
			binding = "cash"
		}
		if amt < pol.MinOrderAmount || amt <= 0 {
			why := "below_min_order"
			if weightCapped {
				why = "single_weight_cap"
			}
			if cashLimited || cashLeft < pol.MinOrderAmount {
				why = "insufficient_cash"
				binding = "cash"
			}
			out.Rejected = append(out.Rejected, RejectedName{
				StockCode: code, PoolRank: c.Rank, Score: c.Score,
				RejectReason: why, RejectMessage: "target amount below min_order",
			})
			continue
		}

		ind := industryOrUnknown(c.Industry)
		if pol.MaxSectorWeight > 0 && equity > 0 {
			proj := sectorMV[ind] + amt
			if proj/equity > pol.MaxSectorWeight+1e-12 {
				out.Rejected = append(out.Rejected, RejectedName{
					StockCode: code, PoolRank: c.Rank, Score: c.Score,
					RejectReason: "sector_cap", RejectMessage: "sector weight would exceed max_sector_weight",
				})
				binding = "sector"
				continue
			}
		}
		if pol.MaxNamesPerSector > 0 {
			n := 0
			for _, s := range out.Selected {
				if industryOrUnknown(s.Industry) == ind {
					n++
				}
			}
			if n >= pol.MaxNamesPerSector {
				out.Rejected = append(out.Rejected, RejectedName{
					StockCode: code, PoolRank: c.Rank, Score: c.Score,
					RejectReason: "sector_name_cap", RejectMessage: "sector name count cap",
				})
				binding = "sector"
				continue
			}
		}

		w := 0.0
		if equity > 0 {
			w = amt / equity
		}
		out.Selected = append(out.Selected, SelectedName{
			StockCode:        code,
			StockName:        strings.TrimSpace(c.StockName),
			Industry:         ind,
			PoolRank:         c.Rank,
			SelectionRank:    len(out.Selected) + 1,
			Score:            c.Score,
			TargetWeight:     w,
			TargetAmount:     amt,
			AllocationReason: reason,
			AlreadyHolding:   holding,
			WeightCapped:     weightCapped,
			CashLimited:      cashLimited,
		})
		cashLeft -= amt
		sectorMV[ind] += amt
		nameMV[code] += amt
	}

	totalAmt := 0.0
	totalW := 0.0
	secW := map[string]float64{}
	for _, s := range out.Selected {
		totalAmt += s.TargetAmount
		totalW += s.TargetWeight
		secW[s.Industry] += s.TargetWeight
	}
	out.Summary.SelectedCount = len(out.Selected)
	out.Summary.RejectedCount = len(out.Rejected)
	out.Summary.TotalTargetAmount = totalAmt
	out.Summary.TotalTargetWeight = totalW
	out.Summary.CashUsed = budget - cashLeft
	out.Summary.CashRemaining = cashLeft
	out.Summary.SectorWeights = secW
	if binding != "none" {
		out.Summary.Binding = binding
	}
	_ = method
	return out
}

func normalizePolicy(p Policy) Policy {
	d := DefaultPolicy()
	if strings.TrimSpace(p.PolicyID) == "" {
		p.PolicyID = d.PolicyID
	}
	if p.MaxNames <= 0 {
		p.MaxNames = d.MaxNames
	}
	if p.MaxSingleWeight <= 0 {
		p.MaxSingleWeight = d.MaxSingleWeight
	}
	if p.MinOrderAmount <= 0 {
		p.MinOrderAmount = d.MinOrderAmount
	}
	if strings.TrimSpace(p.HoldingMode) == "" {
		p.HoldingMode = d.HoldingMode
	}
	if strings.TrimSpace(p.AmountMethod) == "" {
		p.AmountMethod = d.AmountMethod
	}
	return p
}

func rankAndDedup(items []PoolItem) []PoolItem {
	type row struct {
		PoolItem
		idx int
	}
	rows := make([]row, 0, len(items))
	for i, it := range items {
		rows = append(rows, row{PoolItem: it, idx: i})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return lessPool(rows[i].PoolItem, rows[j].PoolItem)
	})
	seen := map[string]bool{}
	out := make([]PoolItem, 0, len(rows))
	for _, r := range rows {
		code := normCode(r.StockCode)
		if code == "" {
			continue
		}
		if seen[code] {
			continue
		}
		seen[code] = true
		it := r.PoolItem
		it.StockCode = code
		out = append(out, it)
	}
	return out
}

func lessPool(a, b PoolItem) bool {
	ar, br := a.Rank, b.Rank
	aRanked := ar > 0
	bRanked := br > 0
	switch {
	case aRanked && bRanked && ar != br:
		return ar < br
	case aRanked != bRanked:
		return aRanked
	case a.Score != b.Score:
		return a.Score > b.Score
	default:
		return normCode(a.StockCode) < normCode(b.StockCode)
	}
}

func normCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

func industryOrUnknown(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	return s
}
