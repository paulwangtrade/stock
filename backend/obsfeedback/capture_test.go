package obsfeedback

import (
	"encoding/json"
	"strings"
	"testing"

	"go-stock/backend/holdingdecision"
	"go-stock/backend/portfolioobs"

	"github.com/stretchr/testify/require"
)

func TestCaptureFromObservation_Record(t *testing.T) {
	cost, px, ret, w, hs := 10.0, 11.0, 0.10, 0.12, 82.0
	obs := &portfolioobs.Observation{
		AsOf:            "2026-08-01T15:00:00+08:00",
		ObservationTime: "2026-08-01T15:05:00+08:00",
		Positions: []portfolioobs.PositionRow{{
			Symbol:         "sz000001",
			DecisionState:  holdingdecision.StateHoldNormal,
			DecisionReason: holdingdecision.ReasonNone,
			HoldingDays:    3,
			Cost:           &cost,
			CurrentPrice:   &px,
			Return:         &ret,
			CurrentWeight:  w,
			HealthScore:    &hs,
			Action:         "none",
		}},
	}

	recs := CaptureFromObservation(obs)
	require.Len(t, recs, 1)
	r := recs[0]
	require.Equal(t, "obs-2026-08-01-sz000001", r.ObservationID)
	require.Equal(t, "sz000001", r.Symbol)
	require.Equal(t, "SZ", r.Market)
	require.Equal(t, "2026-08-01", r.ObservationDate)
	require.Equal(t, holdingdecision.StateHoldNormal, r.DecisionState)
	require.Equal(t, holdingdecision.ReasonNone, r.DecisionReason)
	require.Equal(t, 3, r.HoldingDays)
	require.InDelta(t, 10.0, *r.CostPrice, 1e-9)
	require.InDelta(t, 11.0, *r.MarketPrice, 1e-9)
	require.InDelta(t, 0.10, *r.UnrealizedReturn, 1e-9)
	require.InDelta(t, 0.12, *r.PortfolioWeight, 1e-9)
	require.InDelta(t, 82.0, *r.HealthScore, 1e-9)
	require.Equal(t, SourcePortfolioObservation, r.Source)

	raw, err := json.Marshal(r)
	require.NoError(t, err)
	up := strings.ToUpper(string(raw))
	require.NotContains(t, up, `"BUY"`)
	require.NotContains(t, up, `"SELL"`)
	require.NotContains(t, up, "SIGNAL")
}

func TestCaptureFromObservation_Empty(t *testing.T) {
	require.Empty(t, CaptureFromObservation(nil))
	require.Empty(t, CaptureFromObservation(&portfolioobs.Observation{}))
}
