package provenance

import "time"

// Service evaluates portfolio provenance for one stock code (read-only).
type Service struct{}

// NewService returns the default provenance reader.
func NewService() *Service {
	return &Service{}
}

// Evaluate builds provenance for stockCode. Returns ErrNotFound when no position exists.
func (s *Service) Evaluate(stockCode string) (*View, error) {
	if s == nil {
		s = NewService()
	}
	return Evaluate(stockCode, EvaluateOptions{AsOf: time.Now()})
}
