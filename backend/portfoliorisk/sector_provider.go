package portfoliorisk

import (
	"strings"

	"go-stock/backend/portfolio"
)

// HoldingCodes lists Volume>0 position codes from a Snapshot (for external sectorclass.Classify).
func HoldingCodes(snap *portfolio.Snapshot) []string {
	if snap == nil {
		return nil
	}
	out := make([]string, 0, len(snap.Positions))
	seen := map[string]struct{}{}
	for _, p := range snap.Positions {
		if p.Volume <= 0 {
			continue
		}
		k := normSymbol(p.StockCode)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

// NormalizeIndustryBySymbol lowercases keys and trims sector labels; drops empty sectors.
// Does not invent "unknown" or zero weights — missing names stay absent from the map.
func NormalizeIndustryBySymbol(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		code := normSymbol(k)
		sec := strings.TrimSpace(v)
		if code == "" || sec == "" {
			continue
		}
		if strings.EqualFold(sec, "unknown") || sec == "0" {
			continue
		}
		out[code] = sec
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normSymbol(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func lookupSector(m map[string]string, code string) string {
	if len(m) == 0 {
		return ""
	}
	return strings.TrimSpace(m[normSymbol(code)])
}
