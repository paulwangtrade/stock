// Package security provides low-risk redaction helpers for Beta hardening (Phase13-V.5).
// No encryption, no license changes — string scrubbing only.
package security

import (
	"crypto/sha1"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strings"
)

// RedactedPlaceholder is written into exported config for secret fields.
const RedactedPlaceholder = "[REDACTED]"

var (
	// key=value / key: value for common secret labels (case-insensitive).
	reSecretKV = regexp.MustCompile(`(?i)(api[_\s-]?key|authorization|bearer|tushare[_\s-]?token|access[_\s-]?token|refresh[_\s-]?token|token)\s*[:=]\s*["']?[^\s"',}\]]+`)
	// OpenAI-style sk-... tokens
	reSkToken = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}`)
	// Bearer <token>
	reBearer = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._\-+/=]{8,}`)
	// Absolute paths embedded in free text (Windows drive / Unix).
	reWinAbsPath  = regexp.MustCompile(`[A-Za-z]:[\\/][^\s"',}\]]+`)
	reUnixAbsPath = regexp.MustCompile(`/(?:Users|home|var|tmp|opt|data|mnt)/[^\s"',}\]]+`)
)

// IsRedactedPlaceholder reports whether v is the export sentinel (skip overwrite on save).
func IsRedactedPlaceholder(v string) bool {
	return strings.EqualFold(strings.TrimSpace(v), RedactedPlaceholder)
}

// RedactSecrets masks API keys / tokens in free-form text (logs, errors).
func RedactSecrets(s string) string {
	if s == "" {
		return s
	}
	out := reSecretKV.ReplaceAllStringFunc(s, func(m string) string {
		if i := strings.IndexAny(m, ":="); i >= 0 {
			sep := string(m[i])
			label := strings.TrimSpace(m[:i])
			return label + sep + RedactedPlaceholder
		}
		return RedactedPlaceholder
	})
	out = reBearer.ReplaceAllString(out, "Bearer "+RedactedPlaceholder)
	out = reSkToken.ReplaceAllString(out, "sk-"+RedactedPlaceholder)
	return out
}

// RedactPaths replaces absolute filesystem paths with path#<hash8>.
func RedactPaths(s string) string {
	if s == "" {
		return s
	}
	replace := func(m string) string {
		return "path#" + hash8(m)
	}
	out := reWinAbsPath.ReplaceAllStringFunc(s, replace)
	out = reUnixAbsPath.ReplaceAllStringFunc(out, replace)
	// Also handle space-delimited tokens (legacy diagnostic style).
	parts := strings.Fields(out)
	changed := false
	for i, p := range parts {
		trimmed := strings.Trim(p, `"'`)
		if LooksAbsPath(trimmed) && !strings.HasPrefix(trimmed, "path#") {
			parts[i] = "path#" + hash8(trimmed)
			changed = true
		}
	}
	if changed {
		out = strings.Join(parts, " ")
	}
	return out
}

// SanitizeLogLine redacts secrets then absolute paths for log sinks.
func SanitizeLogLine(s string) string {
	return RedactPaths(RedactSecrets(s))
}

// SanitizeErrorMessage redacts secrets and paths in user/log-facing error strings.
func SanitizeErrorMessage(s string) string {
	return SanitizeLogLine(s)
}

// PathHint returns a short non-absolute hint for recovery messages (basename only).
func PathHint(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	base := filepath.Base(filepath.Clean(p))
	if base == "." || base == string(filepath.Separator) {
		return "path#" + hash8(p)
	}
	return base + " (path#" + hash8(p) + ")"
}

// LooksAbsPath reports whether p looks like an absolute file path.
func LooksAbsPath(p string) bool {
	p = strings.Trim(p, `"'`)
	if len(p) < 4 {
		return false
	}
	if strings.HasPrefix(p, "/") && strings.Count(p, "/") >= 2 {
		return true
	}
	if len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		return true
	}
	return false
}

func hash8(s string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(s)))
	return hex.EncodeToString(sum[:])[:8]
}
