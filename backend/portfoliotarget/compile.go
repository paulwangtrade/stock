package portfoliotarget

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// CompileTargetPortfolio is the pure compile entry (read-only).
func CompileTargetPortfolio(in CompileInput) *TargetPortfolio {
	asOf := in.AsOf
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}
	src := strings.TrimSpace(in.Options.Source)
	if src == "" {
		src = SourceObjectiveCompile
	}
	method := strings.TrimSpace(in.Options.MaterializeMethod)
	if method == "" {
		method = MaterializeEqualWeight
	}

	out := &TargetPortfolio{
		SchemaVersion:      SchemaVersion,
		AsOfIntent:         asOf,
		TradeDate:          strings.TrimSpace(in.TradeDate),
		AccountID:          strings.TrimSpace(in.AccountID),
		Source:             src,
		TargetPositions:    []TargetPosition{},
		TargetSectorWeights: []SectorWeightTarget{},
		UnresolvedNotes:    []string{},
		MaterializeMethod:  method,
		RecordOnly:         true,
		NotATradePlan:      true,
		NotExecution:       true,
		NotAutoRebalance:   true,
		NotAllocWrite:      true,
		Capital:            CapitalTarget{Mode: CapitalModeWeight},
	}

	// --- ceilings from UserRisk (hard) ∩ Objective/Strategy (intent, tighten-only) ---
	ceilGross := pickPositive(in.UserRisk.MaxGrossExposurePct, defaultGrossForTier(in.UserRisk.RiskTier))
	ceilSingle := pickPositive(in.UserRisk.MaxSingleWeight, defaultSingleForTier(in.UserRisk.RiskTier))
	ceilSector := in.UserRisk.MaxSectorWeight

	intentGross := tightenFloat(ceilGross, firstFloat(in.Objective.TargetGrossExposurePct, nil))
	intentSingle := tightenFloat(ceilSingle, firstFloat(in.Objective.MaxSingleWeight, nil))
	intentCash := pickCash(in)
	intentSectorCap := tightenFloatPtr(ceilSector, firstFloat(in.Objective.MaxSectorWeight, in.Sector.MaxSectorWeight))

	// Gross deployable cannot exceed 1 - cash.
	if intentCash < 0 {
		intentCash = 0
	}
	if intentCash > 1 {
		intentCash = 1
	}
	maxDeploy := 1 - intentCash
	if intentGross > maxDeploy {
		intentGross = maxDeploy
		out.UnresolvedNotes = append(out.UnresolvedNotes, "gross_clamped_by_cash_target")
	}
	if intentGross > ceilGross {
		intentGross = ceilGross
		out.UnresolvedNotes = append(out.UnresolvedNotes, "gross_clamped_by_risk_ceiling")
	}
	if intentSingle > ceilSingle {
		intentSingle = ceilSingle
		out.UnresolvedNotes = append(out.UnresolvedNotes, "single_clamped_by_risk_ceiling")
	}

	minNames, maxNames := breadth(in)
	blockNew := mergeBlockNew(in)

	out.TargetCashRatio = intentCash
	out.TargetGrossExposure = intentGross
	out.MaxSingleWeight = intentSingle
	out.Capital = CapitalTarget{
		EquityExposureTarget: intentGross,
		CashTargetWeight:     intentCash,
		Mode:                 CapitalModeWeight,
	}
	if in.EquityRef > 0 {
		out.Capital.CashTargetNotional = intentCash * in.EquityRef
	}
	out.Breadth = BreadthTarget{MinNames: minNames, MaxNames: maxNames}
	out.NamePolicy = NamePolicy{DefaultMaxWeight: intentSingle}
	out.RiskLevel = RiskLevelTarget{
		TargetTier:          normalizeTier(in.UserRisk.RiskTier),
		MaxGrossExposurePct: f64p(intentGross),
		BlockNewEntries:     blockNew,
		Source:              "merged",
	}

	// --- sector block (fail-closed) ---
	out.Sector = SectorTarget{
		Available:    false,
		Taxonomy:     strings.TrimSpace(in.Sector.Taxonomy),
		CoverageNote: strings.TrimSpace(in.Sector.Note),
	}
	if in.Sector.Available && len(in.Sector.BySymbol) > 0 {
		out.Sector.Available = true
		if intentSectorCap != nil && *intentSectorCap > 0 {
			out.Sector.MaxSectorWeight = intentSectorCap
		}
		// Strategy sector intents (optional), clamped later after materialize.
		out.Sector.Targets = strategySectorTargets(in.Strategy.SectorTargetWeights, intentSectorCap)
	} else {
		if out.Sector.CoverageNote == "" {
			out.Sector.CoverageNote = "sector_classification_unavailable"
		}
		out.UnresolvedNotes = append(out.UnresolvedNotes, "sector_targets_disabled")
	}

	// --- materialize positions ---
	symbols := uniqueNorm(in.Symbols)
	if len(symbols) == 0 {
		out.UnresolvedNotes = append(out.UnresolvedNotes, "no_symbols_to_materialize")
		out.Fingerprint = fingerprint(out, in)
		return out
	}
	n := maxNames
	if n <= 0 {
		n = len(symbols)
	}
	if n > len(symbols) {
		n = len(symbols)
	}
	if n < 1 {
		out.UnresolvedNotes = append(out.UnresolvedNotes, "max_names_zero")
		out.Fingerprint = fingerprint(out, in)
		return out
	}
	chosen := symbols[:n]

	switch method {
	case MaterializeEqualWeight:
		out.TargetPositions = materializeEqualWeight(chosen, intentGross, intentSingle, in)
	default:
		out.UnresolvedNotes = append(out.UnresolvedNotes, "unknown_materialize_method_fallback_equal_weight")
		out.TargetPositions = materializeEqualWeight(chosen, intentGross, intentSingle, in)
		out.MaterializeMethod = MaterializeEqualWeight
	}

	if in.Options.ScaleToFit {
		out.TargetPositions = scalePositions(out.TargetPositions, intentCash)
		// refresh amounts
		for i := range out.TargetPositions {
			if in.EquityRef > 0 {
				out.TargetPositions[i].TargetAmount = out.TargetPositions[i].TargetWeight * in.EquityRef
			}
		}
	}

	// Attach sector names / aggregate sector weights when available.
	if out.Sector.Available {
		attachSectors(out, in.Sector.BySymbol)
		agg := aggregateSectorWeights(out.TargetPositions)
		if len(out.Sector.Targets) == 0 {
			out.Sector.Targets = agg
		}
		out.TargetSectorWeights = out.Sector.Targets
		// If strategy provided targets, keep them; still expose realized agg in notes if diverge.
		if len(in.Strategy.SectorTargetWeights) > 0 {
			out.TargetSectorWeights = out.Sector.Targets
		} else {
			out.TargetSectorWeights = agg
			out.Sector.Targets = agg
		}
	} else {
		out.TargetSectorWeights = nil
		// strip any sector fields
		for i := range out.TargetPositions {
			out.TargetPositions[i].SectorName = ""
			out.TargetPositions[i].TargetSectorWeight = 0
		}
	}

	sumW := 0.0
	for _, p := range out.TargetPositions {
		sumW += p.TargetWeight
	}
	out.TargetGrossExposure = sumW
	out.Capital.EquityExposureTarget = sumW
	if sumW+intentCash > 1+weightSumEpsilon {
		out.UnresolvedNotes = append(out.UnresolvedNotes, fmt.Sprintf(
			"weight_sum_exceeds_one: names=%.4f cash=%.4f", sumW, intentCash,
		))
	}

	out.Fingerprint = fingerprint(out, in)
	return out
}

func materializeEqualWeight(symbols []string, gross, maxSingle float64, in CompileInput) []TargetPosition {
	n := len(symbols)
	if n == 0 || gross <= 0 {
		return []TargetPosition{}
	}
	uniform := math.Floor(gross/float64(n)*1e8) / 1e8 // stable-ish
	if uniform <= 0 {
		uniform = gross / float64(n)
	}
	if maxSingle > 0 && uniform > maxSingle {
		uniform = maxSingle
	}
	out := make([]TargetPosition, 0, n)
	for i, sym := range symbols {
		w := uniform
		amt := 0.0
		if in.EquityRef > 0 {
			amt = w * in.EquityRef
		}
		out = append(out, TargetPosition{
			Symbol:       sym,
			TargetWeight: w,
			TargetAmount: amt,
			Priority:     i + 1,
			Reason:       "objective_equal_weight",
			Source:       SourceObjectiveCompile,
		})
	}
	return out
}

func scalePositions(pos []TargetPosition, cash float64) []TargetPosition {
	sum := 0.0
	for _, p := range pos {
		sum += p.TargetWeight
	}
	budget := 1 - cash
	if budget < 0 {
		budget = 0
	}
	if sum <= budget+weightSumEpsilon || sum <= 0 {
		return pos
	}
	scale := budget / sum
	for i := range pos {
		pos[i].TargetWeight *= scale
	}
	return pos
}

func attachSectors(out *TargetPortfolio, by map[string]string) {
	normMap := map[string]string{}
	for k, v := range by {
		nk := normSym(k)
		sv := strings.TrimSpace(v)
		if nk == "" || sv == "" || isSentinel(sv) {
			continue
		}
		normMap[nk] = sv
	}
	secW := map[string]float64{}
	for i := range out.TargetPositions {
		sym := normSym(out.TargetPositions[i].Symbol)
		sec, ok := normMap[sym]
		if !ok {
			out.UnresolvedNotes = append(out.UnresolvedNotes, "sector_missing:"+sym)
			continue
		}
		out.TargetPositions[i].SectorName = sec
		secW[sec] += out.TargetPositions[i].TargetWeight
	}
	for i := range out.TargetPositions {
		sec := out.TargetPositions[i].SectorName
		if sec == "" {
			continue
		}
		out.TargetPositions[i].TargetSectorWeight = secW[sec]
	}
}

func aggregateSectorWeights(pos []TargetPosition) []SectorWeightTarget {
	agg := map[string]float64{}
	for _, p := range pos {
		if p.SectorName == "" {
			continue
		}
		agg[p.SectorName] += p.TargetWeight
	}
	keys := make([]string, 0, len(agg))
	for k := range agg {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]SectorWeightTarget, 0, len(keys))
	for _, k := range keys {
		out = append(out, SectorWeightTarget{SectorName: k, TargetWeight: agg[k]})
	}
	return out
}

func strategySectorTargets(m map[string]float64, cap *float64) []SectorWeightTarget {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]SectorWeightTarget, 0, len(keys))
	for _, k := range keys {
		w := m[k]
		if w < 0 {
			w = 0
		}
		if cap != nil && *cap > 0 && w > *cap {
			w = *cap
		}
		name := strings.TrimSpace(k)
		if name == "" || isSentinel(name) {
			continue
		}
		out = append(out, SectorWeightTarget{SectorName: name, TargetWeight: w})
	}
	return out
}

func pickCash(in CompileInput) float64 {
	if in.Objective.TargetCashRatio != nil && *in.Objective.TargetCashRatio >= 0 {
		return clamp01(*in.Objective.TargetCashRatio)
	}
	if in.UserRisk.TargetCashRatio != nil && *in.UserRisk.TargetCashRatio >= 0 {
		return clamp01(*in.UserRisk.TargetCashRatio)
	}
	return defaultCashForTier(in.UserRisk.RiskTier)
}

func breadth(in CompileInput) (minN, maxN int) {
	maxN = 10
	minN = 1
	if in.Strategy.MaxNames != nil && *in.Strategy.MaxNames > 0 {
		maxN = *in.Strategy.MaxNames
	}
	if in.Objective.MaxNewNames != nil && *in.Objective.MaxNewNames > 0 && *in.Objective.MaxNewNames < maxN {
		maxN = *in.Objective.MaxNewNames
	}
	if in.Strategy.MinNames != nil && *in.Strategy.MinNames > 0 {
		minN = *in.Strategy.MinNames
	}
	if in.Objective.MinNames != nil && *in.Objective.MinNames > 0 {
		minN = *in.Objective.MinNames
	}
	if minN > maxN {
		minN = maxN
	}
	return minN, maxN
}

func mergeBlockNew(in CompileInput) *bool {
	if in.UserRisk.BlockNewEntries != nil && *in.UserRisk.BlockNewEntries {
		t := true
		return &t
	}
	if in.Objective.BlockNewEntries != nil {
		v := *in.Objective.BlockNewEntries
		return &v
	}
	return nil
}

func defaultGrossForTier(tier string) float64 {
	switch normalizeTier(tier) {
	case RiskTierConservative:
		return 0.70
	case RiskTierAggressive:
		return 0.95
	default:
		return 0.85
	}
}

func defaultSingleForTier(tier string) float64 {
	switch normalizeTier(tier) {
	case RiskTierConservative:
		return 0.10
	case RiskTierAggressive:
		return 0.25
	default:
		return 0.20
	}
}

func defaultCashForTier(tier string) float64 {
	switch normalizeTier(tier) {
	case RiskTierConservative:
		return 0.25
	case RiskTierAggressive:
		return 0.05
	default:
		return 0.15
	}
}

func normalizeTier(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case RiskTierConservative, "low":
		return RiskTierConservative
	case RiskTierAggressive, "high":
		return RiskTierAggressive
	case RiskTierModerate, "medium", "":
		return RiskTierModerate
	default:
		return RiskTierModerate
	}
}

func pickPositive(p *float64, def float64) float64 {
	if p != nil && *p > 0 {
		return *p
	}
	return def
}

func firstFloat(a, b *float64) *float64 {
	if a != nil {
		return a
	}
	return b
}

func tightenFloat(ceiling float64, intent *float64) float64 {
	if intent == nil || *intent <= 0 {
		return ceiling
	}
	if *intent < ceiling {
		return *intent
	}
	return ceiling
}

func tightenFloatPtr(ceiling, intent *float64) *float64 {
	if ceiling == nil && intent == nil {
		return nil
	}
	if ceiling == nil {
		if intent != nil && *intent > 0 {
			v := *intent
			return &v
		}
		return nil
	}
	if *ceiling <= 0 {
		return nil
	}
	v := *ceiling
	if intent != nil && *intent > 0 && *intent < v {
		v = *intent
	}
	return &v
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func f64p(v float64) *float64 { return &v }

func normSym(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func isSentinel(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "unknown", "0", "none", "null", "n/a", "na":
		return true
	default:
		return false
	}
}

func uniqueNorm(codes []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		k := normSym(c)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

func fingerprint(out *TargetPortfolio, in CompileInput) string {
	type fp struct {
		Src     string
		Cash    float64
		Gross   float64
		Single  float64
		MinN    int
		MaxN    int
		Tier    string
		SecAvail bool
		Sec     []SectorWeightTarget
		Pos     []TargetPosition
		Syms    []string
		Date    string
	}
	payload, _ := json.Marshal(fp{
		Src: out.Source, Cash: out.TargetCashRatio, Gross: out.TargetGrossExposure,
		Single: out.MaxSingleWeight, MinN: out.Breadth.MinNames, MaxN: out.Breadth.MaxNames,
		Tier: out.RiskLevel.TargetTier, SecAvail: out.Sector.Available,
		Sec: out.TargetSectorWeights, Pos: out.TargetPositions,
		Syms: uniqueNorm(in.Symbols), Date: out.TradeDate,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:8])
}
