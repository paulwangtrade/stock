package signalattribution

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func plusOneDirection(row HitRow) (string, bool) {
	cell, ok := cellByHorizon(row, 1)
	if !ok || cell.Status != StatusOK || cell.ReturnRate == nil {
		return "", false
	}
	switch {
	case *cell.ReturnRate > 0:
		return "up", true
	case *cell.ReturnRate < 0:
		return "down", true
	default:
		return "flat", true
	}
}

func featureMedian(rows []HitRow, pick func(AsOfFeatures) *float64) (float64, bool) {
	vals := make([]float64, 0, len(rows))
	for _, row := range rows {
		v := pick(row.Features)
		if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
			continue
		}
		vals = append(vals, *v)
	}
	if len(vals) == 0 {
		return 0, false
	}
	return median(vals), true
}

func industryLabel(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "未标注"
	}
	return s
}

func industryShares(rows []HitRow) map[string]float64 {
	out := map[string]float64{}
	if len(rows) == 0 {
		return out
	}
	counts := map[string]int{}
	for _, row := range rows {
		counts[industryLabel(row.Features.Industry)]++
	}
	n := float64(len(rows))
	for name, c := range counts {
		out[name] = float64(c) / n
	}
	return out
}

func topIndustry(rows []HitRow) (string, string) {
	if len(rows) == 0 {
		return "数据不足", "数据不足"
	}
	counts := map[string]int{}
	for _, row := range rows {
		counts[industryLabel(row.Features.Industry)]++
	}
	bestName := ""
	bestN := -1
	for name, n := range counts {
		if n > bestN || (n == bestN && (bestName == "" || name < bestName)) {
			bestName = name
			bestN = n
		}
	}
	share := float64(bestN) / float64(len(rows))
	return bestName, fmt.Sprintf("%s %.0f%%（%d/%d）", bestName, share*100, bestN, len(rows))
}

func formatMedianNum(v float64, ok bool, spec string) string {
	if !ok {
		return TextInsufficient
	}
	return fmt.Sprintf(spec, v)
}

func fillGroup(key, label string, rows []HitRow) CohortGroup {
	g := CohortGroup{Key: key, Label: label, Count: len(rows)}
	if len(rows) == 0 {
		g.VolumeRatioText = TextInsufficient
		g.Prior20Text = TextInsufficient
		g.DistMA20Text = TextInsufficient
		g.RSIText = TextInsufficient
		g.TopIndustry = "数据不足"
		g.TopIndustryShareText = "数据不足"
		return g
	}
	if v, ok := featureMedian(rows, func(f AsOfFeatures) *float64 { return f.VolumeRatio }); ok {
		g.VolumeRatioText = fmt.Sprintf("%.2f", v)
	} else {
		g.VolumeRatioText = TextInsufficient
	}
	if v, ok := featureMedian(rows, func(f AsOfFeatures) *float64 { return f.Prior20Return }); ok {
		g.Prior20Text = formatReturnPct(v)
	} else {
		g.Prior20Text = TextInsufficient
	}
	if v, ok := featureMedian(rows, func(f AsOfFeatures) *float64 { return f.DistMA20 }); ok {
		g.DistMA20Text = formatReturnPct(v)
	} else {
		g.DistMA20Text = TextInsufficient
	}
	if v, ok := featureMedian(rows, func(f AsOfFeatures) *float64 { return f.RSI }); ok {
		g.RSIText = fmt.Sprintf("%.1f", v)
	} else {
		g.RSIText = TextInsufficient
	}
	g.TopIndustry, g.TopIndustryShareText = topIndustry(rows)
	return g
}

type scoredContrast struct {
	item  CohortContrast
	score float64
}

func relScore(up, down float64) float64 {
	diff := math.Abs(up - down)
	denom := math.Max(math.Abs(up), math.Abs(down))
	if denom < 1e-9 {
		return diff
	}
	return diff / denom
}

func numericContrast(key, label, upText, downText, diffText string, up, down float64, ok bool) (scoredContrast, bool) {
	if !ok {
		return scoredContrast{}, false
	}
	return scoredContrast{
		item: CohortContrast{
			Key:      key,
			Label:    label,
			UpText:   upText,
			DownText: downText,
			DiffText: diffText,
		},
		score: relScore(up, down),
	}, true
}

func buildContrasts(upRows, downRows []HitRow) []scoredContrast {
	out := make([]scoredContrast, 0, 5)
	upVol, upVolOK := featureMedian(upRows, func(f AsOfFeatures) *float64 { return f.VolumeRatio })
	downVol, downVolOK := featureMedian(downRows, func(f AsOfFeatures) *float64 { return f.VolumeRatio })
	if c, ok := numericContrast("volumeRatio", "量比",
		formatMedianNum(upVol, upVolOK, "%.2f"),
		formatMedianNum(downVol, downVolOK, "%.2f"),
		fmt.Sprintf("差 %+.2f", upVol-downVol),
		upVol, downVol, upVolOK && downVolOK); ok {
		out = append(out, c)
	}
	upP, upPOK := featureMedian(upRows, func(f AsOfFeatures) *float64 { return f.Prior20Return })
	downP, downPOK := featureMedian(downRows, func(f AsOfFeatures) *float64 { return f.Prior20Return })
	if c, ok := numericContrast("prior20", "前20日涨跌",
		formatReturnPct(upP), formatReturnPct(downP), "差 "+formatReturnPct(upP-downP),
		upP, downP, upPOK && downPOK); ok {
		out = append(out, c)
	}
	upD, upDOK := featureMedian(upRows, func(f AsOfFeatures) *float64 { return f.DistMA20 })
	downD, downDOK := featureMedian(downRows, func(f AsOfFeatures) *float64 { return f.DistMA20 })
	if c, ok := numericContrast("distMa20", "距MA20",
		formatReturnPct(upD), formatReturnPct(downD), "差 "+formatReturnPct(upD-downD),
		upD, downD, upDOK && downDOK); ok {
		out = append(out, c)
	}
	upR, upROK := featureMedian(upRows, func(f AsOfFeatures) *float64 { return f.RSI })
	downR, downROK := featureMedian(downRows, func(f AsOfFeatures) *float64 { return f.RSI })
	if c, ok := numericContrast("rsi", "RSI",
		formatMedianNum(upR, upROK, "%.1f"),
		formatMedianNum(downR, downROK, "%.1f"),
		fmt.Sprintf("差 %+.1f", upR-downR),
		upR, downR, upROK && downROK); ok {
		out = append(out, c)
	}

	upShare := industryShares(upRows)
	downShare := industryShares(downRows)
	names := map[string]bool{}
	for name := range upShare {
		names[name] = true
	}
	for name := range downShare {
		names[name] = true
	}
	bestName := ""
	bestDiff := 0.0
	bestScore := -1.0
	for name := range names {
		diff := upShare[name] - downShare[name]
		score := math.Abs(diff)
		if score > bestScore || (score == bestScore && (bestName == "" || name < bestName)) {
			bestName = name
			bestDiff = diff
			bestScore = score
		}
	}
	if bestName != "" {
		out = append(out, scoredContrast{
			item: CohortContrast{
				Key:      "industry",
				Label:    "行业「" + bestName + "」占比",
				UpText:   fmt.Sprintf("%.0f%%", upShare[bestName]*100),
				DownText: fmt.Sprintf("%.0f%%", downShare[bestName]*100),
				DiffText: fmt.Sprintf("差 %+.0f 个百分点", bestDiff*100),
			},
			score: bestScore,
		})
	}
	return out
}

// BuildCohort splits rows by the +1 return. Features are as-of-known only.
// largeSample turns off highlight so a wide snapshot is not described as commonality.
func BuildCohort(rows []HitRow, largeSample bool) CohortPanel {
	panel := CohortPanel{
		Note:        CohortNote,
		SizeNote:    CohortSizeNote,
		LargeSample: largeSample,
		Contrasts:   []CohortContrast{},
	}
	var up, down, flat []HitRow
	for _, row := range rows {
		dir, ok := plusOneDirection(row)
		if !ok {
			panel.Excluded++
			continue
		}
		switch dir {
		case "up":
			up = append(up, row)
		case "down":
			down = append(down, row)
		default:
			flat = append(flat, row)
		}
	}
	panel.Up = fillGroup("up", "上涨", up)
	panel.Down = fillGroup("down", "下跌", down)
	panel.Flat = fillGroup("flat", "持平", flat)

	warns := []string{CohortSingleDayWarning}
	if len(up) > 0 && len(down) > 0 && (len(up) < SmallSampleLimit || len(down) < SmallSampleLimit) {
		warns = append(warns, CohortSmallSampleNote)
	}
	if largeSample {
		warns = append(warns, LargeSampleWarning)
		panel.BrowseNote = CohortLargeBrowseNote
	}
	panel.Warning = strings.Join(warns, "")

	if len(up) == 0 && len(down) == 0 {
		panel.Message = CohortEmptyBoth
		return panel
	}
	if len(up) == 0 || len(down) == 0 {
		panel.Message = CohortEmptySide
		return panel
	}
	panel.OK = true
	scored := buildContrasts(up, down)
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	highlights := 0
	for i := range scored {
		if !largeSample && scored[i].score > 1e-9 && highlights < 3 {
			scored[i].item.Highlight = true
			highlights++
		}
		panel.Contrasts = append(panel.Contrasts, scored[i].item)
	}
	return panel
}
