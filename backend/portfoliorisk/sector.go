package portfoliorisk

import (
	"math"
	"sort"
	"strings"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
)

// buildSector projects SectorBlock from Snapshot + optional IndustryBySymbol (H0.3).
// available=true only when every Volume>0 holding has a non-empty sector (full coverage).
// Never writes max_sector_weight=0; omit pointer when no positive constraint cap.
func buildSector(snap *portfolio.Snapshot, industryBySymbol map[string]string, taxonomy string, constraints *portfoliolayer.ConstraintSet) SectorBlock {
	taxonomy = strings.TrimSpace(taxonomy)
	m := NormalizeIndustryBySymbol(industryBySymbol)
	if snap == nil || !snap.Found {
		return closedSector(NoteSectorUnavailable)
	}
	if snap.TotalEquity <= 0 || math.IsNaN(snap.TotalEquity) || math.IsInf(snap.TotalEquity, 0) {
		return closedSector(NoteEquityNonPositive)
	}
	if len(m) == 0 {
		return closedSector(NoteSectorUnavailable)
	}

	type held struct {
		code   string
		weight float64
		sector string
	}
	helds := make([]held, 0, len(snap.Positions))
	for _, p := range snap.Positions {
		if p.Volume <= 0 {
			continue
		}
		code := normSymbol(p.StockCode)
		if code == "" {
			continue
		}
		w := p.Weight
		if math.IsNaN(w) || math.IsInf(w, 0) || w < 0 {
			w = 0
		}
		helds = append(helds, held{
			code:   code,
			weight: w,
			sector: lookupSector(m, code),
		})
	}

	for _, h := range helds {
		if h.sector == "" {
			out := closedSector(NoteSectorIncompleteCoverage)
			out.Taxonomy = taxonomy
			return out
		}
	}

	// Full coverage (including empty book: vacuously complete).
	buckets := map[string]*SectorWeight{}
	for _, h := range helds {
		b := buckets[h.sector]
		if b == nil {
			b = &SectorWeight{Sector: h.sector}
			buckets[h.sector] = b
		}
		b.Weight += h.weight
		b.NameCount++
	}

	exposure := make([]SectorWeight, 0, len(buckets))
	for _, b := range buckets {
		exposure = append(exposure, *b)
	}
	sort.Slice(exposure, func(i, j int) bool {
		if exposure[i].Weight != exposure[j].Weight {
			return exposure[i].Weight > exposure[j].Weight
		}
		return exposure[i].Sector < exposure[j].Sector
	})

	out := SectorBlock{
		Available:      true,
		SectorExposure: exposure,
		Taxonomy:       taxonomy,
	}
	if out.SectorExposure == nil {
		out.SectorExposure = []SectorWeight{}
	}
	if cap := maxSectorWeightFromConstraints(constraints); cap != nil {
		out.MaxSectorWeight = cap
	}
	return out
}

func maxSectorWeightFromConstraints(constraints *portfoliolayer.ConstraintSet) *float64 {
	if constraints == nil {
		return nil
	}
	w := constraints.Resolve().MaxSectorWeight
	if w <= 0 || math.IsNaN(w) || math.IsInf(w, 0) {
		return nil
	}
	v := w
	return &v
}
