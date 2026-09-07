package outcome

import (
	"strings"
	"time"

	"go-stock/backend/papertrading"
)

// Service is the read-only OutcomeProjection entry point.
type Service struct{}

// NewService returns the default outcome reader.
func NewService() *Service { return &Service{} }

// ProjectStock builds all outcome rows for one stock (OPEN/CLOSED legs; optional NO_TRADE).
func (s *Service) ProjectStock(stockCode string, opts ProjectOptions) ([]OutcomeProjection, error) {
	if s == nil {
		s = NewService()
	}
	code := normalizeCode(stockCode)
	if code == "" {
		return nil, ErrInvalidStockCode
	}
	if opts.AsOf.IsZero() {
		opts.AsOf = time.Now()
	}
	includeNoTrade := opts.IncludeNoTrade ||
		strings.EqualFold(strings.TrimSpace(opts.Status), OutcomeStatusNoTrade)
	rows, err := s.projectStockInternal(code, opts, includeNoTrade)
	if err != nil {
		return nil, err
	}
	if isStockNotFound(rows, includeNoTrade) {
		return nil, ErrNotFound
	}
	return finalizeItems(rows, opts), nil
}

func (s *Service) projectStockInternal(stockCode string, opts ProjectOptions, includeNoTrade bool) ([]OutcomeProjection, error) {
	code := normalizeCode(stockCode)
	if opts.AsOf.IsZero() {
		opts.AsOf = time.Now()
	}

	acc, err := papertrading.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	if acc == nil {
		if includeNoTrade {
			row, err := buildNoTradeOutcome(code, normalizeTradeDate(opts.TradeDate), opts.AsOf)
			if err != nil {
				return nil, err
			}
			return []OutcomeProjection{row}, nil
		}
		return []OutcomeProjection{}, nil
	}

	fills, err := loadFillsForStock(acc.ID, code)
	if err != nil {
		return nil, err
	}
	if len(fills) == 0 {
		if includeNoTrade {
			row, err := buildNoTradeOutcome(code, normalizeTradeDate(opts.TradeDate), opts.AsOf)
			if err != nil {
				return nil, err
			}
			return []OutcomeProjection{row}, nil
		}
		return []OutcomeProjection{}, nil
	}

	fctx, err := loadFillContext(fills)
	if err != nil {
		return nil, err
	}
	legs := matchFIFOLegs(fills)
	out := make([]OutcomeProjection, 0, len(legs))
	for _, leg := range legs {
		out = append(out, buildOutcomeFromLeg(leg, fctx, opts.AsOf))
	}
	return out, nil
}

// ProjectOne returns outcomes for one stock; if multiple FIFO legs exist, all are returned.
func ProjectOne(stockCode string, opts ProjectOptions) ([]OutcomeProjection, error) {
	return NewService().ProjectStock(stockCode, opts)
}

func normalizeTradeDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw != "" {
		return raw
	}
	return time.Now().Format("2006-01-02")
}
