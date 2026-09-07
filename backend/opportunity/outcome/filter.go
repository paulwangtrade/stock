package outcome

import "strings"

func filterOutcomes(items []OutcomeProjection, status string) []OutcomeProjection {
	status = strings.ToUpper(strings.TrimSpace(status))
	if status == "" {
		return items
	}
	out := make([]OutcomeProjection, 0, len(items))
	for _, it := range items {
		if strings.EqualFold(it.OutcomeStatus, status) {
			out = append(out, it)
		}
	}
	return out
}

func applyLimit(items []OutcomeProjection, limit int) []OutcomeProjection {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func finalizeItems(items []OutcomeProjection, opts ProjectOptions) []OutcomeProjection {
	items = filterOutcomes(items, opts.Status)
	items = applyLimit(items, opts.Limit)
	return normalizeOutcomeItems(items)
}

func normalizeOutcomeItems(items []OutcomeProjection) []OutcomeProjection {
	for i := range items {
		items[i].Metadata.Missing = collectOutcomeMissing(items[i])
	}
	return items
}

func collectOutcomeMissing(o OutcomeProjection) []string {
	missing := make([]string, 0, 4)
	if !o.Signal.Present {
		missing = append(missing, "signal")
	}
	if !o.Opportunity.Present {
		missing = append(missing, "candidate_pool_item")
	}
	switch o.OutcomeStatus {
	case OutcomeStatusOpen, OutcomeStatusClosed:
		if !o.Entry.Present {
			missing = append(missing, "entry_fill")
		}
	}
	if o.OutcomeStatus == OutcomeStatusClosed && !o.Exit.Present {
		missing = append(missing, "exit_fill")
	}
	return missing
}
