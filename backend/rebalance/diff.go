package rebalance

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"go-stock/backend/holdingdecision"
)

const intentNone = "none"

// Diff computes Current vs Target gaps. Pure: no DB, Broker, Intent, or orders.
func Diff(current *CurrentView, target *TargetPortfolio, policy DiffPolicy) *View {
	if policy.WeightBand < 0 || math.IsNaN(policy.WeightBand) {
		policy.WeightBand = DefaultPolicy().WeightBand
	}
	out := &View{
		PriceBasis:     PriceBasisSnapshotMark,
		WeightBand:     policy.WeightBand,
		AllowSwitch:    policy.AllowSwitch,
		Items:          []Item{},
		DataSourceNote: dataSourceNote,
	}
	if current != nil {
		if !current.AsOf.IsZero() {
			out.AsOf = current.AsOf.Format(time.RFC3339)
		}
		out.AccountID = current.AccountID
		if current.PriceBasis != "" {
			out.PriceBasis = current.PriceBasis
		}
	}
	equity := 0.0
	if target != nil {
		equity = target.EquityRef
	}
	if equity <= 0 && current != nil {
		equity = current.Equity
	}
	out.EquityRef = equity

	curMap := map[string]CurrentPosition{}
	if current != nil {
		for _, p := range current.Positions {
			code := strings.TrimSpace(p.Symbol)
			if code == "" {
				continue
			}
			curMap[code] = p
		}
	}
	tgtMap := map[string]TargetPosition{}
	if target != nil {
		for _, p := range target.Positions {
			code := strings.TrimSpace(p.Symbol)
			if code == "" {
				continue
			}
			tgtMap[code] = p
		}
	}

	symbols := map[string]struct{}{}
	for k := range curMap {
		symbols[k] = struct{}{}
	}
	for k := range tgtMap {
		symbols[k] = struct{}{}
	}
	list := make([]string, 0, len(symbols))
	for s := range symbols {
		list = append(list, s)
	}
	sort.Strings(list)

	pendingBuy := 0.0
	if current != nil {
		pendingBuy = current.PendingBuyNotional
	}

	for _, sym := range list {
		c, inC := curMap[sym]
		t, inT := tgtMap[sym]
		wCur, aCur := 0.0, 0.0
		if inC {
			wCur = c.Weight
			aCur = c.MarketValue
			if aCur == 0 && equity > 0 {
				aCur = wCur * equity
			}
		}
		wTgt, aTgt := 0.0, 0.0
		if inT {
			wTgt = t.TargetWeight
			aTgt = t.TargetAmount
			if aTgt == 0 && equity > 0 {
				aTgt = wTgt * equity
			}
		}
		dw := wTgt - wCur
		da := aTgt - aCur
		item := Item{
			Symbol:          sym,
			CurrentWeight:   wCur,
			TargetWeight:    wTgt,
			DeltaWeight:     dw,
			CurrentAmount:   aCur,
			TargetAmount:    aTgt,
			DeltaAmount:     da,
			AvailableVolume: c.AvailableVolume,
			LockedVolume:    c.LockedVolume,
			IntentHint:      intentNone,
			SwitchOK:        false,
			DecisionState:   c.DecisionState,
			ConstraintFlags: []string{},
		}
		if !inC {
			item.AvailableVolume = 0
			item.LockedVolume = 0
		}

		held := inC && (c.Volume > 0 || c.MarketValue > 0 || c.Weight > 0)
		wanted := inT && wTgt > 0

		switch {
		case !held && wanted:
			item.Action = ActionAdd
			item.Reason = ReasonNewTarget
			item.ExecutableQty = 0
		case held && !wanted:
			item.Action = ActionRemove
			item.Reason = ReasonNotInTarget
			item.ExecutableQty = c.AvailableVolume
		case held && wanted:
			if withinBand(dw, da, policy) {
				item.Action = ActionKeep
				item.Reason = ReasonWithinBand
				item.ExecutableQty = 0
			} else if dw > 0 {
				item.Action = ActionIncrease
				item.Reason = ReasonBelowTarget
				item.ExecutableQty = 0
			} else {
				item.Action = ActionDecrease
				item.Reason = ReasonAboveTarget
				item.ExecutableQty = c.AvailableVolume
			}
		default:
			item.Action = ActionKeep
			item.Reason = ReasonWithinBand
			item.ExecutableQty = 0
		}

		annotateConstraints(&item, pendingBuy)
		out.Items = append(out.Items, item)
	}

	annotateSwitches(out, policy)
	recount(out)
	return out
}

func withinBand(dw, da float64, policy DiffPolicy) bool {
	if math.Abs(dw) > policy.WeightBand {
		return false
	}
	if policy.MinDeltaAmount > 0 && math.Abs(da) > policy.MinDeltaAmount {
		return false
	}
	return true
}

func annotateConstraints(item *Item, pendingBuy float64) {
	if item.LockedVolume > 0 {
		item.ConstraintFlags = append(item.ConstraintFlags, ConstraintT1Locked)
	}
	if item.Action == ActionRemove || item.Action == ActionDecrease {
		if item.AvailableVolume <= 0 && (item.LockedVolume > 0 || item.CurrentAmount > 0) {
			item.ConstraintFlags = append(item.ConstraintFlags, ConstraintInsufficientAvail)
			item.ExecutableQty = 0
		} else if item.AvailableVolume < item.ExecutableQty {
			item.ConstraintFlags = append(item.ConstraintFlags, ConstraintInsufficientAvail)
			item.ExecutableQty = item.AvailableVolume
		}
	}
	if pendingBuy > 0 && (item.Action == ActionAdd || item.Action == ActionIncrease) {
		item.ConstraintFlags = append(item.ConstraintFlags, ConstraintPendingBuy)
	}
}

func annotateSwitches(out *View, policy DiffPolicy) {
	var removes, adds []int
	for i := range out.Items {
		switch out.Items[i].Action {
		case ActionRemove:
			removes = append(removes, i)
		case ActionAdd:
			adds = append(adds, i)
		}
	}
	if len(removes) == 0 || len(adds) == 0 {
		return
	}
	n := len(removes)
	if len(adds) < n {
		n = len(adds)
	}
	for i := 0; i < n; i++ {
		gid := fmt.Sprintf("switch-%d", i+1)
		ri, ai := removes[i], adds[i]
		out.Items[ri].SwitchGroupID = gid
		out.Items[ai].SwitchGroupID = gid
		out.Items[ri].SwitchOK = false
		out.Items[ai].SwitchOK = false
		reason := ReasonSwitchBlocked
		if !policy.AllowSwitch {
			reason = ReasonSwitchBlocked
		}
		dec := strings.ToUpper(strings.TrimSpace(out.Items[ri].DecisionState))
		if dec == "" || dec == holdingdecision.StateHoldNormal {
			reason = ReasonHoldNormalBlocks
		}
		// D.11: without cost model, never switch_ok.
		out.Items[ri].SwitchReason = reason
		out.Items[ai].SwitchReason = reason
		out.BlockedSwitchCount++
	}
}

func recount(out *View) {
	c := ActionCounts{}
	for _, it := range out.Items {
		c.Total++
		switch it.Action {
		case ActionAdd:
			c.Add++
		case ActionIncrease:
			c.Increase++
		case ActionDecrease:
			c.Decrease++
		case ActionRemove:
			c.Remove++
		default:
			c.Keep++
		}
	}
	out.Counts = c
}
