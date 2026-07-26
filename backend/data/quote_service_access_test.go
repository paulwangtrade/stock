package data

import (
	"testing"

	"go-stock/backend/marketdata"
)

type stubQuoteAccessService struct {
	n int
}

func (s *stubQuoteAccessService) GetQuote(code string) (*marketdata.Quote, error) {
	s.n++
	return &marketdata.Quote{Code: code, Price: 1}, nil
}

func (s *stubQuoteAccessService) GetQuotes(codes []string) ([]marketdata.Quote, error) {
	s.n++
	out := make([]marketdata.Quote, 0, len(codes))
	for _, c := range codes {
		out = append(out, marketdata.Quote{Code: c, Price: 1})
	}
	return out, nil
}

func TestQuoteServiceAccess_LazyFactoryNoDB(t *testing.T) {
	prevSvc := quoteService
	prevFactory := quoteServiceFactory
	t.Cleanup(func() {
		quoteServiceMu.Lock()
		quoteService = prevSvc
		quoteServiceFactory = prevFactory
		quoteServiceMu.Unlock()
	})

	quoteServiceMu.Lock()
	quoteService = nil
	created := 0
	quoteServiceFactory = func() marketdata.QuoteService {
		created++
		return &stubQuoteAccessService{}
	}
	quoteServiceMu.Unlock()

	a := GetQuoteService()
	b := GetQuoteService()
	if a == nil || b == nil {
		t.Fatal("GetQuoteService returned nil")
	}
	if created != 1 {
		t.Fatalf("factory should run once, created=%d", created)
	}
	if a != b {
		t.Fatal("lazy factory should cache the same instance")
	}
}

func TestQuoteServiceAccess_SetQuoteServiceOverrides(t *testing.T) {
	prevSvc := quoteService
	prevFactory := quoteServiceFactory
	t.Cleanup(func() {
		quoteServiceMu.Lock()
		quoteService = prevSvc
		quoteServiceFactory = prevFactory
		quoteServiceMu.Unlock()
	})

	stub := &stubQuoteAccessService{}
	SetQuoteService(stub)
	got := GetQuoteService()
	if got != stub {
		t.Fatal("SetQuoteService did not stick")
	}
	if _, err := got.GetQuote("sz000001"); err != nil {
		t.Fatalf("GetQuote: %v", err)
	}
	if stub.n != 1 {
		t.Fatalf("stub not called, n=%d", stub.n)
	}
}
