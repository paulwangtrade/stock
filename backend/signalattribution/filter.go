package signalattribution

import "strings"

func normalizeSignalTag(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "超" {
		return "趋"
	}
	return tag
}

func signalTagMatches(filterTag, tag string) bool {
	filterTag = normalizeSignalTag(filterTag)
	tag = normalizeSignalTag(tag)
	if filterTag == "" || tag == "" {
		return false
	}
	if filterTag == tag {
		return true
	}
	if filterTag == "卖" && (tag == "止" || tag == "减") {
		return true
	}
	return false
}

func isSTName(name string) bool {
	return strings.Contains(strings.ToUpper(name), "ST")
}

// FilterHits keeps hits whose snapshot tag matches any selected tag.
// An empty tag list returns the input unchanged so an unfiltered snapshot still loads.
// 「弹」drops ST names, and drops RSI above reboundMaxRsi when both are present.
func FilterHits(hits []HitInput, tags []string, reboundMaxRsi *float64) []HitInput {
	cleaned := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			cleaned = append(cleaned, tag)
		}
	}
	if len(cleaned) == 0 {
		return hits
	}
	out := make([]HitInput, 0, len(hits))
	for _, hit := range hits {
		matched := false
		for _, tag := range cleaned {
			if signalTagMatches(tag, hit.Tag) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if normalizeSignalTag(hit.Tag) == "弹" {
			if isSTName(hit.Name) {
				continue
			}
			if reboundMaxRsi != nil && hit.HasRSI && hit.RSI > *reboundMaxRsi {
				continue
			}
		}
		out = append(out, hit)
	}
	return out
}
