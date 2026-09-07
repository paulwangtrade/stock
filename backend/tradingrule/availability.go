package tradingrule

// BuyFillDeltas returns how a buy qty splits into locked vs available.
// Unknown / non-T0 policies fail closed to T1 (all locked).
func BuyFillDeltas(qty int64, policy SellablePolicy) (lockedDelta, availableDelta int64) {
	if qty <= 0 {
		return 0, 0
	}
	if policy == SellableT0 {
		return 0, qty
	}
	return qty, 0
}
