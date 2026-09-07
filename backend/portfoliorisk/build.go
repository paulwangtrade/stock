package portfoliorisk

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/positionstate"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/tradingconfig"
)

// BuildInput accepts only the H.0 domain sources. No PlanFilter / TradePlan / Execution inputs.
type BuildInput struct {
	Snapshot       *portfolio.Snapshot
	TradeDate      string
	Risk           *tradingconfig.RiskView
	Constraints    *portfoliolayer.ConstraintSet
	PositionStates []positionstate.PositionStateView
	// IndustryBySymbol is optional H0.3 symbol→sector map (normalized on Build).
	// nil/empty → sector.available=false. Partial holding coverage → unavailable.
	IndustryBySymbol map[string]string
	// IndustryTaxonomy labels the map source (e.g. eastmoney_industry_v1 / fixture).
	IndustryTaxonomy string
}

type resolvedCaps struct {
	gross     *float64
	single    *float64
	capSource string
	level     int
	blockNew  bool
	mktSource string
}

// Build projects a PortfolioRiskSnapshot. Nil / not-found Snapshot closes metric blocks safely.
func Build(in BuildInput) *PortfolioRiskSnapshot {
	caps := resolveCaps(in)
	out := emptySnapshot(in)
	snap := in.Snapshot
	if snap == nil || !snap.Found {
		out.Found = false
		out.Exposure = closedExposure(NoteFoundFalse)
		out.Concentration = closedConcentration(NoteFoundFalse)
		out.Sector = closedSector(NoteSectorUnavailable)
		out.Theme = closedTheme()
		out.Correlation = closedCorrelation()
		out.Market = buildMarket(caps, false)
		out.PositionLiquidity = buildLiquidity(in.PositionStates)
		out.InputsFingerprint = fingerprint(in, caps, out)
		return out
	}

	out.Found = true
	out.AsOf = snap.AsOf
	out.AccountID = snap.AccountID
	if out.AsOf.IsZero() {
		out.AsOf = time.Now().UTC()
	}

	out.Exposure = buildExposure(snap, caps)
	out.Concentration = buildConcentration(snap, caps)
	out.Sector = buildSector(snap, in.IndustryBySymbol, in.IndustryTaxonomy, in.Constraints)
	out.Theme = closedTheme()
	out.Correlation = closedCorrelation()
	out.Market = buildMarket(caps, true)
	out.PositionLiquidity = buildLiquidity(in.PositionStates)
	out.InputsFingerprint = fingerprint(in, caps, out)
	return out
}

func resolveCaps(in BuildInput) resolvedCaps {
	var out resolvedCaps
	var riskGross, riskSingle *float64
	var consGross, consSingle *float64

	if in.Risk != nil {
		if in.Risk.MaxGrossExposurePct > 0 {
			v := in.Risk.MaxGrossExposurePct
			riskGross = &v
		}
		if in.Risk.MaxSingleNamePct > 0 {
			v := in.Risk.MaxSingleNamePct
			riskSingle = &v
		}
		if in.Risk.MarketLevel > 0 {
			out.level = in.Risk.MarketLevel
			out.blockNew = in.Risk.BlockNewEntriesOnDefense
			out.mktSource = MarketSourceRiskView
		}
	}
	if in.Constraints != nil {
		r := in.Constraints.Resolve()
		if r.MaxGrossExposurePct > 0 {
			v := r.MaxGrossExposurePct
			consGross = &v
		}
		if r.MaxSingleWeight > 0 {
			v := r.MaxSingleWeight
			consSingle = &v
		}
		if out.level <= 0 && r.RiskMarketLevel > 0 {
			out.level = r.RiskMarketLevel
			out.blockNew = r.RiskBlockNewEntries
			out.mktSource = MarketSourceConstraintRisk
		}
	}

	out.gross, out.capSource = mergeCap(riskGross, consGross)
	single, singleSrc := mergeCap(riskSingle, consSingle)
	out.single = single
	if out.capSource == "" {
		out.capSource = singleSrc
	} else if singleSrc != "" && singleSrc != out.capSource {
		out.capSource = CapSourceMerged
	}
	return out
}

func mergeCap(risk, cons *float64) (*float64, string) {
	switch {
	case risk != nil && cons != nil:
		v := *risk
		if *cons < v {
			v = *cons
		}
		return &v, CapSourceMerged
	case risk != nil:
		v := *risk
		return &v, CapSourceRiskView
	case cons != nil:
		v := *cons
		return &v, CapSourceConstraintResolve
	default:
		return nil, ""
	}
}

func emptySnapshot(in BuildInput) *PortfolioRiskSnapshot {
	asOf := time.Time{}
	var accountID uint
	if in.Snapshot != nil {
		asOf = in.Snapshot.AsOf
		accountID = in.Snapshot.AccountID
	}
	return &PortfolioRiskSnapshot{
		SchemaVersion:    SchemaVersionH01,
		AsOf:             asOf,
		TradeDate:        strings.TrimSpace(in.TradeDate),
		AccountID:        accountID,
		RecordOnly:       true,
		NotATradePlan:    true,
		NotPlanFilter:    true,
		NotExecutionRisk: true,
		DataSourceNote:   dataSourceNote,
		Sector: SectorBlock{
			Available:      false,
			SectorExposure: []SectorWeight{},
			Note:           NoteSectorUnavailable,
		},
		Theme: ThemeBlock{
			Available:     false,
			ThemeExposure: []ThemeWeight{},
			Note:          NoteThemeNotImplemented,
		},
		Correlation: CorrelationBlock{
			Available:       false,
			CorrelationRisk: nil,
			Note:            NoteCorrelationUnavailable,
		},
	}
}

func closedExposure(note string) ExposureBlock {
	return ExposureBlock{Available: false, Note: note}
}

func closedConcentration(note string) ConcentrationBlock {
	return ConcentrationBlock{Available: false, Note: note}
}

func closedSector(note string) SectorBlock {
	return SectorBlock{
		Available:      false,
		SectorExposure: []SectorWeight{},
		Note:           note,
	}
}

func closedTheme() ThemeBlock {
	return ThemeBlock{
		Available:     false,
		ThemeExposure: []ThemeWeight{},
		Note:          NoteThemeNotImplemented,
	}
}

func closedCorrelation() CorrelationBlock {
	return CorrelationBlock{
		Available:       false,
		CorrelationRisk: nil,
		Note:            NoteCorrelationUnavailable,
	}
}

func buildExposure(snap *portfolio.Snapshot, caps resolvedCaps) ExposureBlock {
	if snap.TotalEquity <= 0 || math.IsNaN(snap.TotalEquity) || math.IsInf(snap.TotalEquity, 0) {
		return closedExposure(NoteEquityNonPositive)
	}
	gross := snap.TotalExposure / snap.TotalEquity
	cashRatio := snap.Cash / snap.TotalEquity
	eq := snap.TotalEquity
	notional := snap.TotalExposure
	out := ExposureBlock{
		Available:     true,
		GrossExposure: &gross,
		GrossNotional: &notional,
		Equity:        &eq,
		CashRatio:     &cashRatio,
	}
	if caps.gross != nil {
		cap := *caps.gross
		out.CapGross = &cap
		out.CapSource = caps.capSource
		head := cap - gross
		out.HeadroomVsCap = &head
	}
	return out
}

func buildConcentration(snap *portfolio.Snapshot, caps resolvedCaps) ConcentrationBlock {
	weights := make([]float64, 0, len(snap.Positions))
	for _, p := range snap.Positions {
		if p.Volume <= 0 {
			continue
		}
		w := p.Weight
		if math.IsNaN(w) || math.IsInf(w, 0) || w < 0 {
			continue
		}
		weights = append(weights, w)
	}
	sort.Slice(weights, func(i, j int) bool { return weights[i] > weights[j] })

	var top1, top5 float64
	if len(weights) > 0 {
		top1 = weights[0]
		n := 5
		if len(weights) < n {
			n = len(weights)
		}
		for i := 0; i < n; i++ {
			top5 += weights[i]
		}
	}
	out := ConcentrationBlock{
		Available:  true,
		Top1Weight: &top1,
		Top5Weight: &top5,
		NameCount:  snap.PositionCount,
	}
	if caps.single != nil {
		cap := *caps.single
		out.CapSingle = &cap
		out.CapSource = caps.capSource
	}
	return out
}

func buildMarket(caps resolvedCaps, found bool) MarketBlock {
	if caps.level <= 0 {
		src := caps.mktSource
		if src == "" {
			src = MarketSourceUnavailable
		}
		return MarketBlock{
			Available:    found,
			MarketRegime: RegimeUnknown,
			Source:       src,
			PnLAvailable: false,
			Note:         "market_regime_input_level_unset_projection_only",
		}
	}
	src := caps.mktSource
	if src == "" {
		src = MarketSourceRiskView
	}
	return MarketBlock{
		Available:       found,
		MarketRegime:    fmt.Sprintf("configured_level_%d", caps.level),
		Source:          src,
		BlockNewEntries: caps.blockNew,
		MarketLevel:     caps.level,
		PnLAvailable:    false,
		Note:            "market_regime_projected_from_input_market_level_only",
	}
}

func buildLiquidity(states []positionstate.PositionStateView) PositionLiquidityNote {
	if len(states) == 0 {
		return PositionLiquidityNote{Available: false}
	}
	out := PositionLiquidityNote{Available: true, NameCount: len(states)}
	for _, s := range states {
		if s.CanSell && s.AvailableQty > 0 {
			out.SellableNameCount++
		} else if s.TotalQty > 0 {
			out.LockedNameCount++
		}
	}
	return out
}

func fingerprint(in BuildInput, caps resolvedCaps, out *PortfolioRiskSnapshot) string {
	type pos struct {
		Code string  `json:"c"`
		W    float64 `json:"w"`
		MV   float64 `json:"mv"`
		Vol  int64   `json:"v"`
	}
	type ps struct {
		Sym   string `json:"s"`
		Can   bool   `json:"can"`
		Avail int64  `json:"aq"`
		Total int64  `json:"tq"`
	}
	type ind struct {
		Code   string `json:"c"`
		Sector string `json:"s"`
	}
	payload := struct {
		Schema     string   `json:"schema"`
		TradeDate  string   `json:"trade_date"`
		Found      bool     `json:"found"`
		AccountID  uint     `json:"account_id"`
		AsOfUnix   int64    `json:"as_of_unix"`
		Equity     float64  `json:"equity"`
		Cash       float64  `json:"cash"`
		Exposure   float64  `json:"exposure"`
		Positions  []pos    `json:"positions"`
		GrossCap   *float64 `json:"gross_cap"`
		SingleCap  *float64 `json:"single_cap"`
		MktLevel   int      `json:"market_level"`
		PosStates  []ps     `json:"position_states"`
		Taxonomy   string   `json:"taxonomy"`
		Industries []ind    `json:"industries"`
		MaxSector  *float64 `json:"max_sector_weight"`
	}{
		Schema:    SchemaVersionH01,
		TradeDate: out.TradeDate,
		GrossCap:  caps.gross,
		SingleCap: caps.single,
		MktLevel:  caps.level,
		Taxonomy:  strings.TrimSpace(in.IndustryTaxonomy),
		MaxSector: out.Sector.MaxSectorWeight,
	}
	if in.Snapshot != nil {
		payload.Found = in.Snapshot.Found
		payload.AccountID = in.Snapshot.AccountID
		if !in.Snapshot.AsOf.IsZero() {
			payload.AsOfUnix = in.Snapshot.AsOf.UTC().UnixNano()
		}
		payload.Equity = in.Snapshot.TotalEquity
		payload.Cash = in.Snapshot.Cash
		payload.Exposure = in.Snapshot.TotalExposure
		for _, p := range in.Snapshot.Positions {
			payload.Positions = append(payload.Positions, pos{
				Code: strings.ToLower(strings.TrimSpace(p.StockCode)),
				W:    p.Weight,
				MV:   p.MarketValue,
				Vol:  p.Volume,
			})
		}
		sort.Slice(payload.Positions, func(i, j int) bool {
			return payload.Positions[i].Code < payload.Positions[j].Code
		})
	}
	for _, s := range in.PositionStates {
		payload.PosStates = append(payload.PosStates, ps{
			Sym: strings.ToLower(strings.TrimSpace(s.Symbol)),
			Can: s.CanSell, Avail: s.AvailableQty, Total: s.TotalQty,
		})
	}
	sort.Slice(payload.PosStates, func(i, j int) bool {
		return payload.PosStates[i].Sym < payload.PosStates[j].Sym
	})
	for code, sec := range NormalizeIndustryBySymbol(in.IndustryBySymbol) {
		payload.Industries = append(payload.Industries, ind{Code: code, Sector: sec})
	}
	sort.Slice(payload.Industries, func(i, j int) bool {
		return payload.Industries[i].Code < payload.Industries[j].Code
	})
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
