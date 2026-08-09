package featuregate

// User is the commercial subject for FeatureGate checks.
// It is intentionally independent of Trading Plane models.
type User struct {
	ID   string `json:"id"`
	Tier Tier   `json:"tier"`
}

// LocalFreeUser returns the default single-device Free subject.
func LocalFreeUser() User {
	return User{ID: "local-device", Tier: TierFree}
}
