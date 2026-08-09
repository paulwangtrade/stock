package featuregate

// Tier is the commercial plan used to resolve default feature access.
// C1 supports Free / Pro / Enterprise.
type Tier string

const (
	TierFree       Tier = "free"
	TierPro        Tier = "pro"
	TierEnterprise Tier = "enterprise"
)

// NormalizeTier returns a known tier or TierFree for empty/unknown values.
func NormalizeTier(tier Tier) Tier {
	switch tier {
	case TierPro, TierEnterprise:
		return tier
	case TierFree, "":
		return TierFree
	default:
		return TierFree
	}
}
