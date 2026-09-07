package opportunity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// NormalizeStockCode stores codes as lowercase exchange prefix (e.g. sh600363).
func NormalizeStockCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

func normalizeSecucode(secucode string) string {
	return strings.ToUpper(strings.TrimSpace(secucode))
}

// SecucodeToStockCode converts EM secucode (600363.SH) to sh600363.
func SecucodeToStockCode(secucode string) string {
	s := strings.TrimSpace(secucode)
	if s == "" {
		return ""
	}
	parts := strings.Split(s, ".")
	if len(parts) != 2 {
		return NormalizeStockCode(s)
	}
	return strings.ToLower(parts[1]) + parts[0]
}

// BatchKeyFromSnapshot identifies a scan batch for stable opportunity_id scope.
func BatchKeyFromSnapshot(snapshotID uint, tradeDate, session, strategyID string) string {
	if snapshotID > 0 {
		return fmt.Sprintf("snap:%d", snapshotID)
	}
	return fmt.Sprintf("%s|%s|%s",
		strings.TrimSpace(tradeDate),
		strings.TrimSpace(session),
		strings.TrimSpace(strategyID),
	)
}

// BuildOpportunityID creates a stable opportunity projection ID (G1 design).
func BuildOpportunityID(batchKey, secucode, signalTime, signalTag string) string {
	raw := strings.Join([]string{
		strings.TrimSpace(batchKey),
		normalizeSecucode(secucode),
		strings.TrimSpace(signalTime),
		strings.TrimSpace(signalTag),
	}, "|")
	sum := sha256.Sum256([]byte(raw))
	return "opp_" + hex.EncodeToString(sum[:8])
}
