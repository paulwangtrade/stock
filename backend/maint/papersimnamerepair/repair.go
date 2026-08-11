// Package papersimnamerepair CAS-backfills empty paper_sim_positions.stock_name.
// Display quality only: never overwrites non-empty names; never touches
// volume, cost, mark, cash, Gateway, or fill prices.
package papersimnamerepair

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/papertrading"
	"go-stock/backend/stockname"

	"gorm.io/gorm"
)

// Options configures a Run.
type Options struct {
	Apply bool
	Codes []string
}

// Row is one planned or applied repair line.
type Row struct {
	ID            uint   `json:"id"`
	StockCode     string `json:"stock_code"`
	CurrentName   string `json:"current_name"`
	ResolvedName  string `json:"resolved_name"`
	Source        string `json:"source"`
}

// Result summarizes a Run.
type Result struct {
	Mode            string
	EmptyScanned    int
	WouldWrite      int
	Applied         int
	CASMiss         int
	Unresolved      int
	UnresolvedCodes []string
	Rows            []Row
}

// Run resolves names for empty paper_sim_positions and optionally CAS-updates.
func Run(gdb *gorm.DB, opt Options) (*Result, error) {
	if gdb == nil {
		return nil, fmt.Errorf("papersimnamerepair: db is nil")
	}
	mode := "dry-run"
	if opt.Apply {
		mode = "apply"
	}
	out := &Result{Mode: mode, Rows: make([]Row, 0)}

	var rows []papertrading.PaperSimPosition
	q := gdb.Model(&papertrading.PaperSimPosition{}).
		Where("stock_code IS NOT NULL AND TRIM(stock_code) != ''").
		Where("(stock_name IS NULL OR TRIM(stock_name) = '')")
	if len(opt.Codes) > 0 {
		norms := make([]string, 0, len(opt.Codes))
		for _, c := range opt.Codes {
			c = strings.ToLower(strings.TrimSpace(c))
			if c != "" {
				norms = append(norms, c)
			}
		}
		if len(norms) > 0 {
			q = q.Where("LOWER(TRIM(stock_code)) IN ?", norms)
		}
	}
	if err := q.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out.EmptyScanned = len(rows)
	unresolvedSeen := map[string]bool{}
	now := time.Now()

	for _, pos := range rows {
		hint := hintForCode(gdb, pos.StockCode)
		res := stockname.ResolveWithDB(gdb, pos.StockCode, hint)
		if res.Source == stockname.SourceUnknown || strings.TrimSpace(res.Name) == "" || res.Name == stockname.UnknownName {
			out.Unresolved++
			code := strings.TrimSpace(pos.StockCode)
			if code != "" && !unresolvedSeen[code] {
				unresolvedSeen[code] = true
				out.UnresolvedCodes = append(out.UnresolvedCodes, code)
			}
			continue
		}
		row := Row{
			ID:           pos.ID,
			StockCode:    pos.StockCode,
			CurrentName:  pos.StockName,
			ResolvedName: res.Name,
			Source:       res.Source,
		}
		out.Rows = append(out.Rows, row)
		out.WouldWrite++
		if !opt.Apply {
			continue
		}
		ok, err := applyCAS(gdb, pos.ID, res.Name, now)
		if err != nil {
			return out, fmt.Errorf("papersimnamerepair: apply id=%d: %w", pos.ID, err)
		}
		if ok {
			out.Applied++
		} else {
			out.CASMiss++
		}
	}
	return out, nil
}

func hintForCode(gdb *gorm.DB, code string) stockname.Hint {
	var fill papertrading.PaperSimFill
	err := gdb.Where("LOWER(TRIM(stock_code)) = ?", strings.ToLower(strings.TrimSpace(code))).
		Order("id DESC").First(&fill).Error
	if err != nil {
		return stockname.Hint{}
	}
	return stockname.Hint{PlanID: fill.PlanID, PlanItemID: fill.PlanItemID}
}

func applyCAS(gdb *gorm.DB, id uint, name string, at time.Time) (bool, error) {
	name = strings.TrimSpace(name)
	if id == 0 || name == "" || name == stockname.UnknownName {
		return false, nil
	}
	res := gdb.Model(&papertrading.PaperSimPosition{}).
		Where("id = ? AND (stock_name IS NULL OR TRIM(stock_name) = '')", id).
		Updates(map[string]any{
			"stock_name": name,
			"updated_at": at,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// FormatTable prints symbol / current / resolved / source.
func FormatTable(rows []Row) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-6s %-12s %-16s %-16s %s\n",
		"id", "symbol", "current_name", "resolved_name", "source"))
	for _, r := range rows {
		cur := r.CurrentName
		if strings.TrimSpace(cur) == "" {
			cur = "(empty)"
		}
		b.WriteString(fmt.Sprintf("%-6d %-12s %-16s %-16s %s\n",
			r.ID, r.StockCode, cur, r.ResolvedName, r.Source))
	}
	return b.String()
}
