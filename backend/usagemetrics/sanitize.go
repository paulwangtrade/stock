package usagemetrics

import "strings"

// sensitiveMetadataKeys must never be persisted on UsageEvent.Metadata.
// Matching is case-insensitive on the key name.
var sensitiveMetadataKeys = map[string]struct{}{
	"orderid":       {},
	"order_id":      {},
	"fillid":        {},
	"fill_id":       {},
	"positionid":    {},
	"position_id":   {},
	"accountid":     {},
	"account_id":    {},
	"broker":        {},
	"brokeraccount": {},
	"apikey":        {},
	"api_key":       {},
	"secret":        {},
	"password":      {},
	"tradepassword": {},
	"trade_password": {},
	"token":         {},
	"balance":       {},
	"cash":          {},
	"quantity":      {},
	"qty":           {},
	"price":         {},
	"limitprice":    {},
	"avgprice":      {},
	"pnl":           {},
	"portfolio":     {},
	"holding":       {},
	"holdings":      {},
	"position":      {},
	"positions":     {},
	"email":         {},
	"phone":         {},
	"mobile":        {},
	"idcard":        {},
	"id_card":       {},
	"tradeplanid":   {},
	"trade_plan_id": {},
	"planid":        {},
	"plan_id":       {},
	"stockcode":     {},
	"stock_code":    {},
	"symbol":        {},
	"ticker":        {},
}

// IsSensitiveMetadataKey reports whether a metadata key is forbidden.
func IsSensitiveMetadataKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	k = strings.ReplaceAll(k, "-", "_")
	_, ok := sensitiveMetadataKeys[k]
	return ok
}

// SanitizeMetadata copies metadata while dropping trading-sensitive keys.
// Returns nil when the result is empty.
func SanitizeMetadata(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if IsSensitiveMetadataKey(k) {
			continue
		}
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
