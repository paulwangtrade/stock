package portfolioobservation

import (
	"sort"
	"strings"
)

func appendUniqueGap(gaps []string, g string) []string {
	g = strings.TrimSpace(g)
	if g == "" {
		return gaps
	}
	for _, x := range gaps {
		if x == g {
			return gaps
		}
	}
	return append(gaps, g)
}

func uniqueSorted(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func appendUniqueFactor(xs []ExplainFactor, f ExplainFactor) []ExplainFactor {
	for _, x := range xs {
		if x.Code == f.Code && x.Source == f.Source {
			return xs
		}
	}
	return append(xs, f)
}

func firstCode(codes []string, fallback string) string {
	for _, c := range codes {
		c = strings.TrimSpace(c)
		if c != "" {
			return c
		}
	}
	return strings.TrimSpace(fallback)
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return strings.TrimSpace(b)
}
