package portfoliohistory

import (
	"sort"
	"strings"
	"sync"
)

// Store persists DailyRecord rows independently of trade_plans.
type Store interface {
	// Upsert replaces the row for the same trade_date (one row per day).
	Upsert(DailyRecord) error
	List() []DailyRecord
	Get(tradeDate string) (DailyRecord, bool)
}

// MemoryStore is the in-process implementation (tests / process-local history).
type MemoryStore struct {
	mu    sync.RWMutex
	byDay map[string]DailyRecord
}

// NewMemoryStore constructs an empty store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{byDay: make(map[string]DailyRecord)}
}

// Upsert stores or replaces the day row.
func (s *MemoryStore) Upsert(rec DailyRecord) error {
	if s == nil {
		return nil
	}
	td := strings.TrimSpace(rec.TradeDate)
	if td == "" {
		return errTradeDateRequired
	}
	rec.TradeDate = td
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byDay[td] = rec
	return nil
}

// List returns all days sorted by trade_date ascending.
func (s *MemoryStore) List() []DailyRecord {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DailyRecord, 0, len(s.byDay))
	for _, td := range sortedKeys(s.byDay) {
		out = append(out, s.byDay[td])
	}
	return out
}

// Get returns one day if present.
func (s *MemoryStore) Get(tradeDate string) (DailyRecord, bool) {
	if s == nil {
		return DailyRecord{}, false
	}
	td := strings.TrimSpace(tradeDate)
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.byDay[td]
	return rec, ok
}

type tradeDateError string

func (e tradeDateError) Error() string { return string(e) }

const errTradeDateRequired = tradeDateError("portfoliohistory: trade_date required")

func sortedKeys(m map[string]DailyRecord) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// DiscardStore ignores upserts (Enabled=false sink).
type DiscardStore struct{}

func (DiscardStore) Upsert(DailyRecord) error           { return nil }
func (DiscardStore) List() []DailyRecord                { return nil }
func (DiscardStore) Get(string) (DailyRecord, bool)     { return DailyRecord{}, false }

// RecordDay summarizes and upserts when Enabled. When Enabled=false, returns nil without writing.
func RecordDay(store Store, in DayInput) (*DailyRecord, error) {
	if !in.Enabled {
		return nil, nil
	}
	if store == nil {
		store = DiscardStore{}
	}
	rec := SummarizeDay(in)
	if err := store.Upsert(rec); err != nil {
		return nil, err
	}
	return &rec, nil
}
