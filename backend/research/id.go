package research

import (
	"fmt"
	"strings"
)

// MakeCandidateID builds a stable research candidate id.
// Format: rc:signal:{trade_date}:{stock_code}
func MakeCandidateID(tradeDate, stockCode string) string {
	td := strings.TrimSpace(tradeDate)
	code := strings.ToLower(strings.TrimSpace(stockCode))
	return fmt.Sprintf("rc:signal:%s:%s", td, code)
}

// ParseCandidateID extracts trade_date and stock_code from id.
// Returns ok=false when format is invalid.
func ParseCandidateID(id string) (tradeDate, stockCode string, ok bool) {
	id = strings.TrimSpace(id)
	const prefix = "rc:signal:"
	if !strings.HasPrefix(id, prefix) {
		return "", "", false
	}
	rest := id[len(prefix):]
	// trade_date is YYYY-MM-DD (10 chars), then ':', then code
	if len(rest) < 12 || rest[10] != ':' {
		return "", "", false
	}
	tradeDate = rest[:10]
	stockCode = strings.ToLower(strings.TrimSpace(rest[11:]))
	if tradeDate == "" || stockCode == "" {
		return "", "", false
	}
	// light date shape check
	if tradeDate[4] != '-' || tradeDate[7] != '-' {
		return "", "", false
	}
	return tradeDate, stockCode, true
}
