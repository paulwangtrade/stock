package portfolioobs

import "strings"

const (
	agingShortMaxDays  = 4  // SHORT: holding_days < 5
	agingMediumMaxDays = 30 // MEDIUM: 5–30 inclusive; LONG: >30
)

// ClassifyAging maps Evaluation holding_days + first_buy_date → E.3 bucket.
// Missing buy date is UNKNOWN (never upgraded to LONG). Does not sell.
func ClassifyAging(holdingDays int, firstBuyDate string) (bucket string, aging bool) {
	buy := strings.TrimSpace(firstBuyDate)
	if buy == "" {
		return AgingUnknown, false
	}
	if holdingDays < 0 {
		holdingDays = 0
	}
	if holdingDays <= agingShortMaxDays {
		return AgingShort, false
	}
	if holdingDays <= agingMediumMaxDays {
		return AgingMedium, false
	}
	return AgingLong, true
}
