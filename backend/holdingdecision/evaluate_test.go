package holdingdecision

import (
	"encoding/json"
	"strings"
	"testing"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func ptr(f float64) *float64 { return &f }

func stock(code string, ret *float64, risk, profit, period string, price *float64, lots []papertrading.HoldingEvalLotRow) papertrading.HoldingEvalStockRow {
	return papertrading.HoldingEvalStockRow{
		StockCode:          code,
		CurrentPrice:       price,
		UnrealizedReturn:   ret,
		RiskState:          risk,
		ProfitState:        profit,
		HoldingPeriodState: period,
		Lots:               lots,
	}
}

func assertNoSell(t *testing.T, v *View) {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	s := strings.ToUpper(string(raw))
	require.NotContains(t, s, "EXIT_NOW")
	require.NotContains(t, s, "FORCE_CLOSE")
	require.NotContains(t, s, "SELL_APPROVED")
	for _, h := range v.Holdings {
		require.Equal(t, actionNone, h.Action)
		require.NotEqual(t, "SELL", h.State)
		for _, lot := range h.Lots {
			require.Equal(t, actionNone, lot.Action)
		}
	}
}

func TestEvaluate_ProfitNormal(t *testing.T) {
	r := 0.08
	px := 12.0
	v := EvaluateObservation(&papertrading.HoldingEvalObservationView{
		Holdings: []papertrading.HoldingEvalStockRow{
			stock("sz000001", &r, papertrading.RiskStateNormal, papertrading.ProfitStateProfit, papertrading.HoldingPeriodMid, &px, []papertrading.HoldingEvalLotRow{{
				FillID: 1, PlanID: 9, CurrentPrice: &px, ReturnRate: &r, ProfitState: papertrading.ProfitStateProfit,
			}}),
		},
	}, DefaultPolicy())
	require.Equal(t, StateHoldNormal, v.Holdings[0].State)
	require.Equal(t, ReasonNone, v.Holdings[0].Reason)
	require.Equal(t, 1, v.ByState[StateHoldNormal])
	assertNoSell(t, v)
}

func TestEvaluate_MildRiskWatch(t *testing.T) {
	r := -0.06
	px := 9.4
	v := EvaluateObservation(&papertrading.HoldingEvalObservationView{
		Holdings: []papertrading.HoldingEvalStockRow{
			stock("sh600000", &r, papertrading.RiskStateWatch, papertrading.ProfitStateLoss, papertrading.HoldingPeriodShort, &px, []papertrading.HoldingEvalLotRow{{
				FillID: 2, CurrentPrice: &px, ReturnRate: &r,
			}}),
		},
	}, DefaultPolicy())
	require.Equal(t, StateHoldWatch, v.Holdings[0].State)
	require.Equal(t, ReasonRiskIncrease, v.Holdings[0].Reason)
	require.Equal(t, HintKeepWatching, v.Holdings[0].NextHint)
	assertNoSell(t, v)
}

func TestEvaluate_MaterialRiskReview(t *testing.T) {
	r := -0.12
	px := 8.8
	v := EvaluateObservation(&papertrading.HoldingEvalObservationView{
		Holdings: []papertrading.HoldingEvalStockRow{
			stock("sz000002", &r, papertrading.RiskStateDanger, papertrading.ProfitStateLoss, papertrading.HoldingPeriodMid, &px, []papertrading.HoldingEvalLotRow{{
				FillID: 3, CurrentPrice: &px, ReturnRate: &r,
			}}),
		},
	}, DefaultPolicy())
	require.Equal(t, StateHoldReview, v.Holdings[0].State)
	require.Equal(t, ReasonRiskMaterial, v.Holdings[0].Reason)
	require.Equal(t, HintReassessThesis, v.Holdings[0].NextHint)
	require.Equal(t, 0, v.ByState[StateExitCandidate], "EXIT_CANDIDATE default off")
	assertNoSell(t, v)
}

func TestEvaluate_MissingDataDoesNotElevate(t *testing.T) {
	v := EvaluateObservation(&papertrading.HoldingEvalObservationView{
		Holdings: []papertrading.HoldingEvalStockRow{
			stock("sz000003", nil, papertrading.RiskStateDanger, papertrading.ProfitStateLoss, papertrading.HoldingPeriodLong, nil, []papertrading.HoldingEvalLotRow{{
				FillID: 4, CurrentPrice: nil, ReturnRate: nil, ProfitState: papertrading.ProfitStateUnknown,
			}}),
		},
	}, DefaultPolicy())
	require.Equal(t, StateHoldNormal, v.Holdings[0].State)
	require.Equal(t, ReasonDataMissing, v.Holdings[0].Reason)
	require.Equal(t, StateHoldNormal, v.Holdings[0].Lots[0].State)
	assertNoSell(t, v)
}

func TestEvaluate_ProfitWeaknessWatch(t *testing.T) {
	r := -0.02
	px := 9.8
	v := EvaluateObservation(&papertrading.HoldingEvalObservationView{
		Holdings: []papertrading.HoldingEvalStockRow{
			stock("sz000004", &r, papertrading.RiskStateNormal, papertrading.ProfitStateLoss, papertrading.HoldingPeriodShort, &px, []papertrading.HoldingEvalLotRow{{
				FillID: 5, CurrentPrice: &px, ReturnRate: &r, ProfitState: papertrading.ProfitStateLoss,
			}}),
		},
	}, DefaultPolicy())
	require.Equal(t, StateHoldWatch, v.Holdings[0].State)
	require.Equal(t, ReasonProfitWeakness, v.Holdings[0].Reason)
	assertNoSell(t, v)
}

func TestEvaluate_ExitCandidateOnlyWhenEnabled(t *testing.T) {
	r := -0.12
	px := 8.0
	row := stock("sz000005", &r, papertrading.RiskStateDanger, papertrading.ProfitStateLoss, papertrading.HoldingPeriodLong, &px, []papertrading.HoldingEvalLotRow{{
		FillID: 6, CurrentPrice: &px, ReturnRate: &r, HoldingPeriodState: papertrading.HoldingPeriodLong,
	}})
	off := EvaluateObservation(&papertrading.HoldingEvalObservationView{Holdings: []papertrading.HoldingEvalStockRow{row}}, DefaultPolicy())
	require.Equal(t, StateHoldReview, off.Holdings[0].State)

	on := EvaluateObservation(&papertrading.HoldingEvalObservationView{Holdings: []papertrading.HoldingEvalStockRow{row}}, Policy{ExitCandidateEnabled: true})
	require.Equal(t, StateExitCandidate, on.Holdings[0].State)
	require.Equal(t, HintConsiderExitEval, on.Holdings[0].NextHint)
	require.Equal(t, actionNone, on.Holdings[0].Action)
	assertNoSell(t, on)
}

func TestEvaluate_DoesNotMutateInput(t *testing.T) {
	r := 0.05
	px := 10.0
	eval := &papertrading.HoldingEvalObservationView{
		Holdings: []papertrading.HoldingEvalStockRow{
			stock("sz000001", &r, papertrading.RiskStateNormal, papertrading.ProfitStateProfit, "", &px, nil),
		},
	}
	_ = EvaluateObservation(eval, DefaultPolicy())
	require.Equal(t, papertrading.RiskStateNormal, eval.Holdings[0].RiskState)
	require.Nil(t, eval.Holdings[0].Lots)
}
