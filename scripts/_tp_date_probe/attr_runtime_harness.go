// Temporary read-only attribution runtime harness (validation only).
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/papertrading"
)

func main() {
	dsn := `D:\stock\build\bin\data\stock.db?mode=ro&_busy_timeout=5000`
	if len(os.Args) > 1 {
		dsn = os.Args[1]
	}
	db.Init(dsn)
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1_000_000})

	view, err := papertrading.BuildPositionAttribution(papertrading.AttributionOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "build_error: %v\n", err)
		os.Exit(1)
	}
	b, _ := json.MarshalIndent(view, "", "  ")
	fmt.Println(string(b))

	if os.Getenv("ATTR_SERVE") != "1" {
		return
	}
	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	addr := "127.0.0.1:18765"
	fmt.Fprintln(os.Stderr, "listening", addr, time.Now().Format(time.RFC3339))
	_ = http.ListenAndServe(addr, mux)
}
