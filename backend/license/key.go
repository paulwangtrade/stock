package license

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Simulated license-key format (offline only; no payment / no live server):
//
//	GSTK-<TYPE>-<YYYYMMDD|NEVER>-<TOKEN>
//
// Examples:
//
//	GSTK-PRO-20271231-DEMO1234
//	GSTK-ENTERPRISE-NEVER-ORG0001
//
// This is a validation-flow simulation for desktop activation UX — not a store.

const keyPrefix = "GSTK"

// KeyValidation describes a simulated license-key parse / verify outcome.
type KeyValidation struct {
	OK      bool     `json:"ok"`
	Reason  string   `json:"reason,omitempty"`
	License *License `json:"license,omitempty"`
	RawHint string   `json:"raw_hint,omitempty"` // redacted key fragment
}

// ValidateLicenseKey simulates offline license-key verification.
// It does not contact any network; invalid format → OK=false.
func ValidateLicenseKey(key string, now time.Time) KeyValidation {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	raw := strings.TrimSpace(key)
	if raw == "" {
		return KeyValidation{OK: false, Reason: "empty license key"}
	}
	hint := redactKey(raw)
	parts := strings.Split(strings.ToUpper(raw), "-")
	// GSTK TYPE EXPIRY TOKEN…  (≥4 segments; token may contain extra dashes)
	if len(parts) < 4 || parts[0] != keyPrefix {
		return KeyValidation{OK: false, Reason: "invalid key format (expect GSTK-TYPE-EXPIRY-TOKEN)", RawHint: hint}
	}
	switch parts[1] {
	case "FREE", "PRO", "ENTERPRISE":
		// ok
	default:
		return KeyValidation{OK: false, Reason: "unknown license type in key", RawHint: hint}
	}
	typ := Type(parts[1])

	expirePart := parts[2]
	var expireAt time.Time
	if expirePart != "NEVER" {
		if len(expirePart) != 8 {
			return KeyValidation{OK: false, Reason: "invalid expiry segment (YYYYMMDD or NEVER)", RawHint: hint}
		}
		y, errY := strconv.Atoi(expirePart[0:4])
		m, errM := strconv.Atoi(expirePart[4:6])
		d, errD := strconv.Atoi(expirePart[6:8])
		if errY != nil || errM != nil || errD != nil {
			return KeyValidation{OK: false, Reason: "invalid expiry date digits", RawHint: hint}
		}
		expireAt = time.Date(y, time.Month(m), d, 23, 59, 59, 0, time.UTC)
	}
	token := strings.Join(parts[3:], "-")
	if strings.TrimSpace(token) == "" {
		return KeyValidation{OK: false, Reason: "missing key token", RawHint: hint}
	}
	if len(token) < 4 {
		return KeyValidation{OK: false, Reason: "key token too short", RawHint: hint}
	}

	sum := sha256.Sum256([]byte(token))
	id := fmt.Sprintf("key-%s-%s", strings.ToLower(string(typ)), hex.EncodeToString(sum[:4]))
	lic := &License{
		ID:        id,
		Type:      typ,
		Status:    StatusMissing,
		ExpireAt:  expireAt,
		Mode:      ModeOffline,
		IssuedTo:  "license-key",
		NotBefore: now.Add(-time.Minute),
		GraceDays: 7,
		IssuedAt:  now,
		KeyHint:   hint,
	}
	res := NewLocalValidator("").Validate(lic, now)
	if res.Status == StatusExpired || res.Status == StatusInvalid || res.Status == StatusNotYetValid {
		return KeyValidation{
			OK:      false,
			Reason:  "key failed validation: " + res.Reason,
			License: res.License,
			RawHint: hint,
		}
	}
	lic.Status = res.Status
	return KeyValidation{OK: true, Reason: "ok", License: lic, RawHint: hint}
}

func redactKey(key string) string {
	k := strings.TrimSpace(key)
	if len(k) <= 8 {
		return "****"
	}
	return k[:4] + "…****…" + k[len(k)-4:]
}
