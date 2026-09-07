package shadow

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/models"
)

// BuildShadowDecision 由研究输入组装 go_engine QuantDecision（Schema v1）。
// 不写库、不触达 Plan/Execution。
func BuildShadowDecision(in Input) (*models.QuantDecision, error) {
	if strings.TrimSpace(in.Code) == "" {
		return nil, fmt.Errorf("shadow: code required")
	}
	purpose := in.Purpose
	if purpose == "" {
		purpose = models.QuantPurposeWatchlist
	}
	asOf := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	if strings.TrimSpace(in.AsOf) != "" {
		t, err := time.Parse(time.RFC3339, in.AsOf)
		if err != nil {
			return nil, fmt.Errorf("shadow: asOf: %w", err)
		}
		asOf = t
	}

	tag := in.Tag
	daysAgo := in.DaysAgo
	checklistReady := in.ChecklistReady
	if in.HasGate {
		checklistReady = checklistReady || in.Gate.Ready
	}

	action := deriveShadowAction(actionCtx{
		Tag:              tag,
		DaysAgo:          daysAgo,
		IsHistorical:     daysAgo > 0,
		ChecklistReady:   checklistReady,
		SellPositionPct:  in.SellPositionPct,
		AddPositionPct:   in.AddPositionPct,
		RushReducePct:    in.RushReducePct,
		SourceTag:        in.SourceTag,
		Zone:             in.EntryZone,
	})

	gate := models.QuantGate{ReadyThreshold: 0.85}
	if in.HasGate {
		gate = in.Gate
		if gate.ReadyThreshold == 0 {
			gate.ReadyThreshold = 0.85
		}
	}

	size := models.QuantSize{OK: false, Reason: ""}
	if in.HasSize {
		size = in.Size
	}

	existing := in.ExistingVolume
	allowDraft := computeAllowDraft(action, size, in.SellPositionPct, in.RushReducePct, existing)
	action.AllowDraft = allowDraft

	level, key, name, cap := resolveRegime(in.MarketLevel, in.ExposureCap)
	ratio := resolveRatio(in)

	d := &models.QuantDecision{
		AsOf:      asOf,
		TradeDate: in.TradeDate,
		Purpose:   purpose,
		Instrument: models.QuantInstrument{
			StockCode: in.Code,
			StockName: in.Name,
		},
		Regime: models.QuantRegime{
			Level:       level,
			Key:         key,
			Name:        name,
			Source:      "shadow_go",
			ExposureCap: cap,
		},
		Signal: models.QuantSignal{
			Tag:        tag,
			TagKind:    resolveTagKind(tag),
			DaysAgo:    daysAgo,
			Score:      in.SignalScore,
			RatioPct:   ratio,
			SourceTag:  in.SourceTag,
			Summary:    in.StatusText,
			BarIndex:   in.SignalBar,
			DayKey:     in.DayKey,
		},
		EntryZone: in.EntryZone,
		Gate:      gate,
		Risk: models.QuantRiskSlice{
			Passed:  true,
			Code:    "APPROVED",
			Message: "phase2-a shadow placeholder (PlanFilter not wired)",
		},
		Size:   size,
		Action: action,
		Meta: models.QuantDecisionMeta{
			SchemaVersion:  models.QuantDecisionSchemaVersion,
			Producer:       models.QuantProducerGoEngine,
			ExistingVolume: existing,
		},
	}
	d.ID = buildShadowID(d)
	return d, nil
}

func buildShadowID(d *models.QuantDecision) string {
	return fmt.Sprintf("qd%d:%s:%s:%s:%s:%s",
		models.QuantDecisionSchemaVersion,
		models.QuantProducerGoEngine,
		strings.ToLower(d.Instrument.StockCode),
		d.AsOf.UTC().Format(time.RFC3339),
		d.Action.Code,
		d.Action.Label,
	)
}

func resolveTagKind(tag string) string {
	switch tag {
	case "止", "减":
		return models.QuantTagKindExit
	case "冲":
		return models.QuantTagKindRush
	case "加":
		return models.QuantTagKindScaleIn
	case "冰":
		return models.QuantTagKindIce
	case "强", "趋", "转", "突", "买", "弹":
		return models.QuantTagKindEntry
	default:
		if tag == "" {
			return models.QuantTagKindNone
		}
		return models.QuantTagKindNone
	}
}

func resolveRatio(in Input) *float64 {
	if in.SellPositionPct != nil {
		return in.SellPositionPct
	}
	if in.AddPositionPct != nil {
		return in.AddPositionPct
	}
	if in.RushReducePct != nil {
		return in.RushReducePct
	}
	return nil
}

func resolveRegime(level int, exposureCap float64) (int, string, string, float64) {
	if level <= 0 {
		level = 3
	}
	if level > 5 {
		level = 5
	}
	names := map[int]string{
		1: "空仓防守",
		2: "防守观察",
		3: "中性观望",
		4: "积极参与",
		5: "进攻加仓",
	}
	caps := map[int]float64{1: 0.05, 2: 0.1, 3: 0.2, 4: 0.35, 5: 0.5}
	cap := exposureCap
	if cap <= 0 {
		cap = caps[level]
	}
	return level, fmt.Sprintf("level%d", level), names[level], cap
}

func computeAllowDraft(action models.QuantAction, size models.QuantSize, sellPct, rushPct *float64, existing int64) bool {
	if action.Code == "" {
		return false
	}
	if (action.Code == models.QuantActionEnter || action.Code == models.QuantActionScaleIn) &&
		size.OK && (size.AddShares > 0 || size.TargetShares > 0) {
		return true
	}
	var pct float64
	hasPct := false
	if sellPct != nil {
		pct = *sellPct
		hasPct = true
	} else if rushPct != nil {
		pct = *rushPct
		hasPct = true
	}
	if (action.Code == models.QuantActionReduce || action.Code == models.QuantActionExitPartial) &&
		hasPct && pct > 0 && existing > 0 {
		return true
	}
	return false
}
