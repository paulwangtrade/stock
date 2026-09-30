package signalattribution

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// Smoke (go test -v -run WhatIf):
// 基线是筛选后全部命中的等权；假设子集只用 as-of 特征，不按该股票自己的 +1 涨跌入选。
// +1 列是同一样本对照。缺特征、未勾选、无法分组、样本过大默认不套用，都失败关闭。

func featRow(code, industry string, vol, ret float64) HitRow {
	v := vol
	r := ret
	return HitRow{
		Code:     code,
		Features: AsOfFeatures{Industry: industry, VolumeRatio: &v},
		Horizons: []HorizonCell{{
			Horizon: 1, Status: StatusOK, ReturnRate: &r, Text: formatReturnPct(r),
		}},
	}
}

func enableOnly(rules []compiledWhatIf, key string) []compiledWhatIf {
	out := append([]compiledWhatIf(nil), rules...)
	for i := range out {
		out[i].filter.Enabled = out[i].key == key
	}
	return out
}

func statByHorizon(arm WhatIfArm, horizon int) HorizonStat {
	if horizon == 0 {
		return arm.ToDate
	}
	for _, stat := range arm.Horizons {
		if stat.Horizon == horizon {
			return stat
		}
	}
	return HorizonStat{}
}

func filterByKey(filters []WhatIfFilter, key string) (WhatIfFilter, bool) {
	for _, f := range filters {
		if f.Key == key {
			return f, true
		}
	}
	return WhatIfFilter{}, false
}

func TestWhatIf_FilterDoesNotReadProbeReturn(t *testing.T) {
	up := []HitRow{featRow("u", "电子", 3, 0.1), featRow("u2", "电子", 2, 0.2)}
	down := []HitRow{featRow("d", "银行", 0.4, -0.1), featRow("d2", "银行", 0.6, -0.2)}
	rules := enableOnly(compileWhatIfRules(up, down), "volumeRatio")
	if !hasEnabled(rules, "volumeRatio") {
		t.Fatalf("volume rule missing: %+v", rules)
	}

	probe := featRow("p", "电子", 2.2, -0.5)
	probe.Horizons = append(probe.Horizons, HorizonCell{
		Horizon: 3, Status: StatusOK, ReturnRate: floatPtr(9),
	})
	if !whatIfMatch(probe, rules) {
		t.Fatal("high volume must pass even when +1 is down")
	}
	*probe.Horizons[0].ReturnRate = 0.9
	if !whatIfMatch(probe, rules) {
		t.Fatal("flipping +1 must not change membership")
	}
	*probe.Features.VolumeRatio = 0.2
	*probe.Horizons[0].ReturnRate = 0.9
	if whatIfMatch(probe, rules) {
		t.Fatal("low volume must fail even when +1 is up")
	}
	probe.Features.VolumeRatio = nil
	if whatIfMatch(probe, rules) {
		t.Fatal("missing volume fails closed")
	}
}

func hasEnabled(rules []compiledWhatIf, key string) bool {
	for _, rule := range rules {
		if rule.key == key && rule.filter.Enabled {
			return true
		}
	}
	return false
}

func TestWhatIf_RuleTextMatchesPredicate(t *testing.T) {
	up := []HitRow{featRow("u", "电子", 2, 0.1), featRow("u2", "电子", 2, 0.1)}
	down := []HitRow{featRow("d", "银行", 1.35, -0.1), featRow("d2", "银行", 1.35, -0.1)}
	rules := compileWhatIfRules(up, down)
	var vol compiledWhatIf
	found := false
	for _, rule := range rules {
		if rule.key == "volumeRatio" {
			vol = rule
			found = true
		}
	}
	if !found {
		t.Fatal("expected a volume rule")
	}
	vol.filter.Enabled = true
	parts := strings.Fields(vol.filter.Rule)
	if len(parts) < 3 || parts[0] != "量比" {
		t.Fatalf("rule %q", vol.filter.Rule)
	}
	threshold, err := strconv.ParseFloat(parts[len(parts)-1], 64)
	if err != nil {
		t.Fatal(err)
	}
	active := []compiledWhatIf{vol}
	below := featRow("lo", "电子", threshold-0.01, 5)
	at := featRow("at", "电子", threshold, -5)
	if parts[1] == "≥" {
		if whatIfMatch(below, active) || !whatIfMatch(at, active) {
			t.Fatalf("≥ %s below=%v at=%v", vol.filter.Rule, whatIfMatch(below, active), whatIfMatch(at, active))
		}
	} else if parts[1] == "≤" {
		above := featRow("hi", "电子", threshold+0.01, 5)
		if whatIfMatch(above, active) || !whatIfMatch(at, active) {
			t.Fatalf("≤ %s", vol.filter.Rule)
		}
	} else {
		t.Fatalf("operator %q", vol.filter.Rule)
	}
	t.Logf("smoke rule: %s", vol.filter.Rule)
}

func whatIfBars(closes ...float64) []DayBar {
	days := []string{
		"2026-07-17", "2026-07-20", "2026-07-21", "2026-07-22", "2026-07-23", "2026-07-24",
		"2026-07-27", "2026-07-28", "2026-07-29", "2026-07-30", "2026-07-31",
	}
	out := make([]DayBar, 0, len(closes))
	for i, close := range closes {
		out = append(out, DayBar{Date: days[i], Close: close})
	}
	return out
}

func TestWhatIf_SmokeEqualWeight(t *testing.T) {
	meta := SnapshotMeta{TradeDate: "2026-07-17", StrategyID: "s1", StrategyName: "策略甲"}
	hits := []HitInput{
		{Code: "U_hi", BarKey: "U_hi", Industry: "电子", Tag: "强", HasVolumeRatio: true, VolumeRatio: 3},
		{Code: "U_lo", BarKey: "U_lo", Industry: "银行", Tag: "强", HasVolumeRatio: true, VolumeRatio: 0.4},
		{Code: "D_hi", BarKey: "D_hi", Industry: "电子", Tag: "强", HasVolumeRatio: true, VolumeRatio: 2.8},
		{Code: "D_lo", BarKey: "D_lo", Industry: "银行", Tag: "强", HasVolumeRatio: true, VolumeRatio: 0.5},
		{Code: "N", BarKey: "N", Industry: "电子", Tag: "强"},
	}
	bars := map[string][]DayBar{
		"U_hi": whatIfBars(100, 110, 120, 130),
		"U_lo": whatIfBars(100, 102),
		"D_hi": whatIfBars(100, 96),
		"D_lo": whatIfBars(100, 90),
		"N":    whatIfBars(100, 200),
	}
	view := Assemble(meta, hits, bars, weekendCal(), 1, 1)
	if view.Disclaimer != Disclaimer {
		t.Fatalf("disclaimer %s", view.Disclaimer)
	}
	panel := view.WhatIf
	if !panel.OK {
		t.Fatalf("what-if %+v", panel)
	}
	if !strings.Contains(panel.Note, "对照实验，不是买卖指令") || !strings.Contains(panel.Note, "不写入交易计划") {
		t.Fatalf("note %s", panel.Note)
	}
	if !strings.Contains(panel.InSampleNote, "同一样本") || !strings.Contains(panel.WeightNote, "等权") {
		t.Fatalf("notes %s / %s", panel.InSampleNote, panel.WeightNote)
	}
	if !strings.Contains(panel.Warning, "不足 8") {
		t.Fatalf("warning %s", panel.Warning)
	}
	if view.Total != 5 || len(view.Rows) != 1 {
		t.Fatalf("page must not shrink the experiment: total %d rows %d", view.Total, len(view.Rows))
	}
	if view.Cohort.Up.Count != 3 {
		t.Fatalf("up count %d", view.Cohort.Up.Count)
	}
	if panel.Baseline.Label != WhatIfBaselineLabel || panel.Baseline.HitCount != 5 {
		t.Fatalf("baseline %+v", panel.Baseline)
	}
	base1 := statByHorizon(panel.Baseline, 1)
	if base1.Complete != 5 || base1.Mean == nil {
		t.Fatalf("baseline +1 %+v", base1)
	}
	approx(t, *base1.Mean, 0.196)

	// Volume and industry are both on. Intersection is U_hi (+10%) and D_hi (-4%), not the +1-up set.
	if panel.Scenario.HitCount != 2 || panel.Scenario.Label != WhatIfScenarioLabel {
		t.Fatalf("scenario %+v", panel.Scenario)
	}
	sc1 := statByHorizon(panel.Scenario, 1)
	if sc1.Complete != 2 || sc1.Mean == nil || sc1.MeanText != "+3.00%" {
		t.Fatalf("scenario +1 %+v", sc1)
	}
	approx(t, *sc1.Mean, 0.03)
	sc3 := statByHorizon(panel.Scenario, 3)
	if sc3.Complete != 1 || sc3.Mean == nil {
		t.Fatalf("+3 must not zero-fill the missing name: %+v", sc3)
	}
	approx(t, *sc3.Mean, 0.30)
	sc10 := statByHorizon(panel.Scenario, 10)
	if sc10.Complete != 0 || sc10.Mean != nil || sc10.MeanText != EmptyStatText {
		t.Fatalf("+10 %+v", sc10)
	}
	scTo := statByHorizon(panel.Scenario, 0)
	if scTo.Complete != 2 || scTo.Mean == nil {
		t.Fatalf("toDate %+v", scTo)
	}
	approx(t, *scTo.Mean, 0.13)
	vol, ok := filterByKey(panel.Filters, "volumeRatio")
	if !ok || !vol.Enabled {
		t.Fatalf("default volume %+v", panel.Filters)
	}
	t.Logf("smoke baseline hits=%d +1=%s", panel.Baseline.HitCount, base1.MeanText)
	t.Logf("smoke scenario hits=%d +1=%s +3=%s +10=%s toDate=%s", panel.Scenario.HitCount, sc1.MeanText, sc3.MeanText, sc10.MeanText, scTo.MeanText)
	t.Logf("smoke note: %s", panel.Note)
	t.Logf("smoke warning: %s", panel.Warning)

	onlyVol := Assemble(meta, hits, bars, weekendCal(), 1, 50, AssembleOptions{WhatIfSet: true, WhatIfKeys: []string{"volumeRatio"}})
	if !onlyVol.WhatIf.OK || onlyVol.WhatIf.Scenario.HitCount != 2 {
		t.Fatalf("explicit volume %+v", onlyVol.WhatIf)
	}
	ind, _ := filterByKey(onlyVol.WhatIf.Filters, "industry")
	if ind.Enabled {
		t.Fatal("explicit volume must not keep industry on")
	}

	onlyInd := Assemble(meta, hits, bars, weekendCal(), 1, 50, AssembleOptions{WhatIfSet: true, WhatIfKeys: []string{"industry"}})
	if !onlyInd.WhatIf.OK || onlyInd.WhatIf.Scenario.HitCount != 3 {
		t.Fatalf("industry subset should keep the down name in 电子: %+v", onlyInd.WhatIf.Scenario)
	}
	ind1 := statByHorizon(onlyInd.WhatIf.Scenario, 1)
	if ind1.Mean == nil {
		t.Fatal("industry +1 missing")
	}
	approx(t, *ind1.Mean, 1.06/3)

	cleared := Assemble(meta, hits, bars, weekendCal(), 1, 50, AssembleOptions{WhatIfSet: true})
	if cleared.WhatIf.OK || cleared.WhatIf.Scenario.HitCount != 0 || !strings.Contains(cleared.WhatIf.Message, "未勾选") {
		t.Fatalf("cleared %+v", cleared.WhatIf)
	}
	unknown := Assemble(meta, hits, bars, weekendCal(), 1, 50, AssembleOptions{WhatIfSet: true, WhatIfKeys: []string{"nope"}})
	if unknown.WhatIf.OK || !strings.Contains(unknown.WhatIf.Message, "未勾选") {
		t.Fatalf("unknown key %+v", unknown.WhatIf)
	}
}

func TestWhatIf_FailClosedWithoutBothSides(t *testing.T) {
	meta := SnapshotMeta{TradeDate: "2026-07-17"}
	hits := []HitInput{
		{Code: "A", BarKey: "A", Industry: "电子", HasVolumeRatio: true, VolumeRatio: 2},
		{Code: "B", BarKey: "B", Industry: "银行", HasVolumeRatio: true, VolumeRatio: 1},
	}
	bars := map[string][]DayBar{
		"A": whatIfBars(100, 110),
		"B": whatIfBars(100, 120),
	}
	view := Assemble(meta, hits, bars, weekendCal(), 1, 50)
	if view.WhatIf.OK || view.WhatIf.Message != WhatIfNoCohort || len(view.WhatIf.Filters) != 0 {
		t.Fatalf("%+v", view.WhatIf)
	}
	if !strings.Contains(view.WhatIf.Note, "对照实验，不是买卖指令") {
		t.Fatalf("note %s", view.WhatIf.Note)
	}
}

func TestWhatIf_LargeSampleDoesNotAutoApply(t *testing.T) {
	meta := SnapshotMeta{TradeDate: "2026-07-17"}
	hits := make([]HitInput, 0, LargeSampleLimit+1)
	bars := map[string][]DayBar{}
	for i := 0; i < LargeSampleLimit+1; i++ {
		key := fmt.Sprintf("K%03d", i)
		industry := "电子"
		vol := 2.0
		next := 110.0
		if i%2 == 0 {
			industry = "银行"
			vol = 0.5
			next = 90
		}
		hits = append(hits, HitInput{
			Code: key, BarKey: key, Tag: "强", Industry: industry,
			HasVolumeRatio: true, VolumeRatio: vol,
		})
		bars[key] = whatIfBars(100, next)
	}
	view := Assemble(meta, hits, bars, weekendCal(), 1, 50)
	if view.WhatIf.OK || !strings.Contains(view.WhatIf.Message, "样本过大") {
		t.Fatalf("default %+v", view.WhatIf.Message)
	}
	if len(view.WhatIf.Filters) == 0 {
		t.Fatal("filters should stay available for a manual toggle")
	}
	for _, f := range view.WhatIf.Filters {
		if f.Enabled {
			t.Fatalf("auto enabled %+v", f)
		}
	}
	explicit := Assemble(meta, hits, bars, weekendCal(), 1, 50, AssembleOptions{
		WhatIfSet:  true,
		WhatIfKeys: []string{"volumeRatio"},
	})
	if !explicit.WhatIf.OK || explicit.WhatIf.Scenario.HitCount != 50 {
		t.Fatalf("explicit %+v hits %d", explicit.WhatIf.Message, explicit.WhatIf.Scenario.HitCount)
	}
	if !strings.Contains(explicit.WhatIf.Warning, "不是共性结论") {
		t.Fatalf("warning %s", explicit.WhatIf.Warning)
	}
}

func TestWhatIf_PercentRuleUsesShownThreshold(t *testing.T) {
	up := featRow("u", "电子", 1, 0.01)
	up.Features.Prior20Return = floatPtr(0.10)
	down := featRow("d", "银行", 1, -0.01)
	down.Features.Prior20Return = floatPtr(-0.04)
	rules := enableOnly(compileWhatIfRules([]HitRow{up}, []HitRow{down}), "prior20")
	if !hasEnabled(rules, "prior20") {
		t.Fatalf("prior20 rule missing: %+v", rules)
	}
	var rule compiledWhatIf
	for _, item := range rules {
		if item.key == "prior20" && item.filter.Enabled {
			rule = item
		}
	}
	if rule.filter.Rule != "前20日涨跌 ≥ +3.00%" {
		t.Fatalf("rule %s", rule.filter.Rule)
	}
	active := []compiledWhatIf{rule}
	at := featRow("at", "电子", 1, -0.2)
	at.Features.Prior20Return = floatPtr(0.03)
	below := featRow("lo", "电子", 1, 0.9)
	below.Features.Prior20Return = floatPtr(0.029)
	missing := featRow("x", "电子", 1, 0.9)
	missing.Features.Prior20Return = nil
	if !whatIfMatch(at, active) || whatIfMatch(below, active) || whatIfMatch(missing, active) {
		t.Fatalf("at=%v below=%v missing=%v", whatIfMatch(at, active), whatIfMatch(below, active), whatIfMatch(missing, active))
	}
}

func TestEmptyView_WhatIfFailClosed(t *testing.T) {
	view := EmptyView("本地数据库不可用")
	if view.OK || view.WhatIf.OK || view.WhatIf.Message != WhatIfNoCohort {
		t.Fatalf("%+v", view.WhatIf)
	}
	if !strings.Contains(view.WhatIf.Note, "对照实验，不是买卖指令") {
		t.Fatalf("note %s", view.WhatIf.Note)
	}
}
