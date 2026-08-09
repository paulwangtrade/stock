// Package entitlement is the commercial permission source layer.
//
// Flow: Subscription / License / Trial → Entitlement → FeatureGate.
// FeatureGate.Allow delegates here; it must not branch on user.Plan == "Pro".
//
// Trading Engine must not depend on this package.
package entitlement

import (
	"time"

	"go-stock/backend/featuregate"
)

// Source identifies how an entitlement was granted.
type Source string

const (
	SourceManual       Source = "manual"
	SourceSubscription Source = "subscription" // future / sync from subscription
	SourceLicense      Source = "license"      // future
	SourceTrial        Source = "trial"        // future
	SourceTierDefault  Source = "tier_default" // materialize from catalog for a tier
)

// Entitlement is one user×feature commercial grant.
type Entitlement struct {
	UserID    string             `json:"user"`
	Feature   featuregate.Feature `json:"feature"`
	Enabled   bool               `json:"enabled"`
	ExpiresAt time.Time          `json:"expires_at,omitempty"` // zero = no expiry
	Source    Source             `json:"source,omitempty"`
	UpdatedAt time.Time          `json:"updated_at,omitempty"`
}

// IsActive reports whether the grant is currently effective.
func (e *Entitlement) IsActive(now time.Time) bool {
	if e == nil || !e.Enabled {
		return false
	}
	if !e.ExpiresAt.IsZero() && !now.Before(e.ExpiresAt) {
		return false
	}
	return true
}
