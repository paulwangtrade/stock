package intelligence_test

import (
	"errors"
	"testing"
	"time"

	"go-stock/backend/marketdata"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/intelligence"
	"go-stock/backend/portfolio/positionstate"

	"github.com/stretchr/testify/require"
)

type stubQuotes struct {
	quotes []marketdata.Quote
	err    error
}

func (s *stubQuotes) GetQuote(code string) (*marketdata.Quote, error) {
	if s.err != nil {
		return nil, s.err
	}
	for i := range s.quotes {
		if s.quotes[i].Code == code {
			q := s.quotes[i]
			return &q, nil
		}
	}
	return nil, nil
}

func (s *stubQuotes) GetQuotes(codes []string) ([]marketdata.Quote, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.quotes, nil
}

func TestBuild_HoldingProfitAndLoss(t *testing.T) {
	snap := &portfolio.Snapshot{
		Found: true,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", StockName: "平安", Volume: 1000, AvailableVolume: 1000, AvgCost: 10, MarkPrice: 11},
			{StockCode: "sh600000", StockName: "浦发", Volume: 500, AvailableVolume: 500, AvgCost: 10, MarkPrice: 9.7}, // -3% → LOSS, not review
		},
	}
	b := intelligence.Build(intelligence.Options{Snapshot: snap, AsOf: time.Now()})
	require.True(t, b.Found)
	require.Len(t, b.Positions, 2)
	by := map[string]intelligence.PositionIntelligenceView{}
	for _, p := range b.Positions {
		by[p.StockCode] = p
	}
	require.Equal(t, intelligence.StatusHoldingProfit, by["sz000001"].PositionStatus)
	require.InDelta(t, 1000.0, by["sz000001"].PnL, 1e-6)
	require.Equal(t, intelligence.StatusHoldingLoss, by["sh600000"].PositionStatus)
	require.Equal(t, intelligence.StrategyUnknown, by["sz000001"].StrategyStatus)
	require.Contains(t, by["sz000001"].AttentionReason, "缺少策略解释")
}

func TestBuild_NeedReviewOnDeepLoss(t *testing.T) {
	snap := &portfolio.Snapshot{
		Found: true,
		Positions: []portfolio.Position{
			{StockCode: "sz000002", Volume: 100, AvailableVolume: 100, AvgCost: 10, MarkPrice: 8.5}, // -15%
		},
	}
	b := intelligence.Build(intelligence.Options{Snapshot: snap})
	require.Equal(t, intelligence.StatusNeedReview, b.Positions[0].PositionStatus)
	require.Equal(t, intelligence.RiskHigh, b.Positions[0].RiskLevel)
}

func TestBuild_NoPosition(t *testing.T) {
	snap := &portfolio.Snapshot{Found: true, Positions: nil}
	b := intelligence.Build(intelligence.Options{
		Snapshot:   snap,
		ExtraCodes: []string{"sz399001"},
	})
	require.Len(t, b.Positions, 1)
	require.Equal(t, intelligence.StatusNoPosition, b.Positions[0].PositionStatus)
	require.Equal(t, "sz399001", b.Positions[0].StockCode)
	require.Contains(t, b.Positions[0].AttentionReason, "无持仓")
}

func TestBuild_MissingStrategyReason(t *testing.T) {
	snap := &portfolio.Snapshot{
		Found: true,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 100, AvailableVolume: 100, AvgCost: 10, MarkPrice: 10.5},
		},
	}
	b := intelligence.Build(intelligence.Options{Snapshot: snap})
	require.Equal(t, intelligence.StrategyUnknown, b.Positions[0].StrategyStatus)
	require.Contains(t, b.Positions[0].AttentionReason, "缺少策略解释")
}

func TestBuild_QuoteMissingDoesNotError(t *testing.T) {
	snap := &portfolio.Snapshot{
		Found: true,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 100, AvailableVolume: 100, AvgCost: 10, MarkPrice: 10},
		},
	}
	b := intelligence.Build(intelligence.Options{
		Snapshot: snap,
		Quotes:   &stubQuotes{err: errors.New("network down")},
	})
	require.Len(t, b.Positions, 1)
	require.Equal(t, "persisted", b.Positions[0].QuoteSource)
	require.InDelta(t, 10.0, b.Positions[0].MarketPrice, 1e-9)
}

func TestBuild_LiveQuoteOverlay(t *testing.T) {
	snap := &portfolio.Snapshot{
		Found: true,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 100, AvailableVolume: 100, AvgCost: 10, MarkPrice: 10},
		},
	}
	b := intelligence.Build(intelligence.Options{
		Snapshot: snap,
		Quotes: &stubQuotes{quotes: []marketdata.Quote{{Code: "sz000001", Price: 12}}},
	})
	require.Equal(t, "live", b.Positions[0].QuoteSource)
	require.InDelta(t, 12.0, b.Positions[0].MarketPrice, 1e-9)
	require.Equal(t, intelligence.StatusHoldingProfit, b.Positions[0].PositionStatus)
}

func TestBuild_NewPositionLocked(t *testing.T) {
	snap := &portfolio.Snapshot{
		Found: true,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 100, AvailableVolume: 0, LockedVolume: 100, AvgCost: 10, MarkPrice: 10},
		},
	}
	td := "2026-08-15"
	b := intelligence.Build(intelligence.Options{
		Snapshot:  snap,
		TradeDate: td,
		BuyLotsByCode: map[string][]positionstate.LotRecord{
			"sz000001": {{TradeDate: td, Quantity: 100, Side: "BUY"}},
		},
	})
	require.Equal(t, intelligence.StatusNewPosition, b.Positions[0].PositionStatus)
	require.True(t, b.Positions[0].IsNewPosition)
	require.Equal(t, positionstate.S1NewLocked, b.Positions[0].PositionState)
}

func TestBuild_AvailableZero_NotNewWithoutTodayFirstBuy(t *testing.T) {
	snap := &portfolio.Snapshot{
		Found: true,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 100, AvailableVolume: 0, LockedVolume: 100, AvgCost: 10, MarkPrice: 10},
		},
	}
	b := intelligence.Build(intelligence.Options{
		Snapshot:  snap,
		TradeDate: "2026-08-15",
		BuyLotsByCode: map[string][]positionstate.LotRecord{
			"sz000001": {{TradeDate: "2026-08-10", Quantity: 100, Side: "BUY"}},
		},
	})
	require.NotEqual(t, intelligence.StatusNewPosition, b.Positions[0].PositionStatus)
	require.False(t, b.Positions[0].IsNewPosition)
}
