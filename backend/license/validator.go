package license

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// LicenseValidator validates a License without hard online dependency.
type LicenseValidator interface {
	Validate(lic *License, now time.Time) Result
}

// OnlineChecker is an optional future hook for online license refresh.
// Implementations must tolerate failure; Validator never requires success.
type OnlineChecker interface {
	// Check returns nil when online status is acceptable.
	// Err or unreachable must not alone invalidate a still-dated offline license.
	Check(lic *License, now time.Time) error
}

// LocalValidator performs offline date + optional lightweight checksum checks.
type LocalValidator struct {
	// LocalSecret is used only for optional checksum verification.
	// Empty secret → signature field is ignored (unsigned offline licenses allowed).
	LocalSecret string
}

// NewLocalValidator returns an offline-first validator.
func NewLocalValidator(localSecret string) *LocalValidator {
	return &LocalValidator{LocalSecret: localSecret}
}

// Validate implements LicenseValidator.
func (v *LocalValidator) Validate(lic *License, now time.Time) Result {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	out := Result{
		CheckedAt:           now,
		PaperTradingAllowed: true, // INV: simulation never blocked by license
		EffectiveType:       TypeFree,
	}
	if lic == nil {
		out.Status = StatusMissing
		out.Reason = "no license installed"
		return out
	}
	cp := *lic
	cp.Mode = NormalizeMode(cp.Mode)
	cp.Type = NormalizeType(cp.Type)
	out.License = &cp

	if cp.Mode != ModeOffline && cp.Mode != ModeOnline {
		out.Status = StatusInvalid
		out.Reason = "unknown license mode"
		cp.Status = StatusInvalid
		out.License = &cp
		return out
	}

	if v != nil && strings.TrimSpace(v.LocalSecret) != "" && strings.TrimSpace(cp.Signature) != "" {
		expect := ComputeLocalChecksum(&cp, v.LocalSecret)
		if !strings.EqualFold(expect, strings.TrimSpace(cp.Signature)) {
			out.Status = StatusInvalid
			out.Reason = "local checksum mismatch"
			cp.Status = StatusInvalid
			out.License = &cp
			return out
		}
	}

	typ := cp.Type
	if !cp.NotBefore.IsZero() && now.Before(cp.NotBefore.UTC()) {
		out.Status = StatusNotYetValid
		out.Reason = "license not yet valid"
		out.EffectiveType = TypeFree
		cp.Status = StatusNotYetValid
		out.License = &cp
		return out
	}

	if !cp.ExpireAt.IsZero() && now.After(cp.ExpireAt.UTC()) {
		graceDays := cp.GraceDays
		if graceDays < 0 {
			graceDays = 0
		}
		graceEnd := cp.ExpireAt.UTC().AddDate(0, 0, graceDays)
		if graceDays > 0 && !now.After(graceEnd) {
			out.Status = StatusGrace
			out.Reason = "license in grace period"
			out.EffectiveType = typ
			cp.Status = StatusGrace
			out.License = &cp
			return out
		}
		out.Status = StatusExpired
		out.Reason = "license expired"
		out.EffectiveType = TypeFree
		cp.Status = StatusExpired
		out.License = &cp
		return out
	}

	out.Status = StatusValid
	out.Reason = "ok"
	out.EffectiveType = typ
	cp.Status = StatusValid
	out.License = &cp
	return out
}

// CompositeValidator runs LocalValidator, then optionally OnlineChecker (best-effort).
type CompositeValidator struct {
	Local  *LocalValidator
	Online OnlineChecker // optional; nil = skip
}

// NewCompositeValidator builds an offline-first validator with optional online hook.
func NewCompositeValidator(local *LocalValidator, online OnlineChecker) *CompositeValidator {
	if local == nil {
		local = NewLocalValidator("")
	}
	return &CompositeValidator{Local: local, Online: online}
}

// Validate implements LicenseValidator.
func (v *CompositeValidator) Validate(lic *License, now time.Time) Result {
	res := v.Local.Validate(lic, now)
	if v.Online == nil || lic == nil {
		return res
	}
	mode := NormalizeMode(lic.Mode)
	if mode != ModeOnline {
		return res
	}
	// Online is advisory: failure does not downgrade a locally valid/grace license.
	res.OnlineAttempted = true
	if err := v.Online.Check(lic, now); err != nil {
		res.OnlineOK = false
		if res.Reason == "ok" || res.Status == StatusGrace {
			res.Reason = res.Reason + "; online check skipped/failed: " + err.Error()
		}
		return res
	}
	res.OnlineOK = true
	return res
}

// ComputeLocalChecksum is a simple SHA-256 hex digest for optional local integrity.
// Not intended as strong anti-tamper DRM.
func ComputeLocalChecksum(lic *License, secret string) string {
	if lic == nil {
		return ""
	}
	canon := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d|%s",
		lic.ID,
		NormalizeMode(lic.Mode),
		NormalizeType(lic.Type),
		lic.IssuedTo,
		formatTime(lic.NotBefore),
		formatTime(lic.ExpireAt),
		lic.GraceDays,
		secret,
	)
	sum := sha256.Sum256([]byte(canon))
	return hex.EncodeToString(sum[:])
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// IssueOfflineLicense builds an offline license (optional checksum when secret non-empty).
func IssueOfflineLicense(id, issuedTo string, typ Type, notBefore, expireAt time.Time, graceDays int, secret string) *License {
	lic := &License{
		ID:        id,
		Type:      NormalizeType(typ),
		Status:    StatusMissing, // until Validate
		Mode:      ModeOffline,
		IssuedTo:  issuedTo,
		NotBefore: notBefore.UTC(),
		ExpireAt:  expireAt,
		GraceDays: graceDays,
		IssuedAt:  time.Now().UTC(),
	}
	if !expireAt.IsZero() {
		lic.ExpireAt = expireAt.UTC()
	}
	if strings.TrimSpace(secret) != "" {
		lic.Signature = ComputeLocalChecksum(lic, secret)
	}
	return lic
}

// PaperTradingAllowed always returns true (C6 product rule).
func PaperTradingAllowed(_ Result) bool {
	return true
}
