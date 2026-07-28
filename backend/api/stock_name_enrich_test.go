package api

import (
	"testing"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestEnrichObservationPositionNames_KeepsExistingName(t *testing.T) {
	view := &papertrading.DashboardPositions{
		Positions: []papertrading.DashboardPositionRow{{
			StockCode: "sz000001",
			StockName: "平安银行",
		}},
	}
	enrichObservationPositionNames(view, func(codes []string) map[string]string {
		return map[string]string{"sz000001": "不应覆盖"}
	})
	require.Equal(t, "平安银行", view.Positions[0].StockName)
}

func TestEnrichObservationPositionNames_FillsEmptyName(t *testing.T) {
	view := &papertrading.DashboardPositions{
		Positions: []papertrading.DashboardPositionRow{{
			StockCode: "sz000001",
			StockName: "",
		}},
	}
	enrichObservationPositionNames(view, func(codes []string) map[string]string {
		require.Equal(t, []string{"sz000001"}, codes)
		return map[string]string{"sz000001": "平安银行"}
	})
	require.Equal(t, "平安银行", view.Positions[0].StockName)
}

func TestEnrichObservationPositionNames_LookupEmptyKeepsBlank(t *testing.T) {
	view := &papertrading.DashboardPositions{
		Positions: []papertrading.DashboardPositionRow{{
			StockCode: "sz000001",
			StockName: "",
		}},
	}
	enrichObservationPositionNames(view, func(codes []string) map[string]string {
		return map[string]string{}
	})
	require.Equal(t, "", view.Positions[0].StockName)
}

func TestEnrichObservationPositionNames_NilViewNoPanic(t *testing.T) {
	require.NotPanics(t, func() {
		enrichObservationPositionNames(nil, nil)
	})
}
