package portfolioreplay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfoliosim"
	"go-stock/backend/selection"
)

var forbiddenJSONKeys = []string{
	"plan_id", "freeze_at", "enable_execute", "trade_plan",
	"pnl", "return", "sharpe", "max_drawdown",
	"order_id", "orders", "fill_id", "fills", "fill",
}

type caseFile struct {
	SchemaVersion string          `json:"schema_version"`
	CaseID        string          `json:"case_id"`
	DecisionTime  string          `json:"decision_time"`
	TradeDate     string          `json:"trade_date"`
	Source        string          `json:"source"`
	SourceRef     string          `json:"source_ref"`
	Versions      EngineVersions  `json:"versions"`
	Snapshot      snapshotFile    `json:"snapshot"`
	Candidates    candidatesFile  `json:"candidates"`
	Constraints   constraintFile  `json:"constraints"`
	Budget        json.RawMessage `json:"budget"`
	Filter        filterFile      `json:"filter"`
	Notes         string          `json:"notes"`
}

type snapshotFile struct {
	AccountID     uint           `json:"account_id"`
	AsOf          string         `json:"as_of"`
	Found         *bool          `json:"found"`
	Equity        float64        `json:"equity"`
	Cash          float64        `json:"cash"`
	AvailableCash float64        `json:"available_cash"`
	ReservedCash  float64        `json:"reserved_cash"`
	MarketValue   float64        `json:"market_value"`
	Exposure      float64        `json:"exposure"`
	PositionCount int            `json:"position_count"`
	Positions     []positionFile `json:"positions"`
}

type positionFile struct {
	StockCode       string  `json:"stock_code"`
	StockName       string  `json:"stock_name"`
	Volume          int64   `json:"volume"`
	AvailableVolume int64   `json:"available_volume"`
	LockedVolume    int64   `json:"locked_volume"`
	MarketValue     float64 `json:"market_value"`
	Weight          float64 `json:"weight"`
	Industry        string  `json:"industry"`
}

type candidatesFile struct {
	SelectionLimit   int                   `json:"selection_limit"`
	RankedCandidates []selection.Candidate `json:"ranked_candidates"`
}

type constraintFile struct {
	User      preferenceFile `json:"user"`
	Strategy  preferenceFile `json:"strategy"`
	Portfolio preferenceFile `json:"portfolio"`
	Risk      riskFile       `json:"risk"`
}

type preferenceFile struct {
	MaxNewNames         *int     `json:"max_new_names"`
	SkipAlreadyHolding  *bool    `json:"skip_already_holding"`
	AllowAddToHolding   *bool    `json:"allow_add_to_holding"`
	MaxSectorWeight     *float64 `json:"max_sector_weight"`
	MaxNamesPerSector   *int     `json:"max_names_per_sector"`
	ReserveCashRatio    *float64 `json:"reserve_cash_ratio"`
	MinOrderAmount      *float64 `json:"min_order_amount"`
	MaxSingleWeight     *float64 `json:"max_single_weight"`
	MaxGrossExposurePct *float64 `json:"max_gross_exposure_pct"`
}

type riskFile struct {
	MaxGrossExposurePct *float64 `json:"max_gross_exposure_pct"`
	MaxSingleNamePct    *float64 `json:"max_single_name_pct"`
	MarketLevel         int      `json:"market_level"`
	BlockNewEntries     bool     `json:"block_new_entries"`
	MaxDailyLossPct     float64  `json:"max_daily_loss_pct"`
}

type filterFile struct {
	SkipCompatibilityCheck bool `json:"skip_compatibility_check"`
	ScanLimit              int  `json:"scan_limit"`
	MaxNames               int  `json:"max_names"`
}

type datasetFile struct {
	SchemaVersion string            `json:"schema_version"`
	Cases         []json.RawMessage `json:"cases"`
}

// LoadDir loads every *.json fixture in dir (file fixture first). Does not query a database.
func LoadDir(dir string) ([]ReplayCase, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []ReplayCase
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		cases, err := LoadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out = append(out, cases...)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no replay fixtures in %s", dir)
	}
	return out, nil
}

// LoadFile loads a single ReplayCase or a {cases:[...]} dataset.
func LoadFile(path string) ([]ReplayCase, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseFixture(raw, path)
}

func parseFixture(raw []byte, ref string) ([]ReplayCase, error) {
	if err := rejectForbiddenKeys(raw); err != nil {
		return nil, err
	}
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 {
		return nil, fmt.Errorf("empty fixture")
	}
	if trim[0] == '{' {
		var ds datasetFile
		if err := json.Unmarshal(trim, &ds); err == nil && len(ds.Cases) > 0 {
			out := make([]ReplayCase, 0, len(ds.Cases))
			for i, c := range ds.Cases {
				one, err := decodeCase(c, fmt.Sprintf("%s#cases[%d]", ref, i))
				if err != nil {
					return nil, err
				}
				out = append(out, one)
			}
			return out, nil
		}
	}
	one, err := decodeCase(trim, ref)
	if err != nil {
		return nil, err
	}
	return []ReplayCase{one}, nil
}

func decodeCase(raw []byte, ref string) (ReplayCase, error) {
	if err := rejectForbiddenKeys(raw); err != nil {
		return ReplayCase{}, err
	}
	var f caseFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return ReplayCase{}, err
	}
	if f.SchemaVersion != SchemaReplayCaseV1 {
		return ReplayCase{}, fmt.Errorf("schema_version=%q want %s", f.SchemaVersion, SchemaReplayCaseV1)
	}
	if strings.TrimSpace(f.CaseID) == "" {
		return ReplayCase{}, fmt.Errorf("case_id required (%s)", ref)
	}
	if f.Versions.SelectionVersion == "" || f.Versions.ConstraintVersion == "" ||
		f.Versions.AllocationVersion == "" || f.Versions.SimulationVersion == "" {
		return ReplayCase{}, fmt.Errorf("case %s: all engine versions required", f.CaseID)
	}
	dt, err := time.Parse(time.RFC3339, f.DecisionTime)
	if err != nil {
		return ReplayCase{}, fmt.Errorf("case %s: decision_time: %w", f.CaseID, err)
	}
	if f.Snapshot.Found == nil {
		return ReplayCase{}, fmt.Errorf("case %s: snapshot.found is required", f.CaseID)
	}
	snap, err := toSnapshot(f.Snapshot, dt)
	if err != nil {
		return ReplayCase{}, fmt.Errorf("case %s: snapshot: %w", f.CaseID, err)
	}
	if f.Candidates.RankedCandidates == nil {
		return ReplayCase{}, fmt.Errorf("case %s: candidates.ranked_candidates required", f.CaseID)
	}
	for i := range f.Candidates.RankedCandidates {
		f.Candidates.RankedCandidates[i].StockCode = strings.ToLower(strings.TrimSpace(f.Candidates.RankedCandidates[i].StockCode))
	}
	cands := &selection.CandidateSelectionResult{
		SelectionLimit:   f.Candidates.SelectionLimit,
		RankedCandidates: f.Candidates.RankedCandidates,
	}
	budget, err := toBudget(f.Budget)
	if err != nil {
		return ReplayCase{}, fmt.Errorf("case %s: budget: %w", f.CaseID, err)
	}
	src := strings.TrimSpace(f.Source)
	if src == "" {
		src = SourceFileFixture
	}
	return ReplayCase{
		SchemaVersion: f.SchemaVersion,
		CaseID:        f.CaseID,
		DecisionTime:  dt,
		TradeDate:     f.TradeDate,
		Source:        src,
		SourceRef:     firstNonEmpty(f.SourceRef, ref),
		Versions:      f.Versions,
		Snapshot:      snap,
		Candidates:    cands,
		Constraints:   toConstraints(f.Constraints),
		Budget:        budget,
		Filter: portfoliosim.FilterCompatRequest{
			SkipCompatibilityCheck: f.Filter.SkipCompatibilityCheck,
			ScanLimit:              f.Filter.ScanLimit,
			MaxNames:               f.Filter.MaxNames,
		},
		Notes: f.Notes,
	}, nil
}

func toSnapshot(f snapshotFile, decisionTime time.Time) (*portfoliolayer.PortfolioSnapshot, error) {
	asOf := decisionTime
	if strings.TrimSpace(f.AsOf) != "" {
		t, err := time.Parse(time.RFC3339, f.AsOf)
		if err != nil {
			return nil, fmt.Errorf("as_of: %w", err)
		}
		asOf = t
		if absDuration(asOf.Sub(decisionTime)) > asOfMismatchTolerance {
			return nil, fmt.Errorf("as_of_mismatch")
		}
	}
	positions := make([]portfoliolayer.SnapshotPosition, 0, len(f.Positions))
	for _, p := range f.Positions {
		positions = append(positions, portfoliolayer.SnapshotPosition{
			StockCode:       strings.ToLower(strings.TrimSpace(p.StockCode)),
			StockName:       p.StockName,
			Volume:          p.Volume,
			AvailableVolume: p.AvailableVolume,
			LockedVolume:    p.LockedVolume,
			MarketValue:     p.MarketValue,
			Weight:          p.Weight,
			Industry:        p.Industry,
		})
	}
	return &portfoliolayer.PortfolioSnapshot{
		AccountID:     f.AccountID,
		AsOf:          asOf,
		Found:         *f.Found,
		Equity:        f.Equity,
		Cash:          f.Cash,
		AvailableCash: f.AvailableCash,
		ReservedCash:  f.ReservedCash,
		MarketValue:   f.MarketValue,
		Exposure:      f.Exposure,
		PositionCount: f.PositionCount,
		Positions:     positions,
	}, nil
}

func toBudget(raw json.RawMessage) (*portfoliolayer.AllocationBudget, error) {
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 || bytes.Equal(trim, []byte("null")) {
		return nil, nil
	}
	var f struct {
		AvailableCash    float64 `json:"available_cash"`
		ReserveCash      float64 `json:"reserve_cash"`
		RiskBudget       float64 `json:"risk_budget"`
		AvailableCapital float64 `json:"available_capital"`
		Binding          string  `json:"binding"`
		PolicyGrossPct   float64 `json:"policy_gross_pct"`
	}
	if err := json.Unmarshal(trim, &f); err != nil {
		return nil, err
	}
	return &portfoliolayer.AllocationBudget{
		AvailableCash:    f.AvailableCash,
		ReserveCash:      f.ReserveCash,
		RiskBudget:       f.RiskBudget,
		AvailableCapital: f.AvailableCapital,
		Binding:          f.Binding,
		PolicyGrossPct:   f.PolicyGrossPct,
	}, nil
}

func toConstraints(f constraintFile) portfoliolayer.ConstraintSet {
	cp := func(p preferenceFile) portfoliolayer.PreferenceLayer {
		return portfoliolayer.PreferenceLayer{
			MaxNewNames:         p.MaxNewNames,
			SkipAlreadyHolding:  p.SkipAlreadyHolding,
			AllowAddToHolding:   p.AllowAddToHolding,
			MaxSectorWeight:     p.MaxSectorWeight,
			MaxNamesPerSector:   p.MaxNamesPerSector,
			ReserveCashRatio:    p.ReserveCashRatio,
			MinOrderAmount:      p.MinOrderAmount,
			MaxSingleWeight:     p.MaxSingleWeight,
			MaxGrossExposurePct: p.MaxGrossExposurePct,
		}
	}
	return portfoliolayer.ConstraintSet{
		User:      cp(f.User),
		Strategy:  cp(f.Strategy),
		Portfolio: cp(f.Portfolio),
		Risk: portfoliolayer.RiskLayer{
			MaxGrossExposurePct: f.Risk.MaxGrossExposurePct,
			MaxSingleNamePct:    f.Risk.MaxSingleNamePct,
			MarketLevel:         f.Risk.MarketLevel,
			BlockNewEntries:     f.Risk.BlockNewEntries,
			MaxDailyLossPct:     f.Risk.MaxDailyLossPct,
		},
	}
}

func rejectForbiddenKeys(raw []byte) error {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	return walkForbidden(v, "")
}

func walkForbidden(v any, path string) error {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			lk := strings.ToLower(k)
			for _, bad := range forbiddenJSONKeys {
				if lk == bad {
					return fmt.Errorf("forbidden field %s", k)
				}
			}
			if err := walkForbidden(child, path+"."+k); err != nil {
				return err
			}
		}
	case []any:
		for i, child := range t {
			if err := walkForbidden(child, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
