package shadow

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"

	"go-stock/backend/models"
)

// CompareSlices Dual-run 语义切片（与 frontend quantDecisionCompare.js 对齐）。
var CompareSlices = []string{"signal", "entryZone", "gate", "risk", "size", "action"}

// DefaultIgnorePaths 忽略 id / producer / 时间字段。
var DefaultIgnorePaths = []string{
	"id",
	"asOf",
	"tradeDate",
	"meta.producer",
	"meta.asOf",
	"meta.generatedAt",
	"meta.createdAt",
	"meta.updatedAt",
	"_legacyHint",
	// JS assemble 扩展字段（Schema v1 Action 核字段不含）
	"action.type",
	"action.lines",
	"action.tooltip",
	"action.decisionId",
	"action.actionSource",
	"entryZone.instantText",
	"entryZone.extended",
	"entryZone.tag",
	"entryZone.rangeHigh",
}

const floatEps = 1e-9

// CompareDecisions 字段级对比（left=js_legacy baseline，right=go_engine shadow）。
func CompareDecisions(left, right *models.QuantDecision) Report {
	leftP := models.QuantProducerJSLegacy
	rightP := models.QuantProducerGoEngine
	if left != nil && left.Meta.Producer != "" {
		leftP = left.Meta.Producer
	}
	if right != nil && right.Meta.Producer != "" {
		rightP = right.Meta.Producer
	}

	lm := decisionToMap(left)
	rm := decisionToMap(right)
	bySlice := map[string]SliceR{}
	var all []Diff

	for _, slice := range CompareSlices {
		ld := lm[slice]
		rd := rm[slice]
		diffs := diffValues(ld, rd, slice, nil)
		filtered := make([]Diff, 0, len(diffs))
		for _, d := range diffs {
			if shouldIgnore(d.Path) {
				continue
			}
			d.Slice = slice
			filtered = append(filtered, d)
		}
		bySlice[slice] = SliceR{Equal: len(filtered) == 0, Diffs: filtered}
		all = append(all, filtered...)
	}

	equal := len(all) == 0
	summary := fmt.Sprintf("OK %s ≡ %s", leftP, rightP)
	if !equal {
		summary = fmt.Sprintf("DIFF %s vs %s: %d field(s)", leftP, rightP, len(all))
	}
	var bid, cid string
	if left != nil {
		bid = left.ID
	}
	if right != nil {
		cid = right.ID
	}
	return Report{
		Equal:         equal,
		Summary:       summary,
		LeftProducer:  leftP,
		RightProducer: rightP,
		Diffs:         all,
		BySlice:       bySlice,
		Harness:       "quant-decision-shadow-dual-run",
		Phase:         "Phase2-A",
		BaselineID:    bid,
		CandidateID:   cid,
	}
}

func shouldIgnore(path string) bool {
	for _, p := range DefaultIgnorePaths {
		if path == p || strings.HasPrefix(path, p+".") {
			return true
		}
	}
	return false
}

func decisionToMap(d *models.QuantDecision) map[string]any {
	if d == nil {
		return map[string]any{}
	}
	b, err := json.Marshal(d)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	for _, s := range CompareSlices {
		if v, ok := m[s]; ok {
			out[s] = v
		} else {
			out[s] = nil
		}
	}
	return out
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
				diffs = append(diffs, Diff{Path: p, Left: nil, Right: rv})
				continue
			}
			if !rok {
				diffs = append(diffs, Diff{Path: p, Left: lv, Right: nil})
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
				diffs = append(diffs, Diff{Path: p, Left: nil, Right: rArr[i]})
				continue
			}
			if i >= len(rArr) {
				diffs = append(diffs, Diff{Path: p, Left: lArr[i], Right: nil})
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
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Map {
		out := map[string]any{}
		for _, k := range rv.MapKeys() {
			out[fmt.Sprint(k.Interface())] = rv.MapIndex(k).Interface()
		}
		return out, true
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

func approxEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	fa, aOK := toFloat(a)
	fb, bOK := toFloat(b)
	if aOK && bOK {
		if math.IsNaN(fa) && math.IsNaN(fb) {
			return true
		}
		return math.Abs(fa-fb) <= floatEps
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
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// MarshalDecisionJSON 输出 QuantDecision JSON（shadow 可观测产物）。
func MarshalDecisionJSON(d *models.QuantDecision) ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}

// MarshalReportJSON 输出 shadow diff report JSON。
func MarshalReportJSON(r Report) ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// DecisionFromJSON 解析 Decision（用于加载 js_legacy fixture）。
func DecisionFromJSON(b []byte) (*models.QuantDecision, error) {
	var d models.QuantDecision
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	return &d, nil
}
