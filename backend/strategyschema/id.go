package strategyschema

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9_]+`)

// Slugify turns a display name into a stable slug fragment.
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteByte('_')
		}
	}
	out := nonSlug.ReplaceAllString(b.String(), "_")
	out = strings.Trim(out, "_")
	if out == "" {
		out = "strategy"
	}
	if len(out) > 48 {
		out = out[:48]
	}
	return out
}

func strategyIDFromSlug(slug string) string {
	slug = Slugify(slug)
	return "sdef:" + slug
}

func revisionID(strategyID, revision string) string {
	slug := strings.TrimPrefix(strategyID, "sdef:")
	return fmt.Sprintf("srev:%s:%s", slug, revision)
}

func nextRevisionLabel(existing []Revision) string {
	max := -1
	for _, r := range existing {
		label := strings.TrimSpace(r.Revision)
		if !strings.HasPrefix(label, "v") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(label, "v"))
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return fmt.Sprintf("v%d", max+1)
}
