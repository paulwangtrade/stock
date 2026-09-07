package tradingrule

// EnableQuantityPolicy Feature Flag (Phase12-M2.2+ / M2.4 controlled enable).
//
// Resolution order:
//  1. Test/forced override via SetEnableQuantityPolicyForTest (if set)
//  2. data/quantity_policy.json → enableQuantityPolicy (default false)
//
// Production default remains false: Materialize uses calcMorningLotVolume;
// Broker skips Validate gate. Shadow observation always runs (read-only).
func EnableQuantityPolicy() bool {
	qtyOverrideMu.RLock()
	defer qtyOverrideMu.RUnlock()
	if qtyOverride != nil {
		return *qtyOverride
	}
	return GetQuantityPolicyConfig().EnableQuantityPolicy
}

// SetEnableQuantityPolicyForTest forces the flag for tests (does not persist).
func SetEnableQuantityPolicyForTest(enabled bool) {
	qtyOverrideMu.Lock()
	v := enabled
	qtyOverride = &v
	qtyOverrideMu.Unlock()
}

// ResetEnableQuantityPolicyForTest clears the test override (falls back to config / default false).
func ResetEnableQuantityPolicyForTest() {
	qtyOverrideMu.Lock()
	qtyOverride = nil
	qtyOverrideMu.Unlock()
}
