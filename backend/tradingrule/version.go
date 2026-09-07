package tradingrule

// Static pack identity (C.6-H builtin RuleVersion seed).
const (
	StaticVersionID     = "c6h-static-v1"
	StaticSource        = "builtin_pack"
	StaticEffectiveFrom = "1970-01-01" // open-ended history for single-pack bootstrap
	StaticUpdatedAt     = "2026-08-14"
)

// RuleVersion is the version envelope for a trading-rule payload.
// StaticResolver embeds one active version; future repositories may store many.
type RuleVersion struct {
	VersionID     string
	ScopeKey      string // e.g. CN:EQUITY
	Status        string // draft | active | superseded | retired
	EffectiveFrom string // YYYY-MM-DD
	EffectiveTo   string // empty = open
	Source        string
	SourceRef     string
	UpdatedAt     string
	Notes         string
}

// RuleVersion statuses (reserved for repository lifecycle).
const (
	VersionStatusDraft      = "draft"
	VersionStatusActive     = "active"
	VersionStatusSuperseded = "superseded"
	VersionStatusRetired    = "retired"
)

// RuleVersionStore looks up a version envelope by scope and as-of trade date.
// Static pack implements this; a future DB/API repository can replace it without
// changing Resolver callers.
type RuleVersionStore interface {
	// Lookup returns the version that is effective for scopeKey on asOf (YYYY-MM-DD).
	// asOf may be empty → treat as "current" (static store ignores and returns active).
	Lookup(scopeKey, asOf string) (RuleVersion, error)
}

// staticVersionStore is the builtin single-version pack.
type staticVersionStore struct {
	meta RuleVersion
}

func newStaticVersionStore() *staticVersionStore {
	return &staticVersionStore{
		meta: RuleVersion{
			VersionID:     StaticVersionID,
			Status:        VersionStatusActive,
			EffectiveFrom: StaticEffectiveFrom,
			EffectiveTo:   "",
			Source:        StaticSource,
			UpdatedAt:     StaticUpdatedAt,
			Notes:         "Phase10-C.6-M static Availability/PriceLimit/Session defaults",
		},
	}
}

// Lookup implements RuleVersionStore. Scope is stamped onto the returned copy.
func (s *staticVersionStore) Lookup(scopeKey, asOf string) (RuleVersion, error) {
	_ = asOf // single open-ended version; multi-version date windows come later
	v := s.meta
	v.ScopeKey = scopeKey
	return v, nil
}
