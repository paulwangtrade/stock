package papertrading

// PriceKind values carried on Quote for Broker fill_reason selection.
const (
	PriceKindOpen  = "open"
	PriceKindClose = "close"
)

// PriceMode values exposed on ExecutionResult (Gateway observability).
const (
	PriceModeRealtime = "realtime"
	PriceModeClose    = "close"
	PriceModeNone     = "none"
)

// SelectFillProvider chooses the production Fill price source for an allowed session.
//
//	A → RealtimeOpenPriceProvider
//	B → CloseFillProvider
//
// C/closed must be rejected by Session Policy before this is called.
//
// override: StaticPriceProvider (and other test doubles) may override for tests.
// RealtimeOpenPriceProvider / DefaultOpen from cron is NOT treated as override on B —
// Session B always switches to CloseFillProvider.
func SelectFillProvider(session ExecutionSession, override PriceProvider) PriceProvider {
	if isTestPriceOverride(override) {
		return override
	}
	switch session {
	case SessionA:
		return DefaultOpenPriceProvider()
	case SessionB:
		return CloseFillProvider{}
	default:
		return MissingPriceProvider{}
	}
}

func isTestPriceOverride(p PriceProvider) bool {
	if p == nil {
		return false
	}
	switch p.(type) {
	case StaticPriceProvider, *StaticPriceProvider:
		return true
	case MissingPriceProvider, *MissingPriceProvider:
		// Explicit fail-closed injection in tests.
		return true
	default:
		return false
	}
}

func priceModeForSession(session ExecutionSession) string {
	switch session {
	case SessionA:
		return PriceModeRealtime
	case SessionB:
		return PriceModeClose
	default:
		return PriceModeNone
	}
}
