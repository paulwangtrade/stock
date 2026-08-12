package obsfeedback

import (
	"fmt"
	"strings"
	"unicode"

	"go-stock/backend/portfolioobs"
)

// CaptureFromObservation projects today's Decision rows into Observation Records.
// Pure: does not write DB or paper_sim_*.
func CaptureFromObservation(obs *portfolioobs.Observation) []Record {
	if obs == nil {
		return []Record{}
	}
	date := observationDate(obs)
	out := make([]Record, 0, len(obs.Positions))
	for _, row := range obs.Positions {
		symbol := strings.TrimSpace(row.Symbol)
		if symbol == "" {
			continue
		}
		state := strings.ToUpper(strings.TrimSpace(row.DecisionState))
		if state == "" {
			continue
		}
		rec := Record{
			ObservationID:   fmt.Sprintf("obs-%s-%s", date, symbol),
			Symbol:          symbol,
			Market:          marketFromSymbol(symbol),
			ObservationTime: strings.TrimSpace(obs.ObservationTime),
			ObservationDate: date,
			DecisionState:   state,
			DecisionReason:  strings.TrimSpace(row.DecisionReason),
			HoldingDays:     row.HoldingDays,
			Source:          SourcePortfolioObservation,
		}
		if rec.ObservationTime == "" {
			rec.ObservationTime = obs.AsOf
		}
		if row.HealthScore != nil {
			v := *row.HealthScore
			rec.HealthScore = &v
		}
		if row.Cost != nil {
			v := *row.Cost
			rec.CostPrice = &v
		}
		if row.CurrentPrice != nil {
			v := *row.CurrentPrice
			rec.MarketPrice = &v
		}
		if row.Return != nil {
			v := *row.Return
			rec.UnrealizedReturn = &v
		}
		if row.CurrentWeight != 0 {
			v := row.CurrentWeight
			rec.PortfolioWeight = &v
		}
		out = append(out, rec)
	}
	return out
}

func observationDate(obs *portfolioobs.Observation) string {
	if obs == nil {
		return ""
	}
	for _, s := range []string{obs.AsOf, obs.ObservationTime} {
		if d := datePrefix(s); d != "" {
			return d
		}
	}
	return ""
}

func datePrefix(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10]
	}
	return ""
}

func marketFromSymbol(symbol string) string {
	s := strings.ToLower(strings.TrimSpace(symbol))
	switch {
	case strings.HasPrefix(s, "sh") || strings.HasPrefix(s, "6"):
		return "SH"
	case strings.HasPrefix(s, "sz") || strings.HasPrefix(s, "0") || strings.HasPrefix(s, "3"):
		return "SZ"
	case strings.HasPrefix(s, "bj") || strings.HasPrefix(s, "8") || strings.HasPrefix(s, "4"):
		return "BJ"
	}
	for _, r := range s {
		if unicode.IsLetter(r) {
			return strings.ToUpper(s[:2])
		}
		break
	}
	return ""
}

// NormalizeHorizon returns 1, 5, or 10 (default 5).
func NormalizeHorizon(h int) int {
	switch h {
	case HorizonT1, HorizonT5, HorizonT10:
		return h
	default:
		return DefaultHorizon
	}
}

// NormalizeBenchmark returns csi300, csi500, or none.
func NormalizeBenchmark(id string) string {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case BenchmarkCSI500, "000905", "000905.sh":
		return BenchmarkCSI500
	case BenchmarkNone, "off", "false":
		return BenchmarkNone
	default:
		return BenchmarkCSI300
	}
}

// BenchmarkSymbol maps benchmark id → kline code.
func BenchmarkSymbol(id string) string {
	switch NormalizeBenchmark(id) {
	case BenchmarkCSI500:
		return BenchmarkCodeCSI500
	case BenchmarkNone:
		return ""
	default:
		return BenchmarkCodeCSI300
	}
}
