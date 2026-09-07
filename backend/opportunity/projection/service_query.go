package projection

import "time"

// Service is the read-only OpportunityProjection entry point (Phase16-C1).
type Service struct{}

// NewService returns the default projection reader.
func NewService() *Service {
	return &Service{}
}

// QueryInput selects projection query parameters (HTTP/API layer maps query strings here).
type QueryInput struct {
	TradeDate  string
	StockCode  string
	Limit      int
	Status     string // filter by decision.decision_status
	StrategyID string // filter by signal.strategy_id or opportunity.strategy_name
}

// QueryResult is the list response envelope.
type QueryResult struct {
	Items       []OpportunityProjection `json:"items"`
	Total       int                     `json:"total"`
	GeneratedAt time.Time               `json:"generated_at"`
}

// Query runs ProjectOne or ProjectList and applies read filters.
func (s *Service) Query(in QueryInput) (*QueryResult, error) {
	if s == nil {
		s = NewService()
	}
	opts := ProjectOptions{
		TradeDate:       in.TradeDate,
		Limit:           in.Limit,
		IncludeResearch: stringsTrim(in.StockCode) != "",
	}
	generatedAt := time.Now()

	if code := stringsTrim(in.StockCode); code != "" {
		p, err := ProjectOne(code, opts)
		if err != nil {
			return nil, err
		}
		if isEmptyProjection(p) {
			return nil, ErrNotFound
		}
		items := filterProjections([]OpportunityProjection{*p}, in.Status, in.StrategyID)
		return &QueryResult{
			Items:       items,
			Total:       len(items),
			GeneratedAt: generatedAt,
		}, nil
	}

	raw, err := ProjectList(opts)
	if err != nil {
		return nil, err
	}
	items := filterProjections(raw, in.Status, in.StrategyID)
	return &QueryResult{
		Items:       items,
		Total:       len(items),
		GeneratedAt: generatedAt,
	}, nil
}
