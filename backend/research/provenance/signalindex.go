package provenance

import (
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/models"
)

// BuildHitIndex maps normalized A-share sina codes to signal scan hits.
func BuildHitIndex(hits []models.SignalScanHit) map[string]models.SignalScanHit {
	out := map[string]models.SignalScanHit{}
	for _, h := range hits {
		for _, raw := range []string{h.SECUCODE, h.SECURITY_CODE} {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			n, err := data.NormalizeStockCode(raw)
			if err != nil || n.Market != data.MarketCN {
				continue
			}
			code := strings.ToLower(strings.TrimSpace(n.SinaCode))
			if data.IsAShareSinaCode(code) {
				out[code] = h
			}
		}
	}
	return out
}
