package featuregate

// EntitlementChecker is the commercial permission source for FeatureGate.
// Implemented by backend/entitlement (Subscription / License / Trial → grants).
// FeatureGate must not inspect user.Tier / Plan directly when a checker is registered.
type EntitlementChecker interface {
	HasFeature(user *User, feature Feature) bool
}

var entitlementChecker EntitlementChecker

// RegisterEntitlementChecker wires the Entitlement domain into FeatureGate.
func RegisterEntitlementChecker(c EntitlementChecker) {
	entitlementChecker = c
}

// ResetEntitlementCheckerForTest clears the checker (tests only).
func ResetEntitlementCheckerForTest() {
	entitlementChecker = nil
}
