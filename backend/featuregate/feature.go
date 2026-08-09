package featuregate

// Feature identifies a product capability controlled only for UI / entry visibility.
// FeatureGate must never gate Trading Engine paths (TradePlan / Gateway / Broker / Fill).
type Feature string

const (
	FeatureAdvancedRisk        Feature = "AdvancedRisk"
	FeatureAIAnalysis          Feature = "AIAnalysis"
	FeatureBacktest            Feature = "Backtest"
	FeatureRealtimeSignal      Feature = "RealtimeSignal"
	FeatureAdvancedObservation Feature = "AdvancedObservation"
	FeatureMultiAccount        Feature = "MultiAccount" // Phase12-E / Phase13-E Enterprise
)

// KnownFeatures returns the catalog of registered product features.
func KnownFeatures() []Feature {
	return []Feature{
		FeatureAdvancedRisk,
		FeatureAIAnalysis,
		FeatureBacktest,
		FeatureRealtimeSignal,
		FeatureAdvancedObservation,
		FeatureMultiAccount,
	}
}

// CommercialFeatures returns features tracked for Usage commercial analytics (Phase13-E).
func CommercialFeatures() []Feature {
	return []Feature{
		FeatureAIAnalysis,
		FeatureAdvancedRisk,
		FeatureAdvancedObservation,
		FeatureBacktest,
		FeatureMultiAccount,
	}
}

// IsCommercialFeature reports whether feature is in the commercial usage catalog.
func IsCommercialFeature(feature Feature) bool {
	for _, f := range CommercialFeatures() {
		if f == feature {
			return true
		}
	}
	return false
}

// IsKnown reports whether feature is in the catalog.
func IsKnown(feature Feature) bool {
	switch feature {
	case FeatureAdvancedRisk, FeatureAIAnalysis, FeatureBacktest,
		FeatureRealtimeSignal, FeatureAdvancedObservation, FeatureMultiAccount:
		return true
	default:
		return false
	}
}
