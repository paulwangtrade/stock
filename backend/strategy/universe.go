package strategy

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

const (
	defaultMaxCandidates = 30
	defaultMaxPlanNames  = 5
)

// UniverseCandidate 构建候选池前的中间项（不落库）。
type UniverseCandidate struct {
	StockCode       string
	StockName       string
	Industry        string
	Score           float64
	Reason          string
	StrategyName    string
	StrategyVersion string
}

type universeBuildResult struct {
	Source    string
	SourceRef string
	Items     []UniverseCandidate
	Message   string
}

func todayTradeDate() string {
	return time.Now().Format("2006-01-02")
}

func normalizeTradeDate(tradeDate string) string {
	tradeDate = strings.TrimSpace(tradeDate)
	if tradeDate == "" {
		return todayTradeDate()
	}
	if _, err := time.Parse("2006-01-02", tradeDate); err != nil {
		return todayTradeDate()
	}
	return tradeDate
}

func toSinaAShareCode(raw string) (string, error) {
	n, err := data.NormalizeStockCode(raw)
	if err != nil {
		return "", err
	}
	if n.Market != data.MarketCN {
		return "", fmt.Errorf("not CN: %s", raw)
	}
	code := strings.ToLower(strings.TrimSpace(n.SinaCode))
	if !data.IsAShareSinaCode(code) {
		return "", fmt.Errorf("invalid sina A-share: %s", code)
	}
	return code, nil
}

func isSTName(name string) bool {
	u := strings.ToUpper(strings.TrimSpace(name))
	return strings.HasPrefix(u, "*ST") || strings.HasPrefix(u, "ST") || strings.HasPrefix(u, "S*ST")
}

// collectUniverse 优先启用 StockStrategy 最新 run，否则自选（经 data API，不直连 DB）。
func collectUniverse() universeBuildResult {
	api := data.NewStockStrategyApi()
	if strat, err := api.GetFirstEnabled(); err == nil && strat != nil {
		if run, rerr := api.GetLatestRun(strat.ID); rerr == nil && run != nil && run.ResultJSON != "" {
			items := parseStrategyRunItems(strat, run)
			if len(items) > 0 {
				return universeBuildResult{
					Source:    models.CandidatePoolSourceStrategyRun,
					SourceRef: fmt.Sprintf("strategyId=%d;runId=%d", strat.ID, run.ID),
					Items:     items,
					Message:   fmt.Sprintf("from strategy %q run=%d count=%d", strat.Name, run.ID, len(items)),
				}
			}
			logger.SugaredLogger.Warnf("strategy run %d empty, fallback follow", run.ID)
		}
	}

	items := loadFollowCandidates()
	return universeBuildResult{
		Source:    models.CandidatePoolSourceFollow,
		SourceRef: "follow",
		Items:     items,
		Message:   fmt.Sprintf("from follow list count=%d", len(items)),
	}
}

func parseStrategyRunItems(strat *models.StockStrategy, run *models.StockStrategyRun) []UniverseCandidate {
	var view models.StockStrategyRunView
	if err := json.Unmarshal([]byte(run.ResultJSON), &view); err != nil {
		logger.SugaredLogger.Warnf("parse strategy run json: %v", err)
		return nil
	}
	version := fmt.Sprintf("run:%d", run.ID)
	name := strat.Name
	if name == "" {
		name = fmt.Sprintf("strategy_%d", strat.ID)
	}

	rawList, err := json.Marshal(view.DataList)
	if err != nil || len(rawList) == 0 || string(rawList) == "null" {
		return nil
	}

	var rows []map[string]any
	if err := json.Unmarshal(rawList, &rows); err != nil {
		return nil
	}

	out := make([]UniverseCandidate, 0, len(rows))
	seen := map[string]bool{}
	for i, row := range rows {
		codeRaw := firstString(row, "SECUCODE", "secucode", "SECURITY_CODE", "security_code", "stockCode", "StockCode", "code", "Code")
		nameRaw := firstString(row, "SECURITY_NAME_ABBR", "security_name_abbr", "stockName", "StockName", "name", "Name")
		industry := firstString(row, "INDUSTRY", "industry")
		sina, nerr := toSinaAShareCode(codeRaw)
		if nerr != nil {
			continue
		}
		if isSTName(nameRaw) {
			continue
		}
		if seen[sina] {
			continue
		}
		seen[sina] = true
		out = append(out, UniverseCandidate{
			StockCode:       sina,
			StockName:       nameRaw,
			Industry:        industry,
			Score:           float64(len(rows) - i),
			Reason:          "strategy_run",
			StrategyName:    name,
			StrategyVersion: version,
		})
	}
	return out
}

func loadFollowCandidates() []UniverseCandidate {
	list := data.NewStockDataApi().GetFollowList(0)
	if list == nil {
		return nil
	}
	out := make([]UniverseCandidate, 0, len(*list))
	seen := map[string]bool{}
	version := fmt.Sprintf("follow:%s", todayTradeDate())
	for i, f := range *list {
		sina, err := toSinaAShareCode(f.StockCode)
		if err != nil {
			continue
		}
		if isSTName(f.Name) {
			continue
		}
		if seen[sina] {
			continue
		}
		seen[sina] = true
		out = append(out, UniverseCandidate{
			StockCode:       sina,
			StockName:       f.Name,
			Score:           float64(len(*list) - i),
			Reason:          "follow",
			StrategyName:    "follow",
			StrategyVersion: version,
		})
	}
	return out
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch t := v.(type) {
			case string:
				if s := strings.TrimSpace(t); s != "" {
					return s
				}
			default:
				s := strings.TrimSpace(fmt.Sprint(t))
				if s != "" && s != "<nil>" {
					return s
				}
			}
		}
	}
	return ""
}

func truncateCandidates(items []UniverseCandidate, max int) []UniverseCandidate {
	if max <= 0 {
		max = defaultMaxCandidates
	}
	if len(items) <= max {
		return items
	}
	return items[:max]
}
