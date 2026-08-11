// Command: tradeplan name repair
//
// One-shot maintenance CLI for empty trade_plan_items.stock_name backfill.
// Does not touch Gateway / Broker / PaperTrading / prices / status / Intent.
//
// Usage:
//
//	go run ./cmd/tradeplan-name-repair --dry-run [--db path]
//	go run ./cmd/tradeplan-name-repair --apply [--db path]
//
// Positional tokens "tradeplan" "name" "repair" are accepted and ignored
// so operators can type: tradeplan name repair --dry-run
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go-stock/backend/maint/stocknamerepair"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("tradeplan name repair", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	dryRun := fs.Bool("dry-run", false, "plan only: print plan_id/item_id/stock_code/old_name/new_name/source")
	apply := fs.Bool("apply", false, "CAS-update empty stock_name only (never overwrite)")
	dbPath := fs.String("db", "", "sqlite path (default: build/bin/data/stock.db)")
	codes := fs.String("codes", "", "optional comma-separated stock_code filter")
	sincePlanID := fs.Uint("since-plan-id", 0, "optional: only plan_id >= N")
	maxPlanID := fs.Uint("max-plan-id", 0, "optional: only plan_id <= N (historical segment)")

	args = stripCommandTokens(args)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *dryRun == *apply {
		fmt.Fprintln(os.Stderr, "error: exactly one of --dry-run or --apply is required")
		fmt.Fprintln(os.Stderr, "usage: tradeplan name repair --dry-run|--apply [--db path] [--codes a,b] [--since-plan-id N] [--max-plan-id N]")
		return 2
	}

	path := strings.TrimSpace(*dbPath)
	if path == "" {
		path = filepath.Join("build", "bin", "data", "stock.db")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolve db path: %v\n", err)
		return 1
	}
	if _, err := os.Stat(abs); err != nil {
		fmt.Fprintf(os.Stderr, "error: db not found: %s (%v)\n", abs, err)
		return 1
	}

	gdb, err := gorm.Open(sqlite.Open(abs+"?_busy_timeout=10000&_journal_mode=WAL"), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open db: %v\n", err)
		return 1
	}

	opt := stocknamerepair.Options{
		Apply:       *apply,
		SincePlanID: uint(*sincePlanID),
		MaxPlanID:   uint(*maxPlanID),
	}
	if c := strings.TrimSpace(*codes); c != "" {
		for _, p := range strings.Split(c, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				opt.Codes = append(opt.Codes, p)
			}
		}
	}

	res, err := stocknamerepair.Run(gdb, opt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: repair: %v\n", err)
		return 1
	}

	fmt.Printf("mode=%s db=%s empty_scanned=%d would_write=%d applied=%d cas_miss=%d unresolved=%d\n",
		res.Mode, abs, res.EmptyScanned, res.WouldWrite, res.Applied, res.CASMiss, res.Unresolved)
	if len(res.Rows) > 0 {
		fmt.Print(stocknamerepair.FormatTable(res.Rows))
	} else {
		fmt.Println("(no rows to write)")
	}
	if len(res.UnresolvedCodes) > 0 {
		fmt.Printf("WARN unresolved codes (%d): %s\n",
			len(res.UnresolvedCodes), strings.Join(res.UnresolvedCodes, ", "))
	}

	if res.Unresolved > 0 && res.WouldWrite == 0 && res.Applied == 0 {
		return 1
	}
	return 0
}

// stripCommandTokens allows: tradeplan name repair --dry-run
func stripCommandTokens(args []string) []string {
	expect := []string{"tradeplan", "name", "repair"}
	i := 0
	for i < len(args) && i < len(expect) && strings.EqualFold(args[i], expect[i]) {
		i++
	}
	return args[i:]
}
