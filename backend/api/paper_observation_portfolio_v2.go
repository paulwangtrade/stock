package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfolioinsight"
	"go-stock/backend/portfolioobservation"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/sectorcoverage"
	"go-stock/backend/sectorprovider"
)

// GET /api/papertrading/observation/portfolio-v2
// Read-only unified PortfolioObservationView. Never TradePlan / Order / Execution / Provider switch.
func (h *PaperObservationHandler) handlePortfolioObservationV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}

	var warnings []string
	tradeDate := strings.TrimSpace(r.URL.Query().Get("trade_date"))
	accountID := strings.TrimSpace(r.URL.Query().Get("account_id"))

	snap, err := portfolio.NewService().Snapshot(portfolio.SnapshotOptions{})
	if err != nil {
		warnings = append(warnings, "snapshot: "+err.Error())
	}

	var risk *portfoliorisk.PortfolioRiskSnapshot
	var holdings []string
	if snap != nil && snap.Found {
		for _, p := range snap.Positions {
			if p.Volume > 0 {
				holdings = append(holdings, p.StockCode)
			}
		}
		industry, meta, _ := sectorprovider.ForBuildInput(nil, holdings)
		risk = portfoliorisk.Build(portfoliorisk.BuildInput{
			Snapshot:         snap,
			TradeDate:        tradeDate,
			IndustryBySymbol: industry,
			IndustryTaxonomy: meta.Taxonomy,
		})
	} else {
		warnings = append(warnings, "snapshot_missing")
	}

	var insight *portfolioinsight.PortfolioInsight
	if risk != nil {
		insight = portfolioinsight.Build(portfolioinsight.Input{
			AsOf:      time.Now().UTC(),
			TradeDate: tradeDate,
			AccountID: accountID,
			Risk:      risk,
		})
	}

	// No live sector provider on this path → nil provider audit (fail-closed).
	cov := sectorcoverage.Audit(nil, sectorcoverage.AuditInput{
		AsOf:     time.Now().UTC(),
		Holdings: holdings,
	})

	acct := accountID
	if acct == "" && snap != nil && snap.AccountID != 0 {
		acct = strconv.FormatUint(uint64(snap.AccountID), 10)
	}

	view := portfolioobservation.Assemble(portfolioobservation.Input{
		AsOf:      time.Now().UTC(),
		TradeDate: tradeDate,
		AccountID: acct,
		Risk:      risk,
		Insight:   insight,
		SectorCov: cov,
		// Validation / SellSuggestion omitted on default GET (engines default OFF).
		Warnings: warnings,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"code":        0,
		"ok":          true,
		"observation": view,
		"flags": map[string]any{
			"read_only":           true,
			"not_auto_trade":      true,
			"not_a_trade_plan":    true,
			"not_order":           true,
			"not_execution":       true,
			"not_provider_switch": true,
		},
	})
}
