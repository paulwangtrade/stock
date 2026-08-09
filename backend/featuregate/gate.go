package featuregate

// Reason explains a FeatureGate decision for UI (upgrade CTA / disabled entry).
// These codes must not be reused as TradingPreflight / Freeze reject reasons.
type Reason string

const (
	ReasonOK              Reason = "OK"
	ReasonFeatureDisabled Reason = "FEATURE_DISABLED"
	ReasonUnknownFeature  Reason = "UNKNOWN_FEATURE"
	ReasonNilUser         Reason = "NIL_USER"
	ReasonNoEntitlement   Reason = "NO_ENTITLEMENT" // checker missing or deny
)

// Decision is the result of CanAccess / Allow.
type Decision struct {
	Allowed bool    `json:"allowed"`
	Feature Feature `json:"feature"`
	Tier    Tier    `json:"tier"`
	Reason  Reason  `json:"reason"`
}

// defaultMatrix is the static catalog (see CatalogAllows). Not used by Allow directly.
var defaultMatrix = map[Feature]map[Tier]bool{
	FeatureAdvancedRisk: {
		TierFree:       false,
		TierPro:        true,
		TierEnterprise: true,
	},
	FeatureAIAnalysis: {
		TierFree:       false,
		TierPro:        true,
		TierEnterprise: true,
	},
	FeatureBacktest: {
		TierFree:       false,
		TierPro:        true,
		TierEnterprise: true,
	},
	FeatureRealtimeSignal: {
		TierFree:       false,
		TierPro:        false,
		TierEnterprise: true,
	},
	FeatureAdvancedObservation: {
		TierFree:       false,
		TierPro:        true,
		TierEnterprise: true,
	},
	FeatureMultiAccount: {
		TierFree:       false,
		TierPro:        false,
		TierEnterprise: true,
	},
}

// CanAccess reports whether user may see/open the feature entry via Entitlement.
func CanAccess(user *User, feature Feature) Decision {
	if user == nil {
		return Decision{
			Allowed: false,
			Feature: feature,
			Tier:    TierFree,
			Reason:  ReasonNilUser,
		}
	}
	tier := NormalizeTier(user.Tier)
	if !IsKnown(feature) {
		return Decision{
			Allowed: false,
			Feature: feature,
			Tier:    tier,
			Reason:  ReasonUnknownFeature,
		}
	}
	if entitlementChecker == nil {
		return Decision{
			Allowed: false,
			Feature: feature,
			Tier:    tier,
			Reason:  ReasonNoEntitlement,
		}
	}
	if !entitlementChecker.HasFeature(user, feature) {
		return Decision{
			Allowed: false,
			Feature: feature,
			Tier:    tier,
			Reason:  ReasonFeatureDisabled,
		}
	}
	return Decision{
		Allowed: true,
		Feature: feature,
		Tier:    tier,
		Reason:  ReasonOK,
	}
}

// Allow is the convenience predicate for Shell entry checks.
//
//	if featuregate.Allow(user, featuregate.FeatureAdvancedRisk) { ... }
//
// Decisions come from EntitlementService (not user.Plan / Tier if-checks).
func Allow(user *User, feature Feature) bool {
	return CanAccess(user, feature).Allowed
}

// EvaluateAll returns decisions for the full known catalog (UI bootstrap).
func EvaluateAll(user *User) []Decision {
	feats := KnownFeatures()
	out := make([]Decision, 0, len(feats))
	for _, f := range feats {
		out = append(out, CanAccess(user, f))
	}
	return out
}
