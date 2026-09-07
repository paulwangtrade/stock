package opportunity

import (
	"regexp"
	"strings"

	"go-stock/backend/data"
)

var reReadDigitsOnly = regexp.MustCompile(`^\d{1,6}$`)

// NormalizeReadStockCode canonicalizes stock codes for Phase16 read APIs.
// Accepts secucode (301125.SZ), internal code (sz301125), or bare CN digits (301125).
// Output is always lowercase exchange-prefixed internal form (e.g. sz301125).
func NormalizeReadStockCode(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if strings.Contains(s, ".") {
		return SecucodeToStockCode(s)
	}
	if reReadDigitsOnly.MatchString(s) {
		if n, err := data.NormalizeStockCode(s); err == nil && strings.TrimSpace(n.SinaCode) != "" {
			return strings.ToLower(strings.TrimSpace(n.SinaCode))
		}
	}
	return strings.ToLower(s)
}
