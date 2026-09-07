package outcome

import (
	"strings"
	"time"

	"go-stock/backend/papertrading"
)

// ProjectList builds outcome rows for all stocks with fills and optional pool-only NO_TRADE rows.
func (s *Service) ProjectList(opts ProjectOptions) ([]OutcomeProjection, error) {
	if s == nil {
		s = NewService()
	}
	if opts.AsOf.IsZero() {
		opts.AsOf = time.Now()
	}

	acc, err := papertrading.GetDefaultAccount()
	if err != nil {
		return nil, err
	}

	fillCodes := []string{}
	if acc != nil {
		fillCodes, err = loadDistinctFillStockCodes(acc.ID)
		if err != nil {
			return nil, err
		}
	}

	poolCodes, err := loadPoolStockCodes(normalizeTradeDate(opts.TradeDate))
	if err != nil {
		return nil, err
	}
	codes := mergeStockCodes(fillCodes, poolCodes)

	hasFill := map[string]bool{}
	for _, code := range fillCodes {
		hasFill[code] = true
	}

	includeNoTrade := opts.IncludeNoTrade ||
		strings.EqualFold(strings.TrimSpace(opts.Status), OutcomeStatusNoTrade)

	out := make([]OutcomeProjection, 0)
	for _, code := range codes {
		if hasFill[code] {
			rows, err := s.projectStockInternal(code, opts, false)
			if err != nil {
				return nil, err
			}
			out = append(out, rows...)
			continue
		}
		if !includeNoTrade {
			continue
		}
		row, err := buildNoTradeOutcome(code, normalizeTradeDate(opts.TradeDate), opts.AsOf)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}

	return finalizeItems(out, opts), nil
}
