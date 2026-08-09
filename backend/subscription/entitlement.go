package subscription

import "go-stock/backend/featuregate"

// Entitlement is the resolved commercial capability snapshot for a subject.
// It feeds FeatureGate and must not be read by Trading Engine paths.
type Entitlement struct {
	UserID    string                    `json:"userId"`
	PlanCode  PlanCode                  `json:"planCode"`
	Features  map[featuregate.Feature]bool `json:"features"`
	Source    string                    `json:"source"` // subscription | default_free | expired_fallback
}

// HasEntitlement reports whether the resolved snapshot includes feature.
func (e *Entitlement) HasEntitlement(feature featuregate.Feature) bool {
	if e == nil {
		return false
	}
	if !featuregate.IsKnown(feature) {
		return false
	}
	return e.Features[feature]
}

// ToFeatureGateUser maps Entitlement to the FeatureGate subject.
// PROFESSIONAL maps to Pro catalog for C3 (Professional-specific keys can overlay later).
func (e *Entitlement) ToFeatureGateUser() *featuregate.User {
	tier := featuregate.TierFree
	if e != nil {
		switch e.PlanCode {
		case PlanPRO, PlanPROFESSIONAL:
			tier = featuregate.TierPro
		case PlanENTERPRISE:
			tier = featuregate.TierEnterprise
		default:
			tier = featuregate.TierFree
		}
	}
	id := ""
	if e != nil {
		id = e.UserID
	}
	return &featuregate.User{ID: id, Tier: tier}
}

// ResolveEntitlementFromPlan builds an Entitlement from a plan via the product catalog.
// Uses CatalogAllows (not FeatureGate.Allow) so plan resolution does not require prior grants.
func ResolveEntitlementFromPlan(userID string, plan PlanCode, source string) *Entitlement {
	plan = NormalizePlanCode(plan)
	fgUser := (&Entitlement{UserID: userID, PlanCode: plan}).ToFeatureGateUser()
	feats := make(map[featuregate.Feature]bool, len(featuregate.KnownFeatures()))
	for _, f := range featuregate.KnownFeatures() {
		feats[f] = featuregate.CatalogAllows(fgUser.Tier, f)
	}
	if source == "" {
		source = "subscription"
	}
	return &Entitlement{
		UserID:   userID,
		PlanCode: plan,
		Features: feats,
		Source:   source,
	}
}
