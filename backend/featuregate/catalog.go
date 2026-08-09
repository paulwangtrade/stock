package featuregate

// CatalogAllows is the static product matrix (tier × feature).
// It is used by Entitlement materialization (Subscription / Tier defaults).
// FeatureGate.Allow must NOT call this for user decisions — use EntitlementService.
func CatalogAllows(tier Tier, feature Feature) bool {
	tier = NormalizeTier(tier)
	if !IsKnown(feature) {
		return false
	}
	byTier, ok := defaultMatrix[feature]
	if !ok {
		return false
	}
	return byTier[tier]
}
