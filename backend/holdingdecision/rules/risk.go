package rules

// Risk rules consume PortfolioRiskSnapshot blocks only when available=true.
// Never treat unavailable as pass or fake 0.

type riskNameOverCapRule struct{}

func (riskNameOverCapRule) ID() string     { return RuleRiskNameOverCap }
func (riskNameOverCapRule) Family() string { return FamilyRisk }

func (riskNameOverCapRule) Eval(ctx RuleContext) []RuleHit {
	pol := ctx.Policy
	if !pol.EnableRisk || ctx.PortfolioRisk == nil || !ctx.PortfolioRisk.Found {
		return nil
	}
	block := ctx.PortfolioRisk.Concentration
	if !block.Available || block.CapSingle == nil || *block.CapSingle <= 0 {
		return nil
	}
	if !weightAboveCap(ctx.Weight, *block.CapSingle) {
		return nil
	}
	return []RuleHit{hit(RuleRiskNameOverCap, FamilyRisk, ActionReduce, SeverityHigh, ReasonRiskNameOverCap, map[string]any{
		"weight":     ctx.Weight,
		"cap_single": *block.CapSingle,
		"note":       "name weight over PortfolioRisk concentration cap → observe REDUCE",
	})}
}

type riskGrossHotRule struct{}

func (riskGrossHotRule) ID() string     { return RuleRiskGrossHot }
func (riskGrossHotRule) Family() string { return FamilyRisk }

func (riskGrossHotRule) Eval(ctx RuleContext) []RuleHit {
	pol := ctx.Policy
	if !pol.EnableRisk || ctx.PortfolioRisk == nil || !ctx.PortfolioRisk.Found {
		return nil
	}
	ex := ctx.PortfolioRisk.Exposure
	if !ex.Available {
		return nil
	}
	// Headroom exhausted (≤0) and this name has material weight → observe REDUCE.
	if ex.HeadroomVsCap == nil || *ex.HeadroomVsCap > 1e-12 {
		return nil
	}
	if ctx.Weight <= 0.05 { // ignore tiny names for gross trim observation
		return nil
	}
	ev := map[string]any{
		"weight":          ctx.Weight,
		"headroom_vs_cap": *ex.HeadroomVsCap,
		"note":            "gross headroom exhausted → observe REDUCE on material weight",
	}
	if ex.GrossExposure != nil {
		ev["gross_exposure"] = *ex.GrossExposure
	}
	return []RuleHit{hit(RuleRiskGrossHot, FamilyRisk, ActionReduce, SeverityHigh, ReasonRiskGrossHot, ev)}
}

type riskSectorHotRule struct{}

func (riskSectorHotRule) ID() string     { return RuleRiskSectorHot }
func (riskSectorHotRule) Family() string { return FamilyRisk }

func (riskSectorHotRule) Eval(ctx RuleContext) []RuleHit {
	pol := ctx.Policy
	if !pol.EnableRisk || ctx.PortfolioRisk == nil || !ctx.PortfolioRisk.Found {
		return nil
	}
	sec := ctx.PortfolioRisk.Sector
	if !sec.Available || sec.MaxSectorWeight == nil || *sec.MaxSectorWeight <= 0 {
		return nil
	}
	ind := normIndustry(ctx.Industry)
	if ind == "" {
		return nil
	}
	var sectorW float64
	found := false
	for _, sw := range sec.SectorExposure {
		if normIndustry(sw.Sector) == ind {
			sectorW = sw.Weight
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	if sectorW <= *sec.MaxSectorWeight+1e-12 {
		return nil
	}
	return []RuleHit{hit(RuleRiskSectorHot, FamilyRisk, ActionReduce, SeverityHigh, ReasonRiskSectorHot, map[string]any{
		"industry":          ctx.Industry,
		"sector_weight":     sectorW,
		"max_sector_weight": *sec.MaxSectorWeight,
		"note":              "sector over PortfolioRisk max → observe REDUCE",
	})}
}
