package anchor

// Legacy RefSource labels written by FollowedStockAnchorProvider (B.0 bit-compat).
const (
	RefSourcePrevClose         = "prev_close"
	RefSourceStrategySnapshot  = "strategy_snapshot"
	RefSourceFollowedPrice     = "followed_price"       // reserved; not written in B.0
	RefSourceFollowedFollowPrice = "followed_follow_price" // reserved; not written in B.0
)

// PoolSourceStrategyRun matches models.CandidatePoolSourceStrategyRun without
// importing models into this package.
const PoolSourceStrategyRun = "strategy_run"

// Context is the Resolve input for an Execution Intent anchor lookup.
type Context struct {
	StockCode  string
	TradeDate  string // plan trade date (usually T+1)
	SourceDate string // after-close calendar day T; unused by Followed in B.0
	PoolID     uint
	PoolSource string
	Session    string
}

// Result is a successful anchor. Callers must treat ok=false as soft-fail.
type Result struct {
	RefPrice   float64
	RefSource  string
	RefAsOf    string
	Confidence float64 // observational only; not persisted in B.0
}

// Valid reports whether a Result is usable as a positive Intent ref_price.
func Valid(r Result) bool {
	return r.RefPrice > 0 && r.RefSource != ""
}
