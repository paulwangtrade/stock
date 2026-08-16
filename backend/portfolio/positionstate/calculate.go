package positionstate

import "strings"

// CalculatePositionState is the unified calculator (alias Calculate).
func CalculatePositionState(in SnapshotInput) PositionStateView {
	return Calculate(in)
}

// Calculate derives PositionStateView. Never treats available_qty==0 alone as new position.
func Calculate(in SnapshotInput) PositionStateView {
	symbol := strings.TrimSpace(in.Symbol)
	tradeDate := strings.TrimSpace(in.TradeDate)
	current := strings.TrimSpace(in.CurrentDate)
	if current == "" {
		current = tradeDate
	}
	if tradeDate == "" {
		tradeDate = current
	}

	total := in.TotalQty
	if total < 0 {
		total = 0
	}
	avail := in.AvailableQty
	if avail < 0 {
		avail = 0
	}
	var locked int64
	if in.LockedQty != nil {
		locked = *in.LockedQty
		if locked < 0 {
			locked = 0
		}
	} else {
		locked = total - avail
		if locked < 0 {
			locked = 0
		}
	}
	if avail+locked != total && total > 0 {
		if in.LockedQty != nil {
			avail = total - locked
			if avail < 0 {
				avail = 0
			}
		} else {
			locked = total - avail
			if locked < 0 {
				locked = 0
			}
		}
	}

	firstBuy := earliestBuyDate(in.BuyRecords)
	holdingDays := computeHoldingDays(firstBuy, current)
	isNew := false
	if firstBuy != "" && current != "" {
		isNew = firstBuy == current
	}

	out := PositionStateView{
		Symbol:        symbol,
		TotalQty:      total,
		AvailableQty:  avail,
		LockedQty:     locked,
		HoldingDays:   holdingDays,
		IsNewPosition: isNew,
		FirstBuyDate:  firstBuy,
		CanSell:       avail > 0,
	}

	hasSell := hasPositiveLots(in.SellRecords)

	switch {
	case total <= 0:
		out.State = S0NoPosition
		out.CanSell = false
		out.IsNewPosition = false
		out.RiskTag = RiskTagNoPosition
		out.Explanation = "无持仓"
	case avail > 0 && locked > 0:
		out.State = S3PartialLocked
		out.RiskTag = RiskTagPartialLock
		out.Explanation = "部分可卖（老仓）+ 部分 T+1 锁定（今日增量）"
		out.IsNewPosition = false
	case locked > 0 && avail == 0:
		out.State = S1NewLocked
		if isNew {
			out.RiskTag = RiskTagNewLocked
			out.Explanation = "当日买入锁定（T+1）；is_new_position 以首买日为准"
		} else {
			out.IsNewPosition = false
			out.RiskTag = RiskTagStaleLock
			out.Explanation = "全量锁定但非当日首买 — 不作新建仓（修复 available=0 误判）"
		}
	case hasSell && avail > 0 && locked == 0:
		out.State = S4ReducedAvailable
		out.RiskTag = RiskTagReduced
		out.IsNewPosition = false
		out.Explanation = "部分卖出后剩余可卖持仓"
	default:
		out.State = S2Available
		out.IsNewPosition = false
		out.RiskTag = RiskTagNone
		out.Explanation = "正常可卖老仓"
	}

	return out
}
