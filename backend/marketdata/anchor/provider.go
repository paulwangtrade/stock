package anchor

// Provider resolves after-close Execution Intent price anchors.
// ok=false means no reliable price; callers must soft-fail (never fabricate).
type Provider interface {
	Resolve(ctx Context) (Result, bool)
}

// Chain tries Sources in order and returns the first positive Valid result.
// B.0 production uses a single FollowedStockAnchorProvider entry.
type Chain struct {
	Sources []Provider
}

// Resolve implements Provider.
func (c Chain) Resolve(ctx Context) (Result, bool) {
	for _, s := range c.Sources {
		if s == nil {
			continue
		}
		r, ok := s.Resolve(ctx)
		if ok && Valid(r) {
			return r, true
		}
	}
	return Result{}, false
}

// DefaultProvider returns the B.0 production chain (Followed only).
func DefaultProvider() Provider {
	return Chain{Sources: []Provider{FollowedStockAnchorProvider{}}}
}
