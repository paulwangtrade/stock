package version

import (
	"fmt"
	"strconv"
	"strings"
)

// Parsed is a comparable release version.
type Parsed struct {
	Major, Minor, Patch int
	Pre                 string // e.g. "beta", "beta.1"; empty = release
	Raw                 string
}

// Parse accepts forms like "0.1.0-beta", "v0.1.0", "1.2.3".
func Parse(s string) (Parsed, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return Parsed{}, fmt.Errorf("version: empty")
	}
	v := strings.TrimPrefix(raw, "v")
	v = strings.TrimPrefix(v, "V")
	pre := ""
	if i := strings.IndexByte(v, '-'); i >= 0 {
		pre = v[i+1:]
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return Parsed{}, fmt.Errorf("version: invalid format %q", raw)
	}
	nums := make([]int, 3)
	for i := 0; i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return Parsed{}, fmt.Errorf("version: invalid numeric part in %q", raw)
		}
		nums[i] = n
	}
	return Parsed{
		Major: nums[0],
		Minor: nums[1],
		Patch: nums[2],
		Pre:   pre,
		Raw:   raw,
	}, nil
}

// Compare returns -1 if a<b, 0 if equal, 1 if a>b.
// Pre-release is lower than the same numbers without pre (SemVer-ish).
func Compare(a, b Parsed) int {
	if a.Major != b.Major {
		return cmpInt(a.Major, b.Major)
	}
	if a.Minor != b.Minor {
		return cmpInt(a.Minor, b.Minor)
	}
	if a.Patch != b.Patch {
		return cmpInt(a.Patch, b.Patch)
	}
	if a.Pre == b.Pre {
		return 0
	}
	if a.Pre == "" {
		return 1
	}
	if b.Pre == "" {
		return -1
	}
	return strings.Compare(a.Pre, b.Pre)
}

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// IsNewer reports whether candidate is strictly newer than current.
func IsNewer(current, candidate string) (bool, error) {
	c, err := Parse(current)
	if err != nil {
		return false, err
	}
	n, err := Parse(candidate)
	if err != nil {
		return false, err
	}
	return Compare(n, c) > 0, nil
}
