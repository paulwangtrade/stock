package semantic

import (
	"encoding/json"

	"go-stock/backend/models"
)

// NormalizeDecision 语义归一化（Action 仅 code/allowDraft/side）。
func NormalizeDecision(d *models.QuantDecision) *SemanticDecision {
	if d == nil {
		return nil
	}
	out := &SemanticDecision{
		Action: SemanticAction{
			Code:       d.Action.Code,
			AllowDraft: d.Action.AllowDraft,
			Side:       d.Action.Side,
		},
		Signal:    map[string]any{},
		EntryZone: nil,
		Gate:      map[string]any{},
		Risk:      map[string]any{},
		Size:      map[string]any{},
	}
	if out.Action.Side == "" {
		out.Action.Side = "none"
	}

	out.Signal = scrubMap(map[string]any{
		"tag":       d.Signal.Tag,
		"tagKind":   d.Signal.TagKind,
		"daysAgo":   d.Signal.DaysAgo,
		"score":     d.Signal.Score,
		"ratioPct":  d.Signal.RatioPct,
		"sourceTag": emptyToNil(d.Signal.SourceTag),
		"barIndex":  d.Signal.BarIndex,
		"dayKey":    emptyToNil(d.Signal.DayKey),
	})

	if d.EntryZone != nil {
		out.EntryZone = scrubMap(map[string]any{
			"low":          d.EntryZone.Low,
			"high":         d.EntryZone.High,
			"instantPrice": d.EntryZone.InstantPrice,
			"mode":         d.EntryZone.Mode,
			"deferMode":    d.EntryZone.DeferMode,
			"daysAgo":      d.EntryZone.DaysAgo,
			"text":         emptyToNil(d.EntryZone.Text),
		})
	}

	items := make([]any, 0, len(d.Gate.Items))
	for _, it := range d.Gate.Items {
		items = append(items, map[string]any{
			"id":       it.ID,
			"passed":   it.Passed,
			"required": it.Required,
		})
	}
	out.Gate = scrubMap(map[string]any{
		"score":          d.Gate.Score,
		"ready":          d.Gate.Ready,
		"requiredPassed": d.Gate.RequiredPassed,
		"readyThreshold": d.Gate.ReadyThreshold,
		"items":          items,
	})

	out.Risk = map[string]any{
		"passed": d.Risk.Passed,
		"code":   d.Risk.Code,
	}

	out.Size = scrubMap(map[string]any{
		"ok":                d.Size.OK,
		"entryPrice":        d.Size.EntryPrice,
		"stopPrice":         d.Size.StopPrice,
		"riskPerShare":      d.Size.RiskPerShare,
		"confidence":        d.Size.Confidence,
		"targetShares":      d.Size.TargetShares,
		"addShares":         d.Size.AddShares,
		"targetAmount":      d.Size.TargetAmount,
		"positionPct":       d.Size.PositionPct,
		"bindingConstraint": emptyToNil(d.Size.BindingConstraint),
	})

	return out
}

func emptyToNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func scrubMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := map[string]any{}
	for k, v := range m {
		if v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			if t == "" {
				continue
			}
			out[k] = t
		case float64:
			if t == 0 {
				continue
			}
			out[k] = t
		case int:
			if t == 0 {
				continue
			}
			out[k] = t
		case int64:
			if t == 0 {
				continue
			}
			out[k] = t
		case bool:
			out[k] = t
		case []any:
			if len(t) == 0 {
				continue
			}
			out[k] = t
		default:
			out[k] = v
		}
	}
	return out
}

// semanticToMap for diffing.
func semanticToMap(s *SemanticDecision) map[string]any {
	if s == nil {
		return map[string]any{}
	}
	b, err := json.Marshal(s)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}
