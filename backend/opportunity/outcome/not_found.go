package outcome

// isStockNotFound reports whether a single-stock query has no resolvable outcome data.
// Status filtering is applied later and may legitimately return an empty list with HTTP 200.
func isStockNotFound(rows []OutcomeProjection, includeNoTrade bool) bool {
	if len(rows) == 0 {
		return true
	}
	if !includeNoTrade {
		return false
	}
	if len(rows) != 1 {
		return false
	}
	row := rows[0]
	return row.OutcomeStatus == OutcomeStatusNoTrade &&
		!row.Signal.Present &&
		!row.Opportunity.Present &&
		!row.Entry.Present
}
