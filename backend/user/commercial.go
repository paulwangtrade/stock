package user

import "go-stock/backend/featuregate"

// FeatureGateSubject returns a FeatureGate subject keyed by this User.
// Tier defaults to Free; Subscription.SyncToFeatureGate overlays paid entitlements.
//
//	User → Subscription → Entitlement → FeatureGate
func (u *User) FeatureGateSubject() *featuregate.User {
	if u == nil {
		return nil
	}
	return &featuregate.User{ID: u.UserID, Tier: featuregate.TierFree}
}
