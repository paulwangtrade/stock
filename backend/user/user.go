// Package user provides commercial identity domain models and service interfaces.
//
// Phase11-C5 is not a full login system: no OAuth, WeChat, Google Login, or cloud database.
// Persistence (migration / sqlite) is deferred; tests use an in-memory store.
//
// Commercial chain (composition root wires these; this package does not import trading):
//
//	User → Subscription → Entitlement → FeatureGate
//
// Trading Engine (TradePlan / Execution / Broker) must not depend on this package.
package user

import "time"

// Kind distinguishes local desktop identity from a future cloud-backed account.
type Kind string

const (
	KindLocal Kind = "local"
	KindCloud Kind = "cloud" // reserved; no cloud backend in C5
)

// AuthProvider is a placeholder for how identity was established.
// C5 only materializes local/device users; OAuth / WeChat / Google are not implemented.
type AuthProvider string

const (
	AuthProviderLocal  AuthProvider = "local"
	AuthProviderDevice AuthProvider = "device"
	AuthProviderOIDC   AuthProvider = "oidc" // reserved; no OAuth in C5
)

// Status is the commercial account lifecycle flag (not trading readiness).
type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

// User is the commercial identity aggregate root.
type User struct {
	UserID          string       `json:"id"`
	DisplayName     string       `json:"display_name"`
	Kind            Kind         `json:"kind"`
	AuthProvider    AuthProvider `json:"auth_provider,omitempty"`
	ExternalSubject string       `json:"external_subject,omitempty"` // future IdP sub; empty for local
	Status          Status       `json:"status"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at,omitempty"`
}

// ID is an alias accessor for UserID (commercial subject key).
func (u *User) ID() string {
	if u == nil {
		return ""
	}
	return u.UserID
}

// IsLocal reports whether this is a local/device identity (C5 default path).
func (u *User) IsLocal() bool {
	if u == nil {
		return false
	}
	return u.Kind == KindLocal
}

// IsCloud reports whether this is a cloud identity placeholder.
func (u *User) IsCloud() bool {
	if u == nil {
		return false
	}
	return u.Kind == KindCloud
}
