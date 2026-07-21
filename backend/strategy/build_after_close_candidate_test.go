package strategy

import (
	"testing"

	"go-stock/backend/models"
	"go-stock/backend/tradingcalendar"
)

func TestAfterCloseCandidateBuilderBuildsNextTradingDayPool(t *testing.T) {
	var gotTradeDate string
	gotConfig := map[string]any{}
	buildCalls := 0

	builder := &AfterCloseCandidateBuilder{
		Calendar: tradingcalendar.Calendar{},
		build: func(tradeDate string, options ...BuildCandidatePoolOption) (*models.CandidatePool, error) {
			buildCalls++
			gotTradeDate = tradeDate
			for _, option := range options {
				option(gotConfig)
			}
			return &models.CandidatePool{TradeDate: tradeDate}, nil
		},
	}

	pool, err := builder.Build("2026-07-24") // Friday
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if buildCalls != 1 {
		t.Fatalf("BuildCandidatePool calls = %d, want 1", buildCalls)
	}
	if gotTradeDate != "2026-07-27" {
		t.Fatalf("BuildCandidatePool tradeDate = %q, want next Monday", gotTradeDate)
	}
	if pool.TradeDate != gotTradeDate {
		t.Fatalf("pool.TradeDate = %q, want %q", pool.TradeDate, gotTradeDate)
	}
	if gotConfig["session"] != "after_close" {
		t.Fatalf("session = %#v, want after_close", gotConfig["session"])
	}
	if gotConfig["source_date"] != "2026-07-24" {
		t.Fatalf("source_date = %#v, want 2026-07-24", gotConfig["source_date"])
	}
}

func TestAfterCloseCandidateBuilderUsesTradeDateAsEnhancerUpperBound(t *testing.T) {
	builder := &AfterCloseCandidateBuilder{
		Calendar: tradingcalendar.Calendar{
			Holidays: tradingcalendar.StaticHolidays{
				NonTrading: map[string]bool{"2026-07-27": true},
			},
		},
		build: func(tradeDate string, _ ...BuildCandidatePoolOption) (*models.CandidatePool, error) {
			// BuildCandidatePool passes this same value to
			// EnhanceContext.TradeDate, whose repo query is trade_date < tradeDate.
			if tradeDate != "2026-07-28" {
				t.Fatalf("enhancer upper bound = %q, want 2026-07-28", tradeDate)
			}
			return &models.CandidatePool{TradeDate: tradeDate}, nil
		},
	}

	if _, err := builder.Build("2026-07-24"); err != nil {
		t.Fatalf("Build() error = %v", err)
	}
}

func TestAfterCloseCandidateBuilderRejectsInvalidSourceDate(t *testing.T) {
	builder := NewAfterCloseCandidateBuilder()
	if _, err := builder.Build("not-a-date"); err == nil {
		t.Fatal("Build() error = nil, want invalid date error")
	}
}

func TestWithCandidatePoolConfigMergesMetadata(t *testing.T) {
	config := map[string]any{"source": "follow"}
	WithCandidatePoolConfig(map[string]any{
		"session":     "after_close",
		"source_date": "2026-07-24",
	})(config)

	if config["source"] != "follow" {
		t.Fatalf("existing config source = %#v, want follow", config["source"])
	}
	if config["session"] != "after_close" || config["source_date"] != "2026-07-24" {
		t.Fatalf("merged config = %#v", config)
	}
}
