package sectorcoverage

import (
	"sort"
	"strings"
	"time"

	"go-stock/backend/portfoliorisk"
	"go-stock/backend/sectorprovider"
)

// Audit builds SectorCoverageReport from sectorprovider (+ optional portfoliorisk).
// AllowSectorConstraint is true only when holdings coverage meets the minimum,
// no unknown/sentinel labels on holdings, and (when attached) portfoliorisk sector is available.
func Audit(p sectorprovider.Provider, in AuditInput) *SectorCoverageReport {
	rep := &SectorCoverageReport{
		SchemaVersion:                 SchemaVersion,
		AsOf:                          in.AsOf,
		MinCoverageRequired:           in.MinHoldingsCoverage,
		PoolMissing:                   []string{},
		HoldingsMissing:               []string{},
		IndustryClassificationMissing: []string{},
		ClassificationGaps:            []ClassGap{},
		RecordOnly:                    true,
		NotTradePlan:                  true,
		NotExecution:                  true,
		NotBuyChainWrite:              true,
	}
	if rep.AsOf.IsZero() {
		rep.AsOf = time.Now().UTC()
	}
	if rep.MinCoverageRequired <= 0 {
		rep.MinCoverageRequired = DefaultMinHoldingsCoverage
	}
	if p != nil {
		m := p.Meta()
		rep.Source = m.Source
		rep.Version = m.Version
		rep.Taxonomy = m.Taxonomy
	}

	holdings := uniqueNorm(in.Holdings)
	pool := uniqueNorm(in.Pool)
	rep.HoldingsRequested = len(holdings)
	rep.PoolRequested = len(pool)

	if p == nil {
		rep.HoldingsMissing = append([]string(nil), holdings...)
		rep.PoolMissing = append([]string(nil), pool...)
		rep.IndustryClassificationMissing = uniqueNorm(append(append([]string{}, holdings...), pool...))
		for _, s := range holdings {
			rep.ClassificationGaps = append(rep.ClassificationGaps, ClassGap{Symbol: s, Outcome: OutcomeMissing, Bucket: "holdings"})
		}
		for _, s := range pool {
			rep.ClassificationGaps = append(rep.ClassificationGaps, ClassGap{Symbol: s, Outcome: OutcomeMissing, Bucket: "pool"})
		}
		rep.Note = NoteProviderNil
		rep.AllowSectorConstraint = false
		return rep
	}

	holdRes, holdMiss, holdUnk, holdGaps := resolveBucket(p, holdings, "holdings")
	poolRes, poolMiss, poolUnk, poolGaps := resolveBucket(p, pool, "pool")

	rep.HoldingsResolved = holdRes
	rep.HoldingsMissing = holdMiss
	rep.PoolResolved = poolRes
	rep.PoolMissing = poolMiss
	rep.UnknownCount = holdUnk + poolUnk
	rep.ClassificationGaps = append(holdGaps, poolGaps...)
	sort.Slice(rep.ClassificationGaps, func(i, j int) bool {
		if rep.ClassificationGaps[i].Symbol != rep.ClassificationGaps[j].Symbol {
			return rep.ClassificationGaps[i].Symbol < rep.ClassificationGaps[j].Symbol
		}
		return rep.ClassificationGaps[i].Bucket < rep.ClassificationGaps[j].Bucket
	})

	missUnion := map[string]struct{}{}
	for _, s := range holdMiss {
		missUnion[s] = struct{}{}
	}
	for _, s := range poolMiss {
		missUnion[s] = struct{}{}
	}
	for s := range missUnion {
		rep.IndustryClassificationMissing = append(rep.IndustryClassificationMissing, s)
	}
	sort.Strings(rep.IndustryClassificationMissing)

	if rep.HoldingsRequested > 0 {
		rep.HoldingsCoverage = float64(rep.HoldingsResolved) / float64(rep.HoldingsRequested)
	}
	if rep.PoolRequested > 0 {
		rep.PoolCoverage = float64(rep.PoolResolved) / float64(rep.PoolRequested)
	}

	// Gate: holdings coverage only (pool is informational).
	holdingsOK := rep.HoldingsRequested > 0 &&
		len(holdMiss) == 0 &&
		holdUnk == 0 &&
		rep.HoldingsCoverage+1e-12 >= rep.MinCoverageRequired

	rep.HoldingsComplete = holdingsOK || rep.HoldingsRequested == 0

	switch {
	case rep.HoldingsRequested == 0:
		rep.AllowSectorConstraint = false
		rep.HoldingsComplete = true
		rep.Note = NoteEmptyHoldings
	case holdUnk > 0:
		rep.AllowSectorConstraint = false
		rep.HoldingsComplete = false
		rep.Note = NoteUnknownPresent
	case !holdingsOK:
		rep.AllowSectorConstraint = false
		rep.HoldingsComplete = false
		rep.Note = NoteIncomplete
	default:
		rep.AllowSectorConstraint = true
		rep.HoldingsComplete = true
		rep.Note = NoteOK
	}

	if in.AttachPortfolioRisk && in.Snapshot != nil {
		attachRisk(rep, p, in, holdings)
	}

	return rep
}

func attachRisk(rep *SectorCoverageReport, p sectorprovider.Provider, in AuditInput, holdings []string) {
	rep.PortfolioRiskAttached = true
	by, meta, err := sectorprovider.ForBuildInput(p, holdings)
	if err != nil {
		by = nil
	}
	if meta.Taxonomy != "" && rep.Taxonomy == "" {
		rep.Taxonomy = meta.Taxonomy
	}
	built := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot:         in.Snapshot,
		IndustryBySymbol: by,
		IndustryTaxonomy: firstNonEmpty(meta.Taxonomy, rep.Taxonomy),
		Constraints:      in.Constraints,
	})
	avail := false
	note := ""
	if built != nil {
		avail = built.Sector.Available
		note = built.Sector.Note
	}
	rep.PortfolioRiskSectorAvailable = &avail
	rep.PortfolioRiskSectorNote = note
	if !avail {
		rep.AllowSectorConstraint = false
		if rep.Note == NoteOK || rep.Note == "" {
			rep.Note = NoteRiskSectorUnavailable
		}
		if rep.HoldingsRequested > 0 {
			rep.HoldingsComplete = false
		}
	}
}

func resolveBucket(p sectorprovider.Provider, codes []string, bucket string) (resolved int, missing []string, unknown int, gaps []ClassGap) {
	missing = []string{}
	gaps = []ClassGap{}
	for _, c := range codes {
		switch probe(p, c) {
		case OutcomeResolved:
			resolved++
		case OutcomeUnknown:
			unknown++
			missing = append(missing, c)
			gaps = append(gaps, ClassGap{Symbol: c, Outcome: OutcomeUnknown, Bucket: bucket})
		default:
			missing = append(missing, c)
			gaps = append(gaps, ClassGap{Symbol: c, Outcome: OutcomeMissing, Bucket: bucket})
		}
	}
	sort.Strings(missing)
	return resolved, missing, unknown, gaps
}

// probe distinguishes resolved / missing / unknown (sentinel) without inventing zeros.
func probe(p sectorprovider.Provider, symbol string) string {
	code := sectorprovider.NormSymbol(symbol)
	if code == "" {
		return OutcomeMissing
	}
	if info, ok := p.Lookup(code); ok {
		if sectorprovider.IsSentinelLabel(info.Industry) || sectorprovider.IsSentinelLabel(info.EffectiveSector()) {
			return OutcomeUnknown
		}
		return OutcomeResolved
	}
	// Peek raw labels when Provider rejects sentinels at Lookup.
	if ind, sec, present := rawLabels(p, code); present {
		if sectorprovider.IsSentinelLabel(ind) || sectorprovider.IsSentinelLabel(sec) {
			return OutcomeUnknown
		}
		// present but Lookup failed for other reasons → treat as missing
	}
	return OutcomeMissing
}

func rawLabels(p sectorprovider.Provider, code string) (industry, sector string, present bool) {
	switch t := p.(type) {
	case *sectorprovider.TableProvider:
		if t == nil || len(t.Entries) == 0 {
			return "", "", false
		}
		if e, ok := t.Entries[code]; ok {
			ind := strings.TrimSpace(e.Industry)
			sec := strings.TrimSpace(e.Sector)
			if sec == "" {
				sec = ind
			}
			return ind, sec, true
		}
		for k, e := range t.Entries {
			if sectorprovider.NormSymbol(k) == code {
				ind := strings.TrimSpace(e.Industry)
				sec := strings.TrimSpace(e.Sector)
				if sec == "" {
					sec = ind
				}
				return ind, sec, true
			}
		}
	case *sectorprovider.FuncProvider:
		if t == nil || t.Fn == nil {
			return "", "", false
		}
		ind, board := t.Fn(code)
		ind = strings.TrimSpace(ind)
		board = strings.TrimSpace(board)
		sec := ind
		if t.UseBoardAsSector {
			sec = board
			if sec == "" {
				sec = ind
			}
		}
		if ind == "" && board == "" {
			return "", "", false
		}
		return ind, sec, true
	}
	return "", "", false
}

func uniqueNorm(codes []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		k := sectorprovider.NormSymbol(c)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
