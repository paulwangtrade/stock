package main

import (
	"go-stock/backend/entitlement"
	"go-stock/backend/featuregate"
)

// FeatureGateDecisionDTO is the Wails-facing Gate result for UI entry visibility.
type FeatureGateDecisionDTO struct {
	Allowed bool   `json:"allowed"`
	Feature string `json:"feature"`
	Tier    string `json:"tier"`
	Reason  string `json:"reason"`
}

func shellUserForTier(tier string) *featuregate.User {
	user := &featuregate.User{ID: "shell:" + tier, Tier: featuregate.Tier(tier)}
	_ = entitlement.Default().EnsureTierDefaults(user)
	return user
}

// FeatureGateAllow reports whether the given tier may open a Shell feature entry.
// Trading Engine paths must not call this to approve/reject orders or plans.
// Access is decided by EntitlementService (not if user.Plan == "Pro").
func (a *App) FeatureGateAllow(tier string, feature string) bool {
	return featuregate.Allow(shellUserForTier(tier), featuregate.Feature(feature))
}

// FeatureGateCanAccess returns a full decision for UI (upgrade CTA / disable reason).
func (a *App) FeatureGateCanAccess(tier string, feature string) FeatureGateDecisionDTO {
	d := featuregate.CanAccess(shellUserForTier(tier), featuregate.Feature(feature))
	return FeatureGateDecisionDTO{
		Allowed: d.Allowed,
		Feature: string(d.Feature),
		Tier:    string(d.Tier),
		Reason:  string(d.Reason),
	}
}

// FeatureGateEvaluateAll returns catalog decisions for the given tier (UI bootstrap).
func (a *App) FeatureGateEvaluateAll(tier string) []FeatureGateDecisionDTO {
	raw := featuregate.EvaluateAll(shellUserForTier(tier))
	out := make([]FeatureGateDecisionDTO, 0, len(raw))
	for _, d := range raw {
		out = append(out, FeatureGateDecisionDTO{
			Allowed: d.Allowed,
			Feature: string(d.Feature),
			Tier:    string(d.Tier),
			Reason:  string(d.Reason),
		})
	}
	return out
}

// FeatureGateKnownFeatures returns the static feature catalog keys.
func (a *App) FeatureGateKnownFeatures() []string {
	feats := featuregate.KnownFeatures()
	out := make([]string, 0, len(feats))
	for _, f := range feats {
		out = append(out, string(f))
	}
	return out
}
