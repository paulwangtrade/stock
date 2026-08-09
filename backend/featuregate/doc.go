// Package featuregate provides Shell-only product capability checks.
//
// FeatureGate controls UI visibility and feature entry points.
// It must never gate Trading Engine behavior (TradePlan, Gateway, Broker, Fill, Settlement).
//
// Example:
//
//	if featuregate.Allow(user, featuregate.FeatureAdvancedRisk) {
//	    // show Advanced Risk menu entry
//	}
package featuregate
