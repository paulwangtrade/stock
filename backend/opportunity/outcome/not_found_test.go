package outcome

import (
	"testing"

	"go-stock/backend/opportunity/projection"

	"github.com/stretchr/testify/require"
)

func TestIsStockNotFound(t *testing.T) {
	t.Parallel()
	noTradeEmpty := OutcomeProjection{OutcomeStatus: OutcomeStatusNoTrade}
	noTradeWithPool := OutcomeProjection{
		OutcomeStatus: OutcomeStatusNoTrade,
		Signal:        projection.SignalBlock{Present: true},
		Opportunity:   projection.OpportunityBlock{Present: true},
	}
	open := OutcomeProjection{
		OutcomeStatus: OutcomeStatusOpen,
		Entry:         EntryBlock{Present: true},
	}

	require.True(t, isStockNotFound(nil, true))
	require.True(t, isStockNotFound([]OutcomeProjection{}, true))
	require.True(t, isStockNotFound([]OutcomeProjection{noTradeEmpty}, true))
	require.False(t, isStockNotFound([]OutcomeProjection{noTradeWithPool}, true))
	require.False(t, isStockNotFound([]OutcomeProjection{open}, true))
}

func TestCollectOutcomeMissing(t *testing.T) {
	t.Parallel()
	missing := collectOutcomeMissing(OutcomeProjection{
		OutcomeStatus: OutcomeStatusClosed,
		Exit:          ExitBlock{Present: true},
		Entry:         EntryBlock{Present: true},
		Signal:        projection.SignalBlock{Present: true},
		Opportunity:   projection.OpportunityBlock{Present: true},
	})
	require.Empty(t, missing)

	missing = collectOutcomeMissing(OutcomeProjection{
		OutcomeStatus: OutcomeStatusNoTrade,
	})
	require.Equal(t, []string{"signal", "candidate_pool_item"}, missing)
}
