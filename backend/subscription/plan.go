// Package subscription provides the commercial subscription domain (C3).
//
// Responsibility: subscription state only (plan / status / period).
// Not responsible: payment, login, or permission checks.
//
// Permission flow:
//
//	Subscription → Entitlement → FeatureGate
//
// No Stripe, WeChat Pay, or cloud database sync.
// Trading Engine must not depend on this package.
package subscription

import (
	"strings"
	"time"
)

// PlanCode identifies a commercial plan tier.
type PlanCode string

const (
	PlanFREE         PlanCode = "FREE"
	PlanPRO          PlanCode = "PRO"
	PlanPROFESSIONAL PlanCode = "PROFESSIONAL"
	PlanENTERPRISE   PlanCode = "ENTERPRISE"
)

// Plan is a product catalog entry (static in C3; no pricing / checkout).
type Plan struct {
	Code        PlanCode `json:"code"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description,omitempty"`
}

// Catalog returns the static plan list.
func Catalog() []Plan {
	return []Plan{
		{Code: PlanFREE, DisplayName: "Free", Description: "Default local tier"},
		{Code: PlanPRO, DisplayName: "Pro", Description: "Individual paid tier"},
		{Code: PlanPROFESSIONAL, DisplayName: "Professional", Description: "Power user / small team tier"},
		{Code: PlanENTERPRISE, DisplayName: "Enterprise", Description: "Organization tier"},
	}
}

// LookupPlan returns a known plan or FREE for unknown/empty codes.
func LookupPlan(code PlanCode) Plan {
	switch NormalizePlanCode(code) {
	case PlanPRO:
		return Plan{Code: PlanPRO, DisplayName: "Pro"}
	case PlanPROFESSIONAL:
		return Plan{Code: PlanPROFESSIONAL, DisplayName: "Professional"}
	case PlanENTERPRISE:
		return Plan{Code: PlanENTERPRISE, DisplayName: "Enterprise"}
	default:
		return Plan{Code: PlanFREE, DisplayName: "Free"}
	}
}

// NormalizePlanCode maps aliases and unknown values to a catalog code.
func NormalizePlanCode(code PlanCode) PlanCode {
	switch PlanCode(strings.ToUpper(strings.TrimSpace(string(code)))) {
	case PlanPRO:
		return PlanPRO
	case PlanPROFESSIONAL:
		return PlanPROFESSIONAL
	case PlanENTERPRISE:
		return PlanENTERPRISE
	case PlanFREE, "":
		return PlanFREE
	default:
		return PlanFREE
	}
}

// SubscriptionStatus is the commercial lifecycle of a subscription record.
type SubscriptionStatus string

const (
	StatusActive  SubscriptionStatus = "active"
	StatusExpired SubscriptionStatus = "expired"
	StatusNone    SubscriptionStatus = "none" // implicit / assigned free
)

// Subscription binds a user to a Plan for a period.
// C3 does not implement billing, renewal, or payment providers.
type Subscription struct {
	ID        string             `json:"id,omitempty"`
	UserID    string             `json:"user_id"`
	PlanCode  PlanCode           `json:"plan"`
	Status    SubscriptionStatus `json:"status"`
	StartsAt  time.Time          `json:"start_time"`
	ExpiresAt time.Time          `json:"expire_time"` // zero = no expiry
	UpdatedAt time.Time          `json:"updated_at,omitempty"`
}

// IsExpired reports whether the subscription is past its paid window.
// Expired paid rows fall back to FREE when resolving Entitlement (engine unaffected).
func (s *Subscription) IsExpired(now time.Time) bool {
	if s == nil {
		return true
	}
	if s.Status == StatusExpired {
		return true
	}
	if s.Status == StatusNone {
		// Free / none is not a paid window; treat as "no paid sub" for expiry checks.
		return true
	}
	if !s.ExpiresAt.IsZero() && !s.ExpiresAt.After(now) {
		return true
	}
	return false
}

// EffectivePlanCode returns the plan that should drive Entitlement right now.
func (s *Subscription) EffectivePlanCode(now time.Time) PlanCode {
	if s == nil {
		return PlanFREE
	}
	code := NormalizePlanCode(s.PlanCode)
	if code == PlanFREE || s.Status == StatusNone {
		return PlanFREE
	}
	if s.Status == StatusExpired {
		return PlanFREE
	}
	if !s.ExpiresAt.IsZero() && !s.ExpiresAt.After(now) {
		return PlanFREE
	}
	if s.Status != StatusActive {
		return PlanFREE
	}
	return code
}
