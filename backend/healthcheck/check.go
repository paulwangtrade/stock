// Package healthcheck provides read-only SQLite database health probes for Beta runtime.
// It never writes, migrates, or repairs data.
package healthcheck

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/opportunity/outcome"
	"go-stock/backend/portfolio/positionstate"

	"gorm.io/gorm"
)

// RequiredSchemaVersion is the minimum applied schema_migrations version for Phase16 Beta.
const RequiredSchemaVersion = 11

// Status values for the overall health report.
const (
	StatusOK   = "OK"
	StatusWarn = "WARN"
	StatusFail = "FAIL"
)

// Logical table keys exposed in reports (user-facing names).
const (
	TableSignalScanSnapshots = "signal_scan_snapshots"
	TableCandidatePoolItems  = "candidate_pool_items"
	TableTradePlans          = "trade_plans"
	TablePaperSimFills       = "paper_sim_fills"
	TablePositions           = "positions" // physical: paper_sim_positions
	TablePositionStates      = "position_states"
)

var requiredPhysicalTables = map[string]string{
	TableSignalScanSnapshots: "signal_scan_snapshots",
	TableCandidatePoolItems:  "candidate_pool_items",
	TableTradePlans:          "trade_plans",
	TablePaperSimFills:       "paper_sim_fills",
	TablePositions:           "paper_sim_positions",
}

// Result is a structured read-only health report.
type Result struct {
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
	Schema    Schema    `json:"schema"`
	Tables    []Table   `json:"tables"`
	Checks    []Check   `json:"checks"`
	Messages  []string  `json:"messages,omitempty"`
}

// Schema reports schema_migrations version state.
type Schema struct {
	AppliedVersion  int  `json:"applied_version"`
	RequiredVersion int  `json:"required_version"`
	OK              bool `json:"ok"`
	Detail          string `json:"detail,omitempty"`
}

// Table reports table existence and row count.
type Table struct {
	Key           string `json:"key"`
	PhysicalName  string `json:"physical_name"`
	Exists        bool   `json:"exists"`
	RowCount      int64  `json:"row_count"`
	OK            bool   `json:"ok"`
	Note          string `json:"note,omitempty"`
}

// Check is a single consistency probe.
type Check struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Detail  string `json:"detail"`
	Count   int64  `json:"count,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
}

// OK is true when overall status is OK or WARN (no hard failures).
func (r Result) OK() bool {
	return r.Status == StatusOK || r.Status == StatusWarn
}

// Failed is true when status is FAIL.
func (r Result) Failed() bool {
	return r.Status == StatusFail
}

// Summary returns a one-line human summary.
func (r Result) Summary() string {
	if len(r.Messages) > 0 {
		return strings.Join(r.Messages, "; ")
	}
	return fmt.Sprintf("status=%s schema=%d/%d", r.Status, r.Schema.AppliedVersion, r.Schema.RequiredVersion)
}

// Run executes all read-only probes against the given database handle.
func Run(gdb *gorm.DB) Result {
	return run(gdb, RequiredSchemaVersion)
}

func run(gdb *gorm.DB, requiredVersion int) Result {
	res := Result{
		Status:    StatusOK,
		CheckedAt: time.Now().UTC(),
		Schema: Schema{
			RequiredVersion: requiredVersion,
		},
	}
	if gdb == nil {
		res.Status = StatusFail
		res.Messages = append(res.Messages, "database handle is nil")
		return res
	}

	res.Schema = probeSchema(gdb, requiredVersion)
	if !res.Schema.OK {
		res.Status = StatusFail
		res.Messages = append(res.Messages, res.Schema.Detail)
	}

	for key, physical := range requiredPhysicalTables {
		res.Tables = append(res.Tables, probeTable(gdb, key, physical))
	}
	// position_states is a computed view, not a physical table.
	res.Tables = append(res.Tables, probePositionStates())

	for _, t := range res.Tables {
		if t.Key == TablePositionStates {
			continue // service probe, not physical table
		}
		if !t.Exists {
			res.Status = downgrade(res.Status, StatusFail)
			res.Messages = append(res.Messages, fmt.Sprintf("missing table %s (%s)", t.Key, t.PhysicalName))
		}
	}

	res.Checks = append(res.Checks,
		probeFillsHavePlanID(gdb),
		probePositionsHaveStockCode(gdb),
		probeOutcomeReadable(gdb),
		probePositionStatesReadable(gdb),
	)

	for _, c := range res.Checks {
		if c.Skipped {
			if res.Status == StatusOK {
				res.Status = StatusWarn
			}
			continue
		}
		if !c.OK {
			res.Status = downgrade(res.Status, StatusFail)
			res.Messages = append(res.Messages, c.Name+": "+c.Detail)
		}
	}

	if res.Status == StatusOK && len(res.Messages) == 0 {
		res.Messages = append(res.Messages, fmt.Sprintf(
			"schema v%d ready; %d tables verified; consistency checks passed",
			res.Schema.AppliedVersion, len(requiredPhysicalTables),
		))
	}
	return res
}

func downgrade(current, next string) string {
	if current == StatusFail || next == StatusFail {
		return StatusFail
	}
	if current == StatusWarn || next == StatusWarn {
		return StatusWarn
	}
	return next
}

func probeSchema(gdb *gorm.DB, required int) Schema {
	s := Schema{RequiredVersion: required}
	if !tableExists(gdb, "schema_migrations") {
		s.Detail = "schema_migrations table missing"
		return s
	}
	var ver int
	if err := gdb.Raw(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations WHERE status = 'applied' OR status IS NULL OR status = ''`).Scan(&ver).Error; err != nil {
		s.Detail = "cannot read schema_migrations: " + err.Error()
		return s
	}
	// Fallback without status filter if legacy rows lack status.
	if ver == 0 {
		_ = gdb.Raw(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&ver).Error
	}
	s.AppliedVersion = ver
	if ver < required {
		s.Detail = fmt.Sprintf("applied schema version %d < required %d", ver, required)
		return s
	}
	s.OK = true
	s.Detail = fmt.Sprintf("schema version %d >= %d", ver, required)
	return s
}

func probeTable(gdb *gorm.DB, key, physical string) Table {
	t := Table{Key: key, PhysicalName: physical}
	if !tableExists(gdb, physical) {
		t.Note = "table not found in sqlite_master"
		return t
	}
	t.Exists = true
	t.RowCount = countRows(gdb, physical)
	t.OK = true
	return t
}

func probePositionStates() Table {
	return Table{
		Key:          TablePositionStates,
		PhysicalName: "(computed)",
		Exists:       true,
		OK:           true,
		Note:         "read-only projection via portfolio/positionstate; not a physical table",
	}
}

func probeFillsHavePlanID(gdb *gorm.DB) Check {
	name := "fill_has_plan_id"
	if !tableExists(gdb, "paper_sim_fills") {
		return Check{Name: name, OK: false, Detail: "paper_sim_fills table missing"}
	}
	var missing int64
	_ = gdb.Raw(`SELECT COUNT(*) FROM paper_sim_fills WHERE COALESCE(plan_id, 0) = 0`).Scan(&missing).Error
	if missing > 0 {
		return Check{Name: name, OK: false, Detail: fmt.Sprintf("%d fill(s) missing plan_id", missing), Count: missing}
	}
	total := countRows(gdb, "paper_sim_fills")
	return Check{Name: name, OK: true, Detail: fmt.Sprintf("all %d fill(s) have plan_id", total), Count: total}
}

func probePositionsHaveStockCode(gdb *gorm.DB) Check {
	name := "position_has_stock_code"
	if !tableExists(gdb, "paper_sim_positions") {
		return Check{Name: name, OK: false, Detail: "paper_sim_positions table missing"}
	}
	var missing int64
	_ = gdb.Raw(`SELECT COUNT(*) FROM paper_sim_positions WHERE stock_code IS NULL OR TRIM(stock_code) = ''`).Scan(&missing).Error
	if missing > 0 {
		return Check{Name: name, OK: false, Detail: fmt.Sprintf("%d position row(s) missing stock_code", missing), Count: missing}
	}
	total := countRows(gdb, "paper_sim_positions")
	return Check{Name: name, OK: true, Detail: fmt.Sprintf("all %d position row(s) have stock_code", total), Count: total}
}

func probeOutcomeReadable(gdb *gorm.DB) Check {
	name := "outcome_readable"
	if !tableExists(gdb, "paper_sim_fills") {
		return Check{Name: name, Skipped: true, OK: true, Detail: "skipped: paper_sim_fills missing"}
	}
	code := sampleStockCodeFromFills(gdb)
	if code == "" {
		code = sampleStockCodeFromPool(gdb)
	}
	if code == "" {
		return Check{Name: name, Skipped: true, OK: true, Detail: "skipped: no stock_code sample in fills or candidate_pool_items"}
	}
	prev := db.Dao
	db.Dao = gdb
	defer func() { db.Dao = prev }()

	rows, err := outcome.NewService().ProjectStock(code, outcome.ProjectOptions{IncludeNoTrade: true})
	if err != nil {
		return Check{Name: name, OK: false, Detail: fmt.Sprintf("outcome read failed for %s: %v", code, err)}
	}
	return Check{
		Name:   name,
		OK:     true,
		Detail: fmt.Sprintf("outcome projection readable for %s (%d row(s))", code, len(rows)),
		Count:  int64(len(rows)),
	}
}

func probePositionStatesReadable(gdb *gorm.DB) Check {
	name := "position_states_readable"
	prev := db.Dao
	db.Dao = gdb
	defer func() { db.Dao = prev }()

	bundle := positionstate.NewService(nil).Evaluate(positionstate.Query{})
	if bundle == nil {
		return Check{Name: name, OK: false, Detail: "position_states Evaluate returned nil"}
	}
	return Check{
		Name:   name,
		OK:     true,
		Detail: fmt.Sprintf("position_states bundle built (%d position(s))", len(bundle.Positions)),
		Count:  int64(len(bundle.Positions)),
	}
}

func tableExists(gdb *gorm.DB, name string) bool {
	var n int64
	_ = gdb.Raw(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n).Error
	return n > 0
}

func countRows(gdb *gorm.DB, table string) int64 {
	if !tableExists(gdb, table) {
		return 0
	}
	var n int64
	_ = gdb.Raw(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table)).Scan(&n).Error
	return n
}

func sampleStockCodeFromFills(gdb *gorm.DB) string {
	var code string
	_ = gdb.Raw(`SELECT stock_code FROM paper_sim_fills WHERE stock_code IS NOT NULL AND TRIM(stock_code) != '' ORDER BY id DESC LIMIT 1`).Scan(&code).Error
	return strings.TrimSpace(code)
}

func sampleStockCodeFromPool(gdb *gorm.DB) string {
	if !tableExists(gdb, "candidate_pool_items") {
		return ""
	}
	var code string
	_ = gdb.Raw(`SELECT stock_code FROM candidate_pool_items WHERE stock_code IS NOT NULL AND TRIM(stock_code) != '' ORDER BY id DESC LIMIT 1`).Scan(&code).Error
	return strings.TrimSpace(code)
}
