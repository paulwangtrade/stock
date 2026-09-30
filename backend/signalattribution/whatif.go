package signalattribution

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const whatIfSeparation = 1e-9

// WhatIfInput chooses which as-of feature rules form the subset.
// Explicit false uses the highlighted cohort contrasts.
// Explicit true uses Keys as-is, including an empty list.
type WhatIfInput struct {
	Explicit bool
	Keys     []string
}

type whatIfNumeric struct {
	key      string
	label    string
	decimals int
	pct      bool
	pick     func(AsOfFeatures) *float64
}

var whatIfNumerics = []whatIfNumeric{
	{"volumeRatio", "量比", 2, false, func(f AsOfFeatures) *float64 { return f.VolumeRatio }},
	{"prior20", "前20日涨跌", 2, true, func(f AsOfFeatures) *float64 { return f.Prior20Return }},
	{"distMa20", "距MA20", 2, true, func(f AsOfFeatures) *float64 { return f.DistMA20 }},
	{"rsi", "RSI", 1, false, func(f AsOfFeatures) *float64 { return f.RSI }},
}

type compiledWhatIf struct {
	key    string
	score  float64
	filter WhatIfFilter
	match  func(HitRow) bool
}

func whatIfPanelShell() WhatIfPanel {
	return WhatIfPanel{
		Note:         WhatIfNote,
		InSampleNote: WhatIfInSampleNote,
		WeightNote:   WhatIfWeightNote,
		Filters:      []WhatIfFilter{},
	}
}

func armFrom(label string, rows []HitRow) WhatIfArm {
	sum := summarize(rows)
	return WhatIfArm{
		Label:       label,
		HitCount:    sum.HitCount,
		CompleteAll: sum.CompleteAll,
		Horizons:    sum.Horizons,
		ToDate:      sum.ToDate,
	}
}

func finiteFeature(v *float64) (float64, bool) {
	if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return 0, false
	}
	return *v, true
}

func shownNumber(v float64, decimals int) float64 {
	if decimals < 0 {
		return v
	}
	p := math.Pow(10, float64(decimals))
	return math.Round(v*p) / p
}

func compileNumericRule(f whatIfNumeric, upRows, downRows []HitRow) (compiledWhatIf, bool) {
	up, upOK := featureMedian(upRows, f.pick)
	down, downOK := featureMedian(downRows, f.pick)
	if !upOK || !downOK {
		return compiledWhatIf{}, false
	}
	score := relScore(up, down)
	if score <= whatIfSeparation {
		return compiledWhatIf{}, false
	}
	highSide := up >= down
	op := "≥"
	if !highSide {
		op = "≤"
	}
	var shown float64
	var text string
	var pass func(float64) bool
	if f.pct {
		shown = shownNumber(((up+down)/2)*100, f.decimals)
		text = fmt.Sprintf("%s %s %s", f.label, op, formatShownPct(shown, f.decimals))
		pass = func(v float64) bool {
			got := v * 100
			if highSide {
				return got >= shown
			}
			return got <= shown
		}
	} else {
		shown = shownNumber((up+down)/2, f.decimals)
		text = fmt.Sprintf("%s %s %s", f.label, op, strconv.FormatFloat(shown, 'f', f.decimals, 64))
		pass = func(v float64) bool {
			if highSide {
				return v >= shown
			}
			return v <= shown
		}
	}
	pick := f.pick
	return compiledWhatIf{
		key:   f.key,
		score: score,
		filter: WhatIfFilter{
			Key:   f.key,
			Label: f.label,
			Rule:  text,
		},
		match: func(row HitRow) bool {
			v, ok := finiteFeature(pick(row.Features))
			if !ok {
				return false
			}
			return pass(v)
		},
	}, true
}

func formatShownPct(shown float64, decimals int) string {
	spec := "%." + strconv.Itoa(decimals) + "f%%"
	if shown < 0 {
		return fmt.Sprintf(spec, shown)
	}
	return fmt.Sprintf("+"+spec, shown)
}

func compileIndustryRule(upRows, downRows []HitRow) (compiledWhatIf, bool) {
	gap, ok := topIndustryGap(upRows, downRows)
	if !ok || gap.score <= whatIfSeparation {
		return compiledWhatIf{}, false
	}
	name := gap.name
	keep := gap.upShare >= gap.downShare
	rule := "行业不是「" + name + "」"
	if keep {
		rule = "行业为「" + name + "」"
	}
	return compiledWhatIf{
		key:   "industry",
		score: gap.score,
		filter: WhatIfFilter{
			Key:   "industry",
			Label: "行业「" + name + "」占比",
			Rule:  rule,
		},
		match: func(row HitRow) bool {
			in := industryLabel(row.Features.Industry) == name
			if keep {
				return in
			}
			return !in
		},
	}, true
}

func compileWhatIfRules(upRows, downRows []HitRow) []compiledWhatIf {
	out := make([]compiledWhatIf, 0, len(whatIfNumerics)+1)
	for _, f := range whatIfNumerics {
		if rule, ok := compileNumericRule(f, upRows, downRows); ok {
			out = append(out, rule)
		}
	}
	if rule, ok := compileIndustryRule(upRows, downRows); ok {
		out = append(out, rule)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].key < out[j].key
	})
	return out
}

func whatIfMatch(row HitRow, rules []compiledWhatIf) bool {
	saw := false
	for _, rule := range rules {
		if !rule.filter.Enabled {
			continue
		}
		saw = true
		if !rule.match(row) {
			return false
		}
	}
	return saw
}

// BuildWhatIf compares equal-weight returns of every filtered hit with the
// subset that passes the selected as-of feature rules. The rules come from the
// same +1 up/down contrast as the cohort panel. A row is kept or dropped by
// those features only; its own forward return is not a membership test.
func BuildWhatIf(rows []HitRow, cohort CohortPanel, in WhatIfInput) WhatIfPanel {
	panel := whatIfPanelShell()
	up, down, _, _ := splitByPlusOne(rows)
	if len(up) == 0 || len(down) == 0 {
		panel.Message = WhatIfNoCohort
		return panel
	}
	rules := compileWhatIfRules(up, down)
	if len(rules) == 0 {
		panel.Message = WhatIfNoSeparation
		return panel
	}
	highlight := map[string]bool{}
	for _, c := range cohort.Contrasts {
		if c.Highlight {
			highlight[c.Key] = true
		}
	}
	requested := map[string]bool{}
	if in.Explicit {
		for _, key := range in.Keys {
			key = strings.TrimSpace(key)
			if key != "" {
				requested[key] = true
			}
		}
	}
	anyEnabled := false
	for i := range rules {
		on := false
		if in.Explicit {
			on = requested[rules[i].key]
		} else if !cohort.LargeSample && highlight[rules[i].key] {
			on = true
		}
		rules[i].filter.Enabled = on
		if on {
			anyEnabled = true
		}
		panel.Filters = append(panel.Filters, rules[i].filter)
	}
	if !anyEnabled {
		if cohort.LargeSample && !in.Explicit {
			panel.Message = WhatIfLargeDefault
		} else {
			panel.Message = WhatIfNoneSelected
		}
		return panel
	}
	matched := make([]HitRow, 0)
	for _, row := range rows {
		if whatIfMatch(row, rules) {
			matched = append(matched, row)
		}
	}
	panel.OK = true
	panel.Baseline = armFrom(WhatIfBaselineLabel, rows)
	panel.Scenario = armFrom(WhatIfScenarioLabel, matched)
	warns := make([]string, 0, 3)
	if len(matched) < SmallSampleLimit {
		warns = append(warns, WhatIfSmallSampleNote)
	}
	if len(matched) == 0 {
		warns = append(warns, WhatIfNoMatch)
	}
	if cohort.LargeSample {
		warns = append(warns, LargeSampleWarning+"。", CohortLargeBrowseNote)
	}
	panel.Warning = strings.Join(warns, "")
	return panel
}
