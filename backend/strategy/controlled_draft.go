package strategy

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/controlledadoption"
	"go-stock/backend/decisionprovider"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/risk"
	"go-stock/backend/selection"
)

// portfolioDecisionProviderForControlled is the G.8 write-chain Portfolio provider.
// Tests may replace it; production uses NewPortfolioDecisionProvider.
var portfolioDecisionProviderForControlled decisionprovider.DecisionProvider = decisionprovider.NewPortfolioDecisionProvider()

// loadControlledPortfolioSnapshot loads one Snapshot for Controlled Decide (G.2 single snap).
// Tests may replace it. Production uses paper_sim via portfolio.Service.
var loadControlledPortfolioSnapshot = func() *portfoliolayer.PortfolioSnapshot {
	var ledger *portfolio.Snapshot
	func() {
		defer func() { _ = recover() }()
		got, err := portfolio.NewService().Snapshot(portfolio.SnapshotOptions{})
		if err == nil {
			ledger = got
		}
	}()
	return portfoliolayer.FromLedger(ledger)
}

type controlledDraftRoute struct {
	Policy  controlledadoption.ControlledProviderPolicy
	Match   controlledadoption.MatchResult
	Snap    *portfoliolayer.PortfolioSnapshot
	Account uint
	Strat   string
}

func resolveControlledDraftRoute(pool *models.CandidatePool) controlledDraftRoute {
	policy := controlledadoption.Active()
	snap := loadControlledPortfolioSnapshot()
	accountID := uint(0)
	if snap != nil {
		accountID = snap.AccountID
	}
	strat, stratOK := consensusPoolStrategyName(pool)
	tradeDate := ""
	if pool != nil {
		tradeDate = strings.TrimSpace(pool.TradeDate)
	}
	match := controlledadoption.ResolveMatch(policy, controlledadoption.MatchInput{
		AccountID:    accountID,
		StrategyName: strat,
		TradeDate:    tradeDate,
	})
	// No consensus strategy name → never Portfolio (Legacy miss).
	if !stratOK {
		match = controlledadoption.MatchResult{Matched: false, Reason: controlledadoption.ReasonStrategyAmbiguous}
		if strings.ToLower(strings.TrimSpace(policy.Adoption)) != controlledadoption.AdoptionControlled || policy.KillSwitch {
			match.Reason = controlledadoption.ReasonAdoptionOff
			if policy.KillSwitch {
				match.Reason = controlledadoption.ReasonKillSwitch
			}
		}
	}
	return controlledDraftRoute{
		Policy:  policy,
		Match:   match,
		Snap:    snap,
		Account: accountID,
		Strat:   strat,
	}
}

func consensusPoolStrategyName(pool *models.CandidatePool) (string, bool) {
	if pool == nil {
		return "", false
	}
	var seen string
	var set bool
	for _, it := range pool.Items {
		n := strings.TrimSpace(it.StrategyName)
		if n == "" {
			continue
		}
		if !set {
			seen = n
			set = true
			continue
		}
		if !strings.EqualFold(seen, n) {
			return "", false
		}
	}
	return seen, set
}

// filterPoolWithControlledPortfolio runs Portfolio.Decide → PlanFilter.
// On Decide failure: returns error (caller must NOT fall back to Legacy for this vis).
func filterPoolWithControlledPortfolio(
	items []models.CandidatePoolItem,
	sel *selection.CandidateSelectionResult,
	snap *portfoliolayer.PortfolioSnapshot,
	maxNames int,
) (*risk.PlanFilterResult, float64, error) {
	if maxNames <= 0 {
		maxNames = defaultMaxPlanNames
	}
	prov := portfolioDecisionProviderForControlled
	if prov == nil {
		return nil, 0, fmt.Errorf("controlled portfolio provider is nil")
	}
	env, err := prov.Decide(decisionprovider.DecisionContext{
		Selection:    sel,
		DecisionTime: time.Now(),
		Snapshot:     snap,
		Version: decisionprovider.DecisionVersion{
			Contract:   decisionprovider.ContractG21,
			Provider:   decisionprovider.ProviderPortfolioAllocation,
			Allocation: decisionprovider.AllocVersionF1Equal,
		},
	})
	if err != nil {
		return nil, 0, fmt.Errorf("portfolio decide failed (no legacy fallback): %w", err)
	}
	if env == nil || !env.OK {
		code := "ERR_PORTFOLIO_NOT_OK"
		msg := "portfolio decision envelope is not ok"
		if env != nil && env.Error != nil {
			code = env.Error.Code
			msg = env.Error.Message
		}
		return nil, 0, fmt.Errorf("portfolio decide failed (no legacy fallback): %s: %s", code, msg)
	}

	cands := planCandidatesFromPortfolioEnvelope(items, sel, env)
	if len(cands) == 0 {
		return nil, 0, fmt.Errorf("portfolio decide failed (no legacy fallback): no positive allocation lines")
	}
	headerAmount := maxPositiveCandidateAmount(cands)
	if headerAmount <= 0 {
		return nil, 0, fmt.Errorf("portfolio decide failed (no legacy fallback): zero header amount")
	}

	ctx := planFilterContextFn(headerAmount, maxNames)
	if sel != nil {
		ctx.MaxNames = sel.SelectionLimit
	}
	filtered := risk.PlanFilter(cands, ctx)
	if filtered == nil {
		return nil, 0, fmt.Errorf("portfolio decide failed (no legacy fallback): plan filter returned nil")
	}
	logger.SugaredLogger.Infof(
		"controlled portfolio draft: lines=%d accepted=%d rejected=%d headerAmount=%.2f",
		len(cands), filtered.AcceptedCount, filtered.FilteredCount, headerAmount,
	)
	return filtered, headerAmount, nil
}

func planCandidatesFromPortfolioEnvelope(
	items []models.CandidatePoolItem,
	sel *selection.CandidateSelectionResult,
	env *decisionprovider.DecisionEnvelope,
) []risk.PlanCandidate {
	if sel == nil || env == nil {
		return nil
	}
	amtBy := map[string]float64{}
	for _, ln := range env.Lines {
		code := strings.ToLower(strings.TrimSpace(ln.Symbol))
		if code == "" || ln.TargetAmount <= 0 {
			continue
		}
		amtBy[code] = ln.TargetAmount
	}
	ranked := sel.RankedForPlanFilter()
	out := make([]risk.PlanCandidate, 0, len(ranked))
	for _, c := range ranked {
		code := strings.ToLower(strings.TrimSpace(c.StockCode))
		amt, ok := amtBy[code]
		if !ok || amt <= 0 {
			continue // G.8: only positive amounts enter PlanFilter
		}
		src := matchPoolItem(items, c)
		name := strings.TrimSpace(src.StockName)
		if name == "" {
			name = c.StockName
		}
		reason := strings.TrimSpace(src.Reason)
		if reason == "" {
			reason = c.Reason
		}
		stockCode := strings.TrimSpace(src.StockCode)
		if stockCode == "" {
			stockCode = c.StockCode
		}
		out = append(out, risk.PlanCandidate{
			StockCode:       stockCode,
			StockName:       name,
			Rank:            c.Rank,
			Score:           c.Score,
			Reason:          reason,
			StrategyName:    src.StrategyName,
			StrategyVersion: src.StrategyVersion,
			TargetAmount:    amt,
		})
	}
	return out
}

func maxPositiveCandidateAmount(cands []risk.PlanCandidate) float64 {
	max := 0.0
	for _, c := range cands {
		if c.TargetAmount > max {
			max = c.TargetAmount
		}
	}
	return max
}

func stampDraftProviderMetadata(plan *models.TradePlan, route controlledDraftRoute) {
	adoption := strings.ToLower(strings.TrimSpace(route.Policy.Adoption))
	if adoption == controlledadoption.AdoptionControlled {
		models.StampControlledBuyDraftProviderMetadata(plan, route.Match.Matched)
		return
	}
	models.StampLegacyBuyDraftProviderMetadata(plan)
}
