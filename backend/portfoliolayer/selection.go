package portfoliolayer

import (
	"strings"

	"go-stock/backend/selection"
)

// Portfolio-layer reasons (F.1). Never emit PlanFilter RiskCodes from this package.
const (
	ReasonInAllocationSet = "in_allocation_set"
	ReasonWaitlist        = "waitlist"
	ReasonAlreadyHolding  = "already_holding"
	ReasonSectorLimit     = "sector_limit"
	ReasonNameLimit       = "name_limit"
	ReasonNoSnapshot      = "no_snapshot"
)

var forbiddenRiskCodes = []string{
	"CASH_INSUFFICIENT",
	"GROSS_EXPOSURE_EXCEEDED",
	"MARKET_LEVEL_BLOCKED",
	"SINGLE_NAME_EXCEEDED",
	"DAILY_LOSS_EXCEEDED",
	"INVALID_ORDER",
}

// PortfolioConstraints is the F.1 input consumed by SelectPortfolio (already resolved).
type PortfolioConstraints struct {
	MaxNewNames        int
	SkipAlreadyHolding bool
	MaxSectorWeight    float64
	MaxNamesPerSector  int
	AllowAddToHolding  bool
}

// PortfolioSelectionInput is the F.1 selector input. Callers inject ranked_candidates; no DB.
type PortfolioSelectionInput struct {
	RankedCandidates []selection.Candidate
	Snapshot         *PortfolioSnapshot
	Constraints      PortfolioConstraints
}

// PortfolioPick is one name in the allocation set or waitlist.
type PortfolioPick struct {
	Candidate       selection.Candidate
	Rank            int
	PortfolioRank   int
	InAllocationSet bool
	Reason          string
}

// PortfolioReject is a name excluded from Filter scan_list.
type PortfolioReject struct {
	Candidate selection.Candidate
	Rank      int
	Reason    string
}

// PortfolioSelectionResult is the F.1 "who" output. It does not set amounts or RiskCodes.
type PortfolioSelectionResult struct {
	Selected []PortfolioPick
	Waitlist []PortfolioPick
	Rejected []PortfolioReject
	Reasons  []string
}

// ScanList is the future PlanFilter sequence: selected ∥ waitlist (R1-compatible, not a closed TopN).
func (r *PortfolioSelectionResult) ScanList() []selection.Candidate {
	if r == nil {
		return nil
	}
	out := make([]selection.Candidate, 0, len(r.Selected)+len(r.Waitlist))
	for _, p := range r.Selected {
		out = append(out, p.Candidate)
	}
	for _, p := range r.Waitlist {
		out = append(out, p.Candidate)
	}
	return out
}

// AllocationSetCodes returns codes with in_allocation_set=true.
func (r *PortfolioSelectionResult) AllocationSetCodes() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.Selected))
	for _, p := range r.Selected {
		out = append(out, p.Candidate.StockCode)
	}
	return out
}

// SelectPortfolio assigns ranked candidates into selected / waitlist / rejected.
// Skeleton: name cap, optional skip-held, optional sector caps. No cash, no RiskCode, no amounts.
func SelectPortfolio(in PortfolioSelectionInput) *PortfolioSelectionResult {
	out := &PortfolioSelectionResult{
		Selected: []PortfolioPick{},
		Waitlist: []PortfolioPick{},
		Rejected: []PortfolioReject{},
		Reasons:  []string{},
	}
	maxNames := in.Constraints.MaxNewNames
	if maxNames <= 0 {
		maxNames = DefaultMaxNewNames
	}

	noSnap := !isUsableSnapshot(in.Snapshot)
	if noSnap {
		out.Reasons = append(out.Reasons, ReasonNoSnapshot)
	}

	holdings := in.Snapshot.HoldingCodes()
	skipHeld := in.Constraints.SkipAlreadyHolding
	sectorCount := map[string]int{}
	selectedN := 0

	for _, c := range in.RankedCandidates {
		rank := c.Rank
		code := strings.ToLower(strings.TrimSpace(c.StockCode))
		industry := strings.TrimSpace(c.Industry)

		if noSnap {
			out.Waitlist = append(out.Waitlist, waitPick(c, rank, len(out.Waitlist)+1, ReasonNoSnapshot))
			continue
		}

		if skipHeld && code != "" {
			if _, held := holdings[code]; held {
				out.Rejected = append(out.Rejected, PortfolioReject{Candidate: c, Rank: rank, Reason: ReasonAlreadyHolding})
				continue
			}
		}

		if in.Constraints.MaxNamesPerSector > 0 && industry != "" {
			if sectorCount[industry] >= in.Constraints.MaxNamesPerSector {
				out.Waitlist = append(out.Waitlist, waitPick(c, rank, len(out.Waitlist)+1, ReasonSectorLimit))
				continue
			}
		}
		if in.Constraints.MaxSectorWeight > 0 && industry != "" {
			if in.Snapshot.SectorWeight(industry) >= in.Constraints.MaxSectorWeight {
				out.Waitlist = append(out.Waitlist, waitPick(c, rank, len(out.Waitlist)+1, ReasonSectorLimit))
				continue
			}
		}

		if selectedN >= maxNames {
			out.Waitlist = append(out.Waitlist, waitPick(c, rank, len(out.Waitlist)+1, ReasonNameLimit))
			continue
		}

		selectedN++
		if industry != "" {
			sectorCount[industry]++
		}
		out.Selected = append(out.Selected, PortfolioPick{
			Candidate:       c,
			Rank:            rank,
			PortfolioRank:   selectedN,
			InAllocationSet: true,
			Reason:          ReasonInAllocationSet,
		})
	}

	if skipHeld {
		out.Reasons = appendUnique(out.Reasons, "skip_already_holding")
	}
	return out
}

func waitPick(c selection.Candidate, rank, portfolioRank int, reason string) PortfolioPick {
	return PortfolioPick{
		Candidate:       c,
		Rank:            rank,
		PortfolioRank:   portfolioRank,
		InAllocationSet: false,
		Reason:          reason,
	}
}

func appendUnique(dst []string, v string) []string {
	for _, x := range dst {
		if x == v {
			return dst
		}
	}
	return append(dst, v)
}

func usesForbiddenRiskCode(reason string) bool {
	u := strings.ToUpper(strings.TrimSpace(reason))
	for _, code := range forbiddenRiskCodes {
		if u == code {
			return true
		}
	}
	return false
}
