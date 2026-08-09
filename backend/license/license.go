// Package license provides the desktop License domain for go-stock commercialization.
//
// Phase11-C6: domain model, service interface, and validation flow only.
// Supports Offline License now and Online Validation as a future hook.
//
// Forbidden in C6: payment, live license servers, forced networking.
// Trading Engine (TradePlan / Execution / Broker) must not import this package
// for trade reject decisions — PaperTradingAllowed is always true.
package license

import (
	"strings"
	"time"
)

// Type is the commercial license product class.
type Type string

const (
	TypeFree       Type = "FREE"
	TypePro        Type = "PRO"
	TypeEnterprise Type = "ENTERPRISE"
)

// NormalizeType maps aliases to a known type (unknown → FREE).
func NormalizeType(t Type) Type {
	switch Type(strings.ToUpper(strings.TrimSpace(string(t)))) {
	case TypePro:
		return TypePro
	case TypeEnterprise:
		return TypeEnterprise
	case TypeFree, "":
		return TypeFree
	default:
		return TypeFree
	}
}

// Mode distinguishes offline (local) licenses from future online-validated licenses.
type Mode string

const (
	ModeOffline Mode = "offline"
	ModeOnline  Mode = "online" // reserved; online check is optional and best-effort
)

// NormalizeMode defaults empty / "local" to offline.
func NormalizeMode(m Mode) Mode {
	s := strings.ToLower(strings.TrimSpace(string(m)))
	switch s {
	case "online":
		return ModeOnline
	case "offline", "local", "":
		return ModeOffline
	default:
		return ModeOffline
	}
}

// License is a desktop commercial credential (domain model).
// Primary C6 fields: id, type, status, expire_at.
type License struct {
	ID       string        `json:"id"`
	Type     Type          `json:"type"`
	Status   LicenseStatus `json:"status"`    // last validation outcome (updated by Validate flow)
	ExpireAt time.Time     `json:"expire_at"` // zero = no expiry

	Mode      Mode      `json:"mode"`                // offline | online
	IssuedTo  string    `json:"issued_to,omitempty"` // user / device / org label
	NotBefore time.Time `json:"not_before,omitempty"`
	GraceDays int       `json:"grace_days,omitempty"`
	Signature string    `json:"signature,omitempty"` // optional local checksum
	IssuedAt  time.Time `json:"issued_at,omitempty"`
	KeyHint   string    `json:"key_hint,omitempty"` // non-secret fragment of activated key
}

// LicenseStatus is the validation outcome for Shell / entitlement — never a trade reject code.
type LicenseStatus string

const (
	StatusValid       LicenseStatus = "valid"
	StatusGrace       LicenseStatus = "grace" // past ExpireAt but within GraceDays
	StatusExpired     LicenseStatus = "expired"
	StatusNotYetValid LicenseStatus = "not_yet_valid"
	StatusInvalid     LicenseStatus = "invalid"
	StatusMissing     LicenseStatus = "missing"
)

// Result is the full validator / service output.
type Result struct {
	Status        LicenseStatus `json:"status"`
	License       *License      `json:"license,omitempty"`
	EffectiveType Type          `json:"effective_type"` // FREE when missing/expired/invalid
	Reason        string        `json:"reason,omitempty"`
	CheckedAt     time.Time     `json:"checked_at"`

	OnlineAttempted bool `json:"online_attempted"`
	OnlineOK        bool `json:"online_ok"`

	// PaperTradingAllowed is always true in C6: offline/expired must not stop simulation.
	PaperTradingAllowed bool `json:"paper_trading_allowed"`
}
