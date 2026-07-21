package strategy

import (
	"strings"

	"go-stock/backend/models"
	"go-stock/backend/tradingcalendar"
)

const candidatePoolSessionAfterClose = "after_close"

type candidatePoolBuildFunc func(string, ...BuildCandidatePoolOption) (*models.CandidatePool, error)

// AfterCloseCandidateBuilder builds the next trading day's CandidatePool from
// the source trading day close. It never creates a TradePlan.
type AfterCloseCandidateBuilder struct {
	Calendar tradingcalendar.Calendar
	build    candidatePoolBuildFunc
}

// NewAfterCloseCandidateBuilder creates a builder using the default calendar.
func NewAfterCloseCandidateBuilder() *AfterCloseCandidateBuilder {
	return &AfterCloseCandidateBuilder{Calendar: tradingcalendar.Default}
}

// Build resolves sourceDate T to the next trading day T+1 and delegates the
// complete candidate pipeline to BuildCandidatePool(T+1). Because the enhancer
// receives the pool TradeDate, its strict trade_date < T+1 lookup includes T's
// close snapshot.
func (b *AfterCloseCandidateBuilder) Build(sourceDate string) (*models.CandidatePool, error) {
	sourceDate = strings.TrimSpace(sourceDate)
	if sourceDate == "" {
		sourceDate = todayTradeDate()
	}

	tradeDate, err := b.Calendar.NextTradingDayString(sourceDate)
	if err != nil {
		return nil, err
	}

	build := b.build
	if build == nil {
		build = BuildCandidatePool
	}
	return build(tradeDate, WithCandidatePoolConfig(map[string]any{
		"session":     candidatePoolSessionAfterClose,
		"source_date": sourceDate,
	}))
}

// BuildAfterCloseCandidatePool is the default-calendar entry point.
func BuildAfterCloseCandidatePool(sourceDate string) (*models.CandidatePool, error) {
	return NewAfterCloseCandidateBuilder().Build(sourceDate)
}
