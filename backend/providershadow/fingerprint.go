package providershadow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"time"

	"go-stock/backend/decisionprovider"
	"go-stock/backend/selection"
)

func formatAmount(v float64) string {
	return strconv.FormatFloat(v, 'f', 6, 64)
}

func contextFingerprint(ctx decisionprovider.DecisionContext) string {
	type cand struct {
		Symbol string `json:"symbol"`
		Rank   int    `json:"rank"`
		Score  string `json:"score"`
	}
	type pos struct {
		Symbol string `json:"symbol"`
		MV     string `json:"market_value"`
		Vol    int64  `json:"volume"`
	}
	payload := struct {
		DecisionTime      string `json:"decision_time"`
		UniformAmount     string `json:"uniform_amount"`
		SelectionLimit    int    `json:"selection_limit"`
		Ranked            []cand `json:"ranked"`
		SnapshotFound     bool   `json:"snapshot_found"`
		AccountID         uint   `json:"account_id"`
		Cash              string `json:"cash"`
		Equity            string `json:"equity"`
		AvailableCash     string `json:"available_cash"`
		MarketValue       string `json:"market_value"`
		Positions         []pos  `json:"positions"`
		ConstraintVersion string `json:"constraint_version"`
		AllocationVersion string `json:"allocation_version"`
		MaxNewNames       int    `json:"max_new_names"`
		SkipHolding       bool   `json:"skip_already_holding"`
		AllowAdd          bool   `json:"allow_add_to_holding"`
		BudgetPresent     bool   `json:"budget_present"`
		BudgetCapital     string `json:"budget_available_capital,omitempty"`
	}{
		DecisionTime:      ctx.DecisionTime.UTC().Format(time.RFC3339Nano),
		UniformAmount:     formatAmount(ctx.UniformAmount),
		ConstraintVersion: constraintVersionOf(ctx),
		AllocationVersion: allocationVersionOf(ctx),
		BudgetPresent:     ctx.Budget != nil,
	}
	if ctx.Selection != nil {
		payload.SelectionLimit = ctx.Selection.SelectionLimit
		ranked := ctx.Selection.RankedCandidates
		if ranked == nil {
			ranked = []selection.Candidate{}
		}
		payload.Ranked = make([]cand, 0, len(ranked))
		for _, c := range ranked {
			payload.Ranked = append(payload.Ranked, cand{
				Symbol: normSymbol(c.StockCode),
				Rank:   c.Rank,
				Score:  formatAmount(c.Score),
			})
		}
	}
	if ctx.Snapshot != nil {
		payload.SnapshotFound = ctx.Snapshot.Found
		payload.AccountID = ctx.Snapshot.AccountID
		payload.Cash = formatAmount(ctx.Snapshot.Cash)
		payload.Equity = formatAmount(ctx.Snapshot.Equity)
		payload.AvailableCash = formatAmount(ctx.Snapshot.AvailableCash)
		payload.MarketValue = formatAmount(ctx.Snapshot.MarketValue)
		payload.Positions = make([]pos, 0, len(ctx.Snapshot.Positions))
		for _, p := range ctx.Snapshot.Positions {
			payload.Positions = append(payload.Positions, pos{
				Symbol: normSymbol(p.StockCode),
				MV:     formatAmount(p.MarketValue),
				Vol:    p.Volume,
			})
		}
	}
	resolved := ctx.Constraints.Resolve()
	payload.MaxNewNames = resolved.MaxNewNames
	payload.SkipHolding = resolved.SkipAlreadyHolding
	payload.AllowAdd = resolved.AllowAddToHolding
	if ctx.Budget != nil {
		payload.BudgetCapital = formatAmount(ctx.Budget.AvailableCapital)
	}
	return hashJSON(payload)
}

func reportFingerprint(rec ShadowComparisonRecord, report *ProviderComparisonReport) string {
	payload := struct {
		DecisionTime      string            `json:"decision_time"`
		ProviderVersion   string            `json:"provider_version"`
		ConstraintVersion string            `json:"constraint_version"`
		AllocationVersion string            `json:"allocation_version"`
		LegacyOK          bool              `json:"legacy_ok"`
		PortfolioOK       bool              `json:"portfolio_ok"`
		LegacyError       string            `json:"legacy_error,omitempty"`
		PortfolioError    string            `json:"portfolio_error,omitempty"`
		OnlyLegacy        []string          `json:"only_legacy"`
		OnlyPortfolio     []string          `json:"only_portfolio"`
		Common            []string          `json:"common"`
		Amounts           []amountCanon     `json:"amounts"`
		Roles             []RoleDiff        `json:"roles"`
		Reasons           []ReasonDiff      `json:"reasons"`
		Metrics           ComparisonMetrics `json:"metrics"`
		ComparisonSummary ComparisonSummary `json:"comparison_summary"`
		Failure           ShadowFailure     `json:"failure"`
	}{
		DecisionTime:      rec.DecisionTime.UTC().Format(time.RFC3339Nano),
		ProviderVersion:   rec.ProviderVersion,
		ConstraintVersion: rec.ConstraintVersion,
		AllocationVersion: rec.AllocationVersion,
		ComparisonSummary: rec.ComparisonSummary,
		Failure:           rec.Failure,
	}
	if report != nil {
		payload.LegacyOK = report.Legacy.OK
		payload.PortfolioOK = report.Portfolio.OK
		payload.LegacyError = report.Legacy.ErrorCode
		payload.PortfolioError = report.Portfolio.ErrorCode
		payload.OnlyLegacy = report.Symbols.OnlyLegacy
		payload.OnlyPortfolio = report.Symbols.OnlyPortfolio
		payload.Common = report.Symbols.Common
		payload.Amounts = canonAmounts(report.Amounts)
		payload.Roles = report.Roles
		payload.Reasons = report.Reasons
		payload.Metrics = report.Metrics
	}
	return hashJSON(payload)
}

type amountCanon struct {
	Symbol            string `json:"symbol"`
	Kind              string `json:"kind"`
	LegacyPresence    string `json:"legacy_presence"`
	PortfolioPresence string `json:"portfolio_presence"`
	LegacyValue       string `json:"legacy_value,omitempty"`
	PortfolioValue    string `json:"portfolio_value,omitempty"`
	Delta             string `json:"delta,omitempty"`
}

func canonAmounts(in []AmountDiff) []amountCanon {
	out := make([]amountCanon, 0, len(in))
	for _, a := range in {
		row := amountCanon{
			Symbol:            a.Symbol,
			Kind:              a.Kind,
			LegacyPresence:    a.Legacy.Presence,
			PortfolioPresence: a.Portfolio.Presence,
		}
		if a.Legacy.Presence != PresenceAbsent && a.Legacy.Value != nil {
			row.LegacyValue = formatAmount(*a.Legacy.Value)
		}
		if a.Portfolio.Presence != PresenceAbsent && a.Portfolio.Value != nil {
			row.PortfolioValue = formatAmount(*a.Portfolio.Value)
		}
		if a.Delta != nil {
			row.Delta = formatAmount(*a.Delta)
		}
		out = append(out, row)
	}
	return out
}

func hashJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func constraintVersionOf(ctx decisionprovider.DecisionContext) string {
	if ctx.Version.Constraint != "" {
		return ctx.Version.Constraint
	}
	return ConstraintVersionF21
}

func allocationVersionOf(ctx decisionprovider.DecisionContext) string {
	if ctx.Version.Allocation != "" {
		return ctx.Version.Allocation
	}
	return decisionprovider.AllocVersionF1Equal
}

func summaryFromReport(report *ProviderComparisonReport) ComparisonSummary {
	if report == nil {
		return ComparisonSummary{}
	}
	return ComparisonSummary{
		OnlyLegacyCount:    report.Metrics.OnlyLegacyCount,
		OnlyPortfolioCount: report.Metrics.OnlyPortfolioCount,
		CommonCount:        report.Metrics.CommonCount,
		AmountDeltaCount:   report.Metrics.AmountDiffCount,
		RoleChangeCount:    report.Metrics.RoleDiffCount,
	}
}

func failureFromReport(report *ProviderComparisonReport) ShadowFailure {
	if report == nil {
		return ShadowFailure{}
	}
	if report.IncomparableReason != IncomparablePortfolioFailed && report.IncomparableReason != IncomparableBothFailed {
		return ShadowFailure{}
	}
	reason := report.Portfolio.ErrorCode
	if report.Portfolio.ErrorMsg != "" {
		if reason != "" {
			reason = reason + ": " + report.Portfolio.ErrorMsg
		} else {
			reason = report.Portfolio.ErrorMsg
		}
	}
	return ShadowFailure{PortfolioFailed: true, ErrorReason: reason}
}
