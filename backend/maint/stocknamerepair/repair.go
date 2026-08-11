// Package stocknamerepair backfills empty trade_plan_items.stock_name (Phase10 N.1/N.2).
//
// Scope: display quality only. Never overwrites non-empty names; never touches
// prices, volumes, status, Intent, Gateway, Broker, or paper_sim_*.
package stocknamerepair

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/stockname"

	"gorm.io/gorm"
)

// Source tags map onto stockname.Source* (kept for CLI/table compatibility).
const (
	SourceP1SamePlan   = stockname.SourceTradePlan
	SourceP2Pool       = stockname.SourceCandidatePool
	SourceP3OtherLocal = stockname.SourceTradePlan
	SourceP4Followed   = stockname.SourceFollowed
	SourceP4Basic      = stockname.SourceTushare
)

// Options configures a repair run.
type Options struct {
	// Apply=false → dry-run (plan only); Apply=true → CAS update empty names only.
	Apply bool
	// Codes optional filter (normalized lowercase match on stock_code).
	Codes []string
	// SincePlanID when >0 only repairs items with plan_id >= SincePlanID.
	SincePlanID uint
	// MaxPlanID when >0 only repairs items with plan_id <= MaxPlanID
	// (use to confine to historical segment before SHORT_NAME fix).
	MaxPlanID uint
}

// Row is one planned or applied repair line.
type Row struct {
	PlanID    uint   `json:"plan_id"`
	ItemID    uint   `json:"item_id"`
	StockCode string `json:"stock_code"`
	OldName   string `json:"old_name"`
	NewName   string `json:"new_name"`
	Source    string `json:"source"`
}

// Result summarizes a Run.
type Result struct {
	Mode            string // dry-run | apply
	EmptyScanned    int
	WouldWrite      int
	Applied         int
	CASMiss         int
	Unresolved      int
	UnresolvedCodes []string
	Rows            []Row
}

type emptyItem struct {
	ID        uint
	PlanID    uint
	PoolID    uint
	StockCode string
	StockName string
}

// Run resolves names for empty stock_name rows and optionally applies CAS updates.
func Run(gdb *gorm.DB, opt Options) (*Result, error) {
	if gdb == nil {
		return nil, fmt.Errorf("stocknamerepair: db is nil")
	}
	mode := "dry-run"
	if opt.Apply {
		mode = "apply"
	}
	out := &Result{Mode: mode, Rows: make([]Row, 0)}

	items, err := listEmptyItems(gdb, opt)
	if err != nil {
		return nil, err
	}
	out.EmptyScanned = len(items)
	if len(items) == 0 {
		return out, nil
	}

	unresolvedSeen := map[string]bool{}
	now := time.Now()
	for _, it := range items {
		res := stockname.ResolveWithDB(gdb, it.StockCode, stockname.Hint{
			PlanID: it.PlanID, PoolID: it.PoolID, PlanItemID: it.ID,
		})
		name, source := res.Name, res.Source
		if source == stockname.SourceUnknown || name == "" || name == stockname.UnknownName {
			out.Unresolved++
			code := strings.TrimSpace(it.StockCode)
			if code != "" && !unresolvedSeen[code] {
				unresolvedSeen[code] = true
				out.UnresolvedCodes = append(out.UnresolvedCodes, code)
			}
			continue
		}
		row := Row{
			PlanID:    it.PlanID,
			ItemID:    it.ID,
			StockCode: it.StockCode,
			OldName:   it.StockName,
			NewName:   name,
			Source:    source,
		}
		out.Rows = append(out.Rows, row)
		out.WouldWrite++

		if !opt.Apply {
			continue
		}
		ok, err := applyCAS(gdb, it.ID, name, now)
		if err != nil {
			return out, fmt.Errorf("stocknamerepair: apply item_id=%d: %w", it.ID, err)
		}
		if ok {
			out.Applied++
		} else {
			out.CASMiss++
		}
	}
	return out, nil
}

func listEmptyItems(gdb *gorm.DB, opt Options) ([]emptyItem, error) {
	type row struct {
		ID        uint
		PlanID    uint
		PoolID    uint
		StockCode string
		StockName string
	}
	q := gdb.Table("trade_plan_items AS i").
		Select("i.id AS id, i.plan_id AS plan_id, COALESCE(p.pool_id, 0) AS pool_id, i.stock_code AS stock_code, i.stock_name AS stock_name").
		Joins("LEFT JOIN trade_plans AS p ON p.id = i.plan_id").
		Where("(i.stock_name IS NULL OR TRIM(i.stock_name) = '')").
		Where("i.stock_code IS NOT NULL AND TRIM(i.stock_code) != ''")

	if opt.SincePlanID > 0 {
		q = q.Where("i.plan_id >= ?", opt.SincePlanID)
	}
	if opt.MaxPlanID > 0 {
		q = q.Where("i.plan_id <= ?", opt.MaxPlanID)
	}
	if len(opt.Codes) > 0 {
		norms := make([]string, 0, len(opt.Codes))
		for _, c := range opt.Codes {
			c = strings.ToLower(strings.TrimSpace(c))
			if c != "" {
				norms = append(norms, c)
			}
		}
		if len(norms) > 0 {
			q = q.Where("LOWER(TRIM(i.stock_code)) IN ?", norms)
		}
	}

	var rows []row
	if err := q.Order("i.plan_id ASC, i.id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]emptyItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, emptyItem{
			ID: r.ID, PlanID: r.PlanID, PoolID: r.PoolID,
			StockCode: strings.TrimSpace(r.StockCode),
			StockName: r.StockName,
		})
	}
	return out, nil
}

// applyCAS updates stock_name only when still empty (never overwrites).
func applyCAS(gdb *gorm.DB, itemID uint, name string, at time.Time) (bool, error) {
	name = strings.TrimSpace(name)
	if itemID == 0 || name == "" {
		return false, nil
	}
	res := gdb.Model(&models.TradePlanItem{}).
		Where("id = ? AND (stock_name IS NULL OR TRIM(stock_name) = '')", itemID).
		Updates(map[string]any{
			"stock_name": name,
			"updated_at": at,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// FormatTable prints the standard repair table (plan_id item_id stock_code old new source).
func FormatTable(rows []Row) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-8s %-8s %-12s %-16s %-16s %s\n",
		"plan_id", "item_id", "stock_code", "old_name", "new_name", "source"))
	for _, r := range rows {
		old := r.OldName
		if strings.TrimSpace(old) == "" {
			old = "(empty)"
		}
		b.WriteString(fmt.Sprintf("%-8d %-8d %-12s %-16s %-16s %s\n",
			r.PlanID, r.ItemID, r.StockCode, old, r.NewName, r.Source))
	}
	return b.String()
}
