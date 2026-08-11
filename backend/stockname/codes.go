package stockname

import (
	"strings"

	"go-stock/backend/data"
)

// CodeKeys returns lowercase full code plus normalized symbol / sina code.
func CodeKeys(code string) []string {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil
	}
	lower := strings.ToLower(code)
	keys := []string{lower}
	seen := map[string]bool{lower: true}
	add := func(k string) {
		k = strings.ToLower(strings.TrimSpace(k))
		if k == "" || seen[k] {
			return
		}
		seen[k] = true
		keys = append(keys, k)
	}
	if norm, err := data.NormalizeStockCode(code); err == nil {
		add(norm.Symbol)
		add(norm.SinaCode)
	}
	return keys
}
