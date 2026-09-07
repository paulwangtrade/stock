package portfoliolayer

// DefaultShadowEnabled is the compile-time default. Production stays false:
// Observe is a no-op unless the caller passes ObserveOptions{Enabled: true}.
const DefaultShadowEnabled = false

// ShadowStatus is the Observe outcome. Disabled means the decision core did not run.
const (
	ShadowDisabled = "disabled"
	ShadowObserved = "observed"
)

// ObserveOptions opts into the shadow path. Enabled defaults to DefaultShadowEnabled.
type ObserveOptions struct {
	Enabled bool
	// LegacyAmountPerName is the production scalar to compare against.
	// Zero means LegacyFixedAmountPerName. Do not pass a Sizer; this is a number only.
	LegacyAmountPerName float64
}

// ShadowObservation is a read-only sidecar. It must not be written to TradePlan items.
type ShadowObservation struct {
	Status     string
	Enabled    bool
	Decision   *DecisionObservation
	Report     *PortfolioShadowReport
	Evaluation *ShadowEvaluationReport
}

// Observe is the F.4/F.5 shadow entry. Default is off: no Select, no Allocate, no TradePlan.
// Callers on the trading chain must not invoke this. Tests pass Enabled: true.
func Observe(in DecisionInput, opts ObserveOptions) *ShadowObservation {
	enabled := DefaultShadowEnabled || opts.Enabled
	if !enabled {
		return &ShadowObservation{Status: ShadowDisabled, Enabled: false}
	}
	obs, _ := NewSkeletonFlow().Run(in)
	report := BuildShadowReport(in, obs, opts)
	return &ShadowObservation{
		Status:     ShadowObserved,
		Enabled:    true,
		Decision:   obs,
		Report:     report,
		Evaluation: EvaluateShadow(in, report),
	}
}
