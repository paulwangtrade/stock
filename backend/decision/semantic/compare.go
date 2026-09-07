package semantic

import (
	"fmt"
	"math"
	"reflect"
	"sort"

	"go-stock/backend/models"
)

// CompareSemantic 语义相等比较：Action.Code 最高优先级；Label 仅 displayDiff。
func CompareSemantic(left, right *models.QuantDecision) Report {
	leftP := models.QuantProducerJSLegacy
	rightP := models.QuantProducerGoEngine
	if left != nil && left.Meta.Producer != "" {
		leftP = left.Meta.Producer
	}
	if right != nil && right.Meta.Producer != "" {
		rightP = right.Meta.Producer
	}

	nl := NormalizeDecision(left)
	nr := NormalizeDecision(right)
	leftCode := ""
	rightCode := ""
	if nl != nil {
		leftCode = nl.Action.Code
	}
	if nr != nil {
		rightCode = nr.Action.Code
	}
	actionCodeEqual := leftCode == rightCode

	lm := semanticToMap(nl)
	rm := semanticToMap(nr)
	bySlice := map[string]SliceR{}
	var semanticDiffs []Diff

	for _, slice := range NormalizeSlices {
		diffs := diffValues(lm[slice], rm[slice], slice, nil)
		converted := make([]Diff, 0, len(diffs))
		for _, d := range diffs {
			d.Slice = slice
			d.Kind = "semantic"
			if d.Path == "action.code" {
				d.Priority = "highest"
			}
			converted = append(converted, d)
		}
		bySlice[slice] = SliceR{Equal: len(converted) == 0, Diffs: converted}
		semanticDiffs = append(semanticDiffs, converted...)
	}

	if !actionCodeEqual {
		has := false
		for i := range semanticDiffs {
			if semanticDiffs[i].Path == "action.code" {
				semanticDiffs[i].Priority = "highest"
				has = true
			}
		}
		if !has {
			semanticDiffs = append([]Diff{{
				Path: "action.code", Slice: "action", Kind: "semantic", Priority: "highest",
				Left: leftCode, Right: rightCode,
			}}, semanticDiffs...)
			s := bySlice["action"]
			s.Equal = false
			s.Diffs = append([]Diff{{
				Path: "action.code", Slice: "action", Kind: "semantic", Priority: "highest",
				Left: leftCode, Right: rightCode,
			}}, s.Diffs...)
			bySlice["action"] = s
		}
	}

	displayDiffs := collectDisplayDiffs(left, right)
	semanticEqual := actionCodeEqual && len(semanticDiffs) == 0

	summary := fmt.Sprintf("SEMANTIC_OK %s ≡ %s", leftP, rightP)
	if semanticEqual && len(displayDiffs) > 0 {
		summary = fmt.Sprintf("SEMANTIC_OK_DISPLAY_DIFF %s ≈ %s (label)", leftP, rightP)
	} else if !actionCodeEqual {
		summary = fmt.Sprintf("SEMANTIC_FAIL action.code %s ≠ %s", emptyMark(leftCode), emptyMark(rightCode))
	} else if !semanticEqual {
		summary = fmt.Sprintf("SEMANTIC_FAIL %d field(s)", len(semanticDiffs))
	}

	return Report{
		SemanticEqual:   semanticEqual,
		ActionCodeEqual: actionCodeEqual,
		DisplayDiffs:    displayDiffs,
		SemanticDiffs:   semanticDiffs,
		BySlice:         bySlice,
		LeftActionCode:  leftCode,
		RightActionCode: rightCode,
		LeftProducer:    leftP,
		RightProducer:   rightP,
		Summary:         summary,
	}
}

func emptyMark(s string) string {
	if s == "" {
		return "∅"
	}
	return s
}

func collectDisplayDiffs(left, right *models.QuantDecision) []DisplayDiff {
	ll, rl := "", ""
	if left != nil {
		ll = left.Action.Label
	}
	if right != nil {
		rl = right.Action.Label
	}
	if ll == rl {
		return nil
	}
	return []DisplayDiff{{
		Path: "action.label", Kind: "display", Left: ll, Right: rl,
	}}
}

func diffValues(left, right any, path string, diffs []Diff) []Diff {
	if approxEqual(left, right) {
		return diffs
	}
	lMap, lOK := asMap(left)
	rMap, rOK := asMap(right)
	if lOK && rOK {
		keys := map[string]struct{}{}
		for k := range lMap {
			keys[k] = struct{}{}
		}
		for k := range rMap {
			keys[k] = struct{}{}
		}
		ks := make([]string, 0, len(keys))
		for k := range keys {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			p := path + "." + k
			lv, lok := lMap[k]
			rv, rok := rMap[k]
			if !lok {
				if !isLooseEmpty(rv) {
					diffs = append(diffs, Diff{Path: p, Left: nil, Right: rv})
				}
				continue
			}
			if !rok {
				if !isLooseEmpty(lv) {
					diffs = append(diffs, Diff{Path: p, Left: lv, Right: nil})
				}
				continue
			}
			diffs = diffValues(lv, rv, p, diffs)
		}
		return diffs
	}
	lArr, lAOK := asSlice(left)
	rArr, rAOK := asSlice(right)
	if lAOK && rAOK {
		n := len(lArr)
		if len(rArr) > n {
			n = len(rArr)
		}
		for i := 0; i < n; i++ {
			p := fmt.Sprintf("%s[%d]", path, i)
			if i >= len(lArr) {
				if !isLooseEmpty(rArr[i]) {
					diffs = append(diffs, Diff{Path: p, Left: nil, Right: rArr[i]})
				}
				continue
			}
			if i >= len(rArr) {
				if !isLooseEmpty(lArr[i]) {
					diffs = append(diffs, Diff{Path: p, Left: lArr[i], Right: nil})
				}
				continue
			}
			diffs = diffValues(lArr[i], rArr[i], p, diffs)
		}
		return diffs
	}
	diffs = append(diffs, Diff{Path: path, Left: left, Right: right})
	return diffs
}

func asMap(v any) (map[string]any, bool) {
	if v == nil {
		return nil, false
	}
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	return nil, false
}

func asSlice(v any) ([]any, bool) {
	if v == nil {
		return nil, false
	}
	if s, ok := v.([]any); ok {
		return s, true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, false
	}
	out := make([]any, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		out[i] = rv.Index(i).Interface()
	}
	return out, true
}

func isLooseEmpty(v any) bool {
	if v == nil {
		return true
	}
	switch t := v.(type) {
	case string:
		return t == ""
	case bool:
		return !t
	case float64:
		return t == 0
	case int:
		return t == 0
	case int64:
		return t == 0
	default:
		return false
	}
}

func approxEqual(a, b any) bool {
	if isLooseEmpty(a) && isLooseEmpty(b) {
		return true
	}
	fa, aOK := toFloat(a)
	fb, bOK := toFloat(b)
	if aOK && bOK {
		return math.Abs(fa-fb) <= 1e-9
	}
	return reflect.DeepEqual(a, b)
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// HasHighestPriorityCodeDiff helper.
func HasHighestPriorityCodeDiff(r Report) bool {
	for _, d := range r.SemanticDiffs {
		if d.Path == "action.code" && d.Priority == "highest" {
			return true
		}
	}
	return false
}
